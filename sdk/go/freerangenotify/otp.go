package freerangenotify

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

// ── OTPClient ─────────────────────────────────────────────────────────────

// OTPClient handles OTP-as-a-service operations (POST /v1/otp/*).
//
// Usage:
//
//	res, err := client.OTP.Send(ctx, freerangenotify.OTPSendParams{
//	    Channel:   freerangenotify.OTPChannelSMS,
//	    Recipient: "+14155551212",
//	})
//	if err != nil { /* ... */ }
//
//	verify, err := client.OTP.Verify(ctx, freerangenotify.OTPVerifyParams{
//	    RequestID: res.RequestID,
//	    Code:      userInput,
//	})
//	switch {
//	case errors.Is(err, freerangenotify.ErrOTPInvalidCode):
//	    // show "wrong code, X attempts left" using verify.AttemptsRemaining
//	case errors.Is(err, freerangenotify.ErrOTPExpired):
//	    // prompt user to request a new code
//	case err != nil:
//	    return err
//	default:
//	    // verify.Verified == true
//	}
type OTPClient struct {
	client *Client
}

// OTPChannel is the delivery channel for an OTP.
type OTPChannel string

const (
	OTPChannelSMS      OTPChannel = "sms"
	OTPChannelWhatsApp OTPChannel = "whatsapp"
	OTPChannelEmail    OTPChannel = "email"
)

// ── Params / Results ──────────────────────────────────────────────────────

// OTPSendParams is the request for OTP.Send. Exactly one of Recipient,
// UserID, or ExternalID must be supplied:
//   - Recipient:  raw email / E.164 phone. The user record (if any) is
//     auto-created on first use.
//   - UserID:     the internal FRN user_id (UUID). The channel-appropriate
//     contact address (email or phone) is read from the user record.
//   - ExternalID: the caller-owned user identifier. Same resolution behaviour
//     as UserID, but looked up via the users index.
type OTPSendParams struct {
	// Channel is the delivery channel. Required.
	Channel OTPChannel `json:"channel"`
	// Recipient is E.164 phone for sms/whatsapp or an RFC 5322 email address.
	// Mutually exclusive with UserID / ExternalID.
	Recipient string `json:"recipient,omitempty"`
	// UserID is the internal FRN user_id (UUID). Mutually exclusive with
	// Recipient / ExternalID.
	UserID string `json:"user_id,omitempty"`
	// ExternalID is the caller-owned user identifier. Mutually exclusive
	// with Recipient / UserID.
	ExternalID string `json:"external_id,omitempty"`

	// Length is the code length (4–10). 0 → server default (6).
	Length int `json:"length,omitempty"`
	// Alphanumeric draws codes from a lookalike-free alphabet instead of digits.
	Alphanumeric bool `json:"alphanumeric,omitempty"`
	// TTLSeconds is the code lifetime in seconds (30–900). 0 → server default (300).
	TTLSeconds int `json:"ttl_seconds,omitempty"`
	// MaxAttempts is the verify-attempt budget (1–10). 0 → server default (5).
	MaxAttempts int `json:"max_attempts,omitempty"`

	// TemplateBody is an optional bring-your-own message body containing {{code}}.
	// Empty → server uses a channel-appropriate default.
	TemplateBody string `json:"template_body,omitempty"`
	// TemplateData carries extra variables (e.g. {"app_name":"Acme"}) for TemplateBody.
	TemplateData map[string]interface{} `json:"template_data,omitempty"`
}

// OTPSendResult is the response from OTP.Send / OTP.Resend.
type OTPSendResult struct {
	RequestID      string     `json:"request_id"`
	NotificationID string     `json:"notification_id"`
	Channel        OTPChannel `json:"channel"`
	ExpiresAt      string     `json:"expires_at"`
	TTLSeconds     int        `json:"ttl_seconds"`
	MaxAttempts    int        `json:"max_attempts"`
}

// OTPVerifyParams is the request for OTP.Verify.
type OTPVerifyParams struct {
	RequestID string `json:"request_id"`
	Code      string `json:"code"`
}

// OTPVerifyResult is the response from OTP.Verify. On failure, the SDK returns
// (result, err) where result carries AttemptsRemaining and err matches one of
// the ErrOTP* sentinels (use errors.Is).
type OTPVerifyResult struct {
	Verified          bool   `json:"verified"`
	RequestID         string `json:"request_id"`
	VerifiedAt        string `json:"verified_at,omitempty"`
	AttemptsRemaining int    `json:"attempts_remaining,omitempty"`
	Error             string `json:"error,omitempty"`
}

// ── Sentinel errors ───────────────────────────────────────────────────────

// OTP-specific sentinel errors. Use errors.Is for matching:
//
//	if errors.Is(err, freerangenotify.ErrOTPInvalidCode) { ... }
//
// They are returned by OTP.Send / OTP.Verify / OTP.Resend whenever the API
// responds with a known OTP failure mode, regardless of HTTP status. Unknown
// failures fall through as *APIError.
var (
	ErrOTPInvalidCode       = errors.New("freerangenotify: invalid OTP code")
	ErrOTPAttemptsExhausted = errors.New("freerangenotify: OTP attempts exhausted")
	ErrOTPExpired           = errors.New("freerangenotify: OTP expired")
	ErrOTPNotFound          = errors.New("freerangenotify: OTP request not found")
	ErrOTPAlreadyVerified   = errors.New("freerangenotify: OTP already verified")
	ErrOTPResendCooldown    = errors.New("freerangenotify: OTP resend cooldown active")
	ErrOTPRateLimited       = errors.New("freerangenotify: OTP per-recipient rate limit exceeded")
)

// ── Operations ────────────────────────────────────────────────────────────

// Send generates an OTP, hashes it, and dispatches it via the requested channel.
// Returns ErrOTPRateLimited on 429.
func (o *OTPClient) Send(ctx context.Context, params OTPSendParams) (*OTPSendResult, error) {
	var result OTPSendResult
	err := o.client.do(ctx, "POST", "/otp/send", params, &result)
	if err != nil {
		return nil, classifyOTPSendError(err)
	}
	return &result, nil
}

// Verify checks a user-supplied code against the OTP identified by request_id.
//
// On invalid code, returns (result, ErrOTPInvalidCode) so callers can read
// result.AttemptsRemaining to drive UX. On hard failures (expired, not_found,
// attempts exhausted), the corresponding ErrOTP* sentinel is returned and the
// returned result may be partial. Unknown failures return a wrapped *APIError.
func (o *OTPClient) Verify(ctx context.Context, params OTPVerifyParams) (*OTPVerifyResult, error) {
	var result OTPVerifyResult
	err := o.client.do(ctx, "POST", "/otp/verify", params, &result)
	if err == nil {
		return &result, nil
	}
	// The verify endpoint returns its OTPVerifyResponse JSON even on 4xx, so
	// try to surface AttemptsRemaining alongside the sentinel error. We decode
	// the body that the transport already captured into result via the
	// pre-error path is not available, so re-do the request only if needed.
	// In practice the transport in client.go returns *APIError with Body set;
	// we parse it best-effort here.
	if apiErr, ok := err.(*APIError); ok {
		if parsed := parseVerifyBody(apiErr.Body); parsed != nil {
			result = *parsed
		}
	}
	return &result, classifyOTPVerifyError(err, result.Error)
}

// Resend re-issues a fresh code for an existing request_id (60 s cooldown).
// The previous code is invalidated and the attempt counter resets.
func (o *OTPClient) Resend(ctx context.Context, requestID string) (*OTPSendResult, error) {
	var result OTPSendResult
	err := o.client.do(ctx, "POST", "/otp/resend", map[string]string{"request_id": requestID}, &result)
	if err != nil {
		return nil, classifyOTPSendError(err)
	}
	return &result, nil
}

// ── Internal helpers ──────────────────────────────────────────────────────

// classifyOTPSendError maps a transport-level error to an OTP sentinel by
// matching on HTTP status + body. Unknown errors pass through unchanged.
func classifyOTPSendError(err error) error {
	apiErr, ok := err.(*APIError)
	if !ok {
		return err
	}
	body := apiErr.Body
	switch apiErr.StatusCode {
	case 404:
		return wrapAPIErr(apiErr, ErrOTPNotFound)
	case 410:
		return wrapAPIErr(apiErr, ErrOTPExpired)
	case 409:
		if strings.Contains(body, "already verified") {
			return wrapAPIErr(apiErr, ErrOTPAlreadyVerified)
		}
		if strings.Contains(body, "cooldown") {
			return wrapAPIErr(apiErr, ErrOTPResendCooldown)
		}
	case 429:
		return wrapAPIErr(apiErr, ErrOTPRateLimited)
	}
	return apiErr
}

// classifyOTPVerifyError maps verify-endpoint errors. The verify endpoint
// returns a structured body with an `error` discriminator that is more
// reliable than HTTP-status sniffing for the 400 cases (invalid_code vs
// attempts_exhausted both come back as 400).
func classifyOTPVerifyError(err error, errCode string) error {
	apiErr, ok := err.(*APIError)
	if !ok {
		return err
	}
	switch errCode {
	case "invalid_code":
		return wrapAPIErr(apiErr, ErrOTPInvalidCode)
	case "attempts_exhausted":
		return wrapAPIErr(apiErr, ErrOTPAttemptsExhausted)
	case "expired":
		return wrapAPIErr(apiErr, ErrOTPExpired)
	case "not_found":
		return wrapAPIErr(apiErr, ErrOTPNotFound)
	}
	// Fall back to status-based mapping for cases where the body discriminator
	// is missing (e.g. validation errors, 401, 500).
	switch apiErr.StatusCode {
	case 404:
		return wrapAPIErr(apiErr, ErrOTPNotFound)
	case 410:
		return wrapAPIErr(apiErr, ErrOTPExpired)
	}
	return apiErr
}

// wrappedOTPErr keeps the original *APIError available via errors.As while
// matching one of the ErrOTP* sentinels via errors.Is.
type wrappedOTPErr struct {
	sentinel error
	api      *APIError
}

func (w *wrappedOTPErr) Error() string { return w.sentinel.Error() + ": " + w.api.Error() }
func (w *wrappedOTPErr) Unwrap() error { return w.api }
func (w *wrappedOTPErr) Is(target error) bool {
	return target == w.sentinel
}

func wrapAPIErr(api *APIError, sentinel error) error {
	return &wrappedOTPErr{sentinel: sentinel, api: api}
}

// parseVerifyBody attempts to decode an OTPVerifyResult out of an error
// response body. Returns nil on failure — the caller falls back to the
// zero-value result.
func parseVerifyBody(body string) *OTPVerifyResult {
	if body == "" {
		return nil
	}
	var out OTPVerifyResult
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		return nil
	}
	if out.Error == "" && !out.Verified {
		return nil
	}
	return &out
}
