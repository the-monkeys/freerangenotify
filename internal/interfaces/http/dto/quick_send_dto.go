package dto

import (
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
)

// QuickSendRequest is a simplified notification request that accepts
// human-readable identifiers instead of internal UUIDs.
type QuickSendRequest struct {
	// Recipient: email address, external user ID, or internal UUID.
	// If recipient doesn't exist as a user, auto-creates one (email only).
	// Optional for webhook-like channels (webhook, discord, slack, teams).
	To string `json:"to" validate:"omitempty"`

	// Channel: push, email, sms, webhook, in_app, sse, whatsapp, discord, slack, teams.
	// Optional if Template is specified (inferred from template).
	Channel string `json:"channel" validate:"omitempty,oneof=push email sms webhook in_app sse whatsapp discord slack teams"`

	// Template reference: name (string) or UUID.
	// Optional if Subject+Body are provided (inline content).
	Template string `json:"template,omitempty"`

	// Inline content (used when Template is empty).
	Subject string `json:"subject,omitempty"`
	Body    string `json:"body,omitempty"`

	// Template variables (used when Template is specified).
	Data map[string]interface{} `json:"data,omitempty"`

	// Priority: low, normal, high, critical. Defaults to "normal".
	Priority string `json:"priority,omitempty"`

	// Optional scheduling
	ScheduledAt *time.Time `json:"scheduled_at,omitempty"`

	// Optional: explicit webhook URL (for webhook channel without user)
	WebhookURL string `json:"webhook_url,omitempty"`

	// Optional: name of a registered custom webhook provider on the app
	// (e.g. "Slack Alerts"). When set, the worker dispatches through that
	// provider — which knows its kind (slack/discord/teams/generic) and
	// renders the channel-specific payload shape. Prefer this over
	// WebhookURL when the destination is a registered provider, so the
	// message is posted in a shape the destination accepts.
	WebhookTarget string `json:"webhook_target,omitempty"`

	// Optional: digest rule key — notifications with this key are batched by the matching digest rule
	DigestKey string `json:"digest_key,omitempty"`

	// Rich webhook content. All optional. Only honored when the resolved
	// channel is webhook-like (webhook, discord, slack, teams). Per-provider
	// rendering capabilities are documented in API_DOCUMENTATION.md.
	Attachments []notification.Attachment `json:"attachments,omitempty"`
	Actions     []notification.Action     `json:"actions,omitempty"`
	Fields      []notification.Field      `json:"fields,omitempty"`
	Mentions    []notification.Mention    `json:"mentions,omitempty"`
	Poll        *notification.Poll        `json:"poll,omitempty"`
	Style       *notification.Style       `json:"style,omitempty"`

	// EnvironmentID is set by the auth middleware (not from JSON body).
	EnvironmentID string `json:"-"`
}

// QuickSendResponse is the response for quick-send.
type QuickSendResponse struct {
	NotificationID string `json:"notification_id"`
	Status         string `json:"status"`
	UserID         string `json:"user_id"`
	Channel        string `json:"channel"`
	Message        string `json:"message"`
}
