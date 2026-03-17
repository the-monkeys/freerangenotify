package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newAdminCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "Run privileged operational commands",
		Long:  "Operational commands for hosted deployments using the ops auth plane (/v1/ops/*).",
	}

	cmd.AddCommand(newAdminRenewLicenseCmd())
	cmd.AddCommand(newAdminDeleteAccountCmd())

	return cmd
}

func newAdminRenewLicenseCmd() *cobra.Command {
	var apiURL, opsSecret string
	var userID, tenantID, appID, plan, reason string
	var months int

	cmd := &cobra.Command{
		Use:   "renew-license",
		Short: "Renew or create a subscription for a user/tenant",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig()
			if apiURL != "" {
				cfg.APIURL = apiURL
			}
			if opsSecret != "" {
				cfg.OpsSecret = opsSecret
			}
			if cfg.APIURL == "" {
				cfg.APIURL = "http://localhost:8080"
			}
			if cfg.OpsSecret == "" {
				return fmt.Errorf("ops secret required: set FREERANGE_OPS_SECRET or use --ops-secret")
			}
			if strings.TrimSpace(userID) == "" && strings.TrimSpace(tenantID) == "" && strings.TrimSpace(appID) == "" {
				return fmt.Errorf("provide at least one of --user-id, --tenant-id, or --app-id")
			}
			if months <= 0 {
				months = 1
			}

			payload := map[string]interface{}{
				"user_id":   strings.TrimSpace(userID),
				"tenant_id": strings.TrimSpace(tenantID),
				"app_id":    strings.TrimSpace(appID),
				"months":    months,
				"plan":      strings.TrimSpace(plan),
				"reason":    strings.TrimSpace(reason),
			}

			headers, hErr := buildOpsAuthHeaders(http.MethodPost, cfg.APIURL+"/v1/ops/subscriptions/renew", cfg.OpsSecret)
			if hErr != nil {
				return hErr
			}

			respBody, err := doJSONRequest(http.MethodPost, cfg.APIURL+"/v1/ops/subscriptions/renew", payload, headers)
			if err != nil {
				return err
			}
			return printJSON(respBody)
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", "", "API base URL (env: FREERANGE_API_URL)")
	cmd.Flags().StringVar(&opsSecret, "ops-secret", "", "Ops secret (env: FREERANGE_OPS_SECRET)")
	cmd.Flags().StringVar(&userID, "user-id", "", "User ID to grant/renew subscription for")
	cmd.Flags().StringVar(&tenantID, "tenant-id", "", "Tenant ID to grant/renew subscription for")
	cmd.Flags().StringVar(&appID, "app-id", "", "Application ID to grant/renew subscription for")
	cmd.Flags().IntVar(&months, "months", 1, "Number of months to extend (1-24)")
	cmd.Flags().StringVar(&plan, "plan", "ops_granted", "Plan name to set")
	cmd.Flags().StringVar(&reason, "reason", "", "Reason for renewal (recommended for audit)")

	return cmd
}

func newAdminDeleteAccountCmd() *cobra.Command {
	var apiURL, opsSecret, userID, reason string
	var confirm bool

	cmd := &cobra.Command{
		Use:   "delete-account",
		Short: "Delete a user account and all owned data",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig()
			if apiURL != "" {
				cfg.APIURL = apiURL
			}
			if opsSecret != "" {
				cfg.OpsSecret = opsSecret
			}
			if cfg.APIURL == "" {
				cfg.APIURL = "http://localhost:8080"
			}
			if cfg.OpsSecret == "" {
				return fmt.Errorf("ops secret required: set FREERANGE_OPS_SECRET or use --ops-secret")
			}
			if strings.TrimSpace(userID) == "" {
				return fmt.Errorf("--user-id is required")
			}
			if !confirm {
				return fmt.Errorf("destructive action blocked: pass --confirm to proceed")
			}

			target := fmt.Sprintf("%s/v1/ops/users/%s", cfg.APIURL, strings.TrimSpace(userID))
			if strings.TrimSpace(reason) != "" {
				target = target + "?reason=" + url.QueryEscape(strings.TrimSpace(reason))
			}

			headers, hErr := buildOpsAuthHeaders(http.MethodDelete, target, cfg.OpsSecret)
			if hErr != nil {
				return hErr
			}

			respBody, err := doJSONRequest(http.MethodDelete, target, nil, headers)
			if err != nil {
				return err
			}

			fmt.Fprintln(os.Stdout, "Account deletion completed")
			return printJSON(respBody)
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", "", "API base URL (env: FREERANGE_API_URL)")
	cmd.Flags().StringVar(&opsSecret, "ops-secret", "", "Ops secret (env: FREERANGE_OPS_SECRET)")
	cmd.Flags().StringVar(&userID, "user-id", "", "User ID to delete")
	cmd.Flags().StringVar(&reason, "reason", "", "Reason for deletion (recommended for audit)")
	cmd.Flags().BoolVar(&confirm, "confirm", false, "Confirm destructive account deletion")

	return cmd
}

func buildOpsAuthHeaders(method, targetURL, opsSecret string) (map[string]string, error) {
	parsed, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("parse target URL: %w", err)
	}

	reqURI := parsed.RequestURI()
	ts := fmt.Sprintf("%d", time.Now().UTC().Unix())
	nonce, err := generateOpsNonce()
	if err != nil {
		return nil, err
	}

	sig := signOpsRequest(opsSecret, method, reqURI, ts, nonce)
	return map[string]string{
		"Authorization":   "Bearer ops:" + opsSecret,
		"X-Ops-Timestamp": ts,
		"X-Ops-Nonce":     nonce,
		"X-Ops-Signature": sig,
	}, nil
}

func generateOpsNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func signOpsRequest(secret, method, reqURI, ts, nonce string) string {
	message := fmt.Sprintf("%s\n%s\n%s\n%s", strings.ToUpper(strings.TrimSpace(method)), strings.TrimSpace(reqURI), strings.TrimSpace(ts), strings.TrimSpace(nonce))
	mac := hmac.New(sha256.New, []byte(strings.TrimSpace(secret)))
	_, _ = mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}
