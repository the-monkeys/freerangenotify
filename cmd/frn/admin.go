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
	cmd.AddCommand(newAdminGrantCreditsCmd())
	cmd.AddCommand(newAdminDeleteAccountCmd())
	cmd.AddCommand(newAdminBillingCmd())
	cmd.AddCommand(newAdminRebalanceCreditsCmd())

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

func newAdminGrantCreditsCmd() *cobra.Command {
	var apiURL, opsSecret string
	var userID, reason string
	var credits int64

	cmd := &cobra.Command{
		Use:   "grant-credits",
		Short: "Grant credits to a user by user ID",
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
			if credits <= 0 {
				return fmt.Errorf("--credits must be greater than zero")
			}
			if strings.TrimSpace(reason) == "" {
				return fmt.Errorf("--reason is required")
			}

			payload := map[string]interface{}{
				"user_id": strings.TrimSpace(userID),
				"credits": credits,
				"reason":  strings.TrimSpace(reason),
			}

			target := cfg.APIURL + "/v1/ops/credits/grant"
			headers, hErr := buildOpsAuthHeaders(http.MethodPost, target, cfg.OpsSecret)
			if hErr != nil {
				return hErr
			}

			respBody, err := doJSONRequest(http.MethodPost, target, payload, headers)
			if err != nil {
				return err
			}
			fmt.Fprintln(os.Stdout, "Credits granted successfully")
			return printJSON(respBody)
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", "", "API base URL (env: FREERANGE_API_URL)")
	cmd.Flags().StringVar(&opsSecret, "ops-secret", "", "Ops secret (env: FREERANGE_OPS_SECRET)")
	cmd.Flags().StringVar(&userID, "user-id", "", "User ID to grant credits to")
	cmd.Flags().Int64Var(&credits, "credits", 0, "Number of credits to grant")
	cmd.Flags().StringVar(&reason, "reason", "", "Reason for grant (required for audit)")

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

// newAdminRebalanceCreditsCmd re-baselines existing active/trial subscriptions
// onto the active rate card. Defaults to dry-run; pass --apply to write.
//
// Each subscription is topped up so that:
//
//	credits_remaining = max(current_remaining,
//	                        new_plan.credits_included - already_consumed)
//
// where already_consumed = max(0, credits_total - credits_remaining). Already-
// migrated tenants compute delta=0 and are skipped, so re-runs are safe.
func newAdminRebalanceCreditsCmd() *cobra.Command {
	var apiURL, opsSecret, reason string
	var apply bool

	cmd := &cobra.Command{
		Use:   "rebalance-credits",
		Short: "Re-baseline existing subscriptions onto the active rate card",
		Long: "Top up active/trial subscriptions to at least the new plan's credit allotment.\n" +
			"Idempotent: dry-run by default, --apply to actually grant credits.",
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

			payload := map[string]interface{}{
				"apply":  apply,
				"reason": strings.TrimSpace(reason),
			}

			target := cfg.APIURL + "/v1/ops/billing/rebalance-credits"
			headers, hErr := buildOpsAuthHeaders(http.MethodPost, target, cfg.OpsSecret)
			if hErr != nil {
				return hErr
			}

			respBody, err := doJSONRequest(http.MethodPost, target, payload, headers)
			if err != nil {
				return err
			}
			if apply {
				fmt.Fprintln(os.Stdout, "Rebalance applied")
			} else {
				fmt.Fprintln(os.Stdout, "Rebalance dry-run (no changes written) — pass --apply to commit")
			}
			return printJSON(respBody)
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", "", "API base URL (env: FREERANGE_API_URL)")
	cmd.Flags().StringVar(&opsSecret, "ops-secret", "", "Ops secret (env: FREERANGE_OPS_SECRET)")
	cmd.Flags().BoolVar(&apply, "apply", false, "Actually grant credits (default: dry-run)")
	cmd.Flags().StringVar(&reason, "reason", "rebalance-2026 credit migration", "Reason recorded in the credit ledger")

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
