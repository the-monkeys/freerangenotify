package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/attachment"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/providers"
	"go.uber.org/zap"
)

func TestNonRetryableDeliveryErrors(t *testing.T) {
	for _, tc := range []struct {
		name, category string
		retryable      bool
	}{
		{"smtp 535", providers.ErrorTypeAuth, false},
		{"twilio 20003", providers.ErrorTypeAuth, false},
		{"configuration", providers.ErrorTypeConfiguration, false},
		{"invalid", providers.ErrorTypeInvalid, false},
		{"dns servfail", providers.ErrorTypeNetwork, true},
		{"timeout", providers.ErrorTypeTimeout, true},
		{"smtp 4xx", providers.ErrorTypeProviderAPI, true},
		{"http 429", providers.ErrorTypeRateLimit, true},
		{"http 5xx", providers.ErrorTypeProviderAPI, true},
		{"unknown", providers.ErrorTypeUnknown, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := fmt.Errorf("worker wrapper: %w", &providers.DeliveryError{Provider: "smtp", ErrorType: tc.category, Stage: "send", CredentialSource: providers.CredSourceBYOC, Retryable: tc.retryable, Err: errors.New("upstream")})
			if got := isNonRetryableError(err); got == tc.retryable {
				t.Fatalf("terminal=%v retryable=%v", got, tc.retryable)
			}
		})
	}
	if isNonRetryableError(errors.New("unknown legacy error")) || isNonRetryableError(nil) {
		t.Fatal("legacy retry policy changed")
	}
	// Existing attachment sentinels remain terminal even inside a retryable wrapper.
	if !isNonRetryableError(&providers.DeliveryError{Retryable: true, Err: notification.ErrInvalidAttachment}) {
		t.Fatal("attachment sentinel lost")
	}
}

type workerReviewAttachmentReader struct{}

func (workerReviewAttachmentReader) Read([]byte) (int, error) { return 0, context.DeadlineExceeded }

func TestProviderReviewWorkerAttachmentRetryPolicy(t *testing.T) {
	for _, tc := range []struct {
		name     string
		cause    error
		stream   bool
		terminal bool
	}{
		{"resolver timeout", context.DeadlineExceeded, false, false},
		{"resolver temporary", errors.New("temporary fetch failure"), false, false},
		{"streaming timeout", context.DeadlineExceeded, true, false},
		{"validation", notification.ErrInvalidAttachment, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, _ := providers.NewSendGridProvider(providers.SendGridConfig{APIKey: "fixture", FromEmail: "from@example.test"}, zap.NewNop())
			m := providers.NewManager(nil, nil, zap.NewNop())
			m.RegisterProvider(p)
			ctx := context.WithValue(context.Background(), providers.AttachmentResolverKey, providers.AttachmentResolveFunc(func(context.Context, []notification.Attachment) ([]*attachment.Resolved, error) {
				if tc.stream {
					return []*attachment.Resolved{{Filename: "remote.txt", MIMEType: "text/plain", Reader: io.NopCloser(workerReviewAttachmentReader{})}}, nil
				}
				return nil, fmt.Errorf("resolve: %w", tc.cause)
			}))
			_, err := m.Send(ctx, &notification.Notification{Channel: notification.ChannelEmail, Content: notification.Content{Attachments: []notification.Attachment{{URL: "https://example.test/remote.txt"}}}}, &user.User{Email: "to@example.test"})
			if err == nil || isNonRetryableError(fmt.Errorf("worker: %w", err)) != tc.terminal || !errors.Is(err, tc.cause) {
				t.Fatalf("terminal=%v error=%v", isNonRetryableError(err), err)
			}
		})
	}
}

type workerReviewTwilioTransport struct {
	target *url.URL
	base   http.RoundTripper
}

func (rt workerReviewTwilioTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host != "api.twilio.com" {
		return rt.base.RoundTrip(req)
	}
	copy := req.Clone(req.Context())
	copy.URL.Scheme = rt.target.Scheme
	copy.URL.Host = rt.target.Host
	return rt.base.RoundTrip(copy)
}

func TestProviderReviewWorkerWhatsAppRetryPolicy(t *testing.T) {
	for _, tc := range []struct {
		name     string
		status   int
		body     string
		terminal bool
	}{
		{"auth", 401, `{"code":20003,"message":"secret credentials"}`, true},
		{"rate limit", 429, `{"code":20429}`, false},
		{"upstream unavailable", 503, `{"message":"temporary"}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); fmt.Fprint(w, tc.body) }))
			defer server.Close()
			target, _ := url.Parse(server.URL)
			base := http.DefaultTransport
			http.DefaultTransport = workerReviewTwilioTransport{target, base}
			defer func() { http.DefaultTransport = base }()
			p, _ := providers.NewWhatsAppProvider(providers.WhatsAppConfig{AccountSID: "fixture", AuthToken: "fixture", FromNumber: "+123"}, zap.NewNop())
			m := providers.NewManager(nil, nil, zap.NewNop())
			m.RegisterProvider(p)
			ctx := context.WithValue(context.Background(), providers.WhatsAppConfigKey, &application.WhatsAppAppConfig{AccountSID: "customer", AuthToken: "secret", FromNumber: "+123"})
			_, err := m.Send(ctx, &notification.Notification{Channel: notification.ChannelWhatsApp}, &user.User{Phone: "+456"})
			var failure *providers.DeliveryError
			if !errors.As(err, &failure) || isNonRetryableError(fmt.Errorf("worker: %w", err)) != tc.terminal || failure.CredentialSource != providers.CredSourceBYOC {
				t.Fatalf("terminal=%v error=%v", isNonRetryableError(err), err)
			}
		})
	}
}
