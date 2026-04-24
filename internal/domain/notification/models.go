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

// isWebhookLikeChannel returns true for channels that deliver to a URL
// endpoint rather than a specific user (webhook, discord, slack, teams).
func isWebhookLikeChannel(ch Channel) bool {
	switch ch {
	case ChannelWebhook, ChannelDiscord, ChannelSlack, ChannelTeams:
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
	StatusDigested   Status = "digested" // Batched into a digest and delivered via consolidated notification
)

// Valid checks if the status is valid
func (s Status) Valid() bool {
	switch s {
	case StatusPending, StatusQueued, StatusProcessing, StatusSent, StatusDelivered, StatusRead, StatusFailed, StatusCancelled, StatusSnoozed, StatusArchived, StatusDigested:
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
	return s == StatusDelivered || s == StatusRead || s == StatusFailed || s == StatusCancelled || s == StatusArchived || s == StatusDigested
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
	Title    string                 `json:"title" es:"title"`
	Body     string                 `json:"body" es:"body"`
	Data     map[string]interface{} `json:"data,omitempty" es:"data"`
	MediaURL string                 `json:"media_url,omitempty" es:"media_url"`

	// Rich webhook fields (Phase 7 — Webhook channel expansion).
	// All fields are optional and omitted when empty, preserving the legacy
	// JSON shape for back-compat. Per-platform renderers silently drop
	// fields their target does not support.
	Attachments []Attachment `json:"attachments,omitempty" es:"attachments"`
	Actions     []Action     `json:"actions,omitempty"     es:"actions"`
	Fields      []Field      `json:"fields,omitempty"      es:"fields"`
	Mentions    []Mention    `json:"mentions,omitempty"    es:"mentions"`
	Poll        *Poll        `json:"poll,omitempty"        es:"poll"`
	Style       *Style       `json:"style,omitempty"       es:"style"`
}

// Attachment describes a media or file attachment referenced by URL.
// Size is in bytes when known; zero means unknown/not validated.
type Attachment struct {
	Type     string `json:"type"                 es:"type"` // image | video | file | audio
	URL      string `json:"url"                  es:"url"`
	Name     string `json:"name,omitempty"       es:"name"`
	MimeType string `json:"mime_type,omitempty"  es:"mime_type"`
	Size     int64  `json:"size,omitempty"       es:"size"`
	AltText  string `json:"alt_text,omitempty"   es:"alt_text"`
}

// Action describes a call-to-action button.
// type=link renders as an OpenUrl / link button on all platforms.
// type=submit requires an inbound-webhook receiver (out of scope for
// Phase A of the webhook expansion).
type Action struct {
	Type  string `json:"type"                es:"type"` // link | submit | dismiss
	Label string `json:"label"               es:"label"`
	URL   string `json:"url,omitempty"       es:"url"`
	Value string `json:"value,omitempty"     es:"value"`
	Style string `json:"style,omitempty"     es:"style"` // primary | danger | default
}

// Field is a key/value pair rendered as Discord embed fields,
// Slack section fields, or Teams AdaptiveCard FactSet entries.
type Field struct {
	Key    string `json:"key"              es:"key"`
	Value  string `json:"value"            es:"value"`
	Inline bool   `json:"inline,omitempty" es:"inline"`
}

// Mention represents a platform-specific user / channel mention.
// PlatformID is the raw id expected by the target platform
// (e.g. Slack user id "U1234567" or Discord user id).
type Mention struct {
	Platform   string `json:"platform"           es:"platform"` // discord | slack | teams
	PlatformID string `json:"platform_id"        es:"platform_id"`
	Display    string `json:"display,omitempty"  es:"display"`
}

// Poll defines a multiple-choice poll.
// DurationHours is advisory; each platform clamps to its own limits.
type Poll struct {
	Question      string       `json:"question"                es:"question"`
	Choices       []PollChoice `json:"choices"                 es:"choices"`
	MultiSelect   bool         `json:"multi_select,omitempty"  es:"multi_select"`
	DurationHours int          `json:"duration_hours,omitempty" es:"duration_hours"`
}

// PollChoice is one selectable option in a Poll.
type PollChoice struct {
	Label string `json:"label"           es:"label"`
	Emoji string `json:"emoji,omitempty" es:"emoji"`
}

// Style carries presentation hints applied across renderers.
type Style struct {
	Severity string `json:"severity,omitempty" es:"severity"` // info | success | warning | danger
	Color    string `json:"color,omitempty"    es:"color"`    // hex override, e.g. "#3498DB"
}

// Validate checks rich-content invariants. It is a no-op when only the
// legacy fields (Title / Body / Data / MediaURL) are populated, so existing
// callers see no behavior change.
func (c *Content) Validate() error {
	if len(c.Attachments) > 10 {
		return ErrTooManyAttachments
	}
	for i := range c.Attachments {
		a := c.Attachments[i]
		if a.Type == "" || a.URL == "" {
			return ErrInvalidAttachment
		}
	}

	if len(c.Actions) > 5 {
		return ErrTooManyActions
	}
	for i := range c.Actions {
		a := c.Actions[i]
		if a.Type == "" || a.Label == "" {
			return ErrInvalidAction
		}
		if a.Type == "link" && a.URL == "" {
			return ErrInvalidAction
		}
	}

	if len(c.Fields) > 25 {
		return ErrTooManyFields
	}
	for i := range c.Fields {
		f := c.Fields[i]
		if f.Key == "" || f.Value == "" {
			return ErrInvalidField
		}
	}

	if c.Poll != nil {
		if c.Poll.Question == "" {
			return ErrInvalidPoll
		}
		if len(c.Poll.Choices) < 2 || len(c.Poll.Choices) > 10 {
			return ErrInvalidPoll
		}
		for i := range c.Poll.Choices {
			if c.Poll.Choices[i].Label == "" {
				return ErrInvalidPoll
			}
		}
	}

	return nil
}

// NotificationFilter represents query filters for notifications
type NotificationFilter struct {
	AppID             string                 `json:"app_id,omitempty"`
	AppIDs            []string               `json:"app_ids,omitempty"`
	EnvironmentID     string                 `json:"environment_id,omitempty"`
	UserID            string                 `json:"user_id,omitempty"`
	Channel           Channel                `json:"channel,omitempty"`
	Status            Status                 `json:"status,omitempty"`
	Priority          Priority               `json:"priority,omitempty"`
	TemplateID        string                 `json:"template_id,omitempty"`
	FromDate          *time.Time             `json:"from_date,omitempty"`
	ToDate            *time.Time             `json:"to_date,omitempty"`
	Category          string                 `json:"category,omitempty"`
	DigestKey         string                 `json:"digest_key,omitempty"`          // Filter by metadata.digest_key (for digest rule run history)
	ProviderMessageID string                 `json:"provider_message_id,omitempty"` // Filter by metadata.provider_message_id (delivery status webhooks)
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
	Page              int                    `json:"page,omitempty"`
	PageSize          int                    `json:"page_size,omitempty"`
	SortBy            string                 `json:"sort_by,omitempty"`
	SortOrder         string                 `json:"sort_order,omitempty"` // "asc" or "desc"
	Cursor            string                 `json:"cursor,omitempty"`     // opaque cursor for search_after pagination
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
	Count(ctx context.Context, filter NotificationFilter) (int64, error)
	UpdateStatus(ctx context.Context, notificationID string, status Status, errorMessage string, appID string) error
	Cancel(ctx context.Context, notificationID, appID string) error
	CancelBatch(ctx context.Context, notificationIDs []string, appID string) error
	Retry(ctx context.Context, notificationID, appID string) error
	FlushQueued(ctx context.Context, userID string) error
	GetUnreadCount(ctx context.Context, userID, appID string) (int64, error)
	ListUnread(ctx context.Context, userID, appID string) ([]*Notification, error)
	MarkRead(ctx context.Context, notificationIDs []string, appID, userID string) error
	Broadcast(ctx context.Context, req BroadcastRequest) (*BroadcastResult, error)
	// Phase 5: snooze, archive, mark-all-read
	Snooze(ctx context.Context, notificationID, appID string, until time.Time) error
	Unsnooze(ctx context.Context, notificationID, appID string) error
	Archive(ctx context.Context, notificationIDs []string, appID, userID string) error
	MarkAllRead(ctx context.Context, userID, appID, category string) error
	ListSnoozedDue(ctx context.Context) ([]*Notification, error)
}

// BroadcastRequest represents a request to send a notification to all users of an application
type BroadcastRequest struct {
	AppID             string                 `json:"app_id" validate:"required"`
	EnvironmentID     string                 `json:"environment_id,omitempty"`
	Channel           Channel                `json:"channel" validate:"required"`
	Priority          Priority               `json:"priority" validate:"required"`
	Title             string                 `json:"title,omitempty"`
	Body              string                 `json:"body,omitempty"`
	Data              map[string]interface{} `json:"data,omitempty"`
	TemplateID        string                 `json:"template_id"`
	Category          string                 `json:"category,omitempty"`
	ScheduledAt       *time.Time             `json:"scheduled_at,omitempty"`
	WorkflowTriggerID string                 `json:"workflow_trigger_id,omitempty"` // Phase 2: trigger workflow for each recipient instead of sending notification
	TopicKey          string                 `json:"topic_key,omitempty"`           // Phase 2: limit recipients to topic subscribers (by topic key)
	Metadata          map[string]interface{} `json:"metadata,omitempty"`            // Digest: {"digest_key": "rule_key"}
}

// BroadcastResult holds the result of a broadcast operation
type BroadcastResult struct {
	Notifications []*Notification // when sending notifications
	Triggered     int             // when triggering workflows
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
	// TemplateID required only when NOT triggering a workflow
	if r.WorkflowTriggerID == "" && r.TemplateID == "" {
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
		if !isWebhookLikeChannel(n.Channel) {
			return ErrInvalidUserID
		}
		// For webhook-like channels, check if we have the URL in metadata
		hasURL := false
		if n.Metadata != nil {
			if _, ok := n.Metadata["webhook_url"]; ok {
				hasURL = true
			}
		}

		// If no explicit URL, we must have a TemplateID which can resolve via provider config
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
	hasContentSid := n.Content.Data != nil && n.Content.Data["content_sid"] != nil
	if n.TemplateID == "" && n.Content.Title == "" && n.Content.Body == "" && !hasContentSid {
		return ErrEmptyContent
	}
	if err := n.Content.Validate(); err != nil {
		return err
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
	AppID             string                 `json:"app_id" validate:"required"`
	EnvironmentID     string                 `json:"environment_id,omitempty"`
	UserID            string                 `json:"user_id"` // Removed validate:"required"
	Channel           Channel                `json:"channel"` // Optional: inferred from template if empty
	Priority          Priority               `json:"priority" validate:"required"`
	Title             string                 `json:"title,omitempty"`
	Body              string                 `json:"body,omitempty"`
	Data              map[string]interface{} `json:"data,omitempty"`
	TemplateID        string                 `json:"template_id" validate:"required"`
	Category          string                 `json:"category,omitempty"`
	ScheduledAt       *time.Time             `json:"scheduled_at,omitempty"`
	Recurrence        *Recurrence            `json:"recurrence,omitempty"`
	TopicID           string                 `json:"topic_id,omitempty"`            // Phase 2: send to all subscribers of a topic
	WorkflowTriggerID string                 `json:"workflow_trigger_id,omitempty"` // Phase 3: trigger workflow after send
	Metadata          map[string]interface{} `json:"metadata,omitempty"`            // Digest: {"digest_key": "rule_key"} routes to digest batching
	MediaURL          string                 `json:"media_url,omitempty"`
}

// Validate validates the send request
func (r *SendRequest) Validate() error {
	if r.AppID == "" {
		return ErrInvalidAppID
	}
	// Conditional UserID validation — TopicID can substitute for UserID
	if r.UserID == "" && r.TopicID == "" {
		if !isWebhookLikeChannel(r.Channel) {
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
	if r.Priority == "" {
		r.Priority = PriorityNormal
	}
	if !r.Priority.Valid() {
		return ErrInvalidPriority
	}
	if r.TemplateID == "" && (r.Title == "" || r.Body == "") {
		// Allow Twilio Content Templates: content_sid in Data is sufficient
		hasContentSID := false
		if r.Data != nil {
			if _, ok := r.Data["content_sid"]; ok {
				hasContentSID = true
			}
		}
		if !hasContentSID {
			return ErrTemplateRequired
		}
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
	Metadata      map[string]interface{} `json:"metadata,omitempty"` // Digest: {"digest_key": "rule_key"}
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
	hasContentSid := r.Data != nil && r.Data["content_sid"] != nil
	if r.TemplateID == "" && (r.Title == "" || r.Body == "") && !hasContentSid {
		return ErrEmptyContent
	}
	if r.ScheduledAt != nil && r.ScheduledAt.Before(time.Now()) {
		return ErrInvalidScheduleTime
	}
	return nil
}
