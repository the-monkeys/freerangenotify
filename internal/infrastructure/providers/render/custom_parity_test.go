package render

import (
	"encoding/json"
	"testing"

	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
)

// TestCustomDiscordParity guards against the regression that originally caused
// rich-content webhooks (Attachments, Actions, Fields, Mentions, Poll, Style)
// to be silently dropped when delivered through the custom-provider path.
//
// The two payload builders MUST emit byte-identical JSON for the same
// notification so that customers configuring Discord through either the
// dedicated discord channel or via Settings.CustomProviders see the same
// behavior.
func TestCustomDiscordParity(t *testing.T) {
	notif := newRichNotification(notification.ChannelWebhook)

	want, err := json.Marshal(BuildDiscordPayload(notif))
	if err != nil {
		t.Fatalf("marshal discord: %v", err)
	}
	got, err := json.Marshal(BuildCustomDiscordPayload(notif))
	if err != nil {
		t.Fatalf("marshal custom discord: %v", err)
	}
	if string(want) != string(got) {
		t.Fatalf("custom discord payload diverged from discord payload\nwant: %s\ngot:  %s", want, got)
	}
}

// TestCustomSlackParity guards against the same regression for Slack.
func TestCustomSlackParity(t *testing.T) {
	notif := newRichNotification(notification.ChannelWebhook)

	want, err := json.Marshal(BuildSlackPayload(notif))
	if err != nil {
		t.Fatalf("marshal slack: %v", err)
	}
	got, err := json.Marshal(BuildCustomSlackPayload(notif))
	if err != nil {
		t.Fatalf("marshal custom slack: %v", err)
	}
	if string(want) != string(got) {
		t.Fatalf("custom slack payload diverged from slack payload\nwant: %s\ngot:  %s", want, got)
	}
}

// newRichNotification returns a Notification populated with every rich-content
// field so renderer parity tests exercise the full surface area.
func newRichNotification(ch notification.Channel) *notification.Notification {
	return &notification.Notification{
		NotificationID: "notif-rich-1",
		AppID:          "app-1",
		UserID:         "user-1",
		Channel:        ch,
		Priority:       notification.PriorityHigh,
		Status:         notification.StatusPending,
		TemplateID:     "tpl-rich",
		Content: notification.Content{
			Title: "Production Alert",
			Body:  "Disk usage exceeded 90% on web-01",
			Data: map[string]interface{}{
				"action_label": "View",
				"action_url":   "https://dash.example.com/alerts/42",
			},
			MediaURL: "https://cdn.example.com/banner.png",
			Attachments: []notification.Attachment{
				{URL: "https://cdn.example.com/graph.png", Type: "image", Name: "graph"},
			},
			Actions: []notification.Action{
				{Type: "link", Label: "Acknowledge", URL: "https://dash.example.com/ack/42"},
				{Type: "link", Label: "Silence", URL: "https://dash.example.com/silence/42"},
			},
			Fields: []notification.Field{
				{Key: "Host", Value: "web-01", Inline: true},
				{Key: "Region", Value: "us-east-1", Inline: true},
				{Key: "Severity", Value: "critical"},
			},
			Mentions: []notification.Mention{
				{Platform: "discord", PlatformID: "1234567890", Display: "oncall"},
			},
			Poll: &notification.Poll{
				Question: "Page on-call?",
				Choices: []notification.PollChoice{
					{Label: "Yes"},
					{Label: "No"},
				},
			},
			Style: &notification.Style{
				Color:    "#ff0000",
				Severity: "critical",
			},
		},
	}
}
