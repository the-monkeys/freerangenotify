package dto

// OTPSendRequest is the POST /v1/otp/send payload. AppID is taken from the
// authenticated API key (Locals), not the body.
//
// Recipient targeting: provide EXACTLY ONE of `recipient`, `user_id`, or
// `external_id`. When `user_id` or `external_id` is supplied, the channel
// address (email or phone) is read from the user record stored in FRN.
type OTPSendRequest struct {
	Channel      string                 `json:"channel" validate:"required,oneof=sms whatsapp email"`
	Recipient    string                 `json:"recipient,omitempty"`     // raw email / E.164 phone
	UserID       string                 `json:"user_id,omitempty"`       // internal FRN user_id (UUID)
	ExternalID   string                 `json:"external_id,omitempty"`   // caller-owned user identifier
	Length       int                    `json:"length,omitempty"`        // 4-10, default 6
	Alphanumeric bool                   `json:"alphanumeric,omitempty"`  // default false (numeric)
	TTLSeconds   int                    `json:"ttl_seconds,omitempty"`   // 30-900, default 300
	MaxAttempts  int                    `json:"max_attempts,omitempty"`  // 1-10, default 5
	TemplateBody string                 `json:"template_body,omitempty"` // optional BYO body with {{code}}
	TemplateData map[string]interface{} `json:"template_data,omitempty"`
}

// OTPSendResponse is returned for both /v1/otp/send and /v1/otp/resend. The
// notification_id correlates with the underlying notification record for
// audit and delivery-status lookups. The plaintext code is never returned.
type OTPSendResponse struct {
	RequestID      string `json:"request_id"`
	NotificationID string `json:"notification_id"`
	Channel        string `json:"channel"`
	ExpiresAt      string `json:"expires_at"`
	TTLSeconds     int    `json:"ttl_seconds"`
	MaxAttempts    int    `json:"max_attempts"`
}

// OTPVerifyRequest is the POST /v1/otp/verify payload.
type OTPVerifyRequest struct {
	RequestID string `json:"request_id" validate:"required"`
	Code      string `json:"code" validate:"required,min=4,max=10"`
}

// OTPVerifyResponse is the verify outcome. On failure, Error carries a
// machine-readable code (e.g. "invalid_code", "expired", "attempts_exhausted")
// and AttemptsRemaining tells the caller how many tries are left.
type OTPVerifyResponse struct {
	Verified          bool   `json:"verified"`
	RequestID         string `json:"request_id"`
	VerifiedAt        string `json:"verified_at,omitempty"`
	AttemptsRemaining int    `json:"attempts_remaining,omitempty"`
	Error             string `json:"error,omitempty"`
}

// OTPResendRequest is the POST /v1/otp/resend payload.
type OTPResendRequest struct {
	RequestID string `json:"request_id" validate:"required"`
}
