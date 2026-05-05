package scanner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	v1client "github.com/trendmicro/tm-v1-fs-golang-sdk"
	"github.com/jung-kurt/gofpdf"
)

// DefaultConcurrency is the number of files scanned in parallel when using concurrent scan.
const DefaultConcurrency = 8

type TaskStore struct {
	mu         sync.RWMutex
	tasks      map[string]*Task
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
	ReportName   string       `json:"reportName,omitempty"`
	ReportMode   string       `json:"reportMode,omitempty"`
	StartedAt    time.Time    `json:"startedAt"`
	FinishedAt   *time.Time   `json:"finishedAt,omitempty"`
	TotalFiles   int          `json:"totalFiles"`
	ScannedCount int          `json:"scannedCount"`
	CurrentFile  string       `json:"currentFile"`
	Malicious    []Malicious  `json:"malicious"`
	CleanFiles   []string     `json:"cleanFiles,omitempty"`
	ScanErrors   int          `json:"scanErrors"`
	LastScanError string      `json:"lastScanError,omitempty"`
	Error        string       `json:"error,omitempty"`
	ReportPath   string       `json:"reportPath,omitempty"`
	ScanTags     []string     `json:"scanTags,omitempty"`
	mu           sync.RWMutex
	done         chan struct{}
}

type Malicious struct {
	FileName    string `json:"fileName"`
	FilePath    string `json:"filePath"`
	MalwareName string `json:"malwareName"`
	FileHash    string `json:"fileHash,omitempty"`
}

type scanResponse struct {
	ScanResult    any    `json:"scanResult"`
	FileName      string `json:"fileName"`
	FilePath      string `json:"filePath"`
	MalwareName   string `json:"malwareName"`
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

func nonZeroScanResult(v any) bool {
	switch x := v.(type) {
	case float64:
		return x != 0
	case int:
		return x != 0
	case string:
		x = strings.TrimSpace(x)
		return x != "" && x != "0"
	default:
		return false
	}
}

// ScanOptions configures what to do when malware is detected and how to run the scan.
type ScanOptions struct {
	ActionOnMalware string // "log", "quarantine", "delete"
	QuarantinePath  string
	Concurrency     int  // number of files scanned in parallel; 0 = use DefaultConcurrency
	GenerateHashes  bool // when true, compute SHA-256 for malicious files
	PredictiveML    bool // when true, enable predictive machine learning (PML) hints
	ReportMode      string // "stats" (default) or "all"
	ScannerType     string
	LocalScannerURL string
	LocalScannerProtocol string // ignored for local scans (always gRPC); retained for API shape
	LocalScannerTLS bool
	LocalAPIKey     string
	ExtraScanTags   []string // optional user tags (Vision One); v1fs-scanner is always prepended in RunScan
}

func (s *TaskStore) Create(rootPaths []string) *Task {
	if len(rootPaths) == 0 {
		return nil
	}
	display := strings.Join(rootPaths, "; ")
	id := time.Now().Format("20060102150405") + "-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	t := &Task{
		ID:         id,
		Path:       display,
		StartedAt:  time.Now(),
		Malicious:  nil,
		ReportMode: "stats",
		done:       make(chan struct{}),
	}
	s.mu.Lock()
	s.tasks[id] = t
	s.mu.Unlock()
	return t
}

// linuxVirtualSkipPath is true for Linux pseudo-filesystems (/proc, /sys, /dev, /run) that
// cause permission errors or confuse the scanner. Applies to both directories and files
// (e.g. symlinks or bind mounts that surface paths under /sys).
func linuxVirtualSkipPath(absPath string) bool {
	if runtime.GOOS != "linux" {
		return false
	}
	p := filepath.Clean(absPath)
	sep := string(filepath.Separator)
	// Only the real top-level virtual filesystems on Linux (/proc, /sys, /dev, /run).
	// Do not use strings.Contains(..., "/dev/") etc.: that skips every path under a normal
	// folder named "dev", "sys", or "run" (e.g. .../project/dev/src), which collapses full-root scans.
	for _, vp := range []string{"/proc", "/sys", "/dev", "/run"} {
		if p == vp || strings.HasPrefix(p, vp+sep) {
			return true
		}
	}
	return false
}

func collectScanFiles(rootPaths []string) []string {
	seen := make(map[string]struct{})
	var files []string
	for _, root := range rootPaths {
		root = filepath.Clean(root)
		filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if linuxVirtualSkipPath(path) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if !d.Type().IsRegular() {
				return nil
			}
			if _, ok := seen[path]; ok {
				return nil
			}
			seen[path] = struct{}{}
			files = append(files, path)
			return nil
		})
	}
	return files
}

func filterVirtualScanPaths(paths []string) []string {
	if runtime.GOOS != "linux" {
		return paths
	}
	var out []string
	for _, p := range paths {
		if !linuxVirtualSkipPath(p) {
			out = append(out, p)
		}
	}
	return out
}

// reportPDFBasename returns a single safe filename segment (no slashes) for the PDF on disk.
func reportPDFBasename(taskID string) string {
	var b strings.Builder
	for _, r := range taskID {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	s := strings.Trim(strings.TrimSpace(b.String()), "_")
	if s == "" {
		s = "scan"
	}
	return s + ".pdf"
}

func softenScanError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	lower := strings.ToLower(msg)
	if !strings.Contains(lower, "permission denied") && !strings.Contains(lower, "operation not permitted") {
		return err
	}
	if strings.Contains(msg, "/proc/") || strings.Contains(msg, "/sys/") || strings.Contains(msg, "/dev/") ||
		strings.Contains(msg, "/run/") || strings.Contains(lower, "uevent") {
		return errors.New("skipped a restricted system path (expected when scanning broad trees in a container)")
	}
	if strings.Contains(lower, "not ready") && strings.Contains(lower, "permission") {
		return errors.New("scanner hit unreadable system files; use a narrower folder (e.g. /data) instead of the full host root")
	}
	return err
}

func isBenignScanFailure(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.HasPrefix(s, "skipped a restricted system path") ||
		strings.HasPrefix(s, "scanner hit unreadable system files")
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
		copy.CleanFiles = append([]string(nil), t.CleanFiles...)
		t.mu.RUnlock()
		list = append(list, &copy)
	}
	return list
}

// RunningCount returns the number of tasks that have not finished yet.
func (s *TaskStore) RunningCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, t := range s.tasks {
		t.mu.RLock()
		finished := t.FinishedAt != nil
		t.mu.RUnlock()
		if !finished {
			n++
		}
	}
	return n
}

func (t *Task) UpdateProgress(current string, scanned int, total int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.CurrentFile = current
	t.ScannedCount = scanned
	t.TotalFiles = total
}

// IncrementScanned updates ScannedCount by one and sets CurrentFile to path (for concurrent scan).
func (t *Task) IncrementScanned(path string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ScannedCount++
	t.CurrentFile = path
}

func (t *Task) AddMalicious(fileName, filePath, malwareName, fileHash string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Malicious = append(t.Malicious, Malicious{
		FileName:    fileName,
		FilePath:    filePath,
		MalwareName: malwareName,
		FileHash:    fileHash,
	})
}

func (t *Task) AddClean(filePath string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.CleanFiles = append(t.CleanFiles, filePath)
}

func (t *Task) AddScanError(err error) {
	if err == nil {
		return
	}
	soft := softenScanError(err)
	t.mu.Lock()
	defer t.mu.Unlock()
	if isBenignScanFailure(soft) {
		return
	}
	t.ScanErrors++
	t.LastScanError = soft.Error()
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

func (t *Task) SetReportName(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ReportName = name
}

func (t *Task) SetReportMode(mode string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if mode == "all" {
		t.ReportMode = "all"
		return
	}
	t.ReportMode = "stats"
}

func (t *Task) SetScanTags(tags []string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ScanTags = append([]string(nil), tags...)
}

func (t *Task) Snapshot() Task {
	t.mu.RLock()
	defer t.mu.RUnlock()
	snap := *t
	snap.Malicious = append([]Malicious(nil), t.Malicious...)
	snap.CleanFiles = append([]string(nil), t.CleanFiles...)
	snap.ScanTags = append([]string(nil), t.ScanTags...)
	return snap
}

func (s *TaskStore) RunScan(taskID string, rootPaths []string, apiKey, region string, opts ScanOptions) {
	t := s.Get(taskID)
	if t == nil {
		return
	}
	if len(rootPaths) == 0 {
		t.Finish(errors.New("no scan roots"))
		return
	}
	scannerType := opts.ScannerType
	if scannerType == "" {
		scannerType = "saas"
	}
	localProtocol := strings.TrimSpace(strings.ToLower(opts.LocalScannerProtocol))
	if scannerType == "local" {
		localProtocol = "grpc"
	} else if localProtocol != "grpc" {
		localProtocol = "http"
	}

	scanWithSaaS := func(_ string, _ []string) (string, error) {
		return "", errors.New("saas scanner is not configured")
	}
	if scannerType == "saas" {
		client, err := v1client.NewClient(apiKey, region)
		if err != nil {
			t.Finish(err)
			return
		}
		defer client.Destroy()
		scanWithSaaS = func(path string, tags []string) (string, error) {
			return client.ScanFile(path, tags)
		}
	}
	scanWithLocalGRPC := func(_ string, _ []string) (string, error) {
		return "", errors.New("local gRPC scanner is not configured")
	}
	if scannerType == "local" {
		client, err := v1client.NewClientInternal(strings.TrimSpace(opts.LocalAPIKey), strings.TrimSpace(opts.LocalScannerURL), opts.LocalScannerTLS, "")
		if err != nil {
			t.Finish(err)
			return
		}
		// Local gRPC deployments can reject PML/feedback toggles depending on
		// gateway/scanner version. Keep local scans protocol-compatible.
		defer client.Destroy()
		scanWithLocalGRPC = func(path string, tags []string) (string, error) {
			resp, err := client.ScanFile(path, tags)
			if err == nil {
				return resp, nil
			}
			// Some local anti-malware gateway versions reject the ScanFile RPC path
			// with "Unimplemented/compatible" while still supporting ScanBuffer.
			msg := err.Error()
			if !strings.Contains(msg, "code = Unimplemented") && !strings.Contains(strings.ToLower(msg), "not compatible") {
				return "", err
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return "", err
			}
			return client.ScanBuffer(data, filepath.Base(path), tags)
		}
	}

	files := filterVirtualScanPaths(collectScanFiles(rootPaths))

	total := len(files)
	t.UpdateProgress("", 0, total)

	if opts.ActionOnMalware == "" {
		opts.ActionOnMalware = "log"
	}
	if opts.ReportMode != "all" {
		opts.ReportMode = "stats"
	}
	t.SetReportMode(opts.ReportMode)

	tags := []string{"v1fs-scanner"}
	userTags := ParseUserScanTags(opts.ExtraScanTags)
	for _, ut := range userTags {
		if ut == "v1fs-scanner" {
			continue
		}
		tags = append(tags, ut)
	}
	t.SetScanTags(userTags)
	if opts.PredictiveML && !(scannerType == "local" && localProtocol == "grpc") {
		// Align with vendor docs: enable PML and feedback flags.
		tags = append(tags, "pml:true", "feedback:true")
	}
	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = DefaultConcurrency
	}
	if concurrency > total {
		concurrency = total
	}
	if concurrency < 1 {
		concurrency = 1
	}

	jobs := make(chan string, total)
	var wg sync.WaitGroup
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				var fileName, filePath, malwareName string
				var err error
				if scannerType == "local" {
					var resp string
					resp, err = scanWithLocalGRPC(path, tags)
					if err == nil {
						fileName, filePath, malwareName = parseScanResponse(resp, path)
					}
				} else {
					var resp string
					resp, err = scanWithSaaS(path, tags)
					if err == nil {
						fileName, filePath, malwareName = parseScanResponse(resp, path)
					}
				}
				if err != nil {
					t.AddScanError(err)
					t.IncrementScanned(path)
					continue
				}
				if malwareName != "" {
					var hash string
					if opts.GenerateHashes {
						hash = computeSHA256(path)
					}
					t.AddMalicious(fileName, filePath, malwareName, hash)
					performAction(path, opts)
				} else {
					t.AddClean(path)
				}
				t.IncrementScanned(path)
			}
		}()
	}
	for _, path := range files {
		jobs <- path
	}
	close(jobs)
	wg.Wait()

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
	if json.Unmarshal([]byte(resp), &sr) == nil && (nonZeroScanResult(sr.ScanResult) || len(sr.FoundMalwares) > 0 || sr.MalwareName != "") {
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
		} else if sr.MalwareName != "" {
			malwareName = sr.MalwareName
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

const pdfBottomReserveMM = 18

func pdfPageBottom(pdf *gofpdf.Fpdf) float64 {
	_, h := pdf.GetPageSize()
	return h - pdfBottomReserveMM
}

// pdfRow reserves vertical space and starts a new page when needed so long reports
// (thousands of clean files) are not drawn past the end of the page.
func pdfRow(pdf *gofpdf.Fpdf, rowH float64, style string, size float64) {
	if pdf.GetY()+rowH > pdfPageBottom(pdf) {
		pdf.AddPage()
	}
	pdf.SetFont("Helvetica", style, size)
}

func pdfWriteRunesLine(pdf *gofpdf.Fpdf, rowH float64, maxRunes int, text string) {
	runes := []rune(text)
	if len(runes) == 0 {
		pdfRow(pdf, rowH, "", 9)
		pdf.CellFormat(0, rowH, " ", "", 1, "L", false, 0, "")
		return
	}
	for i := 0; i < len(runes); i += maxRunes {
		end := i + maxRunes
		if end > len(runes) {
			end = len(runes)
		}
		pdfRow(pdf, rowH, "", 9)
		pdf.CellFormat(0, rowH, string(runes[i:end]), "", 1, "L", false, 0, "")
	}
}

func (s *TaskStore) writePDF(taskID string, t *Task) (string, error) {
	t.mu.RLock()
	snap := *t
	snap.Malicious = append([]Malicious(nil), t.Malicious...)
	snap.CleanFiles = append([]string(nil), t.CleanFiles...)
	t.mu.RUnlock()

	name := reportPDFBasename(taskID)
	path := filepath.Join(s.reportsDir, name)
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 16)
	pdf.CellFormat(0, 10, "V1 File Security Scan Report", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 10)
	if snap.ReportName != "" {
		pdfRow(pdf, 6, "", 10)
		pdf.CellFormat(0, 6, "Report name: "+snap.ReportName, "", 1, "L", false, 0, "")
	}
	pdfRow(pdf, 6, "", 10)
	pdf.CellFormat(0, 6, "Report mode: "+snap.ReportMode, "", 1, "L", false, 0, "")
	pdfWriteRunesLine(pdf, 5, 95, "Scan path: "+snap.Path)
	pdfRow(pdf, 6, "", 10)
	pdf.CellFormat(0, 6, "Started: "+snap.StartedAt.Format(time.RFC3339), "", 1, "L", false, 0, "")
	if snap.FinishedAt != nil {
		pdfRow(pdf, 6, "", 10)
		pdf.CellFormat(0, 6, "Finished: "+snap.FinishedAt.Format(time.RFC3339), "", 1, "L", false, 0, "")
	}
	pdfRow(pdf, 6, "", 10)
	pdf.CellFormat(0, 6, "Files scanned: "+strconv.Itoa(snap.ScannedCount), "", 1, "L", false, 0, "")
	pdfRow(pdf, 6, "", 10)
	pdf.CellFormat(0, 6, "Malicious found: "+strconv.Itoa(len(snap.Malicious)), "", 1, "L", false, 0, "")
	if len(snap.ScanTags) > 0 {
		pdfRow(pdf, 6, "", 10)
		pdf.CellFormat(0, 6, "Scan tags: "+strings.Join(snap.ScanTags, ", "), "", 1, "L", false, 0, "")
	}
	pdf.Ln(4)

	if len(snap.Malicious) > 0 {
		pdfRow(pdf, 8, "B", 12)
		pdf.CellFormat(0, 8, "Malicious files", "", 1, "L", false, 0, "")
		for _, m := range snap.Malicious {
			pdfWriteRunesLine(pdf, 5, 100, "File: "+m.FileName)
			pdfWriteRunesLine(pdf, 5, 100, "  Path: "+m.FilePath)
			pdfWriteRunesLine(pdf, 5, 100, "  Malware: "+m.MalwareName)
			if m.FileHash != "" {
				pdfWriteRunesLine(pdf, 5, 100, "  SHA-256: "+m.FileHash)
			}
			pdfRow(pdf, 2, "", 9)
			pdf.Ln(2)
		}
	}

	if snap.ReportMode == "all" {
		pdfRow(pdf, 8, "B", 12)
		pdf.CellFormat(0, 8, "Clean files", "", 1, "L", false, 0, "")
		if len(snap.CleanFiles) == 0 {
			pdfRow(pdf, 5, "", 9)
			pdf.CellFormat(0, 5, "None", "", 1, "L", false, 0, "")
		} else {
			pdfRow(pdf, 5, "", 9)
			pdf.CellFormat(0, 5, "Total clean files listed: "+strconv.Itoa(len(snap.CleanFiles)), "", 1, "L", false, 0, "")
			for _, pth := range snap.CleanFiles {
				pdfWriteRunesLine(pdf, 4, 100, pth)
			}
		}
	}

	return path, pdf.OutputFileAndClose(path)
}

// computeSHA256 returns the hex-encoded SHA-256 hash of the file at path.
// On error it returns an empty string.
func computeSHA256(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
