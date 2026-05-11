package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"v1fs-scanner/internal/scanner"
)

func ensureTestSamples(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	// Malware test bytes are assembled on demand via eicarProbe() — not stored statically.
	if err := os.WriteFile(filepath.Join(dir, "malware-test.com"), eicarProbe(), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("Hello World\n"), 0644); err != nil {
		return err
	}
	return nil
}

// startTestScan copies a built-in sample file (EICAR or clean) into a destination
// directory chosen by the user and starts a scan on that directory.
func (h *Handler) startTestScan(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Sample     string `json:"sample"`     // "eicar" or "clean"
		DestDir    string `json:"destDir"`    // where to drop the file
		ReportName string `json:"reportName"` // optional friendly name
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	sample := strings.ToLower(strings.TrimSpace(body.Sample))
	var srcName string
	switch sample {
	case "eicar":
		srcName = "malware-test.com"
	case "clean":
		srcName = "hello.txt"
	default:
		http.Error(w, "sample must be 'eicar' or 'clean'", http.StatusBadRequest)
		return
	}

	destDir := strings.TrimSpace(body.DestDir)
	if destDir == "" {
		http.Error(w, "destDir required", http.StatusBadRequest)
		return
	}
	destDir = filepath.Clean(destDir)

	if err := os.MkdirAll(destDir, 0755); err != nil {
		http.Error(w, "failed to create destination directory", http.StatusBadRequest)
		return
	}

	// Scan walks the tree recursively. If the user picks a broad folder (e.g. /data),
	// we would otherwise pick up /data/test-samples/eicar.com during a "clean" test.
	// Use a fresh leaf directory that only contains the one sample file for this run.
	testRoot := filepath.Join(destDir, "v1fs-test-"+strconv.FormatInt(time.Now().UnixNano(), 10))
	if err := os.MkdirAll(testRoot, 0755); err != nil {
		http.Error(w, "failed to create test folder", http.StatusBadRequest)
		return
	}

	dest := filepath.Join(testRoot, srcName)

	// For EICAR: assemble bytes at submission time — never read from disk.
	// On Windows eicarProbe() returns nil; the test is unavailable there.
	// For clean: read hello.txt from the samples directory (benign file).
	var data []byte
	if sample == "eicar" {
		data = eicarProbe()
		if len(data) == 0 {
			http.Error(w, "malware test is not available on this platform", http.StatusNotImplemented)
			return
		}
	} else {
		src := filepath.Join(h.testSamplesPath, srcName)
		var err error
		data, err = os.ReadFile(src)
		if err != nil {
			if recErr := ensureTestSamples(h.testSamplesPath); recErr != nil {
				http.Error(w, "failed to read sample file", http.StatusInternalServerError)
				return
			}
			data, err = os.ReadFile(src)
			if err != nil {
				http.Error(w, "failed to read sample file", http.StatusInternalServerError)
				return
			}
		}
	}

	if err := os.WriteFile(dest, data, 0644); err != nil {
		http.Error(w, "failed to write sample file", http.StatusBadRequest)
		return
	}

	// Reuse the normal scan logic for destDir.
	apiKey, region := h.cfg.Get()
	scannerType, localURL, localAPIKey, localProtocol, localTLS := h.cfg.GetScanner()
	if scannerType == "saas" && (apiKey == "" || region == "") {
		http.Error(w, "configure API key and region first", http.StatusBadRequest)
		return
	}
	if scannerType == "local" && localURL == "" {
		http.Error(w, "configure local scanner URL first", http.StatusBadRequest)
		return
	}
	action, quarantinePath := h.cfg.GetScanAction()
	if action == "quarantine" && quarantinePath == "" {
		http.Error(w, "quarantine path required when action is 'Move to quarantine'; set it in Settings", http.StatusBadRequest)
		return
	}
	if max := h.cfg.GetMaxConcurrentScans(); max > 0 {
		if h.store.RunningCount() >= max {
			http.Error(w, "maximum number of simultaneous scans reached; wait for a scan to finish or increase the limit in Settings", http.StatusTooManyRequests)
			return
		}
	}
	concurrency := h.cfg.GetScanConcurrency()
	if concurrency <= 0 {
		if s := os.Getenv("V1FS_SCAN_CONCURRENCY"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 {
				concurrency = n
			}
		}
	}
	opts := scanner.ScanOptions{
		ActionOnMalware: action,
		QuarantinePath:  quarantinePath,
		Concurrency:     concurrency,
		GenerateHashes:  h.cfg.GetHashEnabled(),
		PredictiveML:    h.cfg.GetPredictiveML(),
		ReportMode:      h.cfg.GetReportMode(),
		ScannerType:     scannerType,
		LocalScannerURL: normalizeLocalScannerURL(localURL),
		LocalScannerProtocol: localProtocol,
		LocalScannerTLS: localTLS,
		LocalAPIKey:     localAPIKey,
		ExtraScanTags:   h.cfg.GetScanTags(),
	}
	if localProtocol == "grpc" {
		opts.LocalScannerURL = normalizeLocalScannerGRPCAddr(localURL)
	}
	task := h.store.Create([]string{testRoot})
	if task == nil {
		http.Error(w, "failed to create scan task", http.StatusInternalServerError)
		return
	}
	if rn := strings.TrimSpace(body.ReportName); rn != "" {
		task.SetReportName(rn)
	}
	go h.store.RunScan(task.ID, []string{testRoot}, apiKey, region, opts)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"taskId":   task.ID,
		"scanPath": testRoot,
	})
}

