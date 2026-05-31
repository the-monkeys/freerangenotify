package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

func newAdminBillingCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "billing",
		Short: "Manage billing controls",
	}
	cmd.AddCommand(newAdminBillingRatesCmd())
	cmd.AddCommand(newAdminBillingPlansCmd())
	return cmd
}

func newAdminBillingRatesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rates",
		Short: "Manage billing rate cards",
	}
	cmd.AddCommand(newAdminBillingRatesShowCmd())
	cmd.AddCommand(newAdminBillingRatesSetCmd())
	cmd.AddCommand(newAdminBillingRatesActivateCmd())
	cmd.AddCommand(newAdminBillingRatesRollbackCmd())
	return cmd
}

func newAdminBillingRatesShowCmd() *cobra.Command {
	var apiURL, adminToken string
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show active billing rate card",
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

			resp, err := doJSONRequest(http.MethodGet, cfg.APIURL+"/v1/admin/billing/rates", nil, map[string]string{
				"Authorization": "Bearer " + cfg.AdminToken,
			})
			if err != nil {
				return err
			}
			return printJSON(resp)
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", "", "API base URL (env: FREERANGE_API_URL)")
	cmd.Flags().StringVar(&adminToken, "admin-token", "", "Admin JWT token (env: FREERANGE_ADMIN_TOKEN)")
	return cmd
}

func newAdminBillingRatesSetCmd() *cobra.Command {
	var apiURL, adminToken, channel string
	var credits int64

	cmd := &cobra.Command{
		Use:   "set",
		Short: "Create and activate a new rate-card version with updated channel credits",
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
			if channel == "" || credits <= 0 {
				return fmt.Errorf("--channel and --credits (>0) are required")
			}

			resp, err := doJSONRequest(http.MethodPost, cfg.APIURL+"/v1/admin/billing/rates/set", map[string]interface{}{
				"channel": channel,
				"credits": credits,
			}, map[string]string{
				"Authorization": "Bearer " + cfg.AdminToken,
			})
			if err != nil {
				return err
			}
			fmt.Fprintln(os.Stdout, "Billing rates updated and activated")
			return printJSON(resp)
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", "", "API base URL (env: FREERANGE_API_URL)")
	cmd.Flags().StringVar(&adminToken, "admin-token", "", "Admin JWT token (env: FREERANGE_ADMIN_TOKEN)")
	cmd.Flags().StringVar(&channel, "channel", "", "Billing channel (email|sms|whatsapp|inapp|webhook|sse)")
	cmd.Flags().Int64Var(&credits, "credits", 0, "Credits per message/event for channel")
	_ = cmd.MarkFlagRequired("channel")
	_ = cmd.MarkFlagRequired("credits")
	return cmd
}

func newAdminBillingRatesActivateCmd() *cobra.Command {
	var apiURL, adminToken, version string

	cmd := &cobra.Command{
		Use:   "activate",
		Short: "Activate an existing billing rate-card version",
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
			if version == "" {
				return fmt.Errorf("--version is required")
			}

			resp, err := doJSONRequest(http.MethodPost, cfg.APIURL+"/v1/admin/billing/rates/activate", map[string]interface{}{
				"version": version,
			}, map[string]string{
				"Authorization": "Bearer " + cfg.AdminToken,
			})
			if err != nil {
				return err
			}
			return printJSON(resp)
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", "", "API base URL (env: FREERANGE_API_URL)")
	cmd.Flags().StringVar(&adminToken, "admin-token", "", "Admin JWT token (env: FREERANGE_ADMIN_TOKEN)")
	cmd.Flags().StringVar(&version, "version", "", "Rate-card version to activate")
	_ = cmd.MarkFlagRequired("version")
	return cmd
}

func newAdminBillingRatesRollbackCmd() *cobra.Command {
	var apiURL, adminToken, version string

	cmd := &cobra.Command{
		Use:   "rollback",
		Short: "Rollback to a previous billing rate-card version",
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
			if version == "" {
				return fmt.Errorf("--version is required")
			}

			resp, err := doJSONRequest(http.MethodPost, cfg.APIURL+"/v1/admin/billing/rates/rollback", map[string]interface{}{
				"version": version,
			}, map[string]string{
				"Authorization": "Bearer " + cfg.AdminToken,
			})
			if err != nil {
				return err
			}
			return printJSON(resp)
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", "", "API base URL (env: FREERANGE_API_URL)")
	cmd.Flags().StringVar(&adminToken, "admin-token", "", "Admin JWT token (env: FREERANGE_ADMIN_TOKEN)")
	cmd.Flags().StringVar(&version, "version", "", "Previous rate-card version to restore")
	_ = cmd.MarkFlagRequired("version")
	return cmd
}

func newAdminBillingPlansCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plans",
		Short: "Manage billing plan bundles",
	}
	cmd.AddCommand(newAdminBillingPlansSetCmd())
	return cmd
}

func newAdminBillingPlansSetCmd() *cobra.Command {
	var apiURL, adminToken, id, name, currency string
	var amountPaisa, creditsIncluded int64
	var validityDays, displayOrder int
	var active bool

	cmd := &cobra.Command{
		Use:   "set",
		Short: "Create or update a billing plan bundle in the active rate card",
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
			if id == "" {
				return fmt.Errorf("--id is required")
			}

			payload := map[string]interface{}{
				"id":               id,
				"name":             name,
				"amount_paisa":     amountPaisa,
				"currency":         currency,
				"credits_included": creditsIncluded,
				"validity_days":    validityDays,
				"active":           active,
				"display_order":    displayOrder,
			}

			resp, err := doJSONRequest(http.MethodPost, cfg.APIURL+"/v1/admin/billing/plans/set", payload, map[string]string{
				"Authorization": "Bearer " + cfg.AdminToken,
			})
			if err != nil {
				return err
			}
			fmt.Fprintln(os.Stdout, "Billing plan bundle updated")
			return printJSON(resp)
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", "", "API base URL (env: FREERANGE_API_URL)")
	cmd.Flags().StringVar(&adminToken, "admin-token", "", "Admin JWT token (env: FREERANGE_ADMIN_TOKEN)")
	cmd.Flags().StringVar(&id, "id", "", "Plan ID (e.g. lite, starter, pro)")
	cmd.Flags().StringVar(&name, "name", "", "Display name of the plan")
	cmd.Flags().Int64Var(&amountPaisa, "amount-paisa", 0, "Price in paisa (1 INR = 100 paisa)")
	cmd.Flags().StringVar(&currency, "currency", "INR", "Currency code (default INR)")
	cmd.Flags().Int64Var(&creditsIncluded, "credits-included", 0, "Credits included in the plan")
	cmd.Flags().IntVar(&validityDays, "validity-days", 365, "Validity in days (default 365)")
	cmd.Flags().BoolVar(&active, "active", true, "Whether the plan is active")
	cmd.Flags().IntVar(&displayOrder, "display-order", 0, "Display order sequence")

	_ = cmd.MarkFlagRequired("id")
	return cmd
}
