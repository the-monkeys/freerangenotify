package render

import (
	"fmt"

	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
)

// BuildSlackPayload constructs a Slack Block Kit message.
func BuildSlackPayload(notif *notification.Notification) map[string]interface{} {
	blocks := []map[string]interface{}{
		{
			"type": "section",
			"text": map[string]interface{}{
				"type": "mrkdwn",
				"text": fmt.Sprintf("*%s*\n%s", notif.Content.Title, notif.Content.Body),
			},
		},
	}

	// Keep legacy behavior: action_url becomes a button row.
	if notif.Content.Data != nil {
		if actionURL, ok := notif.Content.Data["action_url"].(string); ok && actionURL != "" {
			actionLabel := "View"
			if label, ok := notif.Content.Data["action_label"].(string); ok && label != "" {
				actionLabel = label
			}
			blocks = append(blocks, map[string]interface{}{
				"type": "actions",
				"elements": []map[string]interface{}{
					{
						"type": "button",
						"text": map[string]interface{}{
							"type": "plain_text",
							"text": actionLabel,
						},
						"url": actionURL,
					},
				},
			})
		}
	}

	return map[string]interface{}{
		"text":   notif.Content.Title, // Fallback for accessibility.
		"blocks": blocks,
	}
}

// BuildCustomSlackPayload builds the custom-provider Slack shape.
func BuildCustomSlackPayload(notif *notification.Notification) map[string]interface{} {
	return map[string]interface{}{
		"text": FormatContentString(notif.Content),
	}
}
