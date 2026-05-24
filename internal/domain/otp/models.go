// Package otp defines the public OTP-as-a-service domain.
//
// This is separate from the internal auth OTP used during FRN account
// registration (see internal/usecases/services/auth_service_impl.go) — that
// flow is for FRN's own users, while this package serves FRN customers who
// need to send and verify OTPs to their own end users via SMS, WhatsApp, or
// email.
package otp

import (
	"context"
	"errors"
	"time"
)

// Channel enumerates the delivery channels supported by the OTP service.
type Channel string

const (
	ChannelSMS      Channel = "sms"
	ChannelWhatsApp Channel = "whatsapp"
	ChannelEmail    Channel = "email"
)

// Valid reports whether the channel is a supported OTP delivery channel.
func (c Channel) Valid() bool {
	switch c {
	case ChannelSMS, ChannelWhatsApp, ChannelEmail:
		return true
	}
	return false
}

// Defaults applied when the caller omits a tuneable parameter.
const (
	DefaultCodeLength       = 6
	DefaultTTLSeconds       = 300
	DefaultMaxAttempts      = 5
	DefaultResendCooldownS  = 60
	MinCodeLength           = 4
	MaxCodeLength           = 10
	MaxTTLSeconds           = 15 * 60 // 15 minutes upper bound to prevent stale codes
	MaxAttemptsCap          = 10
	MinAttempts             = 1
)

// Sentinel errors. Kept stable so callers (handlers, tests) can use
// errors.Is() for branch logic.
var (
	ErrInvalidChannel      = errors.New("otp: invalid channel")
	ErrInvalidRecipient    = errors.New("otp: invalid recipient")
	ErrInvalidLength       = errors.New("otp: invalid code length")
	ErrInvalidTTL          = errors.New("otp: invalid ttl")
	ErrInvalidAttempts     = errors.New("otp: invalid max_attempts")
	ErrTemplateMissingCode = errors.New("otp: template body must contain {{code}} placeholder")
	ErrNotFound            = errors.New("otp: request not found or expired")
	ErrExpired             = errors.New("otp: code expired")
	ErrAttemptsExhausted   = errors.New("otp: maximum verification attempts exhausted")
	ErrInvalidCode         = errors.New("otp: invalid code")
	ErrAlreadyVerified     = errors.New("otp: code already verified")
	ErrResendCooldown      = errors.New("otp: resend cooldown in effect")
	ErrRateLimited         = errors.New("otp: rate limit exceeded for recipient")
	// ErrAmbiguousRecipient is returned when the caller supplies more than one
	// of {recipient, user_id, external_id}. The API contract requires exactly one.
	ErrAmbiguousRecipient = errors.New("otp: provide exactly one of recipient, user_id, or external_id")
	// ErrUserNotFound is returned when a user_id or external_id lookup misses.
	ErrUserNotFound = errors.New("otp: user not found")
	// ErrUserMissingChannelAddress is returned when a resolved user lacks the
	// contact field required by the requested channel (e.g. channel=email but
	// the user record has no email).
	ErrUserMissingChannelAddress = errors.New("otp: resolved user has no address for the requested channel")
)

// Request is the persisted OTP envelope. CodeHash and Salt together let us
// verify a code in constant time without ever storing the plaintext.
type Request struct {
	RequestID    string    `json:"request_id"`
	AppID        string    `json:"app_id"`
	Channel      Channel   `json:"channel"`
	Recipient    string    `json:"recipient"` // E.164 phone or RFC-5322 email
	CodeHash     string    `json:"code_hash"` // hex-encoded SHA-256(code || salt)
	Salt         string    `json:"salt"`      // hex-encoded random 16 bytes
	Length       int       `json:"length"`
	Alphanumeric bool      `json:"alphanumeric"`
	Attempts     int       `json:"attempts"`
	MaxAttempts  int       `json:"max_attempts"`
	ExpiresAt    time.Time `json:"expires_at"`
	LastSentAt   time.Time `json:"last_sent_at"`
	Verified     bool      `json:"verified"`
	VerifiedAt   *time.Time `json:"verified_at,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// SendInput is the use-case input for OTPService.Send. The caller must
// identify the recipient using EXACTLY ONE of the three identifier fields:
//   - Recipient: raw email / E.164 phone (no user lookup; auto-creates a user)
//   - UserID:    internal FRN user_id (UUID); the channel address is read off the record
//   - ExternalID: caller-owned user identifier; resolved via the users index
// When UserID or ExternalID is used, the contact field corresponding to the
// channel is read from the user record (Email for email; Phone for sms/whatsapp).
// Validation lives on the DTO layer (interfaces/http/dto/otp_dto.go) so the
// service can assume inputs are syntactically well-formed and focus on policy.
type SendInput struct {
	AppID        string
	Channel      Channel
	Recipient    string // mutually exclusive with UserID / ExternalID
	UserID       string // mutually exclusive with Recipient / ExternalID
	ExternalID   string // mutually exclusive with Recipient / UserID
	Length       int    // 0 → DefaultCodeLength
	Alphanumeric bool
	TTLSeconds   int                    // 0 → DefaultTTLSeconds
	MaxAttempts  int                    // 0 → DefaultMaxAttempts
	TemplateBody string                 // optional BYO body containing {{code}}; empty → system default
	TemplateData map[string]interface{} // extra template vars (e.g. {{app_name}})
}

// SendResult is what SendOTP returns once the underlying notification has
// been queued. NotificationID is included so callers can correlate the OTP
// with the notification pipeline for audit / delivery-status purposes.
type SendResult struct {
	RequestID      string
	NotificationID string
	Channel        Channel
	ExpiresAt      time.Time
	TTLSeconds     int
	MaxAttempts    int
}

// VerifyInput is the use-case input for OTPService.Verify.
type VerifyInput struct {
	RequestID string
	Code      string
}

// VerifyResult reports the outcome of a verification attempt. AttemptsRemaining
// is included on failure so callers can render UX without an extra round-trip.
type VerifyResult struct {
	Verified           bool
	AttemptsRemaining  int
	VerifiedAt         *time.Time
}

// Service is the public OTP-as-a-service contract. Implementations must be
// safe for concurrent use.
type Service interface {
	Send(ctx context.Context, in SendInput) (*SendResult, error)
	Verify(ctx context.Context, in VerifyInput) (*VerifyResult, error)
	Resend(ctx context.Context, requestID string) (*SendResult, error)
}

// Repository is the storage contract. The Redis-backed implementation keys
// records by RequestID with a TTL matching ExpiresAt. Implementations must
// be atomic on IncrementAttempts to prevent brute-force races.
type Repository interface {
	Create(ctx context.Context, req *Request) error
	Get(ctx context.Context, requestID string) (*Request, error)
	IncrementAttempts(ctx context.Context, requestID string) (int, error)
	MarkVerified(ctx context.Context, requestID string, verifiedAt time.Time) error
	Update(ctx context.Context, req *Request) error
	Delete(ctx context.Context, requestID string) error

	// RecipientRateLimit increments and reads a per-app-per-recipient counter
	// over a rolling window. Returns the new count after increment.
	RecipientRateLimit(ctx context.Context, appID, recipient string, windowSeconds int) (int, error)
}
