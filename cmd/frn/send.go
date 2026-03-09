package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

func newSendCmd() *cobra.Command {
	var apiURL, apiKey string
	var to, channel, template, subject, body, priority string
	var dataJSON string

	cmd := &cobra.Command{
		Use:   "send",
		Short: "Send a test notification",
		Long: `Send a notification using the quick-send API.
Requires FREERANGE_API_KEY or --api-key. Uses FREERANGE_API_URL or --api-url (default: http://localhost:8080).`,
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
			if to == "" {
				return fmt.Errorf("recipient required: use --to <email|user_id>")
			}

			req := map[string]interface{}{
				"to": to,
			}
			if channel != "" {
				req["channel"] = channel
			}
			if template != "" {
				req["template"] = template
			}
			if subject != "" {
				req["subject"] = subject
			}
			if body != "" {
				req["body"] = body
			}
			if priority != "" {
				req["priority"] = priority
			}
			if dataJSON != "" {
				var data map[string]interface{}
				if err := json.Unmarshal([]byte(dataJSON), &data); err != nil {
					return fmt.Errorf("invalid --data JSON: %w", err)
				}
				req["data"] = data
			}

			bodyBytes, _ := json.Marshal(req)
			httpReq, err := http.NewRequest(http.MethodPost, cfg.APIURL+"/v1/quick-send", bytes.NewReader(bodyBytes))
			if err != nil {
				return err
			}
			httpReq.Header.Set("Content-Type", "application/json")
			httpReq.Header.Set("X-API-Key", cfg.APIKey)

			resp, err := http.DefaultClient.Do(httpReq)
			if err != nil {
				return fmt.Errorf("request failed: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
				var errBody map[string]interface{}
				_ = json.NewDecoder(resp.Body).Decode(&errBody)
				return fmt.Errorf("API error %d: %v", resp.StatusCode, errBody)
			}

			var result map[string]interface{}
			if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
				return err
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		},
	}

	cmd.Flags().StringVar(&apiURL, "api-url", "", "API base URL (env: FREERANGE_API_URL)")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "Application API key (env: FREERANGE_API_KEY)")
	cmd.Flags().StringVarP(&to, "to", "t", "", "Recipient (email or user ID)")
	cmd.Flags().StringVarP(&channel, "channel", "c", "email", "Channel: push, email, sms, webhook, in_app, sse")
	cmd.Flags().StringVar(&template, "template", "", "Template name or UUID")
	cmd.Flags().StringVarP(&subject, "subject", "s", "", "Subject (inline content)")
	cmd.Flags().StringVarP(&body, "body", "b", "", "Body (inline content)")
	cmd.Flags().StringVarP(&priority, "priority", "p", "normal", "Priority: low, normal, high, critical")
	cmd.Flags().StringVar(&dataJSON, "data", "{}", "Template variables as JSON object")

	return cmd
}
