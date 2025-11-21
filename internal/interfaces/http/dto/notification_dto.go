package dto

import (
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
)

// SendNotificationRequest represents the API request to send a notification
type SendNotificationRequest struct {
	UserID      string                 `json:"user_id" validate:"required"`
	Channel     string                 `json:"channel" validate:"required,oneof=push email sms webhook in_app"`
	Priority    string                 `json:"priority" validate:"required,oneof=low normal high critical"`
	Title       string                 `json:"title" validate:"required"`
	Body        string                 `json:"body" validate:"required"`
	Data        map[string]interface{} `json:"data,omitempty"`
	TemplateID  string                 `json:"template_id,omitempty"`
	Category    string                 `json:"category,omitempty"`
	ScheduledAt *time.Time             `json:"scheduled_at,omitempty"`
	Recurrence  *RecurrenceRequest     `json:"recurrence,omitempty"`
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

	if r.Recurrence != nil {
		req.Recurrence = &notification.Recurrence{
			CronExpression: r.Recurrence.CronExpression,
			EndDate:        r.Recurrence.EndDate,
			Count:          r.Recurrence.Count,
		}
	}

	return req
}

// BulkSendNotificationRequest represents the API request to send notifications to multiple users
type BulkSendNotificationRequest struct {
	UserIDs     []string               `json:"user_ids" validate:"required,min=1"`
	Channel     string                 `json:"channel" validate:"required,oneof=push email sms webhook in_app"`
	Priority    string                 `json:"priority" validate:"required,oneof=low normal high critical"`
	Title       string                 `json:"title" validate:"required"`
	Body        string                 `json:"body" validate:"required"`
	Data        map[string]interface{} `json:"data,omitempty"`
	TemplateID  string                 `json:"template_id,omitempty"`
	Category    string                 `json:"category,omitempty"`
	ScheduledAt *time.Time             `json:"scheduled_at,omitempty"`
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
	}
}

// UpdateStatusRequest represents the API request to update notification status
type UpdateStatusRequest struct {
	Status       string `json:"status" validate:"required,oneof=pending queued processing sent delivered read failed cancelled"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// NotificationResponse represents the API response for a notification
type NotificationResponse struct {
	NotificationID string                 `json:"notification_id"`
	AppID          string                 `json:"app_id"`
	UserID         string                 `json:"user_id"`
	Channel        string                 `json:"channel"`
	Priority       string                 `json:"priority"`
	Status         string                 `json:"status"`
	Title          string                 `json:"title"`
	Body           string                 `json:"body"`
	Data           map[string]interface{} `json:"data,omitempty"`
	TemplateID     string                 `json:"template_id,omitempty"`
	ScheduledAt    *time.Time             `json:"scheduled_at,omitempty"`
	SentAt         *time.Time             `json:"sent_at,omitempty"`
	DeliveredAt    *time.Time             `json:"delivered_at,omitempty"`
	ReadAt         *time.Time             `json:"read_at,omitempty"`
	FailedAt       *time.Time             `json:"failed_at,omitempty"`
	ErrorMessage   string                 `json:"error_message,omitempty"`
	RetryCount     int                    `json:"retry_count"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
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
		Title:          n.Content.Title,
		Body:           n.Content.Body,
		Data:           n.Content.Data,
		TemplateID:     n.TemplateID,
		ScheduledAt:    n.ScheduledAt,
		SentAt:         n.SentAt,
		DeliveredAt:    n.DeliveredAt,
		ReadAt:         n.ReadAt,
		FailedAt:       n.FailedAt,
		ErrorMessage:   n.ErrorMessage,
		RetryCount:     n.RetryCount,
		CreatedAt:      n.CreatedAt,
		UpdatedAt:      n.UpdatedAt,
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
