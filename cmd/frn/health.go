package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

func newHealthCmd() *cobra.Command {
	var apiURL string

	cmd := &cobra.Command{
		Use:   "health",
		Short: "Check API health",
		Long:  `Check if the FreeRangeNotify API is reachable and healthy.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig()
			if apiURL != "" {
				cfg.APIURL = apiURL
			}
			if cfg.APIURL == "" {
				cfg.APIURL = "http://localhost:8080"
			}

			resp, err := http.Get(cfg.APIURL + "/v1/health")
			if err != nil {
				return fmt.Errorf("request failed: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("API unhealthy: status %d", resp.StatusCode)
			}

			fmt.Fprintln(os.Stdout, "API is healthy")
			return nil
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", "", "API base URL (env: FREERANGE_API_URL)")
	return cmd
}
