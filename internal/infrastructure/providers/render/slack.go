package render

import (
	"fmt"
	"strings"

	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
)

// BuildSlackPayload constructs a Slack Block Kit message.
// It renders all rich content fields: Attachments, Actions, Fields, Poll, Style.
func BuildSlackPayload(notif *notification.Notification) map[string]interface{} {
	c := notif.Content
	blocks := []map[string]interface{}{
		{
			"type": "section",
			"text": map[string]interface{}{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*%s*\n%s", c.Title, c.Body),
			},
		},
	}

	// --- Fields → Slack section fields ---
	if len(c.Fields) > 0 {
		fieldTexts := make([]map[string]interface{}, 0, len(c.Fields))
		for _, f := range c.Fields {
			fieldTexts = append(fieldTexts, map[string]interface{}{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*%s*\n%s", f.Key, f.Value),
			})
		}
		blocks = append(blocks, map[string]interface{}{
			"type":   "section",
			"fields": fieldTexts,
		})
	}

	// --- Attachments → image blocks + file links ---
	for _, a := range c.Attachments {
		switch a.Type {
		case "image":
			imgBlock := map[string]interface{}{
				"type":      "image",
				"image_url": a.URL,
				"alt_text":  a.AltText,
			}
			if a.Name != "" {
				imgBlock["title"] = map[string]interface{}{
					"type": "plain_text",
					"text": a.Name,
				}
			}
			blocks = append(blocks, imgBlock)
		default:
			label := a.Name
			if label == "" {
				label = a.Type
			}
			blocks = append(blocks, map[string]interface{}{
				"type": "section",
				"text": map[string]interface{}{
					"type": "mrkdwn",
					"text": fmt.Sprintf("📎 <%s|%s>", a.URL, label),
				},
			})
		}
	}

	// --- Actions → Slack button elements ---
	var buttons []map[string]interface{}

	// Legacy: action_url
	if c.Data != nil {
		if actionURL, ok := c.Data["action_url"].(string); ok && actionURL != "" {
			actionLabel := "View"
			if label, ok := c.Data["action_label"].(string); ok && label != "" {
				actionLabel = label
			}
			buttons = append(buttons, map[string]interface{}{
				"type": "button",
				"text": map[string]interface{}{
					"type": "plain_text",
					"text": actionLabel,
				},
				"url": actionURL,
			})
		}
	}

	// Rich actions
	for _, a := range c.Actions {
		if a.Type == "link" && a.URL != "" {
			btn := map[string]interface{}{
				"type": "button",
				"text": map[string]interface{}{
					"type": "plain_text",
					"text": a.Label,
				},
				"url": a.URL,
			}
			if a.Style == "danger" {
				btn["style"] = "danger"
			} else if a.Style == "primary" {
				btn["style"] = "primary"
			}
			buttons = append(buttons, btn)
		}
	}

	if len(buttons) > 0 {
		blocks = append(blocks, map[string]interface{}{
			"type":     "actions",
			"elements": buttons,
		})
	}

	// --- Poll → rendered as section with numbered options ---
	if c.Poll != nil {
		pollLines := make([]string, 0, len(c.Poll.Choices))
		for i, ch := range c.Poll.Choices {
			emoji := ch.Emoji
			if emoji == "" {
				emoji = fmt.Sprintf("%d️⃣", i+1)
			}
			pollLines = append(pollLines, fmt.Sprintf("%s %s", emoji, ch.Label))
		}
		blocks = append(blocks, map[string]interface{}{
			"type": "section",
			"text": map[string]interface{}{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*📊 %s*\n%s", c.Poll.Question, strings.Join(pollLines, "\n")),
			},
		})
	}

	// --- Mentions → prepend to fallback text ---
	fallback := c.Title
	if len(c.Mentions) > 0 {
		var parts []string
		for _, m := range c.Mentions {
			if m.Platform == "slack" && m.PlatformID != "" {
				parts = append(parts, fmt.Sprintf("<@%s>", m.PlatformID))
			}
		}
		if len(parts) > 0 {
			fallback = strings.Join(parts, " ") + " " + fallback
		}
	}

	// --- Style → wrap all blocks in a colored attachment for sidebar bar ---
	//
	// The Slack convention for a colored vertical bar on the left of a
	// message is to put the message blocks inside attachments[].blocks and
	// set attachments[].color. The previous implementation appended an
	// empty-blocks attachment alongside the top-level blocks, which produced
	// no visible color bar — Slack ignores attachments that contain no
	// renderable content. We now move the blocks into the attachment so the
	// sidebar actually shows up.
	payload := map[string]interface{}{
		"text": fallback,
	}

	if c.Style != nil {
		if color := resolveSlackColor(c.Style); color != "" {
			payload["attachments"] = []map[string]interface{}{
				{"color": color, "blocks": blocks},
			}
			return payload
		}
	}

	payload["blocks"] = blocks
	return payload
}

// resolveSlackColor maps Style to a Slack attachment color hex.
func resolveSlackColor(style *notification.Style) string {
	if style == nil {
		return ""
	}
	if style.Color != "" {
		return style.Color
	}
	switch style.Severity {
	case "success":
		return "#2ECC71"
	case "warning":
		return "#E67E22"
	case "danger":
		return "#E74C3C"
	case "info":
		return "#3498DB"
	default:
		return ""
	}
}

// BuildCustomSlackPayload builds the Slack payload used by the custom-provider
// path (Settings.CustomProviders[].Kind == "slack").
//
// Previously emitted a plain {text: ...} body that ignored Attachments,
// Actions, Fields, Mentions, Poll and Style. The two paths now share a single
// Block Kit renderer so the rich payload is preserved end-to-end.
func BuildCustomSlackPayload(notif *notification.Notification) map[string]interface{} {
	return BuildSlackPayload(notif)
}
