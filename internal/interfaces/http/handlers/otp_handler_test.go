package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/the-monkeys/freerangenotify/internal/domain/otp"
	"github.com/the-monkeys/freerangenotify/pkg/validator"
)

// ─── stub OTP service ──────────────────────────────────────────────────────

type stubOTPService struct {
	sendResult   *otp.SendResult
	sendErr      error
	verifyResult *otp.VerifyResult
	verifyErr    error
	resendResult *otp.SendResult
	resendErr    error

	lastSendInput   otp.SendInput
	lastVerifyInput otp.VerifyInput
	lastResendID    string
}

var _ otp.Service = (*stubOTPService)(nil)

func (s *stubOTPService) Send(_ context.Context, in otp.SendInput) (*otp.SendResult, error) {
	s.lastSendInput = in
	return s.sendResult, s.sendErr
}
func (s *stubOTPService) Verify(_ context.Context, in otp.VerifyInput) (*otp.VerifyResult, error) {
	s.lastVerifyInput = in
	return s.verifyResult, s.verifyErr
}
func (s *stubOTPService) Resend(_ context.Context, id string) (*otp.SendResult, error) {
	s.lastResendID = id
	return s.resendResult, s.resendErr
}

// ─── test app helper ────────────────────────────────────────────────────────

func setupOTPApp(t *testing.T, svc *stubOTPService, appID string) *fiber.App {
	t.Helper()
	logger := zaptest.NewLogger(t)
	h := NewOTPHandler(svc, validator.New(), logger)

	app := fiber.New()
	if appID != "" {
		app.Use(func(c *fiber.Ctx) error {
			c.Locals("app_id", appID)
			return c.Next()
		})
	}
	app.Post("/v1/otp/send", h.Send)
	app.Post("/v1/otp/verify", h.Verify)
	app.Post("/v1/otp/resend", h.Resend)
	return app
}

func doOTPRequest(t *testing.T, app *fiber.App, method, path string, body interface{}) (*http.Response, map[string]interface{}) {
	t.Helper()
	var payload []byte
	if s, ok := body.(string); ok {
		payload = []byte(s)
	} else {
		payload, _ = json.Marshal(body)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, 5000)
	require.NoError(t, err)
	var out map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	resp.Body.Close()
	return resp, out
}

// ─── Send ──────────────────────────────────────────────────────────────────

func TestOTPHandler_Send_Success(t *testing.T) {
	expires := time.Now().Add(5 * time.Minute)
	svc := &stubOTPService{
		sendResult: &otp.SendResult{
			RequestID:      "req-1",
			NotificationID: "notif-1",
			Channel:        otp.ChannelSMS,
			ExpiresAt:      expires,
			TTLSeconds:     300,
			MaxAttempts:    5,
		},
	}
	app := setupOTPApp(t, svc, "app-1")

	resp, body := doOTPRequest(t, app, http.MethodPost, "/v1/otp/send", map[string]interface{}{
		"channel":   "sms",
		"recipient": "+14155551212",
	})

	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
	assert.Equal(t, "req-1", body["request_id"])
	assert.Equal(t, "notif-1", body["notification_id"])
	assert.Equal(t, "sms", body["channel"])
	assert.Equal(t, "app-1", svc.lastSendInput.AppID)
	assert.Equal(t, otp.ChannelSMS, svc.lastSendInput.Channel)
	assert.Equal(t, "+14155551212", svc.lastSendInput.Recipient)
}

func TestOTPHandler_Send_MissingAppID_Returns401(t *testing.T) {
	svc := &stubOTPService{}
	app := setupOTPApp(t, svc, "") // no middleware

	resp, _ := doOTPRequest(t, app, http.MethodPost, "/v1/otp/send", map[string]interface{}{
		"channel":   "sms",
		"recipient": "+14155551212",
	})
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestOTPHandler_Send_BadJSON_Returns400(t *testing.T) {
	svc := &stubOTPService{}
	app := setupOTPApp(t, svc, "app-1")
	resp, _ := doOTPRequest(t, app, http.MethodPost, "/v1/otp/send", "{not-json")
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestOTPHandler_Send_ValidationFails_Returns400(t *testing.T) {
	svc := &stubOTPService{}
	app := setupOTPApp(t, svc, "app-1")
	// missing required `channel` + `recipient`
	resp, _ := doOTPRequest(t, app, http.MethodPost, "/v1/otp/send", map[string]interface{}{})
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestOTPHandler_Send_ErrorMapping(t *testing.T) {
	cases := []struct {
		name string
		err  error
		code int
	}{
		{"not_found", otp.ErrNotFound, http.StatusNotFound},
		{"expired", otp.ErrExpired, http.StatusGone},
		{"already_verified", otp.ErrAlreadyVerified, http.StatusConflict},
		{"resend_cooldown", otp.ErrResendCooldown, http.StatusConflict},
		{"rate_limited", otp.ErrRateLimited, http.StatusTooManyRequests},
		{"invalid_channel", otp.ErrInvalidChannel, http.StatusBadRequest},
		{"invalid_recipient", otp.ErrInvalidRecipient, http.StatusBadRequest},
		{"invalid_length", otp.ErrInvalidLength, http.StatusBadRequest},
		{"invalid_ttl", otp.ErrInvalidTTL, http.StatusBadRequest},
		{"invalid_attempts", otp.ErrInvalidAttempts, http.StatusBadRequest},
		{"template_missing_code", otp.ErrTemplateMissingCode, http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &stubOTPService{sendErr: tc.err}
			app := setupOTPApp(t, svc, "app-1")
			resp, _ := doOTPRequest(t, app, http.MethodPost, "/v1/otp/send", map[string]interface{}{
				"channel":   "sms",
				"recipient": "+14155551212",
			})
			assert.Equal(t, tc.code, resp.StatusCode)
		})
	}
}

func TestOTPHandler_Send_UnknownError_Returns500(t *testing.T) {
	svc := &stubOTPService{sendErr: assertAnyError{}}
	app := setupOTPApp(t, svc, "app-1")
	resp, _ := doOTPRequest(t, app, http.MethodPost, "/v1/otp/send", map[string]interface{}{
		"channel":   "sms",
		"recipient": "+14155551212",
	})
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

// ─── Verify ────────────────────────────────────────────────────────────────

func TestOTPHandler_Verify_Success(t *testing.T) {
	now := time.Now().UTC()
	svc := &stubOTPService{
		verifyResult: &otp.VerifyResult{Verified: true, VerifiedAt: &now},
	}
	app := setupOTPApp(t, svc, "app-1")

	resp, body := doOTPRequest(t, app, http.MethodPost, "/v1/otp/verify", map[string]interface{}{
		"request_id": "req-1",
		"code":       "123456",
	})
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, true, body["verified"])
	assert.Equal(t, "req-1", body["request_id"])
	assert.NotEmpty(t, body["verified_at"])
	assert.Equal(t, "req-1", svc.lastVerifyInput.RequestID)
	assert.Equal(t, "123456", svc.lastVerifyInput.Code)
}

func TestOTPHandler_Verify_MissingAppID_Returns401(t *testing.T) {
	svc := &stubOTPService{}
	app := setupOTPApp(t, svc, "")
	resp, _ := doOTPRequest(t, app, http.MethodPost, "/v1/otp/verify", map[string]interface{}{
		"request_id": "req-1",
		"code":       "123456",
	})
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestOTPHandler_Verify_BadJSON_Returns400(t *testing.T) {
	svc := &stubOTPService{}
	app := setupOTPApp(t, svc, "app-1")
	resp, _ := doOTPRequest(t, app, http.MethodPost, "/v1/otp/verify", "{bad")
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestOTPHandler_Verify_ValidationFails_Returns400(t *testing.T) {
	svc := &stubOTPService{}
	app := setupOTPApp(t, svc, "app-1")
	// missing both fields
	resp, _ := doOTPRequest(t, app, http.MethodPost, "/v1/otp/verify", map[string]interface{}{})
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestOTPHandler_Verify_InvalidCode(t *testing.T) {
	svc := &stubOTPService{
		verifyResult: &otp.VerifyResult{Verified: false, AttemptsRemaining: 3},
		verifyErr:    otp.ErrInvalidCode,
	}
	app := setupOTPApp(t, svc, "app-1")
	resp, body := doOTPRequest(t, app, http.MethodPost, "/v1/otp/verify", map[string]interface{}{
		"request_id": "req-1",
		"code":       "000000",
	})
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, false, body["verified"])
	assert.Equal(t, "invalid_code", body["error"])
	assert.Equal(t, float64(3), body["attempts_remaining"])
}

func TestOTPHandler_Verify_AttemptsExhausted(t *testing.T) {
	svc := &stubOTPService{verifyErr: otp.ErrAttemptsExhausted}
	app := setupOTPApp(t, svc, "app-1")
	resp, body := doOTPRequest(t, app, http.MethodPost, "/v1/otp/verify", map[string]interface{}{
		"request_id": "req-1",
		"code":       "000000",
	})
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "attempts_exhausted", body["error"])
}

func TestOTPHandler_Verify_Expired(t *testing.T) {
	svc := &stubOTPService{verifyErr: otp.ErrExpired}
	app := setupOTPApp(t, svc, "app-1")
	resp, body := doOTPRequest(t, app, http.MethodPost, "/v1/otp/verify", map[string]interface{}{
		"request_id": "req-1",
		"code":       "123456",
	})
	assert.Equal(t, http.StatusGone, resp.StatusCode)
	assert.Equal(t, "expired", body["error"])
}

func TestOTPHandler_Verify_NotFound(t *testing.T) {
	svc := &stubOTPService{verifyErr: otp.ErrNotFound}
	app := setupOTPApp(t, svc, "app-1")
	resp, body := doOTPRequest(t, app, http.MethodPost, "/v1/otp/verify", map[string]interface{}{
		"request_id": "req-missing",
		"code":       "123456",
	})
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Equal(t, "not_found", body["error"])
}

func TestOTPHandler_Verify_UnknownError_Returns500(t *testing.T) {
	svc := &stubOTPService{verifyErr: assertAnyError{}}
	app := setupOTPApp(t, svc, "app-1")
	resp, body := doOTPRequest(t, app, http.MethodPost, "/v1/otp/verify", map[string]interface{}{
		"request_id": "req-1",
		"code":       "123456",
	})
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.Equal(t, "internal_error", body["error"])
}

// ─── Resend ────────────────────────────────────────────────────────────────

func TestOTPHandler_Resend_Success(t *testing.T) {
	svc := &stubOTPService{
		resendResult: &otp.SendResult{
			RequestID:      "req-1",
			NotificationID: "notif-2",
			Channel:        otp.ChannelEmail,
			ExpiresAt:      time.Now().Add(5 * time.Minute),
			TTLSeconds:     300,
			MaxAttempts:    5,
		},
	}
	app := setupOTPApp(t, svc, "app-1")
	resp, body := doOTPRequest(t, app, http.MethodPost, "/v1/otp/resend", map[string]interface{}{
		"request_id": "req-1",
	})
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
	assert.Equal(t, "req-1", body["request_id"])
	assert.Equal(t, "notif-2", body["notification_id"])
	assert.Equal(t, "req-1", svc.lastResendID)
}

func TestOTPHandler_Resend_MissingAppID_Returns401(t *testing.T) {
	svc := &stubOTPService{}
	app := setupOTPApp(t, svc, "")
	resp, _ := doOTPRequest(t, app, http.MethodPost, "/v1/otp/resend", map[string]interface{}{
		"request_id": "req-1",
	})
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestOTPHandler_Resend_BadJSON_Returns400(t *testing.T) {
	svc := &stubOTPService{}
	app := setupOTPApp(t, svc, "app-1")
	resp, _ := doOTPRequest(t, app, http.MethodPost, "/v1/otp/resend", "{")
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestOTPHandler_Resend_ValidationFails_Returns400(t *testing.T) {
	svc := &stubOTPService{}
	app := setupOTPApp(t, svc, "app-1")
	resp, _ := doOTPRequest(t, app, http.MethodPost, "/v1/otp/resend", map[string]interface{}{})
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestOTPHandler_Resend_ErrorMapping(t *testing.T) {
	cases := []struct {
		name string
		err  error
		code int
	}{
		{"not_found", otp.ErrNotFound, http.StatusNotFound},
		{"already_verified", otp.ErrAlreadyVerified, http.StatusConflict},
		{"cooldown", otp.ErrResendCooldown, http.StatusConflict},
		{"rate_limited", otp.ErrRateLimited, http.StatusTooManyRequests},
		{"expired", otp.ErrExpired, http.StatusGone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc := &stubOTPService{resendErr: tc.err}
			app := setupOTPApp(t, svc, "app-1")
			resp, _ := doOTPRequest(t, app, http.MethodPost, "/v1/otp/resend", map[string]interface{}{
				"request_id": "req-1",
			})
			assert.Equal(t, tc.code, resp.StatusCode)
		})
	}
}

// ─── helpers ───────────────────────────────────────────────────────────────

// assertAnyError is a sentinel error type that does NOT match any of the
// otp.Err* sentinels, used to exercise the default 500 branch.
type assertAnyError struct{}

func (assertAnyError) Error() string { return "boom" }
