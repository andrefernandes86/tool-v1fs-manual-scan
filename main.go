package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"v1fs-scanner/internal/api"
	"v1fs-scanner/internal/config"
	"v1fs-scanner/internal/scanner"
)

// createTestSamples seeds the clean test file at startup.
// The EICAR sample is NOT written here — it is assembled on demand
// by eicarProbe() only when a test submission is triggered.
func createTestSamples(dir string) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("create test-samples dir: %v", err)
		return
	}
	helloPath := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(helloPath, []byte("Hello World\n"), 0644); err != nil {
		log.Printf("write hello.txt: %v", err)
	} else {
		log.Printf("created test sample: %s", helloPath)
	}
}

// defaultDataDir returns a writable directory for config and reports.
// In Docker the /data volume is always present; natively falls back to
// the OS user-config directory so the app works on Windows/macOS/Linux.
func defaultDataDir() string {
	if _, err := os.Stat("/data"); err == nil {
		return "/data"
	}
	base, err := os.UserConfigDir()
	if err != nil {
		base, _ = os.UserHomeDir()
	}
	return filepath.Join(base, "V1FSScanner")
}

func main() {
	dataDir := defaultDataDir()

	configPath := os.Getenv("V1FS_CONFIG_PATH")
	if configPath == "" {
		configPath = filepath.Join(dataDir, "config.json")
	}
	reportsDir := os.Getenv("V1FS_REPORTS_DIR")
	if reportsDir == "" {
		reportsDir = filepath.Join(dataDir, "reports")
	}
	if err := os.MkdirAll(reportsDir, 0755); err != nil {
		log.Fatalf("create reports dir: %v", err)
	}

	testSamplesDir := os.Getenv("V1FS_TEST_SAMPLES_DIR")
	if testSamplesDir == "" {
		testSamplesDir = filepath.Join(dataDir, "test-samples")
	}
	createTestSamples(testSamplesDir)

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Printf("config load (will use env if set): %v", err)
		cfg = &config.Config{}
	}
	// Use env only when saved config is empty (so web-configured values persist)
	if k := os.Getenv("TM_V1_API_KEY"); k != "" && cfg.APIKey == "" {
		cfg.APIKey = k
	}
	if r := os.Getenv("TM_V1_REGION"); r != "" && cfg.Region == "" {
		cfg.Region = r
	}

	store := scanner.NewTaskStore(reportsDir)
	handler := api.NewHandler(cfg, configPath, store, testSamplesDir, WebFS)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("V1FS Scanner listening on :%s", port)

	// Platform-specific startup: open browser (macOS/Windows) or print URL (Linux).
	go platformInit(port)

	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatal(err)
	}
}
