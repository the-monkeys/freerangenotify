package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newLicenseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "license",
		Short: "Manage on-prem licensing",
		Long:  "Request, attach, verify, and inspect license status for self-hosted deployments.",
	}

	cmd.AddCommand(newLicenseStatusCmd())
	cmd.AddCommand(newLicenseRequestCmd())
	cmd.AddCommand(newLicenseAttachCmd())
	cmd.AddCommand(newLicenseVerifyCmd())
	cmd.AddCommand(newLicensePatchCmd())
	cmd.AddCommand(newSubscriptionRenewCmd())

	return cmd
}

func newLicensePatchCmd() *cobra.Command {
	var configFile string
	var licenseKey, licenseFile string
	var mode string

	cmd := &cobra.Command{
		Use:   "patch",
		Short: "Patch local config with license",
		Long:  "Patches a config YAML file to set licensing.enabled=true and inject a self-hosted license key.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(licenseKey) == "" && strings.TrimSpace(licenseFile) == "" {
				return fmt.Errorf("provide --license-key or --license-file")
			}

			resolved := configFile
			if resolved == "" {
				resolved = filepath.Join("config", "config.prod.yaml")
			}

			raw, err := os.ReadFile(resolved)
			if err != nil {
				return fmt.Errorf("read config file: %w", err)
			}

			var doc map[string]interface{}
			if err := yaml.Unmarshal(raw, &doc); err != nil {
				return fmt.Errorf("parse yaml: %w", err)
			}

			keyToWrite := strings.TrimSpace(licenseKey)
			if strings.TrimSpace(licenseFile) != "" {
				fileRaw, err := os.ReadFile(licenseFile)
				if err != nil {
					return fmt.Errorf("read license file: %w", err)
				}
				keyToWrite = strings.TrimSpace(string(fileRaw))
				if keyToWrite == "" {
					return fmt.Errorf("license file is empty")
				}
			}

			if err := patchLicensingConfig(doc, mode, keyToWrite); err != nil {
				return err
			}

			out, err := yaml.Marshal(doc)
			if err != nil {
				return fmt.Errorf("marshal yaml: %w", err)
			}

			if err := os.WriteFile(resolved, out, 0600); err != nil {
				return fmt.Errorf("write config file: %w", err)
			}

			fmt.Fprintf(os.Stdout, "Patched license in %s\n", resolved)
			fmt.Fprintln(os.Stdout, "Set licensing.enabled=true")
			fmt.Fprintf(os.Stdout, "Set licensing.deployment_mode=%s\n", mode)
			return nil
		},
	}

	cmd.Flags().StringVar(&configFile, "config-file", filepath.Join("config", "config.prod.yaml"), "Path to target config yaml")
	cmd.Flags().StringVar(&licenseKey, "license-key", "", "Signed self-hosted license key/token")
	cmd.Flags().StringVar(&licenseFile, "license-file", "", "Path to file containing the license token")
	cmd.Flags().StringVar(&mode, "deployment-mode", "self_hosted", "Deployment mode to set: hosted or self_hosted")

	return cmd
}

func patchLicensingConfig(doc map[string]interface{}, mode, licenseKey string) error {
	trimmedMode := strings.TrimSpace(mode)
	if trimmedMode != "hosted" && trimmedMode != "self_hosted" {
		return fmt.Errorf("--deployment-mode must be hosted or self_hosted")
	}

	licensing := getOrCreateMap(doc, "licensing")
	licensing["enabled"] = true
	licensing["deployment_mode"] = trimmedMode

	selfHosted := getOrCreateMap(licensing, "self_hosted")
	selfHosted["license_key"] = strings.TrimSpace(licenseKey)

	return nil
}

func getOrCreateMap(parent map[string]interface{}, key string) map[string]interface{} {
	if existing, ok := parent[key].(map[string]interface{}); ok && existing != nil {
		return existing
	}

	created := map[string]interface{}{}
	parent[key] = created
	return created
}

func newLicenseStatusCmd() *cobra.Command {
	var apiURL, apiKey string

	return &cobra.Command{
		Use:   "status",
		Short: "Get effective license status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig()
			if apiURL != "" {
				cfg.APIURL = apiURL
			}
			if apiKey != "" {
				cfg.APIKey = apiKey
			}
			if cfg.APIURL == "" {
				cfg.APIURL = "http://localhost:8080"
			}
			if cfg.APIKey == "" {
				return fmt.Errorf("API key required: set FREERANGE_API_KEY or use --api-key")
			}

			respBody, err := doJSONRequest(http.MethodGet, cfg.APIURL+"/v1/license/status", nil, map[string]string{
				"X-API-Key": cfg.APIKey,
			})
			if err != nil {
				return err
			}
			return printJSON(respBody)
		},
	}
}

func newLicenseRequestCmd() *cobra.Command {
	var apiURL, adminToken string
	var organization, contactEmail, notes string
	var timelineDays int

	cmd := &cobra.Command{
		Use:   "request",
		Short: "Generate a license request payload (on-prem)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig()
			if apiURL != "" {
				cfg.APIURL = apiURL
			}
			if adminToken != "" {
				cfg.AdminToken = adminToken
			}
			if cfg.APIURL == "" {
				cfg.APIURL = "http://localhost:8080"
			}
			if cfg.AdminToken == "" {
				return fmt.Errorf("admin token required: set FREERANGE_ADMIN_TOKEN or use --admin-token")
			}

			payload := map[string]interface{}{}
			if organization != "" {
				payload["organization"] = organization
			}
			if contactEmail != "" {
				payload["contact_email"] = contactEmail
			}
			if notes != "" {
				payload["notes"] = notes
			}
			if timelineDays > 0 {
				payload["timeline_days"] = timelineDays
			}

			respBody, err := doJSONRequest(http.MethodPost, cfg.APIURL+"/v1/admin/licensing/request", payload, map[string]string{
				"Authorization": "Bearer " + cfg.AdminToken,
			})
			if err != nil {
				return err
			}
			return printJSON(respBody)
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", "", "API base URL (env: FREERANGE_API_URL)")
	cmd.Flags().StringVar(&adminToken, "admin-token", "", "Admin JWT token (env: FREERANGE_ADMIN_TOKEN)")
	cmd.Flags().StringVar(&organization, "organization", "", "Organization name")
	cmd.Flags().StringVar(&contactEmail, "contact-email", "", "Contact email")
	cmd.Flags().StringVar(&notes, "notes", "", "Additional notes")
	cmd.Flags().IntVar(&timelineDays, "timeline-days", 30, "Requested timeline in days")

	return cmd
}

func newLicenseAttachCmd() *cobra.Command {
	var apiURL, adminToken string
	var licenseKey, licenseFile string

	cmd := &cobra.Command{
		Use:   "attach",
		Short: "Attach/activate a signed license",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig()
			if apiURL != "" {
				cfg.APIURL = apiURL
			}
			if adminToken != "" {
				cfg.AdminToken = adminToken
			}
			if cfg.APIURL == "" {
				cfg.APIURL = "http://localhost:8080"
			}
			if cfg.AdminToken == "" {
				return fmt.Errorf("admin token required: set FREERANGE_ADMIN_TOKEN or use --admin-token")
			}

			payload := map[string]interface{}{}
			if licenseKey != "" {
				payload["license_key"] = strings.TrimSpace(licenseKey)
			}
			if licenseFile != "" {
				raw, err := os.ReadFile(licenseFile)
				if err != nil {
					return fmt.Errorf("read license file: %w", err)
				}

				var filePayload map[string]interface{}
				if jsonErr := json.Unmarshal(raw, &filePayload); jsonErr == nil {
					for k, v := range filePayload {
						payload[k] = v
					}
				} else {
					payload["license_key"] = strings.TrimSpace(string(raw))
				}
			}

			if len(payload) == 0 {
				return fmt.Errorf("provide --license-key or --license-file")
			}

			respBody, err := doJSONRequest(http.MethodPost, cfg.APIURL+"/v1/admin/licensing/activate", payload, map[string]string{
				"Authorization": "Bearer " + cfg.AdminToken,
			})
			if err != nil {
				return err
			}

			fmt.Fprintln(os.Stdout, "License attached successfully")
			return printJSON(respBody)
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", "", "API base URL (env: FREERANGE_API_URL)")
	cmd.Flags().StringVar(&adminToken, "admin-token", "", "Admin JWT token (env: FREERANGE_ADMIN_TOKEN)")
	cmd.Flags().StringVar(&licenseKey, "license-key", "", "Signed license key/token")
	cmd.Flags().StringVar(&licenseFile, "license-file", "", "Path to license file (raw token or JSON payload)")

	return cmd
}

func newLicenseVerifyCmd() *cobra.Command {
	var apiURL, adminToken string

	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Force license verification refresh",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig()
			if apiURL != "" {
				cfg.APIURL = apiURL
			}
			if adminToken != "" {
				cfg.AdminToken = adminToken
			}
			if cfg.APIURL == "" {
				cfg.APIURL = "http://localhost:8080"
			}
			if cfg.AdminToken == "" {
				return fmt.Errorf("admin token required: set FREERANGE_ADMIN_TOKEN or use --admin-token")
			}

			respBody, err := doJSONRequest(http.MethodPost, cfg.APIURL+"/v1/admin/licensing/validate", map[string]interface{}{}, map[string]string{
				"Authorization": "Bearer " + cfg.AdminToken,
			})
			if err != nil {
				return err
			}
			return printJSON(respBody)
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", "", "API base URL (env: FREERANGE_API_URL)")
	cmd.Flags().StringVar(&adminToken, "admin-token", "", "Admin JWT token (env: FREERANGE_ADMIN_TOKEN)")

	return cmd
}

// newSubscriptionRenewCmd creates the `frn license renew` subcommand.
// Usage: frn license renew --subscription-id <id> --months 1 --reason "sponsored"
func newSubscriptionRenewCmd() *cobra.Command {
	var apiURL, adminToken string
	var subscriptionID, plan, reason string
	var months int

	cmd := &cobra.Command{
		Use:   "renew",
		Short: "Renew a subscription without payment (admin)",
		Long:  "Renews a subscription for the specified number of months. No payment required. Used for sponsored/free renewals.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig()
			if apiURL != "" {
				cfg.APIURL = apiURL
			}
			if adminToken != "" {
				cfg.AdminToken = adminToken
			}
			if cfg.APIURL == "" {
				cfg.APIURL = "http://localhost:8080"
			}
			if cfg.AdminToken == "" {
				return fmt.Errorf("admin token required: set FREERANGE_ADMIN_TOKEN or use --admin-token")
			}
			if subscriptionID == "" {
				return fmt.Errorf("--subscription-id is required")
			}

			payload := map[string]interface{}{
				"months": months,
				"reason": reason,
			}
			if plan != "" {
				payload["plan"] = plan
			}

			respBody, err := doJSONRequest(
				http.MethodPost,
				fmt.Sprintf("%s/v1/admin/subscriptions/%s/renew", cfg.APIURL, subscriptionID),
				payload,
				map[string]string{
					"Authorization": "Bearer " + cfg.AdminToken,
				},
			)
			if err != nil {
				return err
			}

			fmt.Fprintln(os.Stdout, "Subscription renewed successfully")
			return printJSON(respBody)
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", "", "API base URL (env: FREERANGE_API_URL)")
	cmd.Flags().StringVar(&adminToken, "admin-token", "", "Admin JWT token (env: FREERANGE_ADMIN_TOKEN)")
	cmd.Flags().StringVar(&subscriptionID, "subscription-id", "", "Subscription ID to renew (required)")
	cmd.Flags().StringVar(&plan, "plan", "", "Plan tier to set (optional, keeps current if empty)")
	cmd.Flags().IntVar(&months, "months", 1, "Number of months to renew for")
	cmd.Flags().StringVar(&reason, "reason", "", "Reason for renewal (audit trail)")

	_ = cmd.MarkFlagRequired("subscription-id")

	return cmd
}

func doJSONRequest(method, targetURL string, payload interface{}, headers map[string]string) (map[string]interface{}, error) {
	var bodyReader *bytes.Reader
	if payload == nil {
		bodyReader = bytes.NewReader(nil)
	} else {
		bodyBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequest(method, targetURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if decodeErr := json.NewDecoder(resp.Body).Decode(&result); decodeErr != nil {
		return nil, fmt.Errorf("decode response: %w", decodeErr)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API error %d: %v", resp.StatusCode, result)
	}

	return result, nil
}

func printJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
