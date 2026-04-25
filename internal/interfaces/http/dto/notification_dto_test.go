package dto

import (
	"testing"

	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
)

// TestSendNotificationRequest_RichFieldsPropagation verifies that all rich
// webhook content fields survive the DTO -> domain conversion. Regression
// guard against the bug where Attachments / Actions / Fields / Mentions /
// Poll / Style were silently dropped because the DTO struct lacked them.
func TestSendNotificationRequest_RichFieldsPropagation(t *testing.T) {
	r := &SendNotificationRequest{
		UserID:     "user-1",
		Channel:    "webhook",
		Priority:   "high",
		TemplateID: "tpl-1",
		Title:      "T",
		Body:       "B",
		Attachments: []notification.Attachment{
			{Type: "image", URL: "https://cdn.example.com/i.png", Name: "i"},
		},
		Actions: []notification.Action{
			{Type: "link", Label: "Open", URL: "https://example.com/open"},
		},
		Fields: []notification.Field{
			{Key: "Host", Value: "web-01", Inline: true},
		},
		Mentions: []notification.Mention{
			{Platform: "discord", PlatformID: "1", Display: "ops"},
		},
		Poll: &notification.Poll{
			Question: "Page on-call?",
			Choices:  []notification.PollChoice{{Label: "Yes"}, {Label: "No"}},
		},
		Style: &notification.Style{Color: "#ff0000", Severity: "critical"},
	}

	req := r.ToSendRequest("app-1")

	if len(req.Attachments) != 1 || req.Attachments[0].URL != "https://cdn.example.com/i.png" {
		t.Errorf("attachments not propagated: %+v", req.Attachments)
	}
	if len(req.Actions) != 1 || req.Actions[0].Label != "Open" {
		t.Errorf("actions not propagated: %+v", req.Actions)
	}
	if len(req.Fields) != 1 || req.Fields[0].Key != "Host" {
		t.Errorf("fields not propagated: %+v", req.Fields)
	}
	if len(req.Mentions) != 1 || req.Mentions[0].PlatformID != "1" {
		t.Errorf("mentions not propagated: %+v", req.Mentions)
	}
	if req.Poll == nil || req.Poll.Question != "Page on-call?" {
		t.Errorf("poll not propagated: %+v", req.Poll)
	}
	if req.Style == nil || req.Style.Color != "#ff0000" {
		t.Errorf("style not propagated: %+v", req.Style)
	}
}

func TestBulkSendNotificationRequest_RichFieldsPropagation(t *testing.T) {
	r := &BulkSendNotificationRequest{
		UserIDs:     []string{"u1"},
		Channel:     "webhook",
		Priority:    "high",
		TemplateID:  "tpl-1",
		Attachments: []notification.Attachment{{Type: "image", URL: "https://x"}},
		Poll:        &notification.Poll{Question: "?"},
	}

	req := r.ToBulkSendRequest("app-1")

	if len(req.Attachments) != 1 {
		t.Errorf("attachments lost in bulk send: %+v", req.Attachments)
	}
	if req.Poll == nil {
		t.Errorf("poll lost in bulk send")
	}
}

func TestBroadcastNotificationRequest_RichFieldsPropagation(t *testing.T) {
	r := &BroadcastNotificationRequest{
		Channel:    "webhook",
		Priority:   "high",
		TemplateID: "tpl-1",
		Style:      &notification.Style{Severity: "warning"},
		Fields:     []notification.Field{{Key: "k", Value: "v"}},
	}

	req := r.ToBroadcastRequest("app-1")

	if req.Style == nil || req.Style.Severity != "warning" {
		t.Errorf("style lost in broadcast: %+v", req.Style)
	}
	if len(req.Fields) != 1 {
		t.Errorf("fields lost in broadcast: %+v", req.Fields)
	}
}
