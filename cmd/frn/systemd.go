package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func writeSystemdUnits(cfg installConfig) error {
	unitDir := filepath.Join(cfg.InstallDir, "systemd")
	if err := os.MkdirAll(unitDir, 0755); err != nil {
		return fmt.Errorf("create systemd dir: %w", err)
	}

	serverBin := filepath.Join(cfg.InstallDir, "bin", "server")
	workerBin := filepath.Join(cfg.InstallDir, "bin", "worker")
	configDir := filepath.Join(cfg.InstallDir, "config")

	serverUnit := fmt.Sprintf(`[Unit]
Description=FreeRangeNotify Server
After=network.target

[Service]
Type=simple
WorkingDirectory=%s
ExecStart=%s
Restart=always
RestartSec=3
Environment=FREERANGE_SERVER_PORT=%s

[Install]
WantedBy=multi-user.target
`, cfg.InstallDir, serverBin, cfg.ServerPort)

	workerUnit := fmt.Sprintf(`[Unit]
Description=FreeRangeNotify Worker
After=network.target

[Service]
Type=simple
WorkingDirectory=%s
ExecStart=%s
Restart=always
RestartSec=3
Environment=FREERANGE_CONFIG_PATH=%s

[Install]
WantedBy=multi-user.target
`, cfg.InstallDir, workerBin, configDir)

	if err := os.WriteFile(filepath.Join(unitDir, "freerange-server.service"), []byte(serverUnit), 0644); err != nil {
		return fmt.Errorf("write server unit: %w", err)
	}
	if err := os.WriteFile(filepath.Join(unitDir, "freerange-worker.service"), []byte(workerUnit), 0644); err != nil {
		return fmt.Errorf("write worker unit: %w", err)
	}

	return nil
}

func installAndStartSystemdUnits(cfg installConfig) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("automatic systemd install is only supported on linux")
	}

	if _, err := exec.LookPath("systemctl"); err != nil {
		return fmt.Errorf("systemctl not found")
	}

	copyCmd := exec.Command("sudo", "cp",
		filepath.Join(cfg.InstallDir, "systemd", "freerange-server.service"),
		filepath.Join(cfg.InstallDir, "systemd", "freerange-worker.service"),
		"/etc/systemd/system/")
	copyCmd.Stdout = os.Stdout
	copyCmd.Stderr = os.Stderr
	if err := copyCmd.Run(); err != nil {
		return fmt.Errorf("copy systemd units failed: %w", err)
	}

	reloadCmd := exec.Command("sudo", "systemctl", "daemon-reload")
	reloadCmd.Stdout = os.Stdout
	reloadCmd.Stderr = os.Stderr
	if err := reloadCmd.Run(); err != nil {
		return fmt.Errorf("systemd daemon-reload failed: %w", err)
	}

	enableCmd := exec.Command("sudo", "systemctl", "enable", "--now", "freerange-server", "freerange-worker")
	enableCmd.Stdout = os.Stdout
	enableCmd.Stderr = os.Stderr
	if err := enableCmd.Run(); err != nil {
		return fmt.Errorf("systemd enable/start failed: %w", err)
	}

	return nil
}
