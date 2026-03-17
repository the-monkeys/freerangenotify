package main

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type installConfig struct {
	InstallDir            string
	DeploymentMode        string
	Elasticsearch         string
	ElasticsearchUsername string
	ElasticsearchPassword string
	RedisHost             string
	RedisPort             string
	RedisPassword         string
	ServerPort            string
	LicenseKey            string
	LicenseVerifyURL      string
	ServerBinaryURL       string
	WorkerBinaryURL       string
	LegacyDocker          bool
	StartAfterSetup       bool
	SkipPreflight         bool
}

func newInstallCmd() *cobra.Command {
	cfg := installConfig{
		InstallDir:       ".",
		DeploymentMode:   "self_hosted",
		Elasticsearch:    "http://localhost:9200",
		RedisHost:        "localhost",
		RedisPort:        "6379",
		ServerPort:       "8080",
		LicenseVerifyURL: "https://api.freerangenotify.com/v1/license/verify",
		ServerBinaryURL:  "https://releases.freerangenotify.com/latest/server-selfhosted-linux-amd64",
		WorkerBinaryURL:  "https://releases.freerangenotify.com/latest/worker-selfhosted-linux-amd64",
		StartAfterSetup:  true,
	}

	var yes bool
	var setKV []string
	var envFile string

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install and bootstrap a self-hosted FreeRangeNotify stack",
		Long:  "Installs hosted/self-hosted deployments. Self-hosted mode downloads binaries + config + systemd units. Legacy docker mode remains available.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := applyInstallSetters(&cfg, setKV); err != nil {
				return err
			}

			if envFile != "" {
				envMap, err := readEnvFile(envFile)
				if err != nil {
					return fmt.Errorf("read --env-file: %w", err)
				}
				applyInstallEnvMap(&cfg, envMap)
			}

			if !yes {
				if err := promptInstallConfig(&cfg); err != nil {
					return err
				}
			}

			if cfg.DeploymentMode != "hosted" && cfg.DeploymentMode != "self_hosted" {
				return fmt.Errorf("deployment mode must be hosted or self_hosted")
			}

			if err := os.MkdirAll(cfg.InstallDir, 0755); err != nil {
				return fmt.Errorf("create install dir: %w", err)
			}

			if cfg.DeploymentMode == "self_hosted" && !cfg.LegacyDocker {
				if err := runSelfHostedInstall(cfg); err != nil {
					return err
				}
				return nil
			}

			if !cfg.SkipPreflight {
				if err := runPreflightChecks(cfg); err != nil {
					return err
				}
			}

			composePath, err := ensureComposeFile(cfg.InstallDir)
			if err != nil {
				return err
			}

			if err := writeInstallEnv(cfg); err != nil {
				return err
			}

			if err := patchInstallLicensingConfig(cfg); err != nil {
				return err
			}

			fmt.Fprintf(os.Stdout, "Install files ready in %s\n", cfg.InstallDir)
			fmt.Fprintf(os.Stdout, "Compose file: %s\n", composePath)
			fmt.Fprintf(os.Stdout, "Env file: %s\n", filepath.Join(cfg.InstallDir, ".env"))

			if cfg.StartAfterSetup {
				if err := startCompose(cfg.InstallDir, composePath); err != nil {
					return err
				}
				fmt.Fprintln(os.Stdout, "Stack started successfully")
			}

			fmt.Fprintln(os.Stdout, "Next steps:")
			fmt.Fprintln(os.Stdout, "  1. Verify API health: frn health --api-url http://localhost:"+cfg.ServerPort)
			fmt.Fprintln(os.Stdout, "  2. Verify license state: frn license status --api-key <APP_API_KEY>")
			fmt.Fprintln(os.Stdout, "  3. If needed, attach/patch license: frn license attach|patch ...")
			return nil
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "Use defaults/non-interactive mode")
	cmd.Flags().StringArrayVar(&setKV, "set", nil, "Override values as key=value (repeatable)")
	cmd.Flags().StringVar(&envFile, "env-file", "", "Read values from env file")
	cmd.Flags().StringVar(&cfg.InstallDir, "dir", cfg.InstallDir, "Install directory")
	cmd.Flags().StringVar(&cfg.DeploymentMode, "deployment-mode", cfg.DeploymentMode, "Deployment mode: hosted or self_hosted")
	cmd.Flags().StringVar(&cfg.Elasticsearch, "elasticsearch", cfg.Elasticsearch, "Elasticsearch base URL")
	cmd.Flags().StringVar(&cfg.ElasticsearchUsername, "elasticsearch-username", cfg.ElasticsearchUsername, "Elasticsearch username")
	cmd.Flags().StringVar(&cfg.ElasticsearchPassword, "elasticsearch-password", cfg.ElasticsearchPassword, "Elasticsearch password")
	cmd.Flags().StringVar(&cfg.RedisHost, "redis-host", cfg.RedisHost, "Redis host")
	cmd.Flags().StringVar(&cfg.RedisPort, "redis-port", cfg.RedisPort, "Redis port")
	cmd.Flags().StringVar(&cfg.RedisPassword, "redis-password", cfg.RedisPassword, "Redis password")
	cmd.Flags().StringVar(&cfg.ServerPort, "server-port", cfg.ServerPort, "Server port")
	cmd.Flags().StringVar(&cfg.LicenseKey, "license-key", "", "Signed self-hosted license token")
	cmd.Flags().StringVar(&cfg.LicenseVerifyURL, "license-verify-url", cfg.LicenseVerifyURL, "Remote endpoint for self-hosted license verification")
	cmd.Flags().StringVar(&cfg.ServerBinaryURL, "server-binary-url", cfg.ServerBinaryURL, "URL for self-hosted server binary")
	cmd.Flags().StringVar(&cfg.WorkerBinaryURL, "worker-binary-url", cfg.WorkerBinaryURL, "URL for self-hosted worker binary")
	cmd.Flags().BoolVar(&cfg.LegacyDocker, "legacy-docker", false, "Use legacy Docker/Compose install flow")
	cmd.Flags().BoolVar(&cfg.StartAfterSetup, "start", cfg.StartAfterSetup, "Start stack after generating files")
	cmd.Flags().BoolVar(&cfg.SkipPreflight, "skip-preflight", false, "Skip docker/connectivity checks")

	return cmd
}

func applyInstallSetters(cfg *installConfig, kvPairs []string) error {
	for _, kv := range kvPairs {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid --set value %q, expected key=value", kv)
		}
		key := strings.TrimSpace(strings.ToLower(parts[0]))
		val := strings.TrimSpace(parts[1])

		switch key {
		case "dir", "install_dir":
			cfg.InstallDir = val
		case "deployment_mode":
			cfg.DeploymentMode = val
		case "elasticsearch", "elasticsearch_url", "database_urls":
			cfg.Elasticsearch = val
		case "elasticsearch_username", "database_username":
			cfg.ElasticsearchUsername = val
		case "elasticsearch_password", "database_password":
			cfg.ElasticsearchPassword = val
		case "redis_host":
			cfg.RedisHost = val
		case "redis_port":
			cfg.RedisPort = val
		case "redis_password":
			cfg.RedisPassword = val
		case "server_port":
			cfg.ServerPort = val
		case "license_key":
			cfg.LicenseKey = val
		case "license_verify_url":
			cfg.LicenseVerifyURL = val
		case "server_binary_url":
			cfg.ServerBinaryURL = val
		case "worker_binary_url":
			cfg.WorkerBinaryURL = val
		case "legacy_docker":
			cfg.LegacyDocker = strings.EqualFold(val, "true") || val == "1" || strings.EqualFold(val, "yes")
		case "start":
			cfg.StartAfterSetup = strings.EqualFold(val, "true") || val == "1" || strings.EqualFold(val, "yes")
		case "skip_preflight":
			cfg.SkipPreflight = strings.EqualFold(val, "true") || val == "1" || strings.EqualFold(val, "yes")
		default:
			return fmt.Errorf("unsupported --set key %q", key)
		}
	}
	return nil
}

func applyInstallEnvMap(cfg *installConfig, env map[string]string) {
	if v := env["FREERANGE_DATABASE_URLS"]; v != "" {
		cfg.Elasticsearch = v
	}
	if v := env["FREERANGE_DATABASE_USERNAME"]; v != "" {
		cfg.ElasticsearchUsername = v
	}
	if v := env["FREERANGE_DATABASE_PASSWORD"]; v != "" {
		cfg.ElasticsearchPassword = v
	}
	if v := env["FREERANGE_REDIS_HOST"]; v != "" {
		cfg.RedisHost = v
	}
	if v := env["FREERANGE_REDIS_PORT"]; v != "" {
		cfg.RedisPort = v
	}
	if v := env["FREERANGE_REDIS_PASSWORD"]; v != "" {
		cfg.RedisPassword = v
	}
	if v := env["FREERANGE_SERVER_PORT"]; v != "" {
		cfg.ServerPort = v
	}
	if v := env["FREERANGE_LICENSING_DEPLOYMENT_MODE"]; v != "" {
		cfg.DeploymentMode = v
	}
	if v := env["FREERANGE_LICENSING_SELF_HOSTED_LICENSE_KEY"]; v != "" {
		cfg.LicenseKey = v
	}
	if v := env["FREERANGE_LICENSE_VERIFY_URL"]; v != "" {
		cfg.LicenseVerifyURL = v
	}
	if v := env["FREERANGE_SERVER_BINARY_URL"]; v != "" {
		cfg.ServerBinaryURL = v
	}
	if v := env["FREERANGE_WORKER_BINARY_URL"]; v != "" {
		cfg.WorkerBinaryURL = v
	}
}

func promptInstallConfig(cfg *installConfig) error {
	reader := bufio.NewReader(os.Stdin)

	cfg.InstallDir = promptValue(reader, "Install directory", cfg.InstallDir)
	cfg.DeploymentMode = promptValue(reader, "Deployment mode (hosted/self_hosted)", cfg.DeploymentMode)
	cfg.Elasticsearch = promptValue(reader, "Elasticsearch URL", cfg.Elasticsearch)
	cfg.ElasticsearchUsername = promptValue(reader, "Elasticsearch username (optional)", cfg.ElasticsearchUsername)
	cfg.ElasticsearchPassword = promptValue(reader, "Elasticsearch password (optional)", cfg.ElasticsearchPassword)
	cfg.RedisHost = promptValue(reader, "Redis host", cfg.RedisHost)
	cfg.RedisPort = promptValue(reader, "Redis port", cfg.RedisPort)
	cfg.RedisPassword = promptValue(reader, "Redis password (optional)", cfg.RedisPassword)
	cfg.ServerPort = promptValue(reader, "Server port", cfg.ServerPort)

	if cfg.DeploymentMode == "self_hosted" && cfg.LicenseKey == "" {
		cfg.LicenseKey = promptValue(reader, "Self-hosted license key (optional now, can attach later)", "")
		cfg.LicenseVerifyURL = promptValue(reader, "License verify URL", cfg.LicenseVerifyURL)
		cfg.ServerBinaryURL = promptValue(reader, "Server binary URL", cfg.ServerBinaryURL)
		cfg.WorkerBinaryURL = promptValue(reader, "Worker binary URL", cfg.WorkerBinaryURL)
	}

	confirm := strings.ToLower(promptValue(reader, "Proceed with installation? (yes/no)", "yes"))
	if confirm != "yes" && confirm != "y" {
		return errors.New("installation aborted by user")
	}

	return nil
}

func promptValue(reader *bufio.Reader, label, def string) string {
	if def != "" {
		fmt.Fprintf(os.Stdout, "%s [%s]: ", label, def)
	} else {
		fmt.Fprintf(os.Stdout, "%s: ", label)
	}
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}

func runPreflightChecks(cfg installConfig) error {
	if err := checkCommand("docker", "version", "--format", "{{.Server.Version}}"); err != nil {
		return fmt.Errorf("docker preflight failed: %w", err)
	}
	if err := checkCommand("docker", "compose", "version"); err != nil {
		return fmt.Errorf("docker compose preflight failed: %w", err)
	}

	if err := checkElasticsearch(cfg.Elasticsearch, cfg.ElasticsearchUsername, cfg.ElasticsearchPassword); err != nil {
		return fmt.Errorf("elasticsearch preflight failed: %w", err)
	}

	if cfg.RedisHost != "redis" {
		if err := checkRedis(cfg.RedisHost, cfg.RedisPort); err != nil {
			return fmt.Errorf("redis preflight failed: %w", err)
		}
	} else {
		fmt.Fprintln(os.Stdout, "Skipping redis host preflight for internal compose host 'redis'")
	}

	fmt.Fprintln(os.Stdout, "Preflight checks passed")
	return nil
}

func checkCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func checkElasticsearch(rawURL, username, password string) error {
	if strings.TrimSpace(rawURL) == "" {
		return errors.New("empty URL")
	}

	parts := strings.Split(rawURL, ",")
	u := strings.TrimSpace(parts[0])
	parsed, err := url.Parse(u)
	if err != nil {
		return fmt.Errorf("invalid URL %q: %w", u, err)
	}
	if parsed.Scheme == "" {
		return fmt.Errorf("invalid URL %q: missing scheme", u)
	}

	target := strings.TrimRight(u, "/") + "/_cluster/health"
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	if strings.TrimSpace(username) != "" {
		req.SetBasicAuth(username, password)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}
	return nil
}

func checkRedis(host, port string) error {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 3*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	return nil
}

func ensureComposeFile(installDir string) (string, error) {
	target := filepath.Join(installDir, "docker-compose.yml")
	if _, err := os.Stat(target); err == nil {
		return target, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get cwd: %w", err)
	}

	source := filepath.Join(cwd, "docker-compose.yml")
	raw, err := os.ReadFile(source)
	if err != nil {
		return "", fmt.Errorf("docker-compose.yml not found in install dir or cwd")
	}

	if err := os.WriteFile(target, raw, 0644); err != nil {
		return "", fmt.Errorf("write compose file: %w", err)
	}
	return target, nil
}

func writeInstallEnv(cfg installConfig) error {
	path := filepath.Join(cfg.InstallDir, ".env")
	merged := map[string]string{}

	if _, err := os.Stat(path); err == nil {
		existing, readErr := readEnvFile(path)
		if readErr != nil {
			return fmt.Errorf("read existing .env: %w", readErr)
		}
		for k, v := range existing {
			merged[k] = v
		}
	}

	merged["FREERANGE_DATABASE_URLS"] = cfg.Elasticsearch
	merged["FREERANGE_DATABASE_USERNAME"] = cfg.ElasticsearchUsername
	merged["FREERANGE_DATABASE_PASSWORD"] = cfg.ElasticsearchPassword
	merged["FREERANGE_REDIS_HOST"] = cfg.RedisHost
	merged["FREERANGE_REDIS_PORT"] = cfg.RedisPort
	merged["FREERANGE_REDIS_PASSWORD"] = cfg.RedisPassword
	merged["FREERANGE_SERVER_PORT"] = cfg.ServerPort
	if cfg.DeploymentMode == "self_hosted" {
		merged["FREERANGE_LICENSING_ENABLED"] = "true"
	} else if _, exists := merged["FREERANGE_LICENSING_ENABLED"]; !exists {
		merged["FREERANGE_LICENSING_ENABLED"] = "false"
	}
	merged["FREERANGE_LICENSING_DEPLOYMENT_MODE"] = cfg.DeploymentMode
	if cfg.LicenseKey != "" {
		merged["FREERANGE_LICENSING_SELF_HOSTED_LICENSE_KEY"] = cfg.LicenseKey
	}

	return writeEnvFile(path, merged)
}

func patchInstallLicensingConfig(cfg installConfig) error {
	configPath := filepath.Join(cfg.InstallDir, "config", "config.prod.yaml")
	if _, err := os.Stat(configPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(os.Stdout, "Skipping config license patch: config/config.prod.yaml not found in install dir")
			return nil
		}
		return fmt.Errorf("stat config.prod.yaml: %w", err)
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("read config.prod.yaml: %w", err)
	}

	var doc map[string]interface{}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("parse config.prod.yaml: %w", err)
	}

	if err := patchLicensingConfig(doc, cfg.DeploymentMode, cfg.LicenseKey); err != nil {
		return err
	}

	out, err := yaml.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshal config.prod.yaml: %w", err)
	}

	if err := os.WriteFile(configPath, out, 0600); err != nil {
		return fmt.Errorf("write config.prod.yaml: %w", err)
	}

	return nil
}

func startCompose(installDir, composePath string) error {
	cmd := exec.Command("docker", "compose", "-f", composePath, "up", "-d")
	cmd.Dir = installDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker compose up failed: %w", err)
	}
	return nil
}

func readEnvFile(path string) (map[string]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			continue
		}
		result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return result, nil
}

func writeEnvFile(path string, values map[string]string) error {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, key := range keys {
		b.WriteString(key)
		b.WriteString("=")
		b.WriteString(values[key])
		b.WriteString("\n")
	}

	return os.WriteFile(path, []byte(b.String()), 0644)
}
