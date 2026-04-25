package render

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
)

type TeamsWebhookFlavor string

const (
	TeamsWebhookFlavorLegacy   TeamsWebhookFlavor = "legacy"
	TeamsWebhookFlavorWorkflow TeamsWebhookFlavor = "workflow"
)

// DetectTeamsWebhookFlavor classifies a Teams webhook URL into legacy or workflow flavor.
func DetectTeamsWebhookFlavor(rawURL string) TeamsWebhookFlavor {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return TeamsWebhookFlavorLegacy
	}
	host := strings.ToLower(u.Host)
	path := strings.ToLower(u.Path)

	switch {
	case strings.HasSuffix(host, "webhook.office.com"):
		return TeamsWebhookFlavorLegacy
	case strings.Contains(host, "logic.azure.com") && strings.Contains(path, "/workflows/"):
		return TeamsWebhookFlavorWorkflow
	default:
		return TeamsWebhookFlavorLegacy
	}
}

// BuildTeamsPayload constructs a Teams payload and auto-selects wrapper flavor from URL.
func BuildTeamsPayload(notif *notification.Notification, webhookURL string) map[string]interface{} {
	flavor := DetectTeamsWebhookFlavor(webhookURL)
	cardContent := map[string]interface{}{
		"$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
		"type":    "AdaptiveCard",
		"version": "1.4",
		"body":    buildTeamsCardBody(notif),
	}

	// Wire rich actions into the card
	if actions := buildTeamsActions(notif); len(actions) > 0 {
		cardContent["actions"] = actions
	}

	cardAttachment := map[string]interface{}{
		"contentType": "application/vnd.microsoft.card.adaptive",
		"contentUrl":  nil,
		"content":     cardContent,
	}

	if flavor == TeamsWebhookFlavorWorkflow {
		text := strings.TrimSpace(FormatContentString(notif.Content))
		if text == "" {
			text = "FreeRangeNotify"
		}
		return map[string]interface{}{
			"type":        "message",
			"text":        text,
			"attachments": []map[string]interface{}{cardAttachment},
		}
	}

	return map[string]interface{}{
		"type":        "message",
		"attachments": []map[string]interface{}{cardAttachment},
	}
}

func buildTeamsCardBody(notif *notification.Notification) []map[string]interface{} {
	c := notif.Content
	body := []map[string]interface{}{
		{
			"type":   "TextBlock",
			"text":   c.Title,
			"weight": "Bolder",
			"size":   "Medium",
		},
		{
			"type": "TextBlock",
			"text": c.Body,
			"wrap": true,
		},
	}

	// --- Style → color banner via TextBlock ---
	if c.Style != nil && c.Style.Severity != "" {
		var label string
		switch c.Style.Severity {
		case "success":
			label = "✅ Success"
		case "warning":
			label = "⚠️ Warning"
		case "danger":
			label = "🚨 Danger"
		case "info":
			label = "ℹ️ Info"
		}
		if label != "" {
			body = append(body, map[string]interface{}{
				"type":   "TextBlock",
				"text":   label,
				"weight": "Bolder",
				"color":  resolveTeamsColor(c.Style.Severity),
			})
		}
	}

	// --- Fields → FactSet ---
	if len(c.Fields) > 0 {
		facts := make([]map[string]interface{}, 0, len(c.Fields))
		for _, f := range c.Fields {
			facts = append(facts, map[string]interface{}{
				"title": f.Key,
				"value": f.Value,
			})
		}
		body = append(body, map[string]interface{}{
			"type":  "FactSet",
			"facts": facts,
		})
	}

	// --- Attachments → Image elements + download links ---
	for _, a := range c.Attachments {
		switch a.Type {
		case "image":
			img := map[string]interface{}{
				"type": "Image",
				"url":  a.URL,
				"size": "Large",
			}
			if a.AltText != "" {
				img["altText"] = a.AltText
			}
			body = append(body, img)
		default:
			label := a.Name
			if label == "" {
				label = a.Type
			}
			body = append(body, map[string]interface{}{
				"type": "TextBlock",
				"text": fmt.Sprintf("📎 [%s](%s)", label, a.URL),
				"wrap": true,
			})
		}
	}

	// --- Poll → rendered as FactSet with numbered choices ---
	if c.Poll != nil {
		body = append(body, map[string]interface{}{
			"type":   "TextBlock",
			"text":   fmt.Sprintf("📊 %s", c.Poll.Question),
			"weight": "Bolder",
			"wrap":   true,
		})
		pollFacts := make([]map[string]interface{}, 0, len(c.Poll.Choices))
		for i, ch := range c.Poll.Choices {
			emoji := ch.Emoji
			if emoji == "" {
				emoji = fmt.Sprintf("%d️⃣", i+1)
			}
			pollFacts = append(pollFacts, map[string]interface{}{
				"title": emoji,
				"value": ch.Label,
			})
		}
		body = append(body, map[string]interface{}{
			"type":  "FactSet",
			"facts": pollFacts,
		})
	}

	// --- Mentions → prepend as TextBlock ---
	if len(c.Mentions) > 0 {
		var parts []string
		for _, m := range c.Mentions {
			if m.Platform == "teams" && m.PlatformID != "" {
				display := m.Display
				if display == "" {
					display = m.PlatformID
				}
				parts = append(parts, fmt.Sprintf("<at>%s</at>", display))
			}
		}
		if len(parts) > 0 {
			body = append([]map[string]interface{}{{
				"type": "TextBlock",
				"text": strings.Join(parts, " "),
			}}, body...)
		}
	}

	return body
}

// buildTeamsActions returns an ActionSet for the Adaptive Card.
func buildTeamsActions(notif *notification.Notification) []map[string]interface{} {
	c := notif.Content
	var actions []map[string]interface{}

	// Legacy: action_url
	if c.Data != nil {
		if actionURL, ok := c.Data["action_url"].(string); ok && actionURL != "" {
			actionLabel := "View"
			if label, ok := c.Data["action_label"].(string); ok && label != "" {
				actionLabel = label
			}
			actions = append(actions, map[string]interface{}{
				"type":  "Action.OpenUrl",
				"title": actionLabel,
				"url":   actionURL,
			})
		}
	}

	// Rich actions
	for _, a := range c.Actions {
		if a.Type == "link" && a.URL != "" {
			actions = append(actions, map[string]interface{}{
				"type":  "Action.OpenUrl",
				"title": a.Label,
				"url":   a.URL,
			})
		}
	}

	return actions
}

// resolveTeamsColor maps severity to an AdaptiveCard TextBlock color.
func resolveTeamsColor(severity string) string {
	switch severity {
	case "success":
		return "Good"
	case "warning":
		return "Warning"
	case "danger":
		return "Attention"
	default:
		return "Default"
	}
}
