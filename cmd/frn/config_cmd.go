package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	var setAPIURL, setAPIKey, setAdminToken, setOpsSecret string
	var show bool

	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage CLI configuration",
		Long:  `Get or set API URL and API key. Stored in ~/.frn/config.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig()

			if show {
				fmt.Fprintf(os.Stdout, "Config file: %s\n", ConfigPath())
				fmt.Fprintf(os.Stdout, "API URL: %s\n", maskEmpty(cfg.APIURL, "(not set)"))
				fmt.Fprintf(os.Stdout, "API Key: %s\n", maskSecret(cfg.APIKey))
				fmt.Fprintf(os.Stdout, "Admin Token: %s\n", maskSecret(cfg.AdminToken))
				fmt.Fprintf(os.Stdout, "Ops Secret: %s\n", maskSecret(cfg.OpsSecret))
				return nil
			}

			modified := false
			if setAPIURL != "" {
				cfg.APIURL = setAPIURL
				modified = true
			}
			if setAPIKey != "" {
				cfg.APIKey = setAPIKey
				modified = true
			}
			if setAdminToken != "" {
				cfg.AdminToken = setAdminToken
				modified = true
			}
			if setOpsSecret != "" {
				cfg.OpsSecret = setOpsSecret
				modified = true
			}

			if !modified {
				// Show current config
				cfg.APIURL = maskEmpty(cfg.APIURL, "(not set)")
				cfg.APIKey = maskSecret(cfg.APIKey)
				cfg.AdminToken = maskSecret(cfg.AdminToken)
				cfg.OpsSecret = maskSecret(cfg.OpsSecret)
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(cfg)
			}

			if err := SaveConfig(cfg); err != nil {
				return err
			}
			fmt.Fprintln(os.Stdout, "Config saved to", ConfigPath())
			return nil
		},
	}

	cmd.Flags().BoolVar(&show, "show", false, "Show config with file path")
	cmd.Flags().StringVar(&setAPIURL, "set-api-url", "", "Set API URL")
	cmd.Flags().StringVar(&setAPIKey, "set-api-key", "", "Set API key")
	cmd.Flags().StringVar(&setAdminToken, "set-admin-token", "", "Set admin JWT token")
	cmd.Flags().StringVar(&setOpsSecret, "set-ops-secret", "", "Set ops secret for /v1/ops routes")
	return cmd
}

func maskEmpty(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func maskSecret(s string) string {
	if s == "" {
		return "(not set)"
	}
	if len(s) <= 8 {
		return "***"
	}
	return s[:4] + "..." + s[len(s)-4:]
}
