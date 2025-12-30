package dto

import "github.com/the-monkeys/freerangenotify/internal/domain/user"

// CreateUserRequest represents a request to create a new user
type CreateUserRequest struct {
	UserID      string            `json:"user_id" validate:"omitempty"`
	Email       string            `json:"email" validate:"omitempty,email"`
	Phone       string            `json:"phone" validate:"omitempty"`
	Timezone    string            `json:"timezone" validate:"omitempty"`
	Language    string            `json:"language" validate:"omitempty"`
	WebhookURL  string            `json:"webhook_url" validate:"omitempty,url"`
	Preferences *user.Preferences `json:"preferences,omitempty"`
}

// UpdateUserRequest represents a request to update a user
type UpdateUserRequest struct {
	Email       string            `json:"email" validate:"omitempty,email"`
	Phone       string            `json:"phone" validate:"omitempty"`
	Timezone    string            `json:"timezone" validate:"omitempty"`
	Language    string            `json:"language" validate:"omitempty"`
	WebhookURL  string            `json:"webhook_url" validate:"omitempty,url"`
	Preferences *user.Preferences `json:"preferences,omitempty"`
}

// AddDeviceRequest represents a request to add a device
type AddDeviceRequest struct {
	Platform string `json:"platform" validate:"required,oneof=ios android web"`
	Token    string `json:"token" validate:"required"`
}

// UpdatePreferencesRequest represents a request to update preferences
type UpdatePreferencesRequest struct {
	EmailEnabled *bool                              `json:"email_enabled"`
	PushEnabled  *bool                              `json:"push_enabled"`
	SMSEnabled   *bool                              `json:"sms_enabled"`
	QuietHours   *user.QuietHours                   `json:"quiet_hours,omitempty"`
	DND          bool                               `json:"dnd"`
	Categories   map[string]user.CategoryPreference `json:"categories,omitempty"`
	DailyLimit   int                                `json:"daily_limit"`
}

// UserResponse represents a user response
type UserResponse struct {
	UserID      string           `json:"user_id"`
	AppID       string           `json:"app_id"`
	Email       string           `json:"email,omitempty"`
	Phone       string           `json:"phone,omitempty"`
	Timezone    string           `json:"timezone,omitempty"`
	Language    string           `json:"language,omitempty"`
	WebhookURL  string           `json:"webhook_url,omitempty"`
	Preferences user.Preferences `json:"preferences"`
	Devices     []user.Device    `json:"devices,omitempty"`
	CreatedAt   string           `json:"created_at"`
	UpdatedAt   string           `json:"updated_at"`
}

// ListUsersResponse represents a paginated list of users
type ListUsersResponse struct {
	Users      []UserResponse `json:"users"`
	TotalCount int64          `json:"total_count"`
	Page       int            `json:"page"`
	PageSize   int            `json:"page_size"`
}

// DeviceResponse represents a device response
type DeviceResponse struct {
	DeviceID     string `json:"device_id"`
	Platform     string `json:"platform"`
	Active       bool   `json:"active"`
	RegisteredAt string `json:"registered_at"`
}

// ToUserResponse converts a user entity to a response DTO
func ToUserResponse(u *user.User) UserResponse {
	return UserResponse{
		UserID:      u.UserID,
		AppID:       u.AppID,
		Email:       u.Email,
		Phone:       u.Phone,
		Timezone:    u.Timezone,
		Language:    u.Language,
		WebhookURL:  u.WebhookURL,
		Preferences: u.Preferences,
		Devices:     u.Devices,
		CreatedAt:   u.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:   u.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
