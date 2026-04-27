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
	name               string
	channel            notification.Channel
	kind               string
	webhookURL         string
	headers            map[string]string
	discordNativePolls bool
	signingKey         string
	signatureVersion   string
	httpClient         *http.Client
	logger             *zap.Logger
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

	// Allow opt-in behavior toggles via reserved headers (not forwarded).
	normalizedHeaders := make(map[string]string, len(headers))
	discordNativePolls := false
	for k, v := range headers {
		key := strings.ToLower(strings.TrimSpace(k))
		val := strings.ToLower(strings.TrimSpace(v))
		if key == "x-frn-discord-native-polls" {
			discordNativePolls = (val == "true" || val == "1" || val == "yes" || val == "on")
			continue
		}
		normalizedHeaders[k] = v
	}

	return &CustomProvider{
		name:               name,
		channel:            notification.Channel(channel),
		kind:               normalizedKind,
		webhookURL:         webhookURL,
		headers:            normalizedHeaders,
		discordNativePolls: discordNativePolls,
		signingKey:         signingKey,
		signatureVersion:   normalizedSignatureVersion,
		httpClient:         &http.Client{Timeout: 30 * time.Second},
		logger:             logger,
	}
}

// Send delivers a notification to the custom webhook endpoint.
func (p *CustomProvider) Send(ctx context.Context, notif *notification.Notification, usr *user.User) (*Result, error) {
	start := time.Now()

	// Prefer explicit provider kind; fallback to URL-based inference for legacy rows.
	payloadKind := p.kind
	if payloadKind == "" {
		payloadKind = inferProviderKindFromURL(p.webhookURL)
	}

	body, err := p.buildPayload(payloadKind, notif, usr, true /* preferNativePoll */)
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to marshal custom payload: %w", err), ErrorTypeInvalid), nil
	}

	status, respBody, err := p.postWebhook(ctx, body)
	if err != nil {
		return NewErrorResult(err, ErrorTypeNetwork), nil
	}

	// Discord poll fallback. Plain incoming webhooks reject native poll
	// payloads with HTTP 400 + body `{"proto_data":["poll"]}`. Application-
	// owned webhooks (and some channel types) accept them. We optimistically
	// emit native polls and, on this exact rejection, rebuild the payload
	// with the embed-list fallback and retry once. This gives interactive
	// polls wherever Discord allows them and degrades gracefully elsewhere
	// without requiring per-provider configuration.
	if payloadKind == "discord" && status == http.StatusBadRequest &&
		notif.Content.Poll != nil && isDiscordPollRejection(respBody) {
		p.logger.Info("Discord webhook rejected native poll; falling back to embed list",
			zap.String("provider", p.name),
			zap.String("notification_id", notif.NotificationID))

		body, err = p.buildPayload(payloadKind, notif, usr, false /* embed fallback */)
		if err != nil {
			return NewErrorResult(fmt.Errorf("failed to marshal Discord embed-fallback payload: %w", err), ErrorTypeInvalid), nil
		}
		status, respBody, err = p.postWebhook(ctx, body)
		if err != nil {
			return NewErrorResult(err, ErrorTypeNetwork), nil
		}
	}

	if status < 200 || status >= 300 {
		return NewErrorResult(
			fmt.Errorf("custom provider %s returned status %d: %s", p.name, status, string(respBody)),
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

// buildPayload renders the per-kind webhook body. When native is false the
// Discord renderer is forced into embed-fallback mode for the Poll field;
// when true the renderer emits a native Discord poll object so the caller
// (Send) can let Discord accept it or trigger the fallback retry.
func (p *CustomProvider) buildPayload(payloadKind string, notif *notification.Notification, usr *user.User, native bool) ([]byte, error) {
	switch payloadKind {
	case "discord":
		return json.Marshal(render.BuildCustomDiscordPayloadWithOptions(notif, render.DiscordRenderOptions{
			NativePolls: native,
		}))
	case "slack":
		return json.Marshal(render.BuildCustomSlackPayload(notif))
	case "teams":
		return json.Marshal(render.BuildTeamsPayload(notif, p.webhookURL))
	default:
		return json.Marshal(render.BuildCustomStandardPayload(notif, p.channel, usr))
	}
}

// postWebhook performs a single signed POST to the configured webhook URL.
func (p *CustomProvider) postWebhook(ctx context.Context, body []byte) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.webhookURL, bytes.NewReader(body))
	if err != nil {
		return 0, nil, fmt.Errorf("failed to create custom provider request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "FreeRangeNotify/1.0")
	for k, v := range p.headers {
		req.Header.Set(k, v)
	}
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
		return 0, nil, fmt.Errorf("custom provider request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, respBody, nil
}

// isDiscordPollRejection identifies the specific 400 response Discord returns
// when an incoming webhook receives a payload with a `poll` field it cannot
// honor: `{"proto_data": ["poll"]}` (sometimes nested under `errors`).
func isDiscordPollRejection(respBody []byte) bool {
	// Substring check is sufficient — Discord's error envelope is small and
	// the literal `"poll"` token only appears here in the rejection body.
	s := string(respBody)
	return strings.Contains(s, "proto_data") && strings.Contains(s, "poll")
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
