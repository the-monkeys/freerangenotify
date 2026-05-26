package providers

import (
	"context"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/attachment"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
)

// Provider defines the interface for notification providers
type Provider interface {
	// Send sends a notification to a user
	Send(ctx context.Context, notif *notification.Notification, usr *user.User) (*Result, error)

	// GetName returns the provider name
	GetName() string

	// GetSupportedChannel returns the channel this provider supports
	GetSupportedChannel() notification.Channel

	// IsHealthy checks if the provider is healthy and operational
	IsHealthy(ctx context.Context) bool

	// Close closes the provider and releases resources
	Close() error
}

// Result represents the result of sending a notification
type Result struct {
	// Success indicates if the notification was sent successfully
	Success bool

	// ProviderMessageID is the ID returned by the provider
	ProviderMessageID string

	// DeliveryTime is the time taken to deliver the notification
	DeliveryTime time.Duration

	// Error contains any error that occurred
	Error error

	// ErrorType categorizes the error (network, auth, invalid, etc.)
	ErrorType string

	// Metadata contains provider-specific response data
	Metadata map[string]interface{}
}

// Config holds common configuration for all providers
type Config struct {
	// Timeout for provider operations
	Timeout time.Duration

	// MaxRetries for transient failures
	MaxRetries int

	// RetryDelay between retries
	RetryDelay time.Duration
}

// Error types for categorizing provider errors
const (
	ErrorTypeNetwork       = "network"
	ErrorTypeAuth          = "authentication"
	ErrorTypeInvalid       = "invalid_request"
	ErrorTypeRateLimit     = "rate_limit"
	ErrorTypeProviderAPI   = "provider_api"
	ErrorTypeTimeout       = "timeout"
	ErrorTypeConfiguration = "configuration"
	ErrorTypeUnknown       = "unknown"
)

// NewResult creates a successful result
func NewResult(providerMessageID string, deliveryTime time.Duration) *Result {
	return &Result{
		Success:           true,
		ProviderMessageID: providerMessageID,
		DeliveryTime:      deliveryTime,
		Metadata:          make(map[string]interface{}),
	}
}

// NewErrorResult creates an error result
func NewErrorResult(err error, errorType string) *Result {
	return &Result{
		Success:   false,
		Error:     err,
		ErrorType: errorType,
		Metadata:  make(map[string]interface{}),
	}
}

// Credential source constants for billing metering
const (
	CredSourceSystem   = "system"   // System .env credentials — we pay the carrier
	CredSourceBYOC     = "byoc"     // User's own credentials — they pay the carrier
	CredSourcePlatform = "platform" // No external cost (in-app, SSE, push)
)

// Provider Context Keys
type contextKey string

const (
	EmailConfigKey    contextKey = "email_config"
	WhatsAppConfigKey contextKey = "whatsapp_config"
	SMSConfigKey      contextKey = "sms_config"

	// AttachmentResolverKey carries an AttachmentResolveFunc into provider
	// Send paths so providers can materialise notification.Attachment specs
	// (URL / inline base64 / file_id) without taking a hard dependency on
	// the usecases or file-storage packages.
	AttachmentResolverKey contextKey = "attachment_resolver"
)

// AttachmentResolveFunc resolves a slice of notification.Attachment specs
// into byte-ready attachment.Resolved values. The closure is pre-bound to a
// specific tenant (app_id) by whoever populates the ctx — providers stay
// tenant-agnostic by design.
//
// On error the implementation MUST close any partially-resolved entries
// before returning so callers never see a leaked reader.
// On success the caller MUST invoke (*attachment.Resolved).Close on every
// returned entry (attachment.CloseAll is a convenient helper).
type AttachmentResolveFunc func(ctx context.Context, atts []notification.Attachment) ([]*attachment.Resolved, error)
