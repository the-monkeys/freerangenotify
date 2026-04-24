package providers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"go.uber.org/zap/zaptest"
)

// newTestWhatsAppProvider creates a provider pointing at a test server.
func newTestWhatsAppProvider(t *testing.T, serverURL string) *WhatsAppProvider {
	t.Helper()
	p, err := NewWhatsAppProvider(WhatsAppConfig{
		Config:     Config{Timeout: 5 * time.Second, MaxRetries: 0},
		AccountSID: "ACtest123",
		AuthToken:  "token456",
		FromNumber: "+14155238886",
	}, zaptest.NewLogger(t))
	require.NoError(t, err)
	// Override httpClient to route to test server (Twilio URL is built at
	// send time so we intercept via a custom transport).
	p.httpClient = &http.Client{
		Timeout:   5 * time.Second,
		Transport: &rewriteTransport{target: serverURL},
	}
	return p
}

// rewriteTransport redirects all requests to the test server URL.
type rewriteTransport struct {
	target string
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	parsed, _ := url.Parse(t.target)
	req.URL.Scheme = parsed.Scheme
	req.URL.Host = parsed.Host
	return http.DefaultTransport.RoundTrip(req)
}

func baseNotification() *notification.Notification {
	return &notification.Notification{
		NotificationID: "notif-001",
		Content: notification.Content{
			Title: "Hello",
			Body:  "World",
		},
	}
}

func baseUser() *user.User {
	return &user.User{
		UserID: "user-001",
		Phone:  "+15551234567",
	}
}

// twilioSuccess returns a 201 JSON body mimicking Twilio's Messages.json response.
func twilioSuccess(sid string) []byte {
	b, _ := json.Marshal(map[string]interface{}{
		"sid":    sid,
		"status": "queued",
	})
	return b
}

// ─── Nil / empty user ────────────────────────────────────────────────────────

func TestWhatsAppProvider_Send_NilUser(t *testing.T) {
	p, _ := NewWhatsAppProvider(WhatsAppConfig{
		AccountSID: "AC1", AuthToken: "tok", FromNumber: "+1",
	}, zaptest.NewLogger(t))

	result, err := p.Send(context.Background(), baseNotification(), nil)
	assert.NoError(t, err)
	assert.False(t, result.Success)
	assert.Equal(t, ErrorTypeInvalid, result.ErrorType)
	assert.Contains(t, result.Error.Error(), "no phone number")
}

func TestWhatsAppProvider_Send_EmptyPhone(t *testing.T) {
	p, _ := NewWhatsAppProvider(WhatsAppConfig{
		AccountSID: "AC1", AuthToken: "tok", FromNumber: "+1",
	}, zaptest.NewLogger(t))
	usr := &user.User{UserID: "u1", Phone: ""}

	result, err := p.Send(context.Background(), baseNotification(), usr)
	assert.NoError(t, err)
	assert.False(t, result.Success)
	assert.Equal(t, ErrorTypeInvalid, result.ErrorType)
}

// ─── Missing credentials ─────────────────────────────────────────────────────

func TestWhatsAppProvider_Send_MissingAccountSID(t *testing.T) {
	p, _ := NewWhatsAppProvider(WhatsAppConfig{
		AccountSID: "", AuthToken: "tok", FromNumber: "+1",
	}, zaptest.NewLogger(t))

	result, err := p.Send(context.Background(), baseNotification(), baseUser())
	assert.NoError(t, err)
	assert.False(t, result.Success)
	assert.Equal(t, ErrorTypeConfiguration, result.ErrorType)
	assert.Contains(t, result.Error.Error(), "credentials not configured")
}

func TestWhatsAppProvider_Send_MissingAuthToken(t *testing.T) {
	p, _ := NewWhatsAppProvider(WhatsAppConfig{
		AccountSID: "AC1", AuthToken: "", FromNumber: "+1",
	}, zaptest.NewLogger(t))

	result, err := p.Send(context.Background(), baseNotification(), baseUser())
	assert.NoError(t, err)
	assert.False(t, result.Success)
	assert.Equal(t, ErrorTypeConfiguration, result.ErrorType)
}

func TestWhatsAppProvider_Send_MissingFromNumber(t *testing.T) {
	p, _ := NewWhatsAppProvider(WhatsAppConfig{
		AccountSID: "AC1", AuthToken: "tok", FromNumber: "",
	}, zaptest.NewLogger(t))

	result, err := p.Send(context.Background(), baseNotification(), baseUser())
	assert.NoError(t, err)
	assert.False(t, result.Success)
	assert.Equal(t, ErrorTypeConfiguration, result.ErrorType)
	assert.Contains(t, result.Error.Error(), "sender number not configured")
}

// ─── Free-form text mode (successful) ────────────────────────────────────────

func TestWhatsAppProvider_Send_FreeFormText(t *testing.T) {
	var captured url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		captured, _ = url.ParseQuery(string(body))

		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

		// Verify basic auth
		u, p, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "ACtest123", u)
		assert.Equal(t, "token456", p)

		w.WriteHeader(http.StatusCreated)
		w.Write(twilioSuccess("SM123"))
	}))
	defer server.Close()

	p := newTestWhatsAppProvider(t, server.URL)
	notif := baseNotification()
	notif.Content.Title = "Order Update"
	notif.Content.Body = "Your order #123 has shipped."

	result, err := p.Send(context.Background(), notif, baseUser())

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "SM123", result.ProviderMessageID)
	assert.Equal(t, CredSourceSystem, result.Metadata["credential_source"])
	assert.Equal(t, "whatsapp", result.Metadata["billing_channel"])

	// Verify Twilio form data
	assert.Equal(t, "whatsapp:+15551234567", captured.Get("To"))
	assert.Equal(t, "whatsapp:+14155238886", captured.Get("From"))
	assert.Equal(t, "*Order Update*\n\nYour order #123 has shipped.", captured.Get("Body"))
	assert.Empty(t, captured.Get("ContentSid"))
}

func TestWhatsAppProvider_Send_FreeFormBodyOnly(t *testing.T) {
	var captured url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		captured, _ = url.ParseQuery(string(body))
		w.WriteHeader(http.StatusCreated)
		w.Write(twilioSuccess("SM124"))
	}))
	defer server.Close()

	p := newTestWhatsAppProvider(t, server.URL)
	notif := baseNotification()
	notif.Content.Title = "" // No title — body only
	notif.Content.Body = "Plain message"

	result, err := p.Send(context.Background(), notif, baseUser())

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "Plain message", captured.Get("Body"))
}

func TestWhatsAppProvider_Send_FreeFormWithMedia(t *testing.T) {
	var captured url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		captured, _ = url.ParseQuery(string(body))
		w.WriteHeader(http.StatusCreated)
		w.Write(twilioSuccess("SM125"))
	}))
	defer server.Close()

	p := newTestWhatsAppProvider(t, server.URL)
	notif := baseNotification()
	notif.Content.Title = ""
	notif.Content.Body = "Check this out"
	notif.Content.MediaURL = "https://example.com/image.jpg"

	result, err := p.Send(context.Background(), notif, baseUser())

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "Check this out", captured.Get("Body"))
	assert.Equal(t, "https://example.com/image.jpg", captured.Get("MediaUrl"))
}

// ─── Content Template mode ───────────────────────────────────────────────────

func TestWhatsAppProvider_Send_ContentTemplate_MapVariables(t *testing.T) {
	var captured url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		captured, _ = url.ParseQuery(string(body))
		w.WriteHeader(http.StatusCreated)
		w.Write(twilioSuccess("SM200"))
	}))
	defer server.Close()

	p := newTestWhatsAppProvider(t, server.URL)
	notif := baseNotification()
	notif.Content.Data = map[string]interface{}{
		"content_sid": "HX123abc",
		"content_variables": map[string]interface{}{
			"1": "Dave",
			"2": "12345",
		},
	}

	result, err := p.Send(context.Background(), notif, baseUser())

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "HX123abc", captured.Get("ContentSid"))
	assert.Empty(t, captured.Get("Body"), "Body should NOT be set in template mode")
	assert.Empty(t, captured.Get("MediaUrl"), "MediaUrl should NOT be set in template mode")

	// Parse content_variables JSON
	var vars map[string]string
	require.NoError(t, json.Unmarshal([]byte(captured.Get("ContentVariables")), &vars))
	assert.Equal(t, "Dave", vars["1"])
	assert.Equal(t, "12345", vars["2"])
}

func TestWhatsAppProvider_Send_ContentTemplate_StringVariables(t *testing.T) {
	var captured url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		captured, _ = url.ParseQuery(string(body))
		w.WriteHeader(http.StatusCreated)
		w.Write(twilioSuccess("SM201"))
	}))
	defer server.Close()

	p := newTestWhatsAppProvider(t, server.URL)
	notif := baseNotification()
	notif.Content.Data = map[string]interface{}{
		"content_sid":       "HX456def",
		"content_variables": `{"1":"Dave","2":"order-789"}`,
	}

	result, err := p.Send(context.Background(), notif, baseUser())

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "HX456def", captured.Get("ContentSid"))
	assert.Equal(t, `{"1":"Dave","2":"order-789"}`, captured.Get("ContentVariables"))
}

func TestWhatsAppProvider_Send_ContentTemplate_NoVariables(t *testing.T) {
	var captured url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		captured, _ = url.ParseQuery(string(body))
		w.WriteHeader(http.StatusCreated)
		w.Write(twilioSuccess("SM202"))
	}))
	defer server.Close()

	p := newTestWhatsAppProvider(t, server.URL)
	notif := baseNotification()
	notif.Content.Data = map[string]interface{}{
		"content_sid": "HX789",
	}

	result, err := p.Send(context.Background(), notif, baseUser())

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "HX789", captured.Get("ContentSid"))
	assert.Empty(t, captured.Get("ContentVariables"))
}

// ─── WhatsApp prefix handling ────────────────────────────────────────────────

func TestWhatsAppProvider_Send_PrefixAlreadyPresent(t *testing.T) {
	var captured url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		captured, _ = url.ParseQuery(string(body))
		w.WriteHeader(http.StatusCreated)
		w.Write(twilioSuccess("SM300"))
	}))
	defer server.Close()

	p := newTestWhatsAppProvider(t, server.URL)
	// FromNumber already has prefix
	p.config.FromNumber = "whatsapp:+14155238886"

	usr := &user.User{UserID: "u2", Phone: "whatsapp:+15559998888"}

	result, err := p.Send(context.Background(), baseNotification(), usr)

	require.NoError(t, err)
	assert.True(t, result.Success)
	// Should NOT double-prefix
	assert.Equal(t, "whatsapp:+15559998888", captured.Get("To"))
	assert.Equal(t, "whatsapp:+14155238886", captured.Get("From"))
}

// ─── Per-app credential override (BYOC) ──────────────────────────────────────

func TestWhatsAppProvider_Send_PerAppCredentials(t *testing.T) {
	var capturedAuth [2]string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, _ := r.BasicAuth()
		capturedAuth = [2]string{u, p}

		body, _ := io.ReadAll(r.Body)
		vals, _ := url.ParseQuery(string(body))
		assert.Equal(t, "whatsapp:+19998887777", vals.Get("From"))

		w.WriteHeader(http.StatusCreated)
		w.Write(twilioSuccess("SM400"))
	}))
	defer server.Close()

	p := newTestWhatsAppProvider(t, server.URL)

	appCfg := &application.WhatsAppAppConfig{
		AccountSID: "ACappOverride",
		AuthToken:  "appTokenOverride",
		FromNumber: "+19998887777",
	}
	ctx := context.WithValue(context.Background(), WhatsAppConfigKey, appCfg)

	result, err := p.Send(ctx, baseNotification(), baseUser())

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "ACappOverride", capturedAuth[0])
	assert.Equal(t, "appTokenOverride", capturedAuth[1])
	assert.Equal(t, CredSourceBYOC, result.Metadata["credential_source"])
}

func TestWhatsAppProvider_Send_PerAppCredentials_PartialOverride(t *testing.T) {
	// Per-app config with empty auth token — should fall back to global creds
	var capturedAuth [2]string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, _ := r.BasicAuth()
		capturedAuth = [2]string{u, p}
		w.WriteHeader(http.StatusCreated)
		w.Write(twilioSuccess("SM401"))
	}))
	defer server.Close()

	p := newTestWhatsAppProvider(t, server.URL)

	appCfg := &application.WhatsAppAppConfig{
		AccountSID: "ACpartial",
		AuthToken:  "", // empty — should NOT override
	}
	ctx := context.WithValue(context.Background(), WhatsAppConfigKey, appCfg)

	result, err := p.Send(ctx, baseNotification(), baseUser())

	require.NoError(t, err)
	assert.True(t, result.Success)
	// Should use global creds since AuthToken is empty
	assert.Equal(t, "ACtest123", capturedAuth[0])
	assert.Equal(t, "token456", capturedAuth[1])
	assert.Equal(t, CredSourceSystem, result.Metadata["credential_source"])
}

func TestWhatsAppProvider_Send_PerAppCredentials_NoFromNumber(t *testing.T) {
	// Per-app config without FromNumber — should keep global from number
	var captured url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		captured, _ = url.ParseQuery(string(body))
		w.WriteHeader(http.StatusCreated)
		w.Write(twilioSuccess("SM402"))
	}))
	defer server.Close()

	p := newTestWhatsAppProvider(t, server.URL)

	appCfg := &application.WhatsAppAppConfig{
		AccountSID: "ACappNoFrom",
		AuthToken:  "appToken2",
		FromNumber: "", // empty — keep global
	}
	ctx := context.WithValue(context.Background(), WhatsAppConfigKey, appCfg)

	result, err := p.Send(ctx, baseNotification(), baseUser())

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "whatsapp:+14155238886", captured.Get("From"))
}

// ─── Twilio error responses ──────────────────────────────────────────────────

func TestWhatsAppProvider_Send_TwilioErrorCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error_code":    63016,
			"error_message": "Template not found",
		})
	}))
	defer server.Close()

	p := newTestWhatsAppProvider(t, server.URL)

	result, err := p.Send(context.Background(), baseNotification(), baseUser())

	assert.NoError(t, err)
	assert.False(t, result.Success)
	assert.Equal(t, ErrorTypeProviderAPI, result.ErrorType)
	assert.Contains(t, result.Error.Error(), "63016")
	assert.Contains(t, result.Error.Error(), "Template not found")
}

func TestWhatsAppProvider_Send_TwilioCodeMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code":    20003,
			"message": "Authenticate",
		})
	}))
	defer server.Close()

	p := newTestWhatsAppProvider(t, server.URL)

	result, err := p.Send(context.Background(), baseNotification(), baseUser())

	assert.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error.Error(), "20003")
	assert.Contains(t, result.Error.Error(), "Authenticate")
}

func TestWhatsAppProvider_Send_TwilioPlainMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "Permission denied",
		})
	}))
	defer server.Close()

	p := newTestWhatsAppProvider(t, server.URL)

	result, err := p.Send(context.Background(), baseNotification(), baseUser())

	assert.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error.Error(), "Permission denied")
}

func TestWhatsAppProvider_Send_TwilioEmptyErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("{}"))
	}))
	defer server.Close()

	p := newTestWhatsAppProvider(t, server.URL)

	result, err := p.Send(context.Background(), baseNotification(), baseUser())

	assert.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error.Error(), "status 500")
}

func TestWhatsAppProvider_Send_TwilioMalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("not-json"))
	}))
	defer server.Close()

	p := newTestWhatsAppProvider(t, server.URL)

	result, err := p.Send(context.Background(), baseNotification(), baseUser())

	// Still succeeds (status 201) but ProviderMessageID falls back
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Contains(t, result.ProviderMessageID, "whatsapp-notif-001")
}

// ─── Network errors ──────────────────────────────────────────────────────────

func TestWhatsAppProvider_Send_NetworkError(t *testing.T) {
	p := newTestWhatsAppProvider(t, "http://127.0.0.1:1") // nothing listening

	result, err := p.Send(context.Background(), baseNotification(), baseUser())

	assert.NoError(t, err) // Provider returns Result, not Go error
	assert.False(t, result.Success)
	assert.Equal(t, ErrorTypeNetwork, result.ErrorType)
}

// ─── Context cancellation ────────────────────────────────────────────────────

func TestWhatsAppProvider_Send_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // simulate slow
		w.WriteHeader(http.StatusCreated)
		w.Write(twilioSuccess("SM999"))
	}))
	defer server.Close()

	p := newTestWhatsAppProvider(t, server.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	result, err := p.Send(ctx, baseNotification(), baseUser())

	assert.NoError(t, err)
	assert.False(t, result.Success)
	assert.Equal(t, ErrorTypeNetwork, result.ErrorType)
}

// ─── Twilio status metadata ─────────────────────────────────────────────────

func TestWhatsAppProvider_Send_TwilioStatusInMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"sid":    "SM500",
			"status": "queued",
		})
	}))
	defer server.Close()

	p := newTestWhatsAppProvider(t, server.URL)

	result, err := p.Send(context.Background(), baseNotification(), baseUser())

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "queued", result.Metadata["twilio_status"])
}

// ─── Provider interface methods ──────────────────────────────────────────────

func TestWhatsAppProvider_GetName(t *testing.T) {
	p, _ := NewWhatsAppProvider(WhatsAppConfig{}, zaptest.NewLogger(t))
	assert.Equal(t, "whatsapp", p.GetName())
}

func TestWhatsAppProvider_GetSupportedChannel(t *testing.T) {
	p, _ := NewWhatsAppProvider(WhatsAppConfig{}, zaptest.NewLogger(t))
	assert.Equal(t, notification.ChannelWhatsApp, p.GetSupportedChannel())
}

func TestWhatsAppProvider_IsHealthy(t *testing.T) {
	p, _ := NewWhatsAppProvider(WhatsAppConfig{}, zaptest.NewLogger(t))
	assert.True(t, p.IsHealthy(context.Background()))
}

func TestWhatsAppProvider_Close(t *testing.T) {
	p, _ := NewWhatsAppProvider(WhatsAppConfig{}, zaptest.NewLogger(t))
	assert.NoError(t, p.Close())
}

// ─── NewWhatsAppProvider defaults ────────────────────────────────────────────

func TestNewWhatsAppProvider_DefaultTimeout(t *testing.T) {
	p, err := NewWhatsAppProvider(WhatsAppConfig{
		AccountSID: "AC1", AuthToken: "t", FromNumber: "+1",
	}, zaptest.NewLogger(t))
	require.NoError(t, err)
	assert.Equal(t, 15*time.Second, p.httpClient.Timeout)
}

func TestNewWhatsAppProvider_CustomTimeout(t *testing.T) {
	p, err := NewWhatsAppProvider(WhatsAppConfig{
		Config:     Config{Timeout: 30 * time.Second},
		AccountSID: "AC1", AuthToken: "t", FromNumber: "+1",
	}, zaptest.NewLogger(t))
	require.NoError(t, err)
	assert.Equal(t, 30*time.Second, p.httpClient.Timeout)
}

// ─── Twilio 200 OK (accepted by some endpoints) ─────────────────────────────

func TestWhatsAppProvider_Send_Status200OK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(twilioSuccess("SM600"))
	}))
	defer server.Close()

	p := newTestWhatsAppProvider(t, server.URL)

	result, err := p.Send(context.Background(), baseNotification(), baseUser())

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "SM600", result.ProviderMessageID)
}

// ─── Missing SID in successful response ──────────────────────────────────────

func TestWhatsAppProvider_Send_EmptySIDFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "queued",
		})
	}))
	defer server.Close()

	p := newTestWhatsAppProvider(t, server.URL)

	result, err := p.Send(context.Background(), baseNotification(), baseUser())

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "whatsapp-notif-001", result.ProviderMessageID)
}

// ─── URL path correctness ────────────────────────────────────────────────────

func TestWhatsAppProvider_Send_URLContainsAccountSID(t *testing.T) {
	var capturedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusCreated)
		w.Write(twilioSuccess("SM700"))
	}))
	defer server.Close()

	p := newTestWhatsAppProvider(t, server.URL)

	_, err := p.Send(context.Background(), baseNotification(), baseUser())

	require.NoError(t, err)
	assert.Equal(t, "/2010-04-01/Accounts/ACtest123/Messages.json", capturedPath)
}

// ─── Content template mode ignores body and media ────────────────────────────

func TestWhatsAppProvider_Send_ContentTemplate_IgnoresBodyAndMedia(t *testing.T) {
	var captured url.Values

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		captured, _ = url.ParseQuery(string(body))
		w.WriteHeader(http.StatusCreated)
		w.Write(twilioSuccess("SM800"))
	}))
	defer server.Close()

	p := newTestWhatsAppProvider(t, server.URL)
	notif := &notification.Notification{
		NotificationID: "notif-tmpl",
		Content: notification.Content{
			Title:    "Should be ignored",
			Body:     "This body should not appear",
			MediaURL: "https://example.com/photo.png",
			Data: map[string]interface{}{
				"content_sid": "HXtmpl",
			},
		},
	}

	result, err := p.Send(context.Background(), notif, baseUser())

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "HXtmpl", captured.Get("ContentSid"))
	assert.Empty(t, captured.Get("Body"))
	assert.Empty(t, captured.Get("MediaUrl"))
}
