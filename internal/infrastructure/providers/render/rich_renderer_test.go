package render

import (
	"strings"
	"testing"

	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
)

// ─── Discord ───────────────────────────────────────────────────────────────

func TestBuildDiscordPayload_ColorFromStyleHex(t *testing.T) {
	notif := &notification.Notification{
		Content: notification.Content{
			Title: "T",
			Body:  "B",
			Style: &notification.Style{Color: "#ff0000"},
		},
	}
	p := BuildDiscordPayload(notif)
	embeds := p["embeds"].([]map[string]interface{})
	if got := embeds[0]["color"]; got != 0xff0000 {
		t.Fatalf("color: want %d, got %v", 0xff0000, got)
	}
}

func TestBuildDiscordPayload_ColorFromSeverity(t *testing.T) {
	cases := map[string]int{
		"success": 3066993,
		"warning": 15105570,
		"danger":  15158332,
		"info":    3447003,
	}
	for sev, want := range cases {
		t.Run(sev, func(t *testing.T) {
			notif := &notification.Notification{
				Content: notification.Content{
					Title: "T",
					Body:  "B",
					Style: &notification.Style{Severity: sev},
				},
			}
			p := BuildDiscordPayload(notif)
			embeds := p["embeds"].([]map[string]interface{})
			if got := embeds[0]["color"]; got != want {
				t.Fatalf("severity %q: want color %d, got %v", sev, want, got)
			}
		})
	}
}

func TestBuildDiscordPayload_PollDurationCaps(t *testing.T) {
	cases := []struct {
		name    string
		input   int
		wantDur int
	}{
		{"zero defaults to 24h", 0, 24},
		{"negative defaults to 24h", -5, 24},
		{"in-range passthrough", 48, 48},
		{"caps at 768h", 1000, 768},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			notif := &notification.Notification{
				Content: notification.Content{
					Title: "T",
					Body:  "B",
					Poll: &notification.Poll{
						Question:      "Q?",
						Choices:       []notification.PollChoice{{Label: "Yes"}},
						DurationHours: tc.input,
					},
				},
			}
			p := BuildDiscordPayload(notif)
			poll, ok := p["poll"].(map[string]interface{})
			if !ok {
				t.Fatalf("expected poll object in payload, got %T", p["poll"])
			}
			if got := poll["duration"]; got != tc.wantDur {
				t.Fatalf("duration: want %d, got %v", tc.wantDur, got)
			}
		})
	}
}

func TestBuildDiscordPayload_ActionsRenderedAsMarkdownField(t *testing.T) {
	// Discord webhooks silently drop `components`, so link actions must be
	// emitted as a markdown field — this is the contract that keeps the
	// "Test 3 Actions" use case working.
	notif := &notification.Notification{
		Content: notification.Content{
			Title: "T",
			Body:  "B",
			Actions: []notification.Action{
				{Type: "link", Label: "Open", URL: "https://example.com/open"},
				{Type: "link", Label: "Docs", URL: "https://example.com/docs"},
			},
		},
	}
	p := BuildDiscordPayload(notif)

	if _, hasComponents := p["components"]; hasComponents {
		t.Fatalf("Discord payload must not include components (silently dropped by webhooks)")
	}

	embeds := p["embeds"].([]map[string]interface{})
	fields, ok := embeds[0]["fields"].([]map[string]interface{})
	if !ok || len(fields) == 0 {
		t.Fatalf("expected at least one embed field, got %v", embeds[0]["fields"])
	}
	var actionField map[string]interface{}
	for _, f := range fields {
		if f["name"] == "Actions" {
			actionField = f
			break
		}
	}
	if actionField == nil {
		t.Fatalf("expected an embed field named %q, fields=%v", "Actions", fields)
	}
	value := actionField["value"].(string)
	if !strings.Contains(value, "[Open](https://example.com/open)") {
		t.Fatalf("missing first action markdown link: %q", value)
	}
	if !strings.Contains(value, "[Docs](https://example.com/docs)") {
		t.Fatalf("missing second action markdown link: %q", value)
	}
	if !strings.Contains(value, "•") {
		t.Fatalf("expected bullet separator between actions, got %q", value)
	}
}

func TestBuildDiscordPayload_MentionsPrependedToContent(t *testing.T) {
	notif := &notification.Notification{
		Content: notification.Content{
			Title: "Hello",
			Body:  "B",
			Mentions: []notification.Mention{
				{Platform: "discord", PlatformID: "111"},
				{Platform: "slack", PlatformID: "222"}, // wrong platform → ignored
			},
		},
	}
	p := BuildDiscordPayload(notif)
	content := p["content"].(string)
	if !strings.HasPrefix(content, "<@111>") {
		t.Fatalf("expected discord mention prefix, got %q", content)
	}
	if strings.Contains(content, "<@222>") {
		t.Fatalf("slack mention must not appear in discord payload, got %q", content)
	}
}

// ─── Slack ─────────────────────────────────────────────────────────────────

func TestBuildSlackPayload_NoStyle_TopLevelBlocks(t *testing.T) {
	notif := &notification.Notification{
		Content: notification.Content{Title: "T", Body: "B"},
	}
	p := BuildSlackPayload(notif)
	if _, hasAttach := p["attachments"]; hasAttach {
		t.Fatalf("payload without style must not wrap blocks in attachments")
	}
	blocks, ok := p["blocks"].([]map[string]interface{})
	if !ok || len(blocks) == 0 {
		t.Fatalf("expected top-level blocks, got %T %v", p["blocks"], p["blocks"])
	}
}

func TestBuildSlackPayload_StyleWrapsBlocksInColoredAttachment(t *testing.T) {
	// Regression: Slack Block Kit renders a colored sidebar bar only when
	// blocks live INSIDE attachments[].blocks with attachments[].color set.
	// A top-level `blocks` plus an empty `attachments[0].blocks` produces no
	// sidebar — that was the original bug behind the missing red bar.
	notif := &notification.Notification{
		Content: notification.Content{
			Title: "T",
			Body:  "B",
			Style: &notification.Style{Color: "#ff0000", Severity: "critical"},
		},
	}
	p := BuildSlackPayload(notif)

	if _, hasTopBlocks := p["blocks"]; hasTopBlocks {
		t.Fatalf("payload with style must not emit top-level blocks (would suppress sidebar)")
	}
	attachments, ok := p["attachments"].([]map[string]interface{})
	if !ok || len(attachments) != 1 {
		t.Fatalf("expected exactly one attachment wrapper, got %T %v", p["attachments"], p["attachments"])
	}
	att := attachments[0]
	color, _ := att["color"].(string)
	if color == "" {
		t.Fatalf("attachment must carry a color string for sidebar bar, got %v", att["color"])
	}
	innerBlocks, ok := att["blocks"].([]map[string]interface{})
	if !ok || len(innerBlocks) == 0 {
		t.Fatalf("attachment must contain the actual blocks, got %T %v", att["blocks"], att["blocks"])
	}
	if _, hasFallback := p["text"]; !hasFallback {
		t.Fatalf("payload must include top-level `text` fallback for notifications")
	}
}

func TestBuildSlackPayload_PollAsNumberedList(t *testing.T) {
	// Slack incoming webhooks have no native poll element. The renderer must
	// fall back to a numbered list rather than silently dropping the poll.
	notif := &notification.Notification{
		Content: notification.Content{
			Title: "T",
			Body:  "B",
			Poll: &notification.Poll{
				Question: "Should we deploy on Friday?",
				Choices: []notification.PollChoice{
					{Label: "Yes"},
					{Label: "No"},
					{Label: "Only if blue moon"},
				},
			},
		},
	}
	p := BuildSlackPayload(notif)

	// Find the section block whose text contains the poll question.
	blocks, _ := p["blocks"].([]map[string]interface{})
	if blocks == nil {
		// styled-attachment shape — unwrap.
		if attachments, ok := p["attachments"].([]map[string]interface{}); ok && len(attachments) == 1 {
			blocks, _ = attachments[0]["blocks"].([]map[string]interface{})
		}
	}
	if len(blocks) == 0 {
		t.Fatalf("no blocks in payload: %v", p)
	}

	var pollText string
	for _, b := range blocks {
		text, _ := b["text"].(map[string]interface{})
		if text == nil {
			continue
		}
		if s, _ := text["text"].(string); strings.Contains(s, "Should we deploy on Friday?") {
			pollText = s
			break
		}
	}
	if pollText == "" {
		t.Fatalf("poll question not rendered in any block: %v", blocks)
	}
	for _, choice := range []string{"Yes", "No", "Only if blue moon"} {
		if !strings.Contains(pollText, choice) {
			t.Fatalf("poll choice %q missing from rendered text: %q", choice, pollText)
		}
	}
}

func TestBuildSlackPayload_MentionsPrependedToFallbackText(t *testing.T) {
	notif := &notification.Notification{
		Content: notification.Content{
			Title: "Heads up",
			Body:  "B",
			Style: &notification.Style{Severity: "warning", Color: "#ff9900"},
			Mentions: []notification.Mention{
				{Platform: "slack", PlatformID: "U123"},
				{Platform: "discord", PlatformID: "999"}, // wrong platform → ignored
			},
		},
	}
	p := BuildSlackPayload(notif)
	text, _ := p["text"].(string)
	if !strings.Contains(text, "<@U123>") {
		t.Fatalf("expected slack mention in fallback text, got %q", text)
	}
	if strings.Contains(text, "<@999>") {
		t.Fatalf("discord mention must not appear in slack payload, got %q", text)
	}
}
