package dto

import (
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
)

// SendNotificationRequest represents the API request to send a notification
type SendNotificationRequest struct {
	UserID            string                 `json:"user_id"` // Removed validate:"required"
	Channel           string                 `json:"channel" validate:"omitempty,oneof=push email sms webhook in_app sse whatsapp"`
	MediaURL          string                 `json:"media_url,omitempty" validate:"omitempty,url"`
	Priority          string                 `json:"priority" validate:"required,oneof=low normal high critical"`
	Title             string                 `json:"title,omitempty"`
	Body              string                 `json:"body,omitempty"`
	Data              map[string]interface{} `json:"data,omitempty"`
	TemplateID        string                 `json:"template_id" validate:"required"`
	Category          string                 `json:"category,omitempty"`
	ScheduledAt       *time.Time             `json:"scheduled_at,omitempty"`
	Recurrence        *RecurrenceRequest     `json:"recurrence,omitempty"`
	WebhookURL        string                 `json:"webhook_url,omitempty"`
	WebhookTarget     string                 `json:"webhook_target,omitempty"`
	WorkflowTriggerID string                 `json:"workflow_trigger_id,omitempty"` // Phase 3: trigger workflow after send
	Metadata          map[string]interface{} `json:"metadata,omitempty"`            // Digest: {"digest_key": "rule_key"} routes to digest batching

	// Rich webhook content (Phase 7 — Webhook channel expansion).
	// Domain types are reused so existing client SDKs serialize identically.
	Attachments []notification.Attachment `json:"attachments,omitempty"`
	Actions     []notification.Action     `json:"actions,omitempty"`
	Fields      []notification.Field      `json:"fields,omitempty"`
	Mentions    []notification.Mention    `json:"mentions,omitempty"`
	Poll        *notification.Poll        `json:"poll,omitempty"`
	Style       *notification.Style       `json:"style,omitempty"`
}

// ToSendRequest converts DTO to domain SendRequest
func (r *SendNotificationRequest) ToSendRequest(appID string) notification.SendRequest {
	req := notification.SendRequest{
		AppID:       appID,
		UserID:      r.UserID,
		Channel:     notification.Channel(r.Channel),
		Priority:    notification.Priority(r.Priority),
		Title:       r.Title,
		Body:        r.Body,
		Data:        r.Data,
		TemplateID:  r.TemplateID,
		Category:    r.Category,
		ScheduledAt: r.ScheduledAt,
	}

	if r.WebhookURL != "" {
		if req.Data == nil {
			req.Data = make(map[string]interface{})
		}
		req.Data["webhook_url"] = r.WebhookURL
	}

	if r.WebhookTarget != "" {
		if req.Data == nil {
			req.Data = make(map[string]interface{})
		}
		req.Data["webhook_target"] = r.WebhookTarget
	}

	if r.Recurrence != nil {
		req.Recurrence = &notification.Recurrence{
			CronExpression: r.Recurrence.CronExpression,
			EndDate:        r.Recurrence.EndDate,
			Count:          r.Recurrence.Count,
		}
	}

	req.WorkflowTriggerID = r.WorkflowTriggerID
	req.Metadata = r.Metadata
	req.MediaURL = r.MediaURL

	req.Attachments = r.Attachments
	req.Actions = r.Actions
	req.Fields = r.Fields
	req.Mentions = r.Mentions
	req.Poll = r.Poll
	req.Style = r.Style
	return req
}

// BulkSendNotificationRequest represents the API request to send notifications to multiple users
type BulkSendNotificationRequest struct {
	UserIDs     []string               `json:"user_ids" validate:"required,min=1"`
	Channel     string                 `json:"channel" validate:"required,oneof=push email sms webhook in_app whatsapp"`
	Priority    string                 `json:"priority" validate:"required,oneof=low normal high critical"`
	Title       string                 `json:"title,omitempty"`
	Body        string                 `json:"body,omitempty"`
	Data        map[string]interface{} `json:"data,omitempty"`
	TemplateID  string                 `json:"template_id,omitempty"`
	Category    string                 `json:"category,omitempty"`
	ScheduledAt *time.Time             `json:"scheduled_at,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"` // Digest: {"digest_key": "rule_key"}
	MediaURL    string                 `json:"media_url,omitempty" validate:"omitempty,url"`

	// Rich webhook content (Phase 7).
	Attachments []notification.Attachment `json:"attachments,omitempty"`
	Actions     []notification.Action     `json:"actions,omitempty"`
	Fields      []notification.Field      `json:"fields,omitempty"`
	Mentions    []notification.Mention    `json:"mentions,omitempty"`
	Poll        *notification.Poll        `json:"poll,omitempty"`
	Style       *notification.Style       `json:"style,omitempty"`
}

// ToBulkSendRequest converts DTO to domain BulkSendRequest
func (r *BulkSendNotificationRequest) ToBulkSendRequest(appID string) notification.BulkSendRequest {
	return notification.BulkSendRequest{
		AppID:       appID,
		UserIDs:     r.UserIDs,
		Channel:     notification.Channel(r.Channel),
		Priority:    notification.Priority(r.Priority),
		Title:       r.Title,
		Body:        r.Body,
		Data:        r.Data,
		TemplateID:  r.TemplateID,
		Category:    r.Category,
		ScheduledAt: r.ScheduledAt,
		Metadata:    r.Metadata,
		MediaURL:    r.MediaURL,
		Attachments: r.Attachments,
		Actions:     r.Actions,
		Fields:      r.Fields,
		Mentions:    r.Mentions,
		Poll:        r.Poll,
		Style:       r.Style,
	}
}

// UpdateStatusRequest represents the API request to update notification status
type UpdateStatusRequest struct {
	Status       string `json:"status" validate:"required,oneof=pending queued processing sent delivered read failed cancelled"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// NotificationContentResponse represents the content part of a notification
type NotificationContentResponse struct {
	Title        string                 `json:"title"`
	Body         string                 `json:"body"`
	Data         map[string]interface{} `json:"data,omitempty"`
	Notification string                 `json:"notification,omitempty"`
	MediaURL     string                 `json:"media_url,omitempty"`

	// Rich webhook content (Phase 7).
	Attachments []notification.Attachment `json:"attachments,omitempty"`
	Actions     []notification.Action     `json:"actions,omitempty"`
	Fields      []notification.Field      `json:"fields,omitempty"`
	Mentions    []notification.Mention    `json:"mentions,omitempty"`
	Poll        *notification.Poll        `json:"poll,omitempty"`
	Style       *notification.Style       `json:"style,omitempty"`
}

// NotificationResponse represents the API response for a notification
type NotificationResponse struct {
	NotificationID string                       `json:"notification_id"`
	AppID          string                       `json:"app_id"`
	UserID         string                       `json:"user_id"`
	Channel        string                       `json:"channel"`
	Priority       string                       `json:"priority"`
	Status         string                       `json:"status"`
	Content        *NotificationContentResponse `json:"content"`
	TemplateID     string                       `json:"template_id,omitempty"`
	ScheduledAt    *time.Time                   `json:"scheduled_at,omitempty"`
	SentAt         *time.Time                   `json:"sent_at,omitempty"`
	DeliveredAt    *time.Time                   `json:"delivered_at,omitempty"`
	ReadAt         *time.Time                   `json:"read_at,omitempty"`
	FailedAt       *time.Time                   `json:"failed_at,omitempty"`
	ErrorMessage   string                       `json:"error_message,omitempty"`
	RetryCount     int                          `json:"retry_count"`
	CreatedAt      time.Time                    `json:"created_at"`
	UpdatedAt      time.Time                    `json:"updated_at"`
}

// FromNotification converts domain Notification to response DTO
func FromNotification(n *notification.Notification) *NotificationResponse {
	return &NotificationResponse{
		NotificationID: n.NotificationID,
		AppID:          n.AppID,
		UserID:         n.UserID,
		Channel:        string(n.Channel),
		Priority:       string(n.Priority),
		Status:         string(n.Status),
		Content: &NotificationContentResponse{
			Title:       n.Content.Title,
			Body:        n.Content.Body,
			Data:        n.Content.Data,
			MediaURL:    n.Content.MediaURL,
			Attachments: n.Content.Attachments,
			Actions:     n.Content.Actions,
			Fields:      n.Content.Fields,
			Mentions:    n.Content.Mentions,
			Poll:        n.Content.Poll,
			Style:       n.Content.Style,
		},
		TemplateID:   n.TemplateID,
		ScheduledAt:  n.ScheduledAt,
		SentAt:       n.SentAt,
		DeliveredAt:  n.DeliveredAt,
		ReadAt:       n.ReadAt,
		FailedAt:     n.FailedAt,
		ErrorMessage: n.ErrorMessage,
		RetryCount:   n.RetryCount,
		CreatedAt:    n.CreatedAt,
		UpdatedAt:    n.UpdatedAt,
	}
}

// NotificationListResponse represents the API response for a list of notifications
type NotificationListResponse struct {
	Notifications []*NotificationResponse `json:"notifications"`
	Total         int64                   `json:"total"`
	Page          int                     `json:"page"`
	PageSize      int                     `json:"page_size"`
}

// BulkSendResponse represents the API response for bulk send operation
type BulkSendResponse struct {
	Sent  int                     `json:"sent"`
	Total int                     `json:"total"`
	Items []*NotificationResponse `json:"items"`
}

// RecurrenceRequest represents recurrence rules in API requests
type RecurrenceRequest struct {
	CronExpression string     `json:"cron_expression" validate:"required"`
	EndDate        *time.Time `json:"end_date,omitempty"`
	Count          int        `json:"count,omitempty"`
}

// BroadcastNotificationRequest represents the API request to broadcast a notification to all users
type BroadcastNotificationRequest struct {
	Channel           string                 `json:"channel" validate:"required,oneof=push email sms webhook in_app sse whatsapp"`
	Priority          string                 `json:"priority" validate:"required,oneof=low normal high critical"`
	Title             string                 `json:"title,omitempty"`
	Body              string                 `json:"body,omitempty"`
	Data              map[string]interface{} `json:"data,omitempty"`
	TemplateID        string                 `json:"template_id"` // required when workflow_trigger_id is empty
	Category          string                 `json:"category,omitempty"`
	ScheduledAt       *time.Time             `json:"scheduled_at,omitempty"`
	WebhookURL        string                 `json:"webhook_url,omitempty"`
	WebhookTarget     string                 `json:"webhook_target,omitempty"`
	WorkflowTriggerID string                 `json:"workflow_trigger_id,omitempty"` // Phase 2: trigger workflow for each recipient
	TopicKey          string                 `json:"topic_key,omitempty"`           // Phase 2: limit to topic subscribers
	Metadata          map[string]interface{} `json:"metadata,omitempty"`            // Digest: {"digest_key": "rule_key"}

	// Rich webhook content (Phase 7).
	Attachments []notification.Attachment `json:"attachments,omitempty"`
	Actions     []notification.Action     `json:"actions,omitempty"`
	Fields      []notification.Field      `json:"fields,omitempty"`
	Mentions    []notification.Mention    `json:"mentions,omitempty"`
	Poll        *notification.Poll        `json:"poll,omitempty"`
	Style       *notification.Style       `json:"style,omitempty"`
}

// ToBroadcastRequest converts DTO to domain BroadcastRequest
func (r *BroadcastNotificationRequest) ToBroadcastRequest(appID string) notification.BroadcastRequest {
	return notification.BroadcastRequest{
		AppID:             appID,
		Channel:           notification.Channel(r.Channel),
		Priority:          notification.Priority(r.Priority),
		Title:             r.Title,
		Body:              r.Body,
		Data:              r.Data,
		TemplateID:        r.TemplateID,
		Category:          r.Category,
		ScheduledAt:       r.ScheduledAt,
		WorkflowTriggerID: r.WorkflowTriggerID,
		TopicKey:          r.TopicKey,
		Metadata:          r.Metadata,
		Attachments:       r.Attachments,
		Actions:           r.Actions,
		Fields:            r.Fields,
		Mentions:          r.Mentions,
		Poll:              r.Poll,
		Style:             r.Style,
	}
}

// BatchSendNotificationRequest represents a request to send multiple distinct notifications
type BatchSendNotificationRequest struct {
	Notifications []SendNotificationRequest `json:"notifications" validate:"required,min=1,dive"`
}

// ToBatchSendRequest converts DTO to domain SendRequest list
func (r *BatchSendNotificationRequest) ToBatchSendRequest(appID string) []notification.SendRequest {
	var requests []notification.SendRequest
	for _, req := range r.Notifications {
		requests = append(requests, req.ToSendRequest(appID))
	}
	return requests
}

// BatchCancelRequest represents a request to cancel multiple notifications
type BatchCancelRequest struct {
	NotificationIDs []string `json:"notification_ids" validate:"required,min=1"`
}

// ── Phase 5: Snooze, Archive, Mark-All-Read ────────────────────────

// SnoozeRequest represents a request to snooze a notification.
type SnoozeRequest struct {
	Duration string     `json:"duration,omitempty"` // e.g. "2h", "30m", "1d"
	Until    *time.Time `json:"until,omitempty"`
}

// BulkArchiveRequest represents a request to archive notifications.
type BulkArchiveRequest struct {
	NotificationIDs []string `json:"notification_ids" validate:"required,min=1,max=100"`
	UserID          string   `json:"user_id" validate:"required"`
}

// MarkAllReadRequest represents a request to mark all unread as read.
type MarkAllReadRequest struct {
	UserID   string `json:"user_id" validate:"required"`
	Category string `json:"category,omitempty"`
}
