package dto

import "time"

// QuickSendRequest is a simplified notification request that accepts
// human-readable identifiers instead of internal UUIDs.
type QuickSendRequest struct {
	// Recipient: email address, external user ID, or internal UUID.
	// If recipient doesn't exist as a user, auto-creates one (email only).
	To string `json:"to" validate:"required"`

	// Channel: push, email, sms, webhook, in_app, sse.
	// Optional if Template is specified (inferred from template).
	Channel string `json:"channel" validate:"omitempty,oneof=push email sms webhook in_app sse"`

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

	// Optional: digest rule key — notifications with this key are batched by the matching digest rule
	DigestKey string `json:"digest_key,omitempty"`

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
