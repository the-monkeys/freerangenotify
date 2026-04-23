package freerangenotify

import "time"

// ── Notification Types ──

// NotificationSendParams holds parameters for sending a standard notification.
type NotificationSendParams struct {
	UserID        string                 `json:"user_id"`
	Channel       string                 `json:"channel,omitempty"`
	Priority      string                 `json:"priority,omitempty"`
	Title         string                 `json:"title,omitempty"`
	Body          string                 `json:"body,omitempty"`
	Data          map[string]interface{} `json:"data,omitempty"`
	TemplateID    string                 `json:"template_id,omitempty"`
	Category      string                 `json:"category,omitempty"`
	ScheduledAt   *time.Time             `json:"scheduled_at,omitempty"`
	WebhookURL    string                 `json:"webhook_url,omitempty"`
	WebhookTarget string                 `json:"webhook_target,omitempty"`
}

// SendParams holds parameters for Quick-Send.
type SendParams struct {
	To          string                 `json:"to"`
	Template    string                 `json:"template,omitempty"`
	Subject     string                 `json:"subject,omitempty"`
	Body        string                 `json:"body,omitempty"`
	Data        map[string]interface{} `json:"data,omitempty"`
	Channel     string                 `json:"channel,omitempty"`
	Priority    string                 `json:"priority,omitempty"`
	ScheduledAt *time.Time             `json:"scheduled_at,omitempty"`
}

// SendResult is the response from a send or quick-send operation.
type SendResult struct {
	NotificationID string `json:"notification_id"`
	Status         string `json:"status"`
	UserID         string `json:"user_id"`
	Channel        string `json:"channel"`
}

// BulkSendParams holds parameters for sending a notification to multiple users.
type BulkSendParams struct {
	UserIDs    []string               `json:"user_ids"`
	Channel    string                 `json:"channel,omitempty"`
	Priority   string                 `json:"priority,omitempty"`
	Title      string                 `json:"title,omitempty"`
	Body       string                 `json:"body,omitempty"`
	Data       map[string]interface{} `json:"data,omitempty"`
	TemplateID string                 `json:"template_id,omitempty"`
	Category   string                 `json:"category,omitempty"`
}

// BulkSendResult is the response from a bulk or batch send operation.
type BulkSendResult struct {
	Sent  int          `json:"sent"`
	Total int          `json:"total"`
	Items []SendResult `json:"items"`
}

// BroadcastParams holds parameters for broadcasting a notification to all users.
type BroadcastParams struct {
	Template string                 `json:"template_id"`
	Data     map[string]interface{} `json:"data,omitempty"`
	Channel  string                 `json:"channel,omitempty"`
	Priority string                 `json:"priority,omitempty"`
}

// BroadcastResult is the response from a broadcast operation.
type BroadcastResult struct {
	TotalSent     int          `json:"total_sent"`
	Notifications []SendResult `json:"notifications"`
}

// NotificationResponse represents a full notification object from the API.
type NotificationResponse struct {
	NotificationID string               `json:"notification_id"`
	AppID          string               `json:"app_id"`
	UserID         string               `json:"user_id"`
	Channel        string               `json:"channel"`
	Priority       string               `json:"priority"`
	Status         string               `json:"status"`
	Content        *NotificationContent `json:"content"`
	TemplateID     string               `json:"template_id,omitempty"`
	Category       string               `json:"category,omitempty"`
	ScheduledAt    *time.Time           `json:"scheduled_at,omitempty"`
	SentAt         *time.Time           `json:"sent_at,omitempty"`
	DeliveredAt    *time.Time           `json:"delivered_at,omitempty"`
	ReadAt         *time.Time           `json:"read_at,omitempty"`
	FailedAt       *time.Time           `json:"failed_at,omitempty"`
	SnoozedUntil   *time.Time           `json:"snoozed_until,omitempty"`
	ArchivedAt     *time.Time           `json:"archived_at,omitempty"`
	ErrorMessage   string               `json:"error_message,omitempty"`
	RetryCount     int                  `json:"retry_count"`
	CreatedAt      time.Time            `json:"created_at"`
	UpdatedAt      time.Time            `json:"updated_at"`
}

// NotificationContent holds the rendered content of a notification.
type NotificationContent struct {
	Title    string                 `json:"title"`
	Body     string                 `json:"body"`
	Data     map[string]interface{} `json:"data,omitempty"`
	MediaURL string                 `json:"media_url,omitempty"`

	// Rich webhook fields
	Attachments []ContentAttachment `json:"attachments,omitempty"`
	Actions     []ContentAction     `json:"actions,omitempty"`
	Fields      []ContentField      `json:"fields,omitempty"`
	Mentions    []ContentMention    `json:"mentions,omitempty"`
	Poll        *ContentPoll        `json:"poll,omitempty"`
	Style       *ContentStyle       `json:"style,omitempty"`
}

// ContentAttachment describes a media or file attachment.
type ContentAttachment struct {
	Type     string `json:"type"` // image | video | file | audio
	URL      string `json:"url"`
	Name     string `json:"name,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
	Size     int64  `json:"size,omitempty"`
	AltText  string `json:"alt_text,omitempty"`
}

// ContentAction describes a call-to-action button.
type ContentAction struct {
	Type  string `json:"type"` // link | submit | dismiss
	Label string `json:"label"`
	URL   string `json:"url,omitempty"`
	Value string `json:"value,omitempty"`
	Style string `json:"style,omitempty"` // primary | danger | default
}

// ContentField is a key/value pair rendered as embed fields / FactSet entries.
type ContentField struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

// ContentMention represents a platform-specific user/channel mention.
type ContentMention struct {
	Platform   string `json:"platform"`
	PlatformID string `json:"platform_id"`
	Display    string `json:"display,omitempty"`
}

// ContentPoll defines a multiple-choice poll.
type ContentPoll struct {
	Question      string              `json:"question"`
	Choices       []ContentPollChoice `json:"choices"`
	MultiSelect   bool                `json:"multi_select,omitempty"`
	DurationHours int                 `json:"duration_hours,omitempty"`
}

// ContentPollChoice is one selectable option in a Poll.
type ContentPollChoice struct {
	Label string `json:"label"`
	Emoji string `json:"emoji,omitempty"`
}

// ContentStyle carries presentation hints.
type ContentStyle struct {
	Severity string `json:"severity,omitempty"` // info | success | warning | danger
	Color    string `json:"color,omitempty"`    // hex override
}

// NotificationListResponse is a paginated list of notifications.
type NotificationListResponse struct {
	Notifications []NotificationResponse `json:"notifications"`
	Total         int                    `json:"total"`
	Page          int                    `json:"page"`
	PageSize      int                    `json:"page_size"`
}

// ── User Types ──

// CreateUserParams holds parameters for registering a new user.
type CreateUserParams struct {
	FullName    string       `json:"full_name,omitempty"`
	Email       string       `json:"email,omitempty"`
	Phone       string       `json:"phone,omitempty"`
	Timezone    string       `json:"timezone,omitempty"`
	Language    string       `json:"language,omitempty"`
	ExternalID  string       `json:"external_id,omitempty"`
	WebhookURL  string       `json:"webhook_url,omitempty"`
	Preferences *Preferences `json:"preferences,omitempty"`
}

// UpdateUserParams holds parameters for updating an existing user.
type UpdateUserParams struct {
	FullName    string       `json:"full_name,omitempty"`
	ExternalID  string       `json:"external_id,omitempty"`
	Email       string       `json:"email,omitempty"`
	Phone       string       `json:"phone,omitempty"`
	Timezone    string       `json:"timezone,omitempty"`
	Language    string       `json:"language,omitempty"`
	WebhookURL  string       `json:"webhook_url,omitempty"`
	Preferences *Preferences `json:"preferences,omitempty"`
}

// User is a FreeRangeNotify user profile.
type User struct {
	UserID      string       `json:"user_id"`
	AppID       string       `json:"app_id"`
	ExternalID  string       `json:"external_id"`
	FullName    string       `json:"full_name"`
	Email       string       `json:"email"`
	Phone       string       `json:"phone"`
	Timezone    string       `json:"timezone"`
	Language    string       `json:"language"`
	WebhookURL  string       `json:"webhook_url"`
	Preferences *Preferences `json:"preferences,omitempty"`
	Devices     []Device     `json:"devices,omitempty"`
	CreatedAt   string       `json:"created_at"`
	UpdatedAt   string       `json:"updated_at"`
}

// UserListResponse is a paginated list of users.
type UserListResponse struct {
	Users      []User `json:"users"`
	TotalCount int    `json:"total_count"`
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
}

// BulkCreateUsersResult is the response from a bulk user creation.
type BulkCreateUsersResult struct {
	Created int      `json:"created"`
	Total   int      `json:"total"`
	Errors  []string `json:"errors,omitempty"`
}

// BulkCreateUsersParams holds parameters for a bulk user creation or upsert.
type BulkCreateUsersParams struct {
	Users        []CreateUserParams `json:"users"`
	SkipExisting bool               `json:"skip_existing,omitempty"`
	Upsert       bool               `json:"upsert,omitempty"`
}

// Preferences holds user notification preferences.
type Preferences struct {
	EmailEnabled    *bool                         `json:"email_enabled,omitempty"`
	PushEnabled     *bool                         `json:"push_enabled,omitempty"`
	SMSEnabled      *bool                         `json:"sms_enabled,omitempty"`
	SlackEnabled    *bool                         `json:"slack_enabled,omitempty"`
	DiscordEnabled  *bool                         `json:"discord_enabled,omitempty"`
	WhatsAppEnabled *bool                         `json:"whatsapp_enabled,omitempty"`
	QuietHours      *QuietHours                   `json:"quiet_hours,omitempty"`
	DND             bool                          `json:"dnd,omitempty"`
	Categories      map[string]CategoryPreference `json:"categories,omitempty"`
	DailyLimit      int                           `json:"daily_limit,omitempty"`
}

// QuietHours defines the start and end of quiet hours (HH:MM format).
type QuietHours struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// CategoryPreference represents per-category notification settings.
type CategoryPreference struct {
	Enabled         bool     `json:"enabled"`
	EnabledChannels []string `json:"enabled_channels,omitempty"`
}

// AddDeviceParams holds parameters for registering a device token.
type AddDeviceParams struct {
	Platform string `json:"platform"`
	Token    string `json:"token"`
}

// Device represents a registered push notification device.
type Device struct {
	DeviceID     string `json:"device_id"`
	Platform     string `json:"platform"`
	Active       bool   `json:"active"`
	RegisteredAt string `json:"registered_at"`
}

// ── Template Types ──

// CreateTemplateParams holds parameters for creating a template.
type CreateTemplateParams struct {
	AppID         string                 `json:"app_id"`
	Name          string                 `json:"name"`
	Description   string                 `json:"description,omitempty"`
	Channel       string                 `json:"channel"`
	WebhookTarget string                 `json:"webhook_target,omitempty"`
	Subject       string                 `json:"subject,omitempty"`
	Body          string                 `json:"body"`
	Variables     []string               `json:"variables,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Locale        string                 `json:"locale,omitempty"`
	CreatedBy     string                 `json:"created_by,omitempty"`
}

// UpdateTemplateParams holds parameters for updating a template.
type UpdateTemplateParams struct {
	Description   string                 `json:"description,omitempty"`
	WebhookTarget string                 `json:"webhook_target,omitempty"`
	Subject       string                 `json:"subject,omitempty"`
	Body          string                 `json:"body,omitempty"`
	Variables     []string               `json:"variables,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Status        string                 `json:"status,omitempty"`
	Locale        string                 `json:"locale,omitempty"`
	UpdatedBy     string                 `json:"updated_by,omitempty"`
}

// CreateVersionParams holds parameters for creating a new template version.
type CreateVersionParams struct {
	Description string                 `json:"description,omitempty"`
	Subject     string                 `json:"subject,omitempty"`
	Body        string                 `json:"body,omitempty"`
	Variables   []string               `json:"variables,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Locale      string                 `json:"locale,omitempty"`
	CreatedBy   string                 `json:"created_by,omitempty"`
}

// CloneTemplateParams holds parameters for cloning a library template.
type CloneTemplateParams struct {
	AppID string `json:"app_id"`
}

// Template represents a notification template.
type Template struct {
	ID            string                 `json:"id"`
	AppID         string                 `json:"app_id"`
	Name          string                 `json:"name"`
	Description   string                 `json:"description"`
	Channel       string                 `json:"channel"`
	WebhookTarget string                 `json:"webhook_target,omitempty"`
	Subject       string                 `json:"subject"`
	Body          string                 `json:"body"`
	Variables     []string               `json:"variables"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Controls      []TemplateControl      `json:"controls,omitempty"`
	ControlValues ControlValues          `json:"control_values,omitempty"`
	Version       int                    `json:"version"`
	Status        string                 `json:"status"`
	Locale        string                 `json:"locale"`
	CreatedBy     string                 `json:"created_by"`
	UpdatedBy     string                 `json:"updated_by"`
	CreatedAt     string                 `json:"created_at"`
	UpdatedAt     string                 `json:"updated_at"`
}

// TemplateListResponse is a paginated list of templates.
type TemplateListResponse struct {
	Templates []Template `json:"templates"`
	Total     int        `json:"total"`
	Limit     int        `json:"limit"`
	Offset    int        `json:"offset"`
}

// TemplateDiff shows the differences between two template versions.
type TemplateDiff struct {
	FromVersion int           `json:"from_version"`
	ToVersion   int           `json:"to_version"`
	Changes     []FieldChange `json:"changes"`
}

// FieldChange represents a single field-level change between versions.
type FieldChange struct {
	Field string      `json:"field"`
	From  interface{} `json:"from"`
	To    interface{} `json:"to"`
}

// ── Workflow Types ──

// CreateWorkflowParams holds parameters for creating a workflow.
type CreateWorkflowParams struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	TriggerID   string         `json:"trigger_id"`
	Steps       []WorkflowStep `json:"steps"`
}

// UpdateWorkflowParams holds parameters for updating a workflow.
type UpdateWorkflowParams struct {
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Steps       []WorkflowStep `json:"steps,omitempty"`
	Status      string         `json:"status,omitempty"`
}

// TriggerWorkflowParams holds parameters for triggering a workflow execution.
type TriggerWorkflowParams struct {
	TriggerID string                 `json:"trigger_id"`
	UserID    string                 `json:"user_id"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
}

// Workflow represents a notification workflow definition.
type Workflow struct {
	ID          string         `json:"id"`
	AppID       string         `json:"app_id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	TriggerID   string         `json:"trigger_id"`
	Steps       []WorkflowStep `json:"steps"`
	Status      string         `json:"status"`
	Version     int            `json:"version"`
	CreatedBy   string         `json:"created_by"`
	CreatedAt   string         `json:"created_at"`
	UpdatedAt   string         `json:"updated_at"`
}

// WorkflowStep represents a single step in a workflow.
type WorkflowStep struct {
	ID            string                 `json:"id"`
	Type          string                 `json:"type"` // "channel", "delay", "digest", "condition"
	Name          string                 `json:"name,omitempty"`
	Channel       string                 `json:"channel,omitempty"`
	TemplateID    string                 `json:"template_id,omitempty"`
	DelayDuration string                 `json:"delay_duration,omitempty"`
	DigestKey     string                 `json:"digest_key,omitempty"`
	DigestWindow  string                 `json:"digest_window,omitempty"`
	Condition     *StepCondition         `json:"condition,omitempty"`
	Config        map[string]interface{} `json:"config,omitempty"`
}

// StepCondition defines a conditional branch in a workflow.
type StepCondition struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
	OnTrue   string      `json:"on_true,omitempty"`
	OnFalse  string      `json:"on_false,omitempty"`
}

// WorkflowExecution represents a running or completed workflow execution.
type WorkflowExecution struct {
	ID            string                 `json:"id"`
	WorkflowID    string                 `json:"workflow_id"`
	AppID         string                 `json:"app_id"`
	UserID        string                 `json:"user_id"`
	TransactionID string                 `json:"transaction_id"`
	CurrentStepID string                 `json:"current_step_id"`
	Status        string                 `json:"status"`
	Payload       map[string]interface{} `json:"payload,omitempty"`
	StartedAt     string                 `json:"started_at"`
	CompletedAt   *string                `json:"completed_at,omitempty"`
	UpdatedAt     string                 `json:"updated_at"`
}

// WorkflowListResponse is a paginated list of workflows.
type WorkflowListResponse struct {
	Workflows []Workflow `json:"workflows"`
	Total     int        `json:"total"`
	Page      int        `json:"page"`
	PageSize  int        `json:"page_size"`
}

// ExecutionListResponse is a paginated list of workflow executions.
type ExecutionListResponse struct {
	Executions []WorkflowExecution `json:"executions"`
	Total      int                 `json:"total"`
	Page       int                 `json:"page"`
	PageSize   int                 `json:"page_size"`
}

// ── Topic Types ──

// CreateTopicParams holds parameters for creating a topic.
type CreateTopicParams struct {
	Name        string `json:"name"`
	Key         string `json:"key"`
	Description string `json:"description,omitempty"`
}

// Topic represents a notification topic for subscriber grouping.
type Topic struct {
	ID          string `json:"id"`
	AppID       string `json:"app_id"`
	Name        string `json:"name"`
	Key         string `json:"key"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// TopicListResponse is a paginated list of topics.
type TopicListResponse struct {
	Topics   []Topic `json:"topics"`
	Total    int     `json:"total"`
	Page     int     `json:"page"`
	PageSize int     `json:"page_size"`
}

// TopicSubscription represents a user's subscription to a topic.
type TopicSubscription struct {
	ID        string `json:"id"`
	TopicID   string `json:"topic_id"`
	UserID    string `json:"user_id"`
	CreatedAt string `json:"created_at"`
}

// SubscriberListResponse is a paginated list of topic subscribers.
type SubscriberListResponse struct {
	Subscribers []TopicSubscription `json:"subscribers"`
	Total       int                 `json:"total"`
	Page        int                 `json:"page"`
	PageSize    int                 `json:"page_size"`
}

// ── Presence Types ──

// CheckInParams holds parameters for checking in a user's presence.
type CheckInParams struct {
	UserID     string `json:"user_id"`
	WebhookURL string `json:"webhook_url,omitempty"`
}

// ── Content Control Types ──

// TemplateControl defines a single editable field for non-technical users.
type TemplateControl struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Type        string   `json:"type"`
	Default     string   `json:"default,omitempty"`
	Placeholder string   `json:"placeholder,omitempty"`
	Required    bool     `json:"required,omitempty"`
	Options     []string `json:"options,omitempty"`
	Group       string   `json:"group,omitempty"`
	HelpText    string   `json:"help_text,omitempty"`
}

// ControlValues holds the user-edited values for template controls.
type ControlValues map[string]interface{}

// ControlsResponse holds a template's control definitions and current values.
type ControlsResponse struct {
	Controls      []TemplateControl `json:"controls"`
	ControlValues ControlValues     `json:"control_values"`
}
