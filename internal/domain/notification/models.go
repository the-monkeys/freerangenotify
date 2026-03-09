package notification

import (
	"context"
	"time"
)

// Channel represents notification delivery channels
type Channel string

const (
	ChannelPush     Channel = "push"
	ChannelEmail    Channel = "email"
	ChannelSMS      Channel = "sms"
	ChannelWebhook  Channel = "webhook"
	ChannelInApp    Channel = "in_app"
	ChannelSSE      Channel = "sse"
	ChannelSlack    Channel = "slack"    // Phase 3
	ChannelDiscord  Channel = "discord"  // Phase 3
	ChannelWhatsApp Channel = "whatsapp" // Phase 3
	ChannelTeams    Channel = "teams"    // Phase 3
)

// Valid checks if the channel is valid
func (c Channel) Valid() bool {
	switch c {
	case ChannelPush, ChannelEmail, ChannelSMS, ChannelWebhook, ChannelInApp, ChannelSSE,
		ChannelSlack, ChannelDiscord, ChannelWhatsApp, ChannelTeams:
		return true
	default:
		return false
	}
}

// String returns the string representation of the channel
func (c Channel) String() string {
	return string(c)
}

// Priority represents notification priority levels
type Priority string

const (
	PriorityLow      Priority = "low"
	PriorityNormal   Priority = "normal"
	PriorityHigh     Priority = "high"
	PriorityCritical Priority = "critical"
)

// Valid checks if the priority is valid
func (p Priority) Valid() bool {
	switch p {
	case PriorityLow, PriorityNormal, PriorityHigh, PriorityCritical:
		return true
	default:
		return false
	}
}

// String returns the string representation of the priority
func (p Priority) String() string {
	return string(p)
}

// Status represents notification processing status
type Status string

const (
	StatusPending    Status = "pending"
	StatusQueued     Status = "queued"
	StatusProcessing Status = "processing"
	StatusSent       Status = "sent"
	StatusDelivered  Status = "delivered"
	StatusRead       Status = "read"
	StatusFailed     Status = "failed"
	StatusCancelled  Status = "cancelled"
	StatusSnoozed    Status = "snoozed"  // Phase 5: deferred by user
	StatusArchived   Status = "archived" // Phase 5: dismissed by user
)

// Valid checks if the status is valid
func (s Status) Valid() bool {
	switch s {
	case StatusPending, StatusQueued, StatusProcessing, StatusSent, StatusDelivered, StatusRead, StatusFailed, StatusCancelled, StatusSnoozed, StatusArchived:
		return true
	default:
		return false
	}
}

// String returns the string representation of the status
func (s Status) String() string {
	return string(s)
}

// IsFinal returns true if this is a terminal status
func (s Status) IsFinal() bool {
	return s == StatusDelivered || s == StatusRead || s == StatusFailed || s == StatusCancelled || s == StatusArchived
}

// Notification represents a notification entity
type Notification struct {
	NotificationID       string                 `json:"notification_id" es:"notification_id"`
	AppID                string                 `json:"app_id" es:"app_id"`
	EnvironmentID        string                 `json:"environment_id,omitempty" es:"environment_id"`
	UserID               string                 `json:"user_id" es:"user_id"`
	TemplateID           string                 `json:"template_id,omitempty" es:"template_id"`
	Channel              Channel                `json:"channel" es:"channel"`
	Priority             Priority               `json:"priority" es:"priority"`
	Status               Status                 `json:"status" es:"status"`
	Content              Content                `json:"content" es:"content"`
	Category             string                 `json:"category,omitempty" es:"category"`
	Metadata             map[string]interface{} `json:"metadata,omitempty" es:"metadata"`
	ScheduledAt          *time.Time             `json:"scheduled_at,omitempty" es:"scheduled_at"`
	SentAt               *time.Time             `json:"sent_at,omitempty" es:"sent_at"`
	DeliveredAt          *time.Time             `json:"delivered_at,omitempty" es:"delivered_at"`
	ReadAt               *time.Time             `json:"read_at,omitempty" es:"read_at"`
	FailedAt             *time.Time             `json:"failed_at,omitempty" es:"failed_at"`
	ErrorMessage         string                 `json:"error_message,omitempty" es:"error_message"`
	RetryCount           int                    `json:"retry_count" es:"retry_count"`
	Recurrence           *Recurrence            `json:"recurrence,omitempty" es:"recurrence"`
	SnoozedUntil         *time.Time             `json:"snoozed_until,omitempty" es:"snoozed_until"` // Phase 5
	ArchivedAt           *time.Time             `json:"archived_at,omitempty" es:"archived_at"`     // Phase 5
	CreatedAt            time.Time              `json:"created_at" es:"created_at"`
	UpdatedAt            time.Time              `json:"updated_at" es:"updated_at"`
	RenderedNotification *Content               `json:"rendered_notification,omitempty" es:"-"`
}

// Recurrence defines rules for repeating notifications
type Recurrence struct {
	CronExpression string     `json:"cron_expression" es:"cron_expression"`
	EndDate        *time.Time `json:"end_date,omitempty" es:"end_date"`
	Count          int        `json:"count,omitempty" es:"count"` // Max occurrences
	CurrentCount   int        `json:"current_count,omitempty" es:"current_count"`
}

// Content represents notification content
type Content struct {
	Title string                 `json:"title" es:"title"`
	Body  string                 `json:"body" es:"body"`
	Data  map[string]interface{} `json:"data,omitempty" es:"data"`
}

// NotificationFilter represents query filters for notifications
type NotificationFilter struct {
	AppID         string                 `json:"app_id,omitempty"`
	AppIDs        []string               `json:"app_ids,omitempty"`
	EnvironmentID string                 `json:"environment_id,omitempty"`
	UserID        string                 `json:"user_id,omitempty"`
	Channel       Channel                `json:"channel,omitempty"`
	Status        Status                 `json:"status,omitempty"`
	Priority      Priority               `json:"priority,omitempty"`
	TemplateID    string                 `json:"template_id,omitempty"`
	FromDate      *time.Time             `json:"from_date,omitempty"`
	ToDate        *time.Time             `json:"to_date,omitempty"`
	Category      string                 `json:"category,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Page          int                    `json:"page,omitempty"`
	PageSize      int                    `json:"page_size,omitempty"`
	SortBy        string                 `json:"sort_by,omitempty"`
	SortOrder     string                 `json:"sort_order,omitempty"` // "asc" or "desc"
	Cursor        string                 `json:"cursor,omitempty"`     // opaque cursor for search_after pagination
}

// DefaultFilter returns a filter with default values
func DefaultFilter() NotificationFilter {
	return NotificationFilter{
		Page:      1,
		PageSize:  50,
		SortBy:    "created_at",
		SortOrder: "desc",
	}
}

// Validate validates the filter parameters
func (f *NotificationFilter) Validate() error {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.PageSize < 1 || f.PageSize > 100 {
		f.PageSize = 50
	}
	if f.SortOrder != "asc" && f.SortOrder != "desc" {
		f.SortOrder = "desc"
	}
	if f.Channel != "" && !f.Channel.Valid() {
		return ErrInvalidChannel
	}
	if f.Priority != "" && !f.Priority.Valid() {
		return ErrInvalidPriority
	}
	if f.Status != "" && !f.Status.Valid() {
		return ErrInvalidStatus
	}
	return nil
}

// Repository defines the interface for notification data operations
type Repository interface {
	Create(ctx context.Context, notification *Notification) error
	GetByID(ctx context.Context, id string) (*Notification, error)
	Update(ctx context.Context, notification *Notification) error
	List(ctx context.Context, filter *NotificationFilter) ([]*Notification, error)
	Delete(ctx context.Context, id string) error
	Count(ctx context.Context, filter *NotificationFilter) (int64, error)
	UpdateStatus(ctx context.Context, id string, status Status) error
	BulkUpdateStatus(ctx context.Context, ids []string, status Status) error
	GetPending(ctx context.Context) ([]*Notification, error)
	GetRetryable(ctx context.Context, maxRetries int) ([]*Notification, error)
	IncrementRetryCount(ctx context.Context, id string, errorMessage string) error
	// Phase 5: snooze, archive, mark-all-read
	UpdateSnooze(ctx context.Context, id string, status Status, snoozedUntil *time.Time) error
	BulkArchive(ctx context.Context, ids []string, archivedAt time.Time) error
	MarkAllRead(ctx context.Context, userID, appID, category string) (int, error)
	ListSnoozedDue(ctx context.Context, now time.Time) ([]*Notification, error)
}

// Service defines the business logic interface for notifications
type Service interface {
	Send(ctx context.Context, req SendRequest) (*Notification, error)
	SendBulk(ctx context.Context, req BulkSendRequest) ([]*Notification, error)
	SendBatch(ctx context.Context, requests []SendRequest) ([]*Notification, error)
	Get(ctx context.Context, notificationID, appID string) (*Notification, error)
	List(ctx context.Context, filter NotificationFilter) ([]*Notification, error)
	UpdateStatus(ctx context.Context, notificationID string, status Status, errorMessage string, appID string) error
	Cancel(ctx context.Context, notificationID, appID string) error
	CancelBatch(ctx context.Context, notificationIDs []string, appID string) error
	Retry(ctx context.Context, notificationID, appID string) error
	FlushQueued(ctx context.Context, userID string) error
	GetUnreadCount(ctx context.Context, userID, appID string) (int64, error)
	ListUnread(ctx context.Context, userID, appID string) ([]*Notification, error)
	MarkRead(ctx context.Context, notificationIDs []string, appID, userID string) error
	Broadcast(ctx context.Context, req BroadcastRequest) ([]*Notification, error)
	// Phase 5: snooze, archive, mark-all-read
	Snooze(ctx context.Context, notificationID, appID string, until time.Time) error
	Unsnooze(ctx context.Context, notificationID, appID string) error
	Archive(ctx context.Context, notificationIDs []string, appID, userID string) error
	MarkAllRead(ctx context.Context, userID, appID, category string) error
	ListSnoozedDue(ctx context.Context) ([]*Notification, error)
}

// BroadcastRequest represents a request to send a notification to all users of an application
type BroadcastRequest struct {
	AppID         string                 `json:"app_id" validate:"required"`
	EnvironmentID string                 `json:"environment_id,omitempty"`
	Channel       Channel                `json:"channel" validate:"required"`
	Priority      Priority               `json:"priority" validate:"required"`
	Title         string                 `json:"title,omitempty"`
	Body          string                 `json:"body,omitempty"`
	Data          map[string]interface{} `json:"data,omitempty"`
	TemplateID    string                 `json:"template_id" validate:"required"`
	Category      string                 `json:"category,omitempty"`
	ScheduledAt   *time.Time             `json:"scheduled_at,omitempty"`
}

// Validate validates the broadcast request
func (r *BroadcastRequest) Validate() error {
	if r.AppID == "" {
		return ErrInvalidAppID
	}
	if !r.Channel.Valid() {
		return ErrInvalidChannel
	}
	if !r.Priority.Valid() {
		return ErrInvalidPriority
	}
	if r.TemplateID == "" {
		return ErrTemplateRequired
	}
	return nil
}

// Validate validates the notification entity
func (n *Notification) Validate() error {
	if n.NotificationID == "" {
		return ErrInvalidNotificationID
	}
	if n.AppID == "" {
		return ErrInvalidAppID
	}
	if n.UserID == "" {
		if n.Channel != ChannelWebhook {
			return ErrInvalidUserID
		}
		// For webhook, check if we have the URL in metadata
		hasURL := false
		if n.Metadata != nil {
			if _, ok := n.Metadata["webhook_url"]; ok {
				hasURL = true
			}
		}

		// If no explicit URL, we must have a TemplateID which can resolve via WebhookTarget
		if !hasURL && n.TemplateID == "" {
			return ErrInvalidUserID
		}
	}
	if !n.Channel.Valid() {
		return ErrInvalidChannel
	}
	if !n.Priority.Valid() {
		return ErrInvalidPriority
	}
	if !n.Status.Valid() {
		return ErrInvalidStatus
	}
	if n.TemplateID == "" && n.Content.Title == "" && n.Content.Body == "" {
		return ErrEmptyContent
	}
	return nil
}

// CanRetry returns true if the notification can be retried
func (n *Notification) CanRetry(maxRetries int) bool {
	return n.Status == StatusFailed && n.RetryCount < maxRetries
}

// IsScheduled returns true if the notification is scheduled for future delivery
func (n *Notification) IsScheduled() bool {
	return n.ScheduledAt != nil && n.ScheduledAt.After(time.Now())
}

// SendRequest represents a request to send a notification
type SendRequest struct {
	AppID         string                 `json:"app_id" validate:"required"`
	EnvironmentID string                 `json:"environment_id,omitempty"`
	UserID        string                 `json:"user_id"` // Removed validate:"required"
	Channel       Channel                `json:"channel"` // Optional: inferred from template if empty
	Priority      Priority               `json:"priority" validate:"required"`
	Title         string                 `json:"title,omitempty"`
	Body          string                 `json:"body,omitempty"`
	Data          map[string]interface{} `json:"data,omitempty"`
	TemplateID    string                 `json:"template_id" validate:"required"`
	Category      string                 `json:"category,omitempty"`
	ScheduledAt   *time.Time             `json:"scheduled_at,omitempty"`
	Recurrence    *Recurrence            `json:"recurrence,omitempty"`
	TopicID       string                 `json:"topic_id,omitempty"` // Phase 2: send to all subscribers of a topic
}

// Validate validates the send request
func (r *SendRequest) Validate() error {
	if r.AppID == "" {
		return ErrInvalidAppID
	}
	// Conditional UserID validation — TopicID can substitute for UserID
	if r.UserID == "" && r.TopicID == "" {
		if r.Channel != ChannelWebhook {
			return ErrInvalidUserID
		}
		// For webhook, if UserID is empty, we must have a webhook_url OR webhook_target in Data OR a TemplateID
		hasURL := false
		hasTarget := false
		if r.Data != nil {
			if _, ok := r.Data["webhook_url"]; ok {
				hasURL = true
			}
			if _, ok := r.Data["webhook_target"]; ok {
				hasTarget = true
			}
		}

		if !hasURL && !hasTarget && r.TemplateID == "" {
			return ErrInvalidUserID
		}
	}

	// Channel is optional when TemplateID is present (inferred from template in service layer)
	if r.Channel != "" && !r.Channel.Valid() {
		return ErrInvalidChannel
	}
	if r.Channel == "" && r.TemplateID == "" {
		return ErrInvalidChannel
	}
	if !r.Priority.Valid() {
		return ErrInvalidPriority
	}
	if r.TemplateID == "" {
		return ErrTemplateRequired
	}
	if r.ScheduledAt != nil && r.ScheduledAt.Before(time.Now()) {
		return ErrInvalidScheduleTime
	}
	return nil
}

// BulkSendRequest represents a request to send notifications to multiple users
type BulkSendRequest struct {
	AppID         string                 `json:"app_id" validate:"required"`
	EnvironmentID string                 `json:"environment_id,omitempty"`
	UserIDs       []string               `json:"user_ids" validate:"required,min=1"`
	Channel       Channel                `json:"channel" validate:"required"`
	Priority      Priority               `json:"priority" validate:"required"`
	Title         string                 `json:"title" validate:"required"`
	Body          string                 `json:"body" validate:"required"`
	Data          map[string]interface{} `json:"data,omitempty"`
	TemplateID    string                 `json:"template_id,omitempty"`
	Category      string                 `json:"category,omitempty"`
	ScheduledAt   *time.Time             `json:"scheduled_at,omitempty"`
	Recurrence    *Recurrence            `json:"recurrence,omitempty"`
}

// Validate validates the bulk send request
func (r *BulkSendRequest) Validate() error {
	if r.AppID == "" {
		return ErrInvalidAppID
	}
	if len(r.UserIDs) == 0 {
		return ErrInvalidUserID
	}
	if !r.Channel.Valid() {
		return ErrInvalidChannel
	}
	if !r.Priority.Valid() {
		return ErrInvalidPriority
	}
	if r.TemplateID == "" && (r.Title == "" || r.Body == "") {
		return ErrEmptyContent
	}
	if r.ScheduledAt != nil && r.ScheduledAt.Before(time.Now()) {
		return ErrInvalidScheduleTime
	}
	return nil
}
