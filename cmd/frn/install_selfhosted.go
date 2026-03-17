package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	verifySelfHostedLicenseFn = verifySelfHostedLicense
	downloadFileFn            = downloadFile
)

func runSelfHostedInstall(cfg installConfig) error {
	if cfg.LicenseKey == "" {
		return fmt.Errorf("self-hosted installation requires --license-key")
	}

	if cfg.LicenseVerifyURL == "" {
		cfg.LicenseVerifyURL = "https://api.freerangenotify.com/v1/license/verify"
	}

	if cfg.ServerBinaryURL == "" || cfg.WorkerBinaryURL == "" {
		return fmt.Errorf("self-hosted installation requires --server-binary-url and --worker-binary-url")
	}

	if !cfg.SkipPreflight {
		if err := checkElasticsearch(cfg.Elasticsearch, cfg.ElasticsearchUsername, cfg.ElasticsearchPassword); err != nil {
			return fmt.Errorf("elasticsearch preflight failed: %w", err)
		}
		if err := checkRedis(cfg.RedisHost, cfg.RedisPort); err != nil {
			return fmt.Errorf("redis preflight failed: %w", err)
		}
	}

	if err := verifySelfHostedLicenseFn(cfg.LicenseVerifyURL, cfg.LicenseKey); err != nil {
		return err
	}

	serverTarget := filepath.Join(cfg.InstallDir, "bin", "server")
	workerTarget := filepath.Join(cfg.InstallDir, "bin", "worker")

	if err := downloadFileFn(cfg.ServerBinaryURL, serverTarget); err != nil {
		return err
	}
	if err := downloadFileFn(cfg.WorkerBinaryURL, workerTarget); err != nil {
		return err
	}

	if err := writeSelfHostedConfig(cfg); err != nil {
		return err
	}

	if err := writeSystemdUnits(cfg); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "Self-hosted binaries installed in %s\n", cfg.InstallDir)
	fmt.Fprintf(os.Stdout, "Systemd units generated in %s\n", filepath.Join(cfg.InstallDir, "systemd"))

	if cfg.StartAfterSetup {
		if err := installAndStartSystemdUnits(cfg); err != nil {
			fmt.Fprintf(os.Stdout, "Automatic systemd install/start skipped: %v\n", err)
			fmt.Fprintln(os.Stdout, "Install systemd units manually:")
			fmt.Fprintln(os.Stdout, "  sudo cp systemd/freerange-*.service /etc/systemd/system/")
			fmt.Fprintln(os.Stdout, "  sudo systemctl daemon-reload")
			fmt.Fprintln(os.Stdout, "  sudo systemctl enable --now freerange-server freerange-worker")
		} else {
			fmt.Fprintln(os.Stdout, "Systemd services installed and started")
		}
	} else {
		fmt.Fprintln(os.Stdout, "Install systemd units manually:")
		fmt.Fprintln(os.Stdout, "  sudo cp systemd/freerange-*.service /etc/systemd/system/")
		fmt.Fprintln(os.Stdout, "  sudo systemctl daemon-reload")
		fmt.Fprintln(os.Stdout, "  sudo systemctl enable --now freerange-server freerange-worker")
	}

	return nil
}

func writeSelfHostedConfig(cfg installConfig) error {
	configDir := filepath.Join(cfg.InstallDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	doc := map[string]interface{}{
		"app": map[string]interface{}{
			"environment": "production",
			"debug":       false,
		},
		"server": map[string]interface{}{
			"host": "0.0.0.0",
			"port": mustAtoi(cfg.ServerPort, 8080),
		},
		"database": map[string]interface{}{
			"urls": []string{cfg.Elasticsearch},
		},
		"redis": map[string]interface{}{
			"host": cfg.RedisHost,
			"port": mustAtoi(cfg.RedisPort, 6379),
		},
		"licensing": map[string]interface{}{
			"enabled":         true,
			"deployment_mode": "self_hosted",
			"fail_mode":       "fail_closed",
			"self_hosted": map[string]interface{}{
				"license_key":                cfg.LicenseKey,
				"license_server_url":         cfg.LicenseVerifyURL,
				"heartbeat_url":              deriveHeartbeatURL(cfg.LicenseVerifyURL),
				"verify_interval_seconds":    300,
				"heartbeat_interval_seconds": 21600,
			},
		},
	}

	if cfg.ElasticsearchUsername != "" {
		doc["database"].(map[string]interface{})["username"] = cfg.ElasticsearchUsername
	}
	if cfg.ElasticsearchPassword != "" {
		doc["database"].(map[string]interface{})["password"] = cfg.ElasticsearchPassword
	}
	if cfg.RedisPassword != "" {
		doc["redis"].(map[string]interface{})["password"] = cfg.RedisPassword
	}

	out, err := yaml.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	path := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(path, out, 0600); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}
	return nil
}

func mustAtoi(raw string, fallback int) int {
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return fallback
	}
	return v
}

func deriveHeartbeatURL(verifyURL string) string {
	verifyURL = strings.TrimSpace(verifyURL)
	if verifyURL == "" {
		return ""
	}
	if strings.HasSuffix(verifyURL, "/verify") {
		return strings.TrimSuffix(verifyURL, "/verify") + "/heartbeat"
	}
	return verifyURL
}
