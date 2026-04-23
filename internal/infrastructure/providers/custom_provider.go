package providers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/providers/render"
	"go.uber.org/zap"
)

// CustomProvider delivers notifications to a user-registered webhook endpoint.
// It acts as a generic relay, signing payloads with HMAC-SHA256 for security.
type CustomProvider struct {
	name             string
	channel          notification.Channel
	kind             string
	webhookURL       string
	headers          map[string]string
	signingKey       string
	signatureVersion string
	httpClient       *http.Client
	logger           *zap.Logger
}

// NewCustomProvider creates a custom webhook-based provider.
func NewCustomProvider(name, channel, kind, webhookURL, signingKey, signatureVersion string, headers map[string]string, logger *zap.Logger) *CustomProvider {
	normalizedKind := strings.ToLower(strings.TrimSpace(kind))
	if normalizedKind == "" {
		normalizedKind = inferProviderKindFromURL(webhookURL)
	}
	normalizedSignatureVersion := strings.ToLower(strings.TrimSpace(signatureVersion))
	if normalizedSignatureVersion != "v2" {
		normalizedSignatureVersion = "v1"
	}

	return &CustomProvider{
		name:             name,
		channel:          notification.Channel(channel),
		kind:             normalizedKind,
		webhookURL:       webhookURL,
		headers:          headers,
		signingKey:       signingKey,
		signatureVersion: normalizedSignatureVersion,
		httpClient:       &http.Client{Timeout: 30 * time.Second},
		logger:           logger,
	}
}

// Send delivers a notification to the custom webhook endpoint.
func (p *CustomProvider) Send(ctx context.Context, notif *notification.Notification, usr *user.User) (*Result, error) {
	start := time.Now()

	var body []byte
	var err error

	// Prefer explicit provider kind; fallback to URL-based inference for legacy rows.
	payloadKind := p.kind
	if payloadKind == "" {
		payloadKind = inferProviderKindFromURL(p.webhookURL)
	}
	switch payloadKind {
	case "discord":
		body, err = json.Marshal(render.BuildCustomDiscordPayload(notif))
	case "slack":
		body, err = json.Marshal(render.BuildCustomSlackPayload(notif))
	case "teams":
		body, err = json.Marshal(render.BuildTeamsPayload(notif, p.webhookURL))
	default:
		body, err = json.Marshal(render.BuildCustomStandardPayload(notif, p.channel, usr))
	}
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to marshal custom payload: %w", err), ErrorTypeInvalid), nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.webhookURL, bytes.NewReader(body))
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to create custom provider request: %w", err), ErrorTypeUnknown), nil
	}

	// Set standard headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "FreeRangeNotify/1.0")

	// Set custom headers
	for k, v := range p.headers {
		req.Header.Set(k, v)
	}

	// Emit canonical signature header and keep X-FRN-Signature in parallel during migration.
	if p.signingKey != "" {
		signature, timestamp := p.sign(body)
		req.Header.Set("X-Webhook-Signature", signature)
		req.Header.Set("X-FRN-Signature", signature)
		if timestamp != "" {
			req.Header.Set("X-Webhook-Timestamp", timestamp)
		}
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return NewErrorResult(fmt.Errorf("custom provider request failed: %w", err), ErrorTypeNetwork), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return NewErrorResult(
			fmt.Errorf("custom provider %s returned status %d: %s", p.name, resp.StatusCode, string(respBody)),
			ErrorTypeProviderAPI,
		), nil
	}

	p.logger.Info("Custom provider notification delivered",
		zap.String("provider", p.name),
		zap.String("notification_id", notif.NotificationID),
		zap.Duration("delivery_time", time.Since(start)))

	result := NewResult(fmt.Sprintf("custom-%s-%s", p.name, notif.NotificationID), time.Since(start))
	result.Metadata["credential_source"] = CredSourceBYOC
	result.Metadata["billing_channel"] = "custom"
	return result, nil
}

// GetName returns the provider name.
func (p *CustomProvider) GetName() string { return p.name }

// GetSupportedChannel returns the channel this provider supports.
func (p *CustomProvider) GetSupportedChannel() notification.Channel { return p.channel }

// IsHealthy checks if the provider is healthy.
func (p *CustomProvider) IsHealthy(_ context.Context) bool { return true }

// Close releases provider resources.
func (p *CustomProvider) Close() error { return nil }

func inferProviderKindFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return "generic"
	}
	host := strings.ToLower(u.Host)
	path := strings.ToLower(u.Path)
	switch {
	case (strings.Contains(host, "discord.com") || strings.Contains(host, "discordapp.com")) && strings.HasPrefix(path, "/api/webhooks"):
		return "discord"
	case strings.Contains(host, "hooks.slack.com"):
		return "slack"
	case strings.HasSuffix(host, "webhook.office.com"):
		return "teams"
	case strings.Contains(host, "logic.azure.com") && strings.Contains(path, "/workflows/"):
		return "teams"
	default:
		return "generic"
	}
}

func (p *CustomProvider) sign(body []byte) (string, string) {
	mac := hmac.New(sha256.New, []byte(p.signingKey))
	timestamp := ""

	if p.signatureVersion == "v2" {
		timestamp = strconv.FormatInt(time.Now().UTC().Unix(), 10)
		mac.Write([]byte(timestamp))
		mac.Write([]byte("."))
	}
	mac.Write(body)

	return hex.EncodeToString(mac.Sum(nil)), timestamp
}
