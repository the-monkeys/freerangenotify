package freerangenotify

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Send ───────────────────────────────────────────────────────────────────

func TestOTPClient_Send_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/otp/send", r.URL.Path)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		var body OTPSendParams
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, OTPChannelSMS, body.Channel)
		assert.Equal(t, "+14155551212", body.Recipient)

		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(OTPSendResult{
			RequestID:      "req-1",
			NotificationID: "notif-1",
			Channel:        OTPChannelSMS,
			ExpiresAt:      "2026-05-24T12:35:00Z",
			TTLSeconds:     300,
			MaxAttempts:    5,
		})
	}))
	defer ts.Close()

	c := New("test-key", WithBaseURL(ts.URL+"/v1"))
	res, err := c.OTP.Send(context.Background(), OTPSendParams{
		Channel:   OTPChannelSMS,
		Recipient: "+14155551212",
	})
	require.NoError(t, err)
	assert.Equal(t, "req-1", res.RequestID)
	assert.Equal(t, "notif-1", res.NotificationID)
	assert.Equal(t, OTPChannelSMS, res.Channel)
	assert.Equal(t, 300, res.TTLSeconds)
}

func TestOTPClient_Send_AllOptionalFieldsForwarded(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "email", body["channel"])
		assert.Equal(t, float64(8), body["length"])
		assert.Equal(t, true, body["alphanumeric"])
		assert.Equal(t, float64(600), body["ttl_seconds"])
		assert.Equal(t, float64(3), body["max_attempts"])
		assert.Equal(t, "Your code is {{code}}", body["template_body"])
		assert.Equal(t, map[string]interface{}{"app_name": "Acme"}, body["template_data"])
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"request_id":"r","channel":"email"}`))
	}))
	defer ts.Close()

	c := New("test-key", WithBaseURL(ts.URL+"/v1"))
	_, err := c.OTP.Send(context.Background(), OTPSendParams{
		Channel:      OTPChannelEmail,
		Recipient:    "user@example.com",
		Length:       8,
		Alphanumeric: true,
		TTLSeconds:   600,
		MaxAttempts:  3,
		TemplateBody: "Your code is {{code}}",
		TemplateData: map[string]interface{}{"app_name": "Acme"},
	})
	require.NoError(t, err)
}

func TestOTPClient_Send_OmitsZeroValues(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		_, hasLen := body["length"]
		_, hasTTL := body["ttl_seconds"]
		_, hasMax := body["max_attempts"]
		_, hasTpl := body["template_body"]
		assert.False(t, hasLen, "length should be omitted when zero")
		assert.False(t, hasTTL, "ttl_seconds should be omitted when zero")
		assert.False(t, hasMax, "max_attempts should be omitted when zero")
		assert.False(t, hasTpl, "template_body should be omitted when empty")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"request_id":"r"}`))
	}))
	defer ts.Close()

	c := New("test-key", WithBaseURL(ts.URL+"/v1"))
	_, err := c.OTP.Send(context.Background(), OTPSendParams{
		Channel:   OTPChannelSMS,
		Recipient: "+14155551212",
	})
	require.NoError(t, err)
}

func TestOTPClient_Send_RateLimited(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
	}))
	defer ts.Close()

	c := New("test-key", WithBaseURL(ts.URL+"/v1"))
	res, err := c.OTP.Send(context.Background(), OTPSendParams{Channel: OTPChannelSMS, Recipient: "+1"})
	assert.Nil(t, res)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrOTPRateLimited))

	// Original *APIError must still be reachable via errors.As.
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, http.StatusTooManyRequests, apiErr.StatusCode)
}

func TestOTPClient_Send_ValidationError_UnchangedAPIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid recipient"}`))
	}))
	defer ts.Close()

	c := New("test-key", WithBaseURL(ts.URL+"/v1"))
	_, err := c.OTP.Send(context.Background(), OTPSendParams{Channel: OTPChannelSMS, Recipient: "bogus"})
	require.Error(t, err)
	// 400 is not classified by the SDK — caller inspects *APIError.
	assert.False(t, errors.Is(err, ErrOTPRateLimited))
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.True(t, apiErr.IsValidationError())
}

// ─── Verify ─────────────────────────────────────────────────────────────────

func TestOTPClient_Verify_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/otp/verify", r.URL.Path)
		var body OTPVerifyParams
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "req-1", body.RequestID)
		assert.Equal(t, "482913", body.Code)

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"verified":true,"request_id":"req-1","verified_at":"2026-05-24T12:31:42Z"}`))
	}))
	defer ts.Close()

	c := New("test-key", WithBaseURL(ts.URL+"/v1"))
	res, err := c.OTP.Verify(context.Background(), OTPVerifyParams{RequestID: "req-1", Code: "482913"})
	require.NoError(t, err)
	assert.True(t, res.Verified)
	assert.Equal(t, "2026-05-24T12:31:42Z", res.VerifiedAt)
}

func TestOTPClient_Verify_InvalidCode_ReturnsSentinelAndAttemptsRemaining(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"verified":false,"request_id":"req-1","error":"invalid_code","attempts_remaining":3}`))
	}))
	defer ts.Close()

	c := New("test-key", WithBaseURL(ts.URL+"/v1"))
	res, err := c.OTP.Verify(context.Background(), OTPVerifyParams{RequestID: "req-1", Code: "000000"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrOTPInvalidCode))
	require.NotNil(t, res)
	assert.False(t, res.Verified)
	assert.Equal(t, 3, res.AttemptsRemaining)
}

func TestOTPClient_Verify_AttemptsExhausted(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"verified":false,"error":"attempts_exhausted"}`))
	}))
	defer ts.Close()

	c := New("test-key", WithBaseURL(ts.URL+"/v1"))
	_, err := c.OTP.Verify(context.Background(), OTPVerifyParams{RequestID: "req-1", Code: "000000"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrOTPAttemptsExhausted))
	// Must NOT also match ErrOTPInvalidCode.
	assert.False(t, errors.Is(err, ErrOTPInvalidCode))
}

func TestOTPClient_Verify_Expired(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusGone)
		_, _ = w.Write([]byte(`{"verified":false,"error":"expired"}`))
	}))
	defer ts.Close()

	c := New("test-key", WithBaseURL(ts.URL+"/v1"))
	_, err := c.OTP.Verify(context.Background(), OTPVerifyParams{RequestID: "req-1", Code: "1"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrOTPExpired))
}

func TestOTPClient_Verify_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"verified":false,"error":"not_found"}`))
	}))
	defer ts.Close()

	c := New("test-key", WithBaseURL(ts.URL+"/v1"))
	_, err := c.OTP.Verify(context.Background(), OTPVerifyParams{RequestID: "missing", Code: "1"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrOTPNotFound))
}

func TestOTPClient_Verify_StatusFallback_NoErrorDiscriminator(t *testing.T) {
	// When the body doesn't carry the `error` field (older servers / proxies),
	// the SDK must still classify via HTTP status for the unambiguous cases.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusGone)
		_, _ = w.Write([]byte(`{"error":"OTP expired"}`))
	}))
	defer ts.Close()

	c := New("test-key", WithBaseURL(ts.URL+"/v1"))
	_, err := c.OTP.Verify(context.Background(), OTPVerifyParams{RequestID: "r", Code: "1"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrOTPExpired))
}

func TestOTPClient_Verify_ServerError_UnchangedAPIError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer ts.Close()

	c := New("test-key", WithBaseURL(ts.URL+"/v1"))
	_, err := c.OTP.Verify(context.Background(), OTPVerifyParams{RequestID: "r", Code: "1"})
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrOTPInvalidCode))
	assert.False(t, errors.Is(err, ErrOTPExpired))
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, 500, apiErr.StatusCode)
}

// ─── Resend ─────────────────────────────────────────────────────────────────

func TestOTPClient_Resend_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/otp/resend", r.URL.Path)
		var body map[string]string
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		assert.Equal(t, "req-1", body["request_id"])
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"request_id":"req-1","notification_id":"notif-2","channel":"sms"}`))
	}))
	defer ts.Close()

	c := New("test-key", WithBaseURL(ts.URL+"/v1"))
	res, err := c.OTP.Resend(context.Background(), "req-1")
	require.NoError(t, err)
	assert.Equal(t, "req-1", res.RequestID)
	assert.Equal(t, "notif-2", res.NotificationID)
}

func TestOTPClient_Resend_ErrorMapping(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		body     string
		sentinel error
	}{
		{"not_found", http.StatusNotFound, `{"error":"not found"}`, ErrOTPNotFound},
		{"expired", http.StatusGone, `{"error":"expired"}`, ErrOTPExpired},
		{"already_verified", http.StatusConflict, `{"error":"already verified"}`, ErrOTPAlreadyVerified},
		{"cooldown", http.StatusConflict, `{"error":"resend cooldown active"}`, ErrOTPResendCooldown},
		{"rate_limited", http.StatusTooManyRequests, `{"error":"rate limited"}`, ErrOTPRateLimited},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer ts.Close()

			c := New("test-key", WithBaseURL(ts.URL+"/v1"))
			_, err := c.OTP.Resend(context.Background(), "req-1")
			require.Error(t, err)
			assert.True(t, errors.Is(err, tc.sentinel), "expected sentinel %v, got %v", tc.sentinel, err)
		})
	}
}

// ─── Wiring ─────────────────────────────────────────────────────────────────

func TestOTPClient_IsWiredIntoClient(t *testing.T) {
	c := New("test-key")
	require.NotNil(t, c.OTP)
	assert.Same(t, c, c.OTP.client)
}
