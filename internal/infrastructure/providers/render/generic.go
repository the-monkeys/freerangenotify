package render

import (
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
)

// GenericWebhookPayload represents the structured payload sent by the webhook provider.
type GenericWebhookPayload struct {
	ID         string                 `json:"id"`
	AppID      string                 `json:"app_id"`
	UserID     string                 `json:"user_id"`
	Channel    string                 `json:"channel"`
	Priority   string                 `json:"priority"`
	Status     string                 `json:"status"`
	TemplateID string                 `json:"template_id"`
	Template   *TemplateInfo          `json:"template,omitempty"`
	Content    notification.Content   `json:"content"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
}

// TemplateInfo contains template details for the receiver.
type TemplateInfo struct {
	Name      string   `json:"name"`
	Subject   string   `json:"subject"`
	Body      string   `json:"body"`
	Variables []string `json:"variables"`
}

// BuildGenericWebhookPayload builds the payload used by the webhook provider.
func BuildGenericWebhookPayload(notif *notification.Notification) GenericWebhookPayload {
	return GenericWebhookPayload{
		ID:         notif.NotificationID,
		AppID:      notif.AppID,
		UserID:     notif.UserID,
		Channel:    string(notif.Channel),
		Priority:   string(notif.Priority),
		Status:     string(notif.Status),
		TemplateID: notif.TemplateID,
		Content:    notif.Content,
		Metadata:   notif.Metadata,
		CreatedAt:  notif.CreatedAt,
	}
}

// BuildCustomStandardPayload builds the default custom-provider JSON envelope.
func BuildCustomStandardPayload(notif *notification.Notification, channel notification.Channel, usr *user.User) map[string]interface{} {
	payload := map[string]interface{}{
		"notification_id": notif.NotificationID,
		"app_id":          notif.AppID,
		"user_id":         notif.UserID,
		"channel":         string(channel),
		"content":         notif.Content,
		"metadata":        notif.Metadata,
		"priority":        string(notif.Priority),
		"category":        notif.Category,
		"created_at":      notif.CreatedAt,
	}
	if usr != nil {
		payload["user"] = map[string]interface{}{
			"email":       usr.Email,
			"phone":       usr.Phone,
			"external_id": usr.ExternalID,
			"timezone":    usr.Timezone,
			"language":    usr.Language,
		}
	}
	return payload
}
