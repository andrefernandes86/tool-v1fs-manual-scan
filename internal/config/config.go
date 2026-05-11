package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Config struct {
	APIKey             string `json:"apiKey,omitempty"`
	Region             string `json:"region,omitempty"`
	ScannerType        string `json:"scannerType,omitempty"` // "saas" (default) or "local"
	LocalScannerURL    string `json:"localScannerUrl,omitempty"`
	LocalScannerAPIKey string `json:"localScannerApiKey,omitempty"`
	LocalScannerProtocol string `json:"localScannerProtocol,omitempty"` // always "grpc" when scannerType is local
	LocalScannerTLS      bool   `json:"localScannerTls,omitempty"`
	ActionOnMalware    string `json:"actionOnMalware,omitempty"`    // "log", "quarantine", "delete"
	QuarantinePath     string `json:"quarantinePath,omitempty"`
	ScanConcurrency    int    `json:"scanConcurrency,omitempty"`    // 0 = use default (8)
	MaxConcurrentScans int    `json:"maxConcurrentScans,omitempty"` // 0 = unlimited
	HashEnabled        bool   `json:"hashEnabled,omitempty"`        // generate hashes for malicious files in reports
	PredictiveML       bool   `json:"predictiveML,omitempty"`       // enable predictive machine learning (PML)
	ReportMode         string `json:"reportMode,omitempty"`         // "stats" or "all"
	ScanTags           []string `json:"scanTags,omitempty"`         // optional extra SDK tags for Vision One
	mu                 sync.RWMutex
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *Config) Get() (apiKey, region string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.APIKey, c.Region
}

func (c *Config) GetScanner() (scannerType, localURL, localAPIKey, localProtocol string, localTLS bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	scannerType = c.ScannerType
	if scannerType == "" {
		scannerType = "saas"
	}
	localProtocol = strings.TrimSpace(strings.ToLower(c.LocalScannerProtocol))
	localTLS = c.LocalScannerTLS
	if scannerType == "local" {
		localProtocol = "grpc"
		raw := strings.TrimSpace(c.LocalScannerURL)
		low := strings.ToLower(raw)
		if localProtocol == "grpc" && !localTLS {
			if strings.HasPrefix(low, "grpcs://") || strings.HasPrefix(low, "https://") {
				localTLS = true
			}
		}
	} else {
		if localProtocol == "" {
			localProtocol = "grpc"
		}
	}
	return scannerType, c.LocalScannerURL, c.LocalScannerAPIKey, localProtocol, localTLS
}

func (c *Config) Set(apiKey, region string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.APIKey = apiKey
	c.Region = region
}

func (c *Config) SetScanner(scannerType, localURL, localAPIKey, localProtocol string, localTLS bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if scannerType != "local" {
		scannerType = "saas"
	}
	localProtocol = strings.TrimSpace(strings.ToLower(localProtocol))
	if scannerType == "local" {
		localProtocol = "grpc"
	} else if localProtocol == "" {
		localProtocol = "grpc"
	}
	c.ScannerType = scannerType
	c.LocalScannerURL = localURL
	c.LocalScannerAPIKey = localAPIKey
	c.LocalScannerProtocol = localProtocol
	c.LocalScannerTLS = localTLS
}

func (c *Config) GetScanAction() (action, quarantinePath string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	action = c.ActionOnMalware
	if action == "" {
		action = "log"
	}
	return action, c.QuarantinePath
}

func (c *Config) GetScanConcurrency() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ScanConcurrency
}

func (c *Config) GetMaxConcurrentScans() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.MaxConcurrentScans
}

func (c *Config) GetHashEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.HashEnabled
}

func (c *Config) GetPredictiveML() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.PredictiveML
}

func (c *Config) GetReportMode() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.ReportMode == "all" {
		return "all"
	}
	return "stats"
}

func (c *Config) GetScanTags() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.ScanTags) == 0 {
		return nil
	}
	return append([]string(nil), c.ScanTags...)
}

func (c *Config) SetScanTags(tags []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ScanTags = tags
}

func (c *Config) SetScanAction(action, quarantinePath string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if action == "" {
		action = "log"
	}
	c.ActionOnMalware = action
	c.QuarantinePath = quarantinePath
}

func (c *Config) SetScanConcurrency(n int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ScanConcurrency = n
}

func (c *Config) SetMaxConcurrentScans(n int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.MaxConcurrentScans = n
}

func (c *Config) SetHashEnabled(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.HashEnabled = enabled
}

func (c *Config) SetPredictiveML(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.PredictiveML = enabled
}

func (c *Config) SetReportMode(mode string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if mode == "all" {
		c.ReportMode = "all"
		return
	}
	c.ReportMode = "stats"
}

func (c *Config) Save(path string) error {
	c.mu.RLock()
	data, err := json.MarshalIndent(c, "", "  ")
	c.mu.RUnlock()
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, data, 0600)
}
