package render

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
)

var updateGoldenBaseline = flag.Bool("update-golden-baseline", false, "update render baseline golden files")

func TestRendererBaselines(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	notif := &notification.Notification{
		NotificationID: "notif-1",
		AppID:          "app-1",
		UserID:         "user-1",
		Channel:        notification.ChannelWebhook,
		Priority:       notification.PriorityNormal,
		Status:         notification.StatusPending,
		TemplateID:     "tpl-1",
		Content: notification.Content{
			Title: "Title",
			Body:  "Body",
			Data: map[string]interface{}{
				"action_label": "Open",
				"action_url":   "https://example.com/open",
			},
		},
		Metadata: map[string]interface{}{
			"k":           "v",
			"webhook_url": "https://example.com/hook",
		},
		Category:  "ops",
		CreatedAt: ts,
	}

	usr := &user.User{
		Email:      "u@example.com",
		Phone:      "+123",
		ExternalID: "ext-1",
		Timezone:   "UTC",
		Language:   "en",
	}

	assertGoldenJSON(t, "generic_webhook_legacy.json", BuildGenericWebhookPayload(notif))
	assertGoldenJSON(t, "discord_legacy.json", BuildDiscordPayload(notif))
	assertGoldenJSON(t, "slack_legacy.json", BuildSlackPayload(notif))
	assertGoldenJSON(t, "teams_legacy.json", BuildTeamsPayload(notif, "https://outlook.webhook.office.com/webhookb2/abc"))
	assertGoldenJSON(t, "teams_workflow.json", BuildTeamsPayload(notif, "https://prod-12.westus.logic.azure.com:443/workflows/abc/triggers/manual"))
	assertGoldenJSON(t, "custom_generic_legacy.json", BuildCustomStandardPayload(notif, notification.ChannelWebhook, usr))
	assertGoldenJSON(t, "custom_discord_legacy.json", BuildCustomDiscordPayload(notif))
	assertGoldenJSON(t, "custom_slack_legacy.json", BuildCustomSlackPayload(notif))
	assertGoldenJSON(t, "custom_teams_workflow.json", BuildTeamsPayload(notif, "https://prod-12.westus.logic.azure.com:443/workflows/abc/triggers/manual"))
}

func assertGoldenJSON(t *testing.T, name string, value interface{}) {
	t.Helper()

	got, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal %s: %v", name, err)
	}

	goldenPath := filepath.Join("testdata", "baseline", name)
	if *updateGoldenBaseline {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("mkdir baseline dir: %v", err)
		}
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatalf("write golden %s: %v", name, err)
		}
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden %s: %v", name, err)
	}

	if string(got) != string(want) {
		t.Fatalf("golden mismatch for %s\nwant: %s\ngot:  %s", name, string(want), string(got))
	}
}
