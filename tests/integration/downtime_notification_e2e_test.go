//go:build integration
// +build integration

package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// This E2E test simulates worker downtime across a scheduled notification.
// It is opt-in to avoid interfering with running environments:
//   - Requires env FREERANGE_API_KEY, FREERANGE_APP_ID, FREERANGE_USER_ID
//   - Set RUN_DOWNTIME_E2E=1 to run
//   - Assumes docker compose service names from docker-compose.yml
func TestDowntimeScheduledNotificationE2E(t *testing.T) {
	if os.Getenv("RUN_DOWNTIME_E2E") != "1" {
		t.Skip("set RUN_DOWNTIME_E2E=1 to run")
	}

	apiKey := os.Getenv("FREERANGE_API_KEY")
	appID := os.Getenv("FREERANGE_APP_ID")
	userID := os.Getenv("FREERANGE_USER_ID")
	baseURL := os.Getenv("FREERANGE_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	require.NotEmpty(t, apiKey, "FREERANGE_API_KEY is required")
	require.NotEmpty(t, appID, "FREERANGE_APP_ID is required")
	require.NotEmpty(t, userID, "FREERANGE_USER_ID is required")

	composeFile := os.Getenv("FREERANGE_COMPOSE_FILE")
	if composeFile == "" {
		composeFile = "../../docker-compose.yml"
	}

	stopStart := func(cmd string) {
		c := exec.Command("docker", "compose", "-f", composeFile, cmd, "notification-worker")
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		require.NoError(t, c.Run(), "docker compose %s", cmd)
	}

	// Stop worker to simulate downtime before scheduled time
	stopStart("stop")

	scheduledAt := time.Now().Add(30 * time.Second).UTC()
	payload := map[string]any{
		"user_id":      userID,
		"app_id":       appID,
		"channel":      "email",
		"title":        "downtime test",
		"body":         "should send after recovery",
		"scheduled_at": scheduledAt.Format(time.RFC3339),
	}
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", baseURL+"/v1/notifications", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var respBody struct {
		NotificationID string `json:"notification_id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&respBody)
	resp.Body.Close()
	require.NotEmpty(t, respBody.NotificationID)

	// Wait past scheduled time while worker is down
	time.Sleep(45 * time.Second)

	// Restart worker
	stopStart("start")

	// Poll for status to move to queued/sent
	statusURL := baseURL + "/v1/notifications/" + respBody.NotificationID
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(5 * time.Second)
		r, err := http.NewRequest("GET", statusURL, nil)
		require.NoError(t, err)
		r.Header.Set("X-API-Key", apiKey)
		res, err := client.Do(r)
		if err != nil {
			continue
		}
		if res.StatusCode != http.StatusOK {
			res.Body.Close()
			continue
		}
		var body struct {
			Status string `json:"status"`
		}
		_ = json.NewDecoder(res.Body).Decode(&body)
		res.Body.Close()
		if body.Status == "queued" || body.Status == "sent" || body.Status == "delivered" {
			return
		}
	}
	t.Fatalf("notification did not progress after worker restart")
}
