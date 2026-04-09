package api

import (
	"encoding/json"
	"io"
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
	case r.URL.Path == "/api/scanner/compat" && r.Method == http.MethodPost:
		h.compatScanner(w, r)
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
	scannerType, localURL, _, localProtocol, localTLS := h.cfg.GetScanner()
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
		"localScannerTls":    localTLS,
		"actionOnMalware":    action,
		"quarantinePath":     quarantinePath,
		"scanConcurrency":    concurrency,
		"maxConcurrentScans": maxScans,
		"hashEnabled":        hashEnabled,
		"predictiveML":       predictiveML,
		"reportMode":         reportMode,
		"runningInContainer": runningInContainer(),
	}
	if runningInContainer() {
		out["containerScanRootHint"] = "Scans are recursive. In Docker, / is only this container (often a few hundred files), not your host. To scan the host, run with e.g. -v /:/host:ro and add /host under Scan targets."
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
		low := strings.ToLower(localURLRaw)
		if strings.HasPrefix(low, "grpcs://") || strings.HasPrefix(low, "https://") {
			body.LocalScannerTLS = true
		}
	} else {
		if body.Region == "" || body.APIKey == "" {
			http.Error(w, "apiKey and region required for saas scanner", http.StatusBadRequest)
			return
		}
	}
	h.cfg.Set(body.APIKey, body.Region)
	h.cfg.SetScanner(body.ScannerType, localURLRaw, strings.TrimSpace(body.LocalScannerAPIKey), body.LocalScannerProtocol, body.LocalScannerTLS)
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
	scannerType, localURL, localAPIKey, _, localTLS := h.cfg.GetScanner()
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
		client, err := v1client.NewClientInternal(strings.TrimSpace(localAPIKey), addr, localTLS, "")
		if err != nil {
			http.Error(w, "local gRPC scanner is not reachable: "+err.Error(), http.StatusBadGateway)
			return
		}
		client.Destroy()
		w.Header().Set("Content-Type", "application/json")
		msg := "local gRPC scanner is reachable at " + addr
		if !localTLS {
			msg += " (TLS disabled)"
		}
		json.NewEncoder(w).Encode(result{OK: true, Message: msg})
		return
	}
	// SaaS mode: verify required configuration. Network checks can fail in restricted
	// environments even when scanner configuration is valid, so keep this check reliable.
	apiKey, region := h.cfg.Get()
	if apiKey == "" || region == "" {
		http.Error(w, "api key and region are required for saas scanner", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result{OK: true, Message: "saas scanner configuration looks valid"})
}

func (h *Handler) compatScanner(w http.ResponseWriter, r *http.Request) {
	scannerType, localURL, localAPIKey, _, localTLS := h.cfg.GetScanner()
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
		client, err := v1client.NewClientInternal(strings.TrimSpace(localAPIKey), addr, localTLS, "")
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
		msg := "local gRPC scanner accepted a probe scan (same tags and ScanFile→ScanBuffer path as directory scans)"
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
	probe := []byte("X5O!P%@AP[4\\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*")
	_, err = client.ScanBuffer(probe, "eicar-compat-check.com", []string{"compat-check"})
	if err != nil {
		http.Error(w, "saas compatibility check failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result{OK: true, Message: "saas scanner is compatible and accepted a probe scan"})
}

func (h *Handler) saveScanAction(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ActionOnMalware    string `json:"actionOnMalware"`
		QuarantinePath     string `json:"quarantinePath"`
		ScanConcurrency    *int   `json:"scanConcurrency"`
		MaxConcurrentScans *int   `json:"maxConcurrentScans"`
		HashEnabled        *bool  `json:"hashEnabled"`
		PredictiveML       *bool  `json:"predictiveML"`
		ReportMode         string `json:"reportMode"`
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
				out["scanHint"] = "Recursive scan of every file under /. In Docker this is only the container image (often ~300–800 files), not your Mac/PC host. Mount the host (e.g. docker run … -v /:/host:ro …) and scan /host to include the full host tree."
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
	probe := []byte("X5O!P%@AP[4\\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*")
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
	tags := []string{"v1fs-scanner"}
	_, errFile := client.ScanFile(path, tags)
	if errFile == nil {
		return "", nil
	}
	if !grpcLocalProbeSoftFailure(errFile) {
		return "", errFile
	}
	_, errBuf := client.ScanBuffer(probe, "eicar.com", tags)
	if errBuf == nil {
		return "", nil
	}
	if grpcLocalProbeSoftFailure(errBuf) {
		return "Gateway is reachable but rejected the malware probe with a version-handshake error. If directory or EICAR test scans still complete, your deployment is usable; otherwise upgrade the gateway or File Security SDK to matching versions.", nil
	}
	return "", errBuf
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
