package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config holds CLI configuration
type Config struct {
	APIURL     string `json:"api_url"`
	APIKey     string `json:"api_key"`
	AdminToken string `json:"admin_token"`
	OpsSecret  string `json:"ops_secret"`
}

const configDir = ".frn"
const configFile = "config.json"

// LoadConfig reads config from env, then from ~/.frn/config.json
func LoadConfig() *Config {
	cfg := &Config{}
	if v := os.Getenv("FREERANGE_API_URL"); v != "" {
		cfg.APIURL = v
	}
	if v := os.Getenv("FREERANGE_API_KEY"); v != "" {
		cfg.APIKey = v
	}
	if v := os.Getenv("FREERANGE_ADMIN_TOKEN"); v != "" {
		cfg.AdminToken = v
	}
	if v := os.Getenv("FREERANGE_OPS_SECRET"); v != "" {
		cfg.OpsSecret = v
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return cfg
	}
	path := filepath.Join(home, configDir, configFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	var fileCfg Config
	if err := json.Unmarshal(data, &fileCfg); err != nil {
		return cfg
	}
	if cfg.APIURL == "" && fileCfg.APIURL != "" {
		cfg.APIURL = fileCfg.APIURL
	}
	if cfg.APIKey == "" && fileCfg.APIKey != "" {
		cfg.APIKey = fileCfg.APIKey
	}
	if cfg.AdminToken == "" && fileCfg.AdminToken != "" {
		cfg.AdminToken = fileCfg.AdminToken
	}
	if cfg.OpsSecret == "" && fileCfg.OpsSecret != "" {
		cfg.OpsSecret = fileCfg.OpsSecret
	}
	return cfg
}

// SaveConfig writes config to ~/.frn/config.json
func SaveConfig(cfg *Config) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("home dir: %w", err)
	}
	dir := filepath.Join(home, configDir)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	path := filepath.Join(dir, configFile)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// ConfigPath returns the path to the config file
func ConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, configDir, configFile)
}
