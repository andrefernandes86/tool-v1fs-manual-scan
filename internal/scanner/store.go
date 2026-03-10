package scanner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	v1client "github.com/trendmicro/tm-v1-fs-golang-sdk"
	"github.com/jung-kurt/gofpdf"
)

type TaskStore struct {
	mu        sync.RWMutex
	tasks     map[string]*Task
	reportsDir string
}

func NewTaskStore(reportsDir string) *TaskStore {
	return &TaskStore{
		tasks:      make(map[string]*Task),
		reportsDir: reportsDir,
	}
}

func (s *TaskStore) ReportsDir() string {
	return s.reportsDir
}

type Task struct {
	ID           string       `json:"id"`
	Path         string       `json:"path"`
	StartedAt    time.Time    `json:"startedAt"`
	FinishedAt   *time.Time   `json:"finishedAt,omitempty"`
	TotalFiles   int          `json:"totalFiles"`
	ScannedCount int          `json:"scannedCount"`
	CurrentFile  string       `json:"currentFile"`
	Malicious    []Malicious  `json:"malicious"`
	Error        string       `json:"error,omitempty"`
	ReportPath   string       `json:"reportPath,omitempty"`
	mu           sync.RWMutex
	done         chan struct{}
}

type Malicious struct {
	FileName    string `json:"fileName"`
	FilePath    string `json:"filePath"`
	MalwareName string `json:"malwareName"`
}

type scanResponse struct {
	ScanResult    int    `json:"scanResult"`
	FileName      string `json:"fileName"`
	FilePath      string `json:"filePath"`
	FoundMalwares []struct {
		FileName    string `json:"fileName"`
		MalwareName string `json:"malwareName"`
	} `json:"foundMalwares"`
}

// verboseScanResponse matches SDK verbose JSON format (result.atse.malwareCount, result.atse.malware)
type verboseScanResponse struct {
	FileName string `json:"fileName"`
	Result   struct {
		Atse struct {
			MalwareCount int `json:"malwareCount"`
			Malware      []struct {
				Name     string `json:"name"`
				FileName string `json:"fileName"`
			} `json:"malware"`
		} `json:"atse"`
	} `json:"result"`
}

// ScanOptions configures what to do when malware is detected
type ScanOptions struct {
	ActionOnMalware string // "log", "quarantine", "delete"
	QuarantinePath  string
}

func (s *TaskStore) Create(path string) *Task {
	id := time.Now().Format("20060102-150405") + "-" + filepath.Base(path)
	if id == "" || id == "." {
		id = time.Now().Format("20060102-150405")
	}
	t := &Task{
		ID:        id,
		Path:      path,
		StartedAt: time.Now(),
		Malicious: nil,
		done:      make(chan struct{}),
	}
	s.mu.Lock()
	s.tasks[id] = t
	s.mu.Unlock()
	return t
}

func (s *TaskStore) Get(id string) *Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tasks[id]
}

func (s *TaskStore) List() []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]*Task, 0, len(s.tasks))
	for _, t := range s.tasks {
		t.mu.RLock()
		copy := *t
		copy.Malicious = append([]Malicious(nil), t.Malicious...)
		t.mu.RUnlock()
		list = append(list, &copy)
	}
	return list
}

func (t *Task) UpdateProgress(current string, scanned int, total int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.CurrentFile = current
	t.ScannedCount = scanned
	t.TotalFiles = total
}

func (t *Task) AddMalicious(fileName, filePath, malwareName string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Malicious = append(t.Malicious, Malicious{
		FileName:    fileName,
		FilePath:    filePath,
		MalwareName: malwareName,
	})
}

func (t *Task) Finish(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	t.FinishedAt = &now
	if err != nil {
		t.Error = err.Error()
	}
	select {
	case <-t.done:
	default:
		close(t.done)
	}
}

func (t *Task) Snapshot() Task {
	t.mu.RLock()
	defer t.mu.RUnlock()
	snap := *t
	snap.Malicious = append([]Malicious(nil), t.Malicious...)
	return snap
}

func (s *TaskStore) RunScan(taskID string, rootPath string, apiKey, region string, opts ScanOptions) {
	t := s.Get(taskID)
	if t == nil {
		return
	}

	client, err := v1client.NewClient(apiKey, region)
	if err != nil {
		t.Finish(err)
		return
	}
	defer client.Destroy()

	var files []string
	filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.Mode().IsRegular() {
			files = append(files, path)
		}
		return nil
	})

	total := len(files)
	t.UpdateProgress("", 0, total)

	if opts.ActionOnMalware == "" {
		opts.ActionOnMalware = "log"
	}

	tags := []string{"v1fs-scanner"}
	for i, path := range files {
		t.UpdateProgress(path, i, total)
		resp, err := client.ScanFile(path, tags)
		if err != nil {
			continue
		}

		fileName, filePath, malwareName := parseScanResponse(resp, path)
		if fileName == "" && filePath == "" {
			continue
		}
		if malwareName != "" {
			t.AddMalicious(fileName, filePath, malwareName)
			performAction(path, opts)
		}
	}

	t.UpdateProgress("", total, total)
	reportPath, err := s.writePDF(taskID, t)
	if err != nil {
		t.Finish(err)
		return
	}
	t.mu.Lock()
	t.ReportPath = reportPath
	now := time.Now()
	t.FinishedAt = &now
	t.mu.Unlock()
	t.Finish(nil)
}

// parseScanResponse returns (fileName, filePath, malwareName). If malware detected, malwareName is non-empty.
// Supports both concise and verbose SDK response formats.
func parseScanResponse(resp string, actualFilePath string) (fileName, filePath, malwareName string) {
	var sr scanResponse
	if json.Unmarshal([]byte(resp), &sr) == nil && sr.ScanResult != 0 {
		filePath = sr.FilePath
		if filePath == "" {
			filePath = actualFilePath
		}
		fileName = sr.FileName
		if fileName == "" {
			fileName = filepath.Base(actualFilePath)
		}
		if len(sr.FoundMalwares) > 0 {
			malwareName = sr.FoundMalwares[0].MalwareName
			for j := 1; j < len(sr.FoundMalwares); j++ {
				malwareName += ", " + sr.FoundMalwares[j].MalwareName
			}
		} else {
			malwareName = "Detected"
		}
		return
	}

	// Try verbose format (SDK may return this when concise has no malware or different structure)
	var vsr verboseScanResponse
	if json.Unmarshal([]byte(resp), &vsr) != nil {
		return "", "", ""
	}
	if vsr.Result.Atse.MalwareCount <= 0 {
		return "", "", ""
	}
	fileName = vsr.FileName
	if fileName == "" {
		fileName = filepath.Base(actualFilePath)
	}
	filePath = actualFilePath
	if len(vsr.Result.Atse.Malware) > 0 {
		malwareName = vsr.Result.Atse.Malware[0].Name
		for j := 1; j < len(vsr.Result.Atse.Malware); j++ {
			malwareName += ", " + vsr.Result.Atse.Malware[j].Name
		}
	} else {
		malwareName = "Detected"
	}
	return
}

func performAction(filePath string, opts ScanOptions) {
	switch opts.ActionOnMalware {
	case "quarantine":
		if opts.QuarantinePath == "" {
			return
		}
		if err := os.MkdirAll(opts.QuarantinePath, 0755); err != nil {
			return
		}
		dest := filepath.Join(opts.QuarantinePath, filepath.Base(filePath))
		for n := 0; ; n++ {
			if _, err := os.Stat(dest); os.IsNotExist(err) {
				break
			}
			ext := filepath.Ext(filePath)
			base := filepath.Base(filePath)
			if ext != "" {
				base = base[:len(base)-len(ext)]
			}
			dest = filepath.Join(opts.QuarantinePath, base+"_"+strconv.Itoa(n)+ext)
		}
		if err := os.Rename(filePath, dest); err != nil {
			// Cross-filesystem: copy then remove
			data, err := os.ReadFile(filePath)
			if err != nil {
				return
			}
			if err := os.WriteFile(dest, data, 0644); err != nil {
				return
			}
			os.Remove(filePath)
		}
	case "delete":
		os.Remove(filePath)
	default:
		// log only, nothing to do
	}
}

func (s *TaskStore) writePDF(taskID string, t *Task) (string, error) {
	t.mu.RLock()
	snap := *t
	snap.Malicious = append([]Malicious(nil), t.Malicious...)
	t.mu.RUnlock()

	name := taskID + ".pdf"
	path := filepath.Join(s.reportsDir, name)
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(0, 10, "V1 File Security Scan Report", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	pdf.CellFormat(0, 6, "Scan path: "+snap.Path, "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, "Started: "+snap.StartedAt.Format(time.RFC3339), "", 1, "L", false, 0, "")
	if snap.FinishedAt != nil {
		pdf.CellFormat(0, 6, "Finished: "+snap.FinishedAt.Format(time.RFC3339), "", 1, "L", false, 0, "")
	}
	pdf.CellFormat(0, 6, "Files scanned: "+strconv.Itoa(snap.ScannedCount), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, "Malicious found: "+strconv.Itoa(len(snap.Malicious)), "", 1, "L", false, 0, "")
	pdf.Ln(6)

	if len(snap.Malicious) > 0 {
		pdf.SetFont("Helvetica", "B", 12)
		pdf.CellFormat(0, 8, "Malicious files", "", 1, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 9)
		for _, m := range snap.Malicious {
			pdf.CellFormat(0, 5, "File: "+m.FileName, "", 1, "L", false, 0, "")
			pdf.CellFormat(0, 5, "  Path: "+m.FilePath, "", 1, "L", false, 0, "")
			pdf.CellFormat(0, 5, "  Malware: "+m.MalwareName, "", 1, "L", false, 0, "")
			pdf.Ln(2)
		}
	}

	return path, pdf.OutputFileAndClose(path)
}
