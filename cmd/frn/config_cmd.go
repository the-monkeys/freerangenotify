package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	var setAPIURL, setAPIKey string
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

			if !modified {
				// Show current config
				cfg.APIURL = maskEmpty(cfg.APIURL, "(not set)")
				cfg.APIKey = maskSecret(cfg.APIKey)
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
