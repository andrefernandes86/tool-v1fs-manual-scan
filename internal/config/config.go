package config

import (
	"encoding/json"
	"os"
	"sync"
)

type Config struct {
	APIKey          string `json:"apiKey,omitempty"`
	Region          string `json:"region,omitempty"`
	ActionOnMalware string `json:"actionOnMalware,omitempty"` // "log", "quarantine", "delete"
	QuarantinePath  string `json:"quarantinePath,omitempty"`
	ScanConcurrency int    `json:"scanConcurrency,omitempty"` // 0 = use default (8)
	mu              sync.RWMutex
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

func (c *Config) Set(apiKey, region string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.APIKey = apiKey
	c.Region = region
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

func (c *Config) Save(path string) error {
	c.mu.RLock()
	data, err := json.MarshalIndent(c, "", "  ")
	c.mu.RUnlock()
	if err != nil {
		return err
	}
	dir := path
	for i := len(dir) - 1; i >= 0; i-- {
		if dir[i] == '/' {
			dir = dir[:i]
			break
		}
	}
	if dir != path {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, data, 0600)
}
