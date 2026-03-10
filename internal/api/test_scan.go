package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"v1fs-scanner/internal/scanner"
)

// startTestScan copies a built-in sample file (EICAR or clean) into a destination
// directory chosen by the user and starts a scan on that directory.
func (h *Handler) startTestScan(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Sample  string `json:"sample"`  // "eicar" or "clean"
		DestDir string `json:"destDir"` // where to drop the file
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	sample := strings.ToLower(strings.TrimSpace(body.Sample))
	var srcName string
	switch sample {
	case "eicar":
		srcName = "eicar.com"
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

	src := filepath.Join(h.testSamplesPath, srcName)
	dest := filepath.Join(destDir, srcName)

	data, err := os.ReadFile(src)
	if err != nil {
		http.Error(w, "failed to read sample file", http.StatusInternalServerError)
		return
	}
	if err := os.WriteFile(dest, data, 0644); err != nil {
		http.Error(w, "failed to write sample file", http.StatusBadRequest)
		return
	}

	// Reuse the normal scan logic for destDir.
	apiKey, region := h.cfg.Get()
	if apiKey == "" || region == "" {
		http.Error(w, "configure API key and region first", http.StatusBadRequest)
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
	}
	task := h.store.Create(destDir)
	go h.store.RunScan(task.ID, destDir, apiKey, region, opts)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"taskId": task.ID})
}

