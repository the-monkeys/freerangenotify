package user

import (
	"context"
	"time"
)

// User represents a user entity
type User struct {
	UserID         string      `json:"user_id" es:"user_id"`
	AppID          string      `json:"app_id" es:"app_id"`
	ExternalUserID string      `json:"external_user_id" es:"external_user_id"`
	Email          string      `json:"email,omitempty" es:"email"`
	Phone          string      `json:"phone,omitempty" es:"phone"`
	Timezone       string      `json:"timezone,omitempty" es:"timezone"`
	Language       string      `json:"language,omitempty" es:"language"`
	WebhookURL     string      `json:"webhook_url,omitempty" es:"webhook_url"`
	Preferences    Preferences `json:"preferences" es:"preferences"`
	Devices        []Device    `json:"devices,omitempty" es:"devices"`
	CreatedAt      time.Time   `json:"created_at" es:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at" es:"updated_at"`
}

// Preferences represents user notification preferences
type Preferences struct {
	EmailEnabled *bool                         `json:"email_enabled" es:"email_enabled"`
	PushEnabled  *bool                         `json:"push_enabled" es:"push_enabled"`
	SMSEnabled   *bool                         `json:"sms_enabled" es:"sms_enabled"`
	QuietHours   QuietHours                    `json:"quiet_hours" es:"quiet_hours"`
	DND          bool                          `json:"dnd" es:"dnd"`                 // Global Do Not Disturb
	Categories   map[string]CategoryPreference `json:"categories" es:"categories"`   // Category-specific preferences
	DailyLimit   int                           `json:"daily_limit" es:"daily_limit"` // Max notifications per day
}

// CategoryPreference represents preferences for a specific notification category
type CategoryPreference struct {
	Enabled         bool     `json:"enabled" es:"enabled"`
	EnabledChannels []string `json:"enabled_channels" es:"enabled_channels"`
}

// QuietHours represents user's do-not-disturb hours
type QuietHours struct {
	Start string `json:"start" es:"start"` // Format: "HH:MM"
	End   string `json:"end" es:"end"`     // Format: "HH:MM"
}

// Device represents a user's device for push notifications
type Device struct {
	DeviceID     string    `json:"device_id" es:"device_id"`
	Platform     string    `json:"platform" es:"platform"` // ios, android, web
	Token        string    `json:"token" es:"token"`
	Active       bool      `json:"active" es:"active"`
	RegisteredAt time.Time `json:"registered_at" es:"registered_at"`
	LastSeen     time.Time `json:"last_seen" es:"last_seen"`
}

// UserFilter represents query filters for users
type UserFilter struct {
	AppID          string `json:"app_id,omitempty"`
	ExternalUserID string `json:"external_user_id,omitempty"`
	Email          string `json:"email,omitempty"`
	Timezone       string `json:"timezone,omitempty"`
	Language       string `json:"language,omitempty"`
	Limit          int    `json:"limit,omitempty"`
	Offset         int    `json:"offset,omitempty"`
}

// Repository defines the interface for user data operations
type Repository interface {
	Create(ctx context.Context, user *User) error
	GetByID(ctx context.Context, id string) (*User, error)
	GetByExternalID(ctx context.Context, appID, externalUserID string) (*User, error)
	GetByEmail(ctx context.Context, appID, email string) (*User, error)
	Update(ctx context.Context, user *User) error
	List(ctx context.Context, filter UserFilter) ([]*User, error)
	Delete(ctx context.Context, id string) error
	AddDevice(ctx context.Context, userID string, device Device) error
	RemoveDevice(ctx context.Context, userID, deviceID string) error
	UpdatePreferences(ctx context.Context, userID string, preferences Preferences) error
	Count(ctx context.Context, filter UserFilter) (int64, error)
	BulkCreate(ctx context.Context, users []*User) error
}

// Service defines the business logic interface for users
type Service interface {
	Create(ctx context.Context, req *CreateRequest) (*User, error)
	GetByID(ctx context.Context, id string) (*User, error)
	GetByExternalID(ctx context.Context, appID, externalUserID string) (*User, error)
	Update(ctx context.Context, id string, req *UpdateRequest) (*User, error)
	Delete(ctx context.Context, id string) error
	RegisterDevice(ctx context.Context, userID string, device Device) error
	UnregisterDevice(ctx context.Context, userID, deviceID string) error
	UpdatePreferences(ctx context.Context, userID string, preferences Preferences) error
}

// CreateRequest represents a request to create a user
type CreateRequest struct {
	AppID          string      `json:"app_id" validate:"required"`
	ExternalUserID string      `json:"external_user_id" validate:"required"`
	Email          string      `json:"email,omitempty" validate:"omitempty,email"`
	Phone          string      `json:"phone,omitempty"`
	Timezone       string      `json:"timezone,omitempty"`
	Language       string      `json:"language,omitempty"`
	WebhookURL     string      `json:"webhook_url,omitempty" validate:"omitempty,url"`
	Preferences    Preferences `json:"preferences"`
}

// UpdateRequest represents a request to update a user
type UpdateRequest struct {
	Email       *string      `json:"email,omitempty" validate:"omitempty,email"`
	Phone       *string      `json:"phone,omitempty"`
	Timezone    *string      `json:"timezone,omitempty"`
	Language    *string      `json:"language,omitempty"`
	WebhookURL  *string      `json:"webhook_url,omitempty" validate:"omitempty,url"`
	Preferences *Preferences `json:"preferences,omitempty"`
}
