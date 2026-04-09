package api

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"v1fs-scanner/internal/config"
	"v1fs-scanner/internal/scanner"
)

type Handler struct {
	cfg            *config.Config
	configPath     string
	store          *scanner.TaskStore
	testSamplesPath string
}

func NewHandler(cfg *config.Config, configPath string, store *scanner.TaskStore, testSamplesPath string) *Handler {
	return &Handler{cfg: cfg, configPath: configPath, store: store, testSamplesPath: testSamplesPath}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.URL.Path == "/" || r.URL.Path == "/index.html":
		h.serveStatic("index.html", w, r)
		return
	case r.URL.Path == "/app.js":
		h.serveStatic("app.js", w, r)
		return
	case r.URL.Path == "/style.css":
		h.serveStatic("style.css", w, r)
		return
	case r.URL.Path == "/api/config" && r.Method == http.MethodGet:
		h.getConfig(w, r)
		return
	case r.URL.Path == "/api/config" && r.Method == http.MethodPost:
		h.saveConfig(w, r)
		return
	case r.URL.Path == "/api/scanner/test" && r.Method == http.MethodPost:
		h.testScanner(w, r)
		return
	case r.URL.Path == "/api/config/scan-action" && r.Method == http.MethodPost:
		h.saveScanAction(w, r)
		return
	case r.URL.Path == "/api/test-samples" && r.Method == http.MethodGet:
		h.getTestSamples(w, r)
		return
	case r.URL.Path == "/api/test-scan" && r.Method == http.MethodPost:
		h.startTestScan(w, r)
		return
	case r.URL.Path == "/api/dirs" && r.Method == http.MethodGet:
		h.listDirs(w, r)
		return
	case r.URL.Path == "/api/scan/start" && r.Method == http.MethodPost:
		h.startScan(w, r)
		return
	case strings.HasPrefix(r.URL.Path, "/api/scan/status/") && r.Method == http.MethodGet:
		id := strings.TrimPrefix(r.URL.Path, "/api/scan/status/")
		h.scanStatus(w, r, id)
		return
	case r.URL.Path == "/api/scan/history" && r.Method == http.MethodGet:
		h.scanHistory(w, r)
		return
	case strings.HasPrefix(r.URL.Path, "/api/reports/") && r.Method == http.MethodGet:
		name := strings.TrimPrefix(r.URL.Path, "/api/reports/")
		h.downloadReport(w, r, name)
		return
	}
	http.NotFound(w, r)
}

func (h *Handler) serveStatic(name string, w http.ResponseWriter, r *http.Request) {
	// Embedded or relative to binary; fallback to current dir for dev
	dir := "web"
	if _, err := os.Stat(dir); err != nil {
		dir = "."
	}
	path := filepath.Join(dir, name)
	data, err := os.ReadFile(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	switch name {
	case "app.js":
		w.Header().Set("Content-Type", "application/javascript")
	case "style.css":
		w.Header().Set("Content-Type", "text/css")
	default:
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	}
	w.Write(data)
}

func (h *Handler) getConfig(w http.ResponseWriter, r *http.Request) {
	apiKey, region := h.cfg.Get()
	scannerType, localURL, _ := h.cfg.GetScanner()
	action, quarantinePath := h.cfg.GetScanAction()
	concurrency := h.cfg.GetScanConcurrency()
	maxScans := h.cfg.GetMaxConcurrentScans()
	hashEnabled := h.cfg.GetHashEnabled()
	predictiveML := h.cfg.GetPredictiveML()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"apiKeySet":          apiKey != "",
		"region":             region,
		"configured":         apiKey != "" && region != "",
		"scannerType":        scannerType,
		"localScannerUrl":    localURL,
		"actionOnMalware":    action,
		"quarantinePath":     quarantinePath,
		"scanConcurrency":    concurrency,
		"maxConcurrentScans": maxScans,
		"hashEnabled":        hashEnabled,
		"predictiveML":       predictiveML,
	})
}

func (h *Handler) saveConfig(w http.ResponseWriter, r *http.Request) {
	var body struct {
		APIKey             string `json:"apiKey"`
		Region             string `json:"region"`
		ScannerType        string `json:"scannerType"`
		LocalScannerURL    string `json:"localScannerUrl"`
		LocalScannerAPIKey string `json:"localScannerApiKey"`
		ActionOnMalware    string `json:"actionOnMalware"`
		QuarantinePath     string `json:"quarantinePath"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	body.Region = strings.TrimSpace(body.Region)
	body.APIKey = strings.TrimSpace(body.APIKey)
	body.ScannerType = strings.TrimSpace(strings.ToLower(body.ScannerType))
	if body.ScannerType == "" {
		body.ScannerType = "saas"
	}
	if body.ScannerType == "local" {
		if strings.TrimSpace(body.LocalScannerURL) == "" {
			http.Error(w, "localScannerUrl required for local scanner", http.StatusBadRequest)
			return
		}
	} else {
		if body.Region == "" || body.APIKey == "" {
			http.Error(w, "apiKey and region required for saas scanner", http.StatusBadRequest)
			return
		}
	}
	h.cfg.Set(body.APIKey, body.Region)
	h.cfg.SetScanner(body.ScannerType, strings.TrimSpace(body.LocalScannerURL), strings.TrimSpace(body.LocalScannerAPIKey))
	action := strings.TrimSpace(body.ActionOnMalware)
	if action != "log" && action != "quarantine" && action != "delete" {
		action = "log"
	}
	h.cfg.SetScanAction(action, strings.TrimSpace(body.QuarantinePath))
	if err := h.cfg.Save(h.configPath); err != nil {
		http.Error(w, "failed to save config", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
}

func (h *Handler) testScanner(w http.ResponseWriter, r *http.Request) {
	scannerType, localURL, _ := h.cfg.GetScanner()
	type result struct {
		OK      bool   `json:"ok"`
		Message string `json:"message"`
	}
	if scannerType == "local" {
		if localURL == "" {
			http.Error(w, "local scanner URL is not configured", http.StatusBadRequest)
			return
		}
		u := strings.TrimRight(localURL, "/")
		client := &http.Client{Timeout: 5 * time.Second}
		req, _ := http.NewRequest(http.MethodGet, u+"/health", nil)
		resp, err := client.Do(req)
		if err != nil {
			// Try root endpoint as fallback
			req2, _ := http.NewRequest(http.MethodGet, u, nil)
			resp2, err2 := client.Do(req2)
			if err2 != nil {
				http.Error(w, "local scanner is not reachable: "+err.Error(), http.StatusBadGateway)
				return
			}
			defer resp2.Body.Close()
			if resp2.StatusCode < 200 || resp2.StatusCode >= 500 {
				http.Error(w, "local scanner responded unexpectedly", http.StatusBadGateway)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(result{OK: true, Message: "local scanner responded successfully"})
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 500 {
			http.Error(w, "local scanner health check failed", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result{OK: true, Message: "local scanner is reachable"})
		return
	}
	// SaaS mode: verify configured credentials and region by opening TCP to region endpoint.
	apiKey, region := h.cfg.Get()
	if apiKey == "" || region == "" {
		http.Error(w, "api key and region are required for saas scanner", http.StatusBadRequest)
		return
	}
	host := region + ".xdr.trendmicro.com:443"
	conn, err := net.DialTimeout("tcp", host, 5*time.Second)
	if err != nil {
		http.Error(w, "saas endpoint is not reachable for region "+region, http.StatusBadGateway)
		return
	}
	conn.Close()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result{OK: true, Message: "saas scanner endpoint is reachable"})
}

func (h *Handler) saveScanAction(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ActionOnMalware    string `json:"actionOnMalware"`
		QuarantinePath     string `json:"quarantinePath"`
		ScanConcurrency    *int   `json:"scanConcurrency"`
		MaxConcurrentScans *int   `json:"maxConcurrentScans"`
		HashEnabled        *bool  `json:"hashEnabled"`
		PredictiveML       *bool  `json:"predictiveML"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	action := strings.TrimSpace(body.ActionOnMalware)
	if action != "log" && action != "quarantine" && action != "delete" {
		action = "log"
	}
	h.cfg.SetScanAction(action, strings.TrimSpace(body.QuarantinePath))
	if body.ScanConcurrency != nil {
		n := *body.ScanConcurrency
		if n < 0 {
			n = 0
		}
		if n > 64 {
			n = 64
		}
		h.cfg.SetScanConcurrency(n)
	}
	if body.MaxConcurrentScans != nil {
		n := *body.MaxConcurrentScans
		if n < 0 {
			n = 0
		}
		if n > 1000 {
			n = 1000
		}
		h.cfg.SetMaxConcurrentScans(n)
	}
	if body.HashEnabled != nil {
		h.cfg.SetHashEnabled(*body.HashEnabled)
	}
	if body.PredictiveML != nil {
		h.cfg.SetPredictiveML(*body.PredictiveML)
	}
	if err := h.cfg.Save(h.configPath); err != nil {
		http.Error(w, "failed to save config", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
}

func (h *Handler) getTestSamples(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"path": h.testSamplesPath,
		"eicarFile": "eicar.com",
		"cleanFile":  "hello.txt",
	})
}

func (h *Handler) listDirs(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	entries, err := listRootsOrDirs(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

func (h *Handler) startScan(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path       string `json:"path"`
		ReportName string `json:"reportName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	path := strings.TrimSpace(body.Path)
	if path == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}
	path = filepath.Clean(path)
	if path == "." {
		path = "/"
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		http.Error(w, "path is not a valid directory", http.StatusBadRequest)
		return
	}
	apiKey, region := h.cfg.Get()
	scannerType, localURL, localAPIKey := h.cfg.GetScanner()
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
	// Enforce maximum number of concurrent scans if configured (>0).
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
		ScannerType:     scannerType,
		LocalScannerURL: localURL,
		LocalAPIKey:     localAPIKey,
	}
	task := h.store.Create(path)
	if rn := strings.TrimSpace(body.ReportName); rn != "" {
		task.SetReportName(rn)
	}
	go h.store.RunScan(task.ID, path, apiKey, region, opts)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"taskId": task.ID})
}

func (h *Handler) scanStatus(w http.ResponseWriter, r *http.Request, id string) {
	task := h.store.Get(id)
	if task == nil {
		http.NotFound(w, r)
		return
	}
	snap := task.Snapshot()
	// Return path relative to reports dir for download link
	reportRel := ""
	if snap.ReportPath != "" {
		reportRel = filepath.Base(snap.ReportPath)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":           snap.ID,
		"path":         snap.Path,
		"startedAt":    snap.StartedAt,
		"finishedAt":   snap.FinishedAt,
		"totalFiles":   snap.TotalFiles,
		"scannedCount": snap.ScannedCount,
		"reportName":   snap.ReportName,
		"currentFile":  snap.CurrentFile,
		"malicious":    snap.Malicious,
		"error":        snap.Error,
		"reportPath":   reportRel,
	})
}

func (h *Handler) scanHistory(w http.ResponseWriter, r *http.Request) {
	list := h.store.List()
	type item struct {
		ID           string  `json:"id"`
		Path         string  `json:"path"`
		ReportName   string  `json:"reportName,omitempty"`
		StartedAt    string  `json:"startedAt"`
		FinishedAt   *string `json:"finishedAt,omitempty"`
		TotalFiles   int     `json:"totalFiles"`
		ScannedCount int     `json:"scannedCount"`
		MaliciousCount int        `json:"maliciousCount"`
		Error        string       `json:"error,omitempty"`
		ReportPath   string       `json:"reportPath,omitempty"`
	}
	out := make([]item, 0, len(list))
	for _, t := range list {
		fin := ""
		if t.FinishedAt != nil {
			fin = t.FinishedAt.Format("2006-01-02T15:04:05Z07:00")
		}
		rep := ""
		if t.ReportPath != "" {
			rep = filepath.Base(t.ReportPath)
		}
		out = append(out, item{
			ID:             t.ID,
			Path:           t.Path,
			ReportName:     t.ReportName,
			StartedAt:      t.StartedAt.Format("2006-01-02T15:04:05Z07:00"),
			FinishedAt:     ptrOrNil(fin),
			TotalFiles:     t.TotalFiles,
			ScannedCount:   t.ScannedCount,
			MaliciousCount: len(t.Malicious),
			Error:          t.Error,
			ReportPath:     rep,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func ptrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (h *Handler) downloadReport(w http.ResponseWriter, r *http.Request, name string) {
	if name == "" || strings.Contains(name, "..") || filepath.Clean(name) != name {
		http.Error(w, "invalid report name", http.StatusBadRequest)
		return
	}
	path := filepath.Join(h.store.ReportsDir(), name)
	f, err := os.Open(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Disposition", "attachment; filename="+name)
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Length", fmtSize(info.Size()))
	io.Copy(w, f)
}

func fmtSize(n int64) string {
	return strconv.FormatInt(n, 10)
}
