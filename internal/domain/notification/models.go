package notification

import (
	"context"
	"time"
)

// Channel represents notification delivery channels
type Channel string

const (
	ChannelPush    Channel = "push"
	ChannelEmail   Channel = "email"
	ChannelSMS     Channel = "sms"
	ChannelWebhook Channel = "webhook"
)

// Priority represents notification priority levels
type Priority string

const (
	PriorityLow      Priority = "low"
	PriorityNormal   Priority = "normal"
	PriorityHigh     Priority = "high"
	PriorityCritical Priority = "critical"
)

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
)

// Notification represents a notification entity
type Notification struct {
	NotificationID string                 `json:"notification_id" es:"notification_id"`
	AppID          string                 `json:"app_id" es:"app_id"`
	UserID         string                 `json:"user_id" es:"user_id"`
	TemplateID     string                 `json:"template_id,omitempty" es:"template_id"`
	Channel        Channel                `json:"channel" es:"channel"`
	Priority       Priority               `json:"priority" es:"priority"`
	Status         Status                 `json:"status" es:"status"`
	Content        Content                `json:"content" es:"content"`
	Metadata       map[string]interface{} `json:"metadata,omitempty" es:"metadata"`
	ScheduledAt    *time.Time             `json:"scheduled_at,omitempty" es:"scheduled_at"`
	SentAt         *time.Time             `json:"sent_at,omitempty" es:"sent_at"`
	DeliveredAt    *time.Time             `json:"delivered_at,omitempty" es:"delivered_at"`
	ReadAt         *time.Time             `json:"read_at,omitempty" es:"read_at"`
	ErrorMessage   string                 `json:"error_message,omitempty" es:"error_message"`
	RetryCount     int                    `json:"retry_count" es:"retry_count"`
	CreatedAt      time.Time              `json:"created_at" es:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at" es:"updated_at"`
}

// Content represents notification content
type Content struct {
	Title string                 `json:"title" es:"title"`
	Body  string                 `json:"body" es:"body"`
	Data  map[string]interface{} `json:"data,omitempty" es:"data"`
}

// NotificationFilter represents query filters for notifications
type NotificationFilter struct {
	AppID     string    `json:"app_id,omitempty"`
	UserID    string    `json:"user_id,omitempty"`
	Channel   string    `json:"channel,omitempty"`
	Status    string    `json:"status,omitempty"`
	Priority  string    `json:"priority,omitempty"`
	DateFrom  time.Time `json:"date_from,omitempty"`
	DateTo    time.Time `json:"date_to,omitempty"`
	SortBy    string    `json:"sort_by,omitempty"`
	SortOrder string    `json:"sort_order,omitempty"`
	Limit     int       `json:"limit,omitempty"`
	Offset    int       `json:"offset,omitempty"`
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
}

// Service defines the business logic interface for notifications
type Service interface {
	Send(ctx context.Context, req *SendRequest) (*Notification, error)
	GetByID(ctx context.Context, id string) (*Notification, error)
	List(ctx context.Context, filter *NotificationFilter) ([]*Notification, error)
	Cancel(ctx context.Context, id string) error
	Retry(ctx context.Context, id string) error
}

// SendRequest represents a request to send a notification
type SendRequest struct {
	AppID       string                 `json:"app_id" validate:"required"`
	Users       []string               `json:"users" validate:"required,min=1"`
	Channels    []Channel              `json:"channels" validate:"required,min=1"`
	TemplateID  string                 `json:"template_id,omitempty"`
	Content     Content                `json:"content"`
	Priority    Priority               `json:"priority"`
	ScheduledAt *time.Time             `json:"scheduled_at,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}
