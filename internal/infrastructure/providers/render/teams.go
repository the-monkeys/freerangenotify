package render

import (
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
	cardAttachment := map[string]interface{}{
		"contentType": "application/vnd.microsoft.card.adaptive",
		"contentUrl":  nil,
		"content": map[string]interface{}{
			"$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
			"type":    "AdaptiveCard",
			"version": "1.4",
			"body":    buildTeamsCardBody(notif),
		},
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
	body := []map[string]interface{}{
		{
			"type":   "TextBlock",
			"text":   notif.Content.Title,
			"weight": "Bolder",
			"size":   "Medium",
		},
		{
			"type": "TextBlock",
			"text": notif.Content.Body,
			"wrap": true,
		},
	}

	// Keep legacy behavior: action_url becomes an Action.OpenUrl action set.
	if notif.Content.Data != nil {
		if actionURL, ok := notif.Content.Data["action_url"].(string); ok && actionURL != "" {
			actionLabel := "View"
			if label, ok := notif.Content.Data["action_label"].(string); ok && label != "" {
				actionLabel = label
			}
			body = append(body, map[string]interface{}{
				"type": "ActionSet",
				"actions": []map[string]interface{}{
					{
						"type":  "Action.OpenUrl",
						"title": actionLabel,
						"url":   actionURL,
					},
				},
			})
		}
	}

	return body
}
