package handlers

import (
	"errors"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"testing"
)

func TestOTPAdmissionErrorIsClientError(t *testing.T) {
	svc := &stubOTPService{sendErr: &notification.AdmissionError{Message: "approved WhatsApp template required", Cause: errors.New("private-provider-credential")}}
	app := setupOTPApp(t, svc, "app-a")
	resp, body := doOTPRequest(t, app, "POST", "/v1/otp/send", map[string]interface{}{"channel": "whatsapp", "recipient": "+14155551234"})
	require.Equal(t, 400, resp.StatusCode)
	require.Equal(t, "approved WhatsApp template required", body["error"])
}
