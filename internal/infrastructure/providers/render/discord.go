package render

import "github.com/the-monkeys/freerangenotify/internal/domain/notification"

// BuildDiscordPayload constructs a Discord webhook payload with embeds.
func BuildDiscordPayload(notif *notification.Notification) map[string]interface{} {
	embed := map[string]interface{}{
		"title":       notif.Content.Title,
		"description": notif.Content.Body,
		"color":       3447003, // Discord blue (#3498DB)
	}

	// Keep legacy behavior: action_url becomes embed URL.
	if notif.Content.Data != nil {
		if actionURL, ok := notif.Content.Data["action_url"].(string); ok && actionURL != "" {
			embed["url"] = actionURL
		}
	}

	return map[string]interface{}{
		"content": notif.Content.Title,
		"embeds":  []map[string]interface{}{embed},
	}
}

// BuildCustomDiscordPayload builds the custom-provider Discord shape.
func BuildCustomDiscordPayload(notif *notification.Notification) map[string]interface{} {
	text := FormatContentString(notif.Content)
	payload := map[string]interface{}{
		"content": text,
	}
	if notif.Content.Title != "" {
		payload["embeds"] = []map[string]interface{}{
			{
				"title":       notif.Content.Title,
				"description": notif.Content.Body,
			},
		}
		payload["content"] = nil
	}
	return payload
}
