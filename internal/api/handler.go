package api

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	v1client "github.com/trendmicro/tm-v1-fs-golang-sdk"
	"v1fs-scanner/internal/config"
	"v1fs-scanner/internal/scanner"
)

type Handler struct {
	cfg             *config.Config
	configPath      string
	store           *scanner.TaskStore
	testSamplesPath string
	webFS           fs.FS
}

func NewHandler(cfg *config.Config, configPath string, store *scanner.TaskStore, testSamplesPath string, webFS fs.FS) *Handler {
	return &Handler{cfg: cfg, configPath: configPath, store: store, testSamplesPath: testSamplesPath, webFS: webFS}
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
	case r.URL.Path == "/api/scanner/compat" && r.Method == http.MethodPost:
		h.compatScanner(w, r)
		return
	case r.URL.Path == "/api/scanner/status" && r.Method == http.MethodGet:
		h.scannerStatus(w, r)
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
	data, err := fs.ReadFile(h.webFS, name)
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

func scanTagsJSON(tags []string) []string {
	if len(tags) == 0 {
		return []string{}
	}
	return tags
}

func (h *Handler) getConfig(w http.ResponseWriter, r *http.Request) {
	apiKey, region := h.cfg.Get()
	scannerType, localURL, _, localProtocol, _ := h.cfg.GetScanner()
	action, quarantinePath := h.cfg.GetScanAction()
	concurrency := h.cfg.GetScanConcurrency()
	maxScans := h.cfg.GetMaxConcurrentScans()
	hashEnabled := h.cfg.GetHashEnabled()
	predictiveML := h.cfg.GetPredictiveML()
	reportMode := h.cfg.GetReportMode()
	w.Header().Set("Content-Type", "application/json")
	out := map[string]interface{}{
		"apiKeySet":          apiKey != "",
		"region":             region,
		"configured":         apiKey != "" && region != "",
		"scannerType":        scannerType,
		"localScannerUrl":    localURL,
		"localScannerProtocol": localProtocol,
		"actionOnMalware":    action,
		"quarantinePath":     quarantinePath,
		"scanConcurrency":    concurrency,
		"maxConcurrentScans": maxScans,
		"hashEnabled":        hashEnabled,
		"predictiveML":       predictiveML,
		"reportMode":         reportMode,
		"scanTags":           scanTagsJSON(h.cfg.GetScanTags()),
		"runningInContainer": runningInContainer(),
	}
	if runningInContainer() {
		out["containerScanRootHint"] = "The container can only see itself. To scan your USB drive, you must mount it first.\n\n==== STEP 1: STOP OLD CONTAINER ====\n\ndocker stop v1fs-scanner && docker rm v1fs-scanner\n\n\n==== STEP 2: PICK YOUR OS, FIND YOUR DRIVE, COPY COMMAND ====\n\n\n[macOS]\n\n1. Find your USB drive location:\n   Open Finder > Locations\n   Look at external drives listed\n   Example: /Volumes/MyDrive\n\n2. Copy and run this (CHANGE /Volumes/MyDrive to YOUR drive):\n\n   docker run -d -p 8080:8080 -v v1fs-data:/data -v /Volumes/MyDrive:/mnt/usb --name v1fs-scanner v1fs-scanner:latest\n\n3. Then go to Scanner and scan: /mnt/usb\n\n\n[Linux]\n\n1. Find your USB drive location:\n   Type in terminal: lsblk   OR   df -h\n   Look at the output\n   Example: /mnt/usb\n\n2. Copy and run this (CHANGE /mnt/usb to YOUR drive):\n\n   docker run -d -p 8080:8080 -v v1fs-data:/data -v /mnt/usb:/mnt/usb --name v1fs-scanner v1fs-scanner:latest\n\n3. Then go to Scanner and scan: /mnt/usb\n\n\n[Windows]\n\n1. Find your USB drive location:\n   Open File Explorer\n   Look at external drives\n   Example: D:/ or E:/\n   Enable file sharing in Docker Desktop Settings > Resources > File Sharing\n\n2. Copy and run this (CHANGE D:/ to YOUR drive):\n\n   docker run -d -p 8080:8080 -v v1fs-data:/data -v D:/:/mnt/usb --name v1fs-scanner v1fs-scanner:latest\n\n3. Then go to Scanner and scan: /mnt/usb"
	}
	json.NewEncoder(w).Encode(out)
}

func (h *Handler) saveConfig(w http.ResponseWriter, r *http.Request) {
	var body struct {
		APIKey             string `json:"apiKey"`
		Region             string `json:"region"`
		ScannerType        string `json:"scannerType"`
		LocalScannerURL    string `json:"localScannerUrl"`
		LocalScannerAPIKey string `json:"localScannerApiKey"`
		LocalScannerProtocol string `json:"localScannerProtocol"`
		LocalScannerTLS      bool   `json:"localScannerTls"`
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
	localURLRaw := strings.TrimSpace(body.LocalScannerURL)
	if body.ScannerType == "local" {
		if localURLRaw == "" {
			http.Error(w, "local scanner endpoint is required", http.StatusBadRequest)
			return
		}
		body.LocalScannerProtocol = "grpc"
		if normalizeLocalScannerGRPCAddr(localURLRaw) == "" {
			http.Error(w, "invalid gRPC address for local scanner", http.StatusBadRequest)
			return
		}
	} else {
		if body.Region == "" || body.APIKey == "" {
			http.Error(w, "apiKey and region required for saas scanner", http.StatusBadRequest)
			return
		}
	}
	h.cfg.Set(body.APIKey, body.Region)
	h.cfg.SetScanner(body.ScannerType, localURLRaw, strings.TrimSpace(body.LocalScannerAPIKey), body.LocalScannerProtocol, false)
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
	scannerType, localURL, localAPIKey, _, _ := h.cfg.GetScanner()
	type result struct {
		OK      bool   `json:"ok"`
		Message string `json:"message"`
	}
	if scannerType == "local" {
		if localURL == "" {
			http.Error(w, "local scanner URL is not configured", http.StatusBadRequest)
			return
		}
		addr := normalizeLocalScannerGRPCAddr(localURL)
		if addr == "" {
			http.Error(w, "local scanner gRPC address is invalid", http.StatusBadRequest)
			return
		}
		if !strings.Contains(addr, ":") {
			http.Error(w, "gRPC address must include a port (e.g., host:port)", http.StatusBadRequest)
			return
		}
		client, err := v1client.NewClientInternal(strings.TrimSpace(localAPIKey), addr, false, "")
		if err != nil {
			http.Error(w, "gRPC scanner connection failed: "+err.Error(), http.StatusBadGateway)
			return
		}
		defer client.Destroy()
		msg := "gRPC scanner is responding correctly at " + addr
		if probe := eicarProbe(); len(probe) > 0 {
			if _, probeErr := client.ScanBuffer(probe, "scanner-test.internal", []string{"v1fs-scanner", "scanner-connection-test"}); probeErr != nil {
				http.Error(w, "gRPC gateway connected but the anti-malware engine is not responding (check if the backend scanner service is running)", http.StatusBadGateway)
				return
			}
		} else {
			msg = "gRPC scanner is reachable at " + addr + " (malware detection test skipped on this platform)"
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result{OK: true, Message: msg})
		return
	}
	apiKey, region := h.cfg.Get()
	if apiKey == "" || region == "" {
		http.Error(w, "api key and region are required for saas scanner", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result{OK: true, Message: "saas scanner configuration looks valid"})
}

func (h *Handler) compatScanner(w http.ResponseWriter, r *http.Request) {
	scannerType, localURL, localAPIKey, _, _ := h.cfg.GetScanner()
	type result struct {
		OK      bool   `json:"ok"`
		Message string `json:"message"`
	}
	if scannerType == "local" {
		if localURL == "" {
			http.Error(w, "local scanner URL is not configured", http.StatusBadRequest)
			return
		}
		addr := normalizeLocalScannerGRPCAddr(localURL)
		if addr == "" {
			http.Error(w, "local scanner gRPC address is invalid", http.StatusBadRequest)
			return
		}
		client, err := v1client.NewClientInternal(strings.TrimSpace(localAPIKey), addr, false, "")
		if err != nil {
			http.Error(w, "compatibility check failed to connect: "+err.Error(), http.StatusBadGateway)
			return
		}
		defer client.Destroy()
		warn, probeErr := localGRPCCompatProbe(client)
		if probeErr != nil {
			http.Error(w, "scanner compatibility check failed: "+probeErr.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		msg := "local gRPC scanner accepted a probe scan (same tags and ScanFile->ScanBuffer path as directory scans)"
		if warn != "" {
			msg = warn
		}
		json.NewEncoder(w).Encode(result{OK: true, Message: msg})
		return
	}

	apiKey, region := h.cfg.Get()
	if apiKey == "" || region == "" {
		http.Error(w, "api key and region are required for saas scanner", http.StatusBadRequest)
		return
	}
	client, err := v1client.NewClient(apiKey, region)
	if err != nil {
		http.Error(w, "failed to initialize saas scanner: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer client.Destroy()
	w.Header().Set("Content-Type", "application/json")
	probe := eicarProbe()
	if len(probe) == 0 {
		json.NewEncoder(w).Encode(result{OK: true, Message: "SaaS scanner configuration looks valid (malware detection test skipped on this platform)"})
		return
	}
	verdict, err := client.ScanBuffer(probe, "usb-scanner-heartbeat.internal", []string{"v1fs-scanner", "usb-scanner-heartbeat"})
	if err != nil {
		http.Error(w, "malware detection test failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	if verdict == "" {
		http.Error(w, "malware test file was not detected as malicious", http.StatusBadGateway)
		return
	}
	json.NewEncoder(w).Encode(result{OK: true, Message: "malware detection is working (test file detected as: " + verdict + ")"})
}

func (h *Handler) scannerStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	scannerType, localURL, localAPIKey, _, _ := h.cfg.GetScanner()
	apiKey, region := h.cfg.Get()

	available := false

	if scannerType == "local" {
		if localURL != "" {
			addr := normalizeLocalScannerGRPCAddr(localURL)
			if addr != "" {
				client, err := v1client.NewClientInternal(strings.TrimSpace(localAPIKey), addr, false, "")
				if err == nil {
					defer client.Destroy()
					probe := eicarProbe()
					if len(probe) == 0 {
						probe = []byte("hello") // benign connectivity check
					}
					_, err := client.ScanBuffer(probe, "usb-scanner-heartbeat.internal", []string{"v1fs-scanner", "usb-scanner-heartbeat"})
					if err == nil {
						available = true
					}
				}
			}
		}
	} else {
		if apiKey != "" && region != "" {
			client, err := v1client.NewClient(apiKey, region)
			if err == nil {
				defer client.Destroy()
				probe := eicarProbe()
				if len(probe) == 0 {
					probe = []byte("hello") // benign connectivity check
				}
				_, err := client.ScanBuffer(probe, "usb-scanner-heartbeat.internal", []string{"v1fs-scanner", "usb-scanner-heartbeat"})
				if err == nil {
					available = true
				}
			}
		}
	}

	json.NewEncoder(w).Encode(map[string]bool{"available": available})
}

func (h *Handler) saveScanAction(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ActionOnMalware    string   `json:"actionOnMalware"`
		QuarantinePath     string   `json:"quarantinePath"`
		ScanConcurrency    *int     `json:"scanConcurrency"`
		MaxConcurrentScans *int     `json:"maxConcurrentScans"`
		HashEnabled        *bool    `json:"hashEnabled"`
		PredictiveML       *bool    `json:"predictiveML"`
		ReportMode         string   `json:"reportMode"`
		ScanTags           []string `json:"scanTags"`
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
	h.cfg.SetReportMode(strings.TrimSpace(strings.ToLower(body.ReportMode)))
	h.cfg.SetScanTags(scanner.ParseUserScanTags(body.ScanTags))
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
		"malwareTestFile": "malware-test.com",
		"cleanFile":  "hello.txt",
	})
}

func (h *Handler) listDirs(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	listing, err := ListDirectoryListing(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(listing)
}

const maxScanRoots = 32

func (h *Handler) startScan(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path       string   `json:"path"`
		Paths      []string `json:"paths"`
		ReportName string   `json:"reportName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	var roots []string
	for _, p := range body.Paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		p = filepath.Clean(p)
		if p == "." {
			p = string(filepath.Separator)
		}
		roots = append(roots, p)
	}
	if len(roots) == 0 {
		p := strings.TrimSpace(body.Path)
		if p == "" {
			http.Error(w, "path or paths required", http.StatusBadRequest)
			return
		}
		p = filepath.Clean(p)
		if p == "." {
			p = string(filepath.Separator)
		}
		roots = []string{p}
	}
	if len(roots) > maxScanRoots {
		http.Error(w, "too many paths (max "+strconv.Itoa(maxScanRoots)+")", http.StatusBadRequest)
		return
	}
	uniq := make(map[string]struct{})
	var scanRoots []string
	for _, path := range roots {
		if _, dup := uniq[path]; dup {
			continue
		}
		uniq[path] = struct{}{}
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			http.Error(w, "path is not a valid directory: "+path, http.StatusBadRequest)
			return
		}
		scanRoots = append(scanRoots, path)
	}
	if len(scanRoots) == 0 {
		http.Error(w, "no valid directories to scan", http.StatusBadRequest)
		return
	}
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
	task := h.store.Create(scanRoots)
	if task == nil {
		http.Error(w, "failed to create scan task", http.StatusInternalServerError)
		return
	}
	if rn := strings.TrimSpace(body.ReportName); rn != "" {
		task.SetReportName(rn)
	}
	go h.store.RunScan(task.ID, scanRoots, apiKey, region, opts)

	w.Header().Set("Content-Type", "application/json")
	out := map[string]interface{}{"taskId": task.ID}
	if runningInContainer() {
		for _, p := range scanRoots {
			if p == string(filepath.Separator) {
				out["scanHint"] = "The root / here is only this container (few hundred files). To scan your real disks, restart with -v HOST:/CONTAINER and target that path—for example -v /Users/you:/mnt/data and scan /mnt/data, or -v /:/host:ro and scan /host (Linux)."
				break
			}
		}
	}
	json.NewEncoder(w).Encode(out)
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
		"scanErrors":   snap.ScanErrors,
		"lastScanError": snap.LastScanError,
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

// localGRPCCompatClient matches the scan methods used by RunScan for local gRPC.
type localGRPCCompatClient interface {
	ScanFile(path string, tags []string) (string, error)
	ScanBuffer(data []byte, filename string, tags []string) (string, error)
}

func grpcLocalProbeSoftFailure(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	low := strings.ToLower(msg)
	return strings.Contains(msg, "code = Unimplemented") ||
		strings.Contains(low, "not compatible") ||
		strings.Contains(low, "please upgrade")
}

// localGRPCCompatProbe mirrors directory scans: ScanFile first, then ScanBuffer with v1fs-scanner tags.
// Some gateways return Unimplemented / "not compatible" for buffer-only probes while still scanning files.
func localGRPCCompatProbe(client localGRPCCompatClient) (warning string, err error) {
	probe := eicarProbe()
	if len(probe) == 0 {
		return "compatibility probe skipped on this platform (no malware test available)", nil
	}
	tmp, err := os.CreateTemp("", "v1fs-compat-*.com")
	if err != nil {
		return "", err
	}
	path := tmp.Name()
	defer os.Remove(path)
	if _, werr := tmp.Write(probe); werr != nil {
		tmp.Close()
		return "", werr
	}
	if cerr := tmp.Close(); cerr != nil {
		return "", cerr
	}
	tags := []string{"v1fs-scanner", "usb-scanner-heartbeat"}
	verdict, errFile := client.ScanFile(path, tags)
	if errFile == nil && verdict != "" {
		return "", nil
	}
	if errFile != nil && !grpcLocalProbeSoftFailure(errFile) {
		return "", errFile
	}
	verdict, errBuf := client.ScanBuffer(probe, "usb-scanner-heartbeat.internal", tags)
	if errBuf == nil && verdict != "" {
		return "", nil
	}
	if errBuf != nil && !grpcLocalProbeSoftFailure(errBuf) {
		return "", errBuf
	}
	if grpcLocalProbeSoftFailure(errBuf) {
		return "Gateway is reachable but rejected the malware probe with a version-handshake error. If directory or malware test scans still complete, your deployment is usable; otherwise upgrade the gateway or File Security SDK to matching versions.", nil
	}
	return "", fmt.Errorf("malware test file was not detected as malicious")
}

func normalizeLocalScannerURL(raw string) string {
	u := strings.TrimSpace(raw)
	if u == "" {
		return ""
	}
	if !strings.HasPrefix(strings.ToLower(u), "http://") && !strings.HasPrefix(strings.ToLower(u), "https://") {
		u = "http://" + u
	}
	return u
}

func normalizeLocalScannerGRPCAddr(raw string) string {
	v := strings.TrimSpace(raw)
	if v == "" {
		return ""
	}
	v = strings.TrimPrefix(v, "grpc://")
	v = strings.TrimPrefix(v, "grpcs://")
	v = strings.TrimPrefix(v, "http://")
	v = strings.TrimPrefix(v, "https://")
	v = strings.TrimSuffix(v, "/")
	return v
}

