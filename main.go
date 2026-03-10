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

// EICAR standard antivirus test file content (68 bytes, no newline)
const eicarContent = "X5O!P%@AP[4\\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*"

func createTestSamples(dir string) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("create test-samples dir: %v", err)
		return
	}
	eicarPath := filepath.Join(dir, "eicar.com")
	if err := os.WriteFile(eicarPath, []byte(eicarContent), 0644); err != nil {
		log.Printf("write eicar.com: %v", err)
	} else {
		log.Printf("created test sample: %s", eicarPath)
	}
	helloPath := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(helloPath, []byte("Hello World\n"), 0644); err != nil {
		log.Printf("write hello.txt: %v", err)
	} else {
		log.Printf("created test sample: %s", helloPath)
	}
}

func main() {
	configPath := os.Getenv("V1FS_CONFIG_PATH")
	if configPath == "" {
		configPath = "/data/config.json"
	}
	reportsDir := os.Getenv("V1FS_REPORTS_DIR")
	if reportsDir == "" {
		reportsDir = "/data/reports"
	}
	if err := os.MkdirAll(reportsDir, 0755); err != nil {
		log.Fatalf("create reports dir: %v", err)
	}

	testSamplesDir := os.Getenv("V1FS_TEST_SAMPLES_DIR")
	if testSamplesDir == "" {
		testSamplesDir = filepath.Join(filepath.Dir(reportsDir), "test-samples")
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
	handler := api.NewHandler(cfg, configPath, store, testSamplesDir)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("V1FS Scanner listening on :%s", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatal(err)
	}
}
