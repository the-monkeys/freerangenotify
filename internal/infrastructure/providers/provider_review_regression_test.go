package providers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/attachment"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"go.uber.org/zap"
)

type reviewAttachmentReader struct{ cause error }

func (r reviewAttachmentReader) Read([]byte) (int, error) { return 0, r.cause }

func TestProviderReviewEmailAttachmentRetryPolicy(t *testing.T) {
	for _, name := range []string{"sendgrid", "ses", "mailgun", "postmark", "resend"} {
		t.Run(name, func(t *testing.T) {
			p, err := GetFactory(name)(map[string]interface{}{"enabled": true, "api_key": "fixture", "server_token": "fixture", "region": "us-east-1", "access_key_id": "fixture", "secret_access_key": "fixture", "domain": "example.test", "from_email": "from@example.test"}, zap.NewNop())
			if err != nil {
				t.Fatal(err)
			}
			m := NewManager(nil, nil, zap.NewNop())
			if err := m.RegisterProvider(p); err != nil {
				t.Fatal(err)
			}
			for _, tc := range []struct {
				name     string
				cause    error
				stream   bool
				retry    bool
				category string
			}{
				{"resolver timeout", context.DeadlineExceeded, false, true, ErrorTypeTimeout},
				{"resolver DNS", &net.DNSError{Err: "server misbehaving", Name: "attachment.test", IsTemporary: true}, false, true, ErrorTypeNetwork},
				{"resolver temporary", errors.New("temporary fetch failure"), false, true, ErrorTypeUnknown},
				{"streaming timeout", context.DeadlineExceeded, true, true, ErrorTypeTimeout},
				{"permanent validation", notification.ErrInvalidAttachment, false, false, ErrorTypeInvalid},
			} {
				t.Run(tc.name, func(t *testing.T) {
					ctx := context.WithValue(context.Background(), AttachmentResolverKey, AttachmentResolveFunc(func(context.Context, []notification.Attachment) ([]*attachment.Resolved, error) {
						if tc.stream {
							return []*attachment.Resolved{{Filename: "remote.txt", MIMEType: "text/plain", Reader: io.NopCloser(reviewAttachmentReader{tc.cause})}}, nil
						}
						return nil, fmt.Errorf("resolve: %w", tc.cause)
					}))
					notif := testNotificationWithAttachment()
					notif.Channel = notification.ChannelEmail
					result, err := m.Send(ctx, notif, testUser())
					var failure *DeliveryError
					if !errors.As(err, &failure) || failure.Retryable != tc.retry || failure.ErrorType != tc.category || !errors.Is(err, tc.cause) || result.Error != err {
						t.Fatalf("result=%+v err=%v retry=%v", result, err, tc.retry)
					}
				})
			}
		})
	}
}

func TestProviderReviewFallbackSkippedAdapters(t *testing.T) {
	m := NewManager(nil, nil, zap.NewNop())
	cause := context.DeadlineExceeded
	p := &deliveryFixture{name: "attempted", channel: notification.ChannelEmail, result: NewErrorResult(cause, ErrorTypeTimeout)}
	m.RegisterProvider(p)
	result, err := m.SendWithFallback(context.Background(), &notification.Notification{Channel: notification.ChannelEmail}, &user.User{}, []string{"missing-before", "attempted", "missing-after"})
	var failure *DeliveryError
	if !errors.As(err, &failure) || !failure.Retryable || failure.Provider != "attempted" || !errors.Is(err, cause) || result.Metadata["failure_code"] != ErrorTypeTimeout || p.calls != 1 {
		t.Fatalf("result=%+v err=%v calls=%d", result, err, p.calls)
	}
	result, err = m.SendWithFallback(context.Background(), &notification.Notification{Channel: notification.ChannelEmail}, &user.User{}, []string{"missing-one", "missing-two"})
	if !errors.As(err, &failure) || failure.Retryable || failure.ErrorType != ErrorTypeConfiguration || result.Metadata["failure_code"] != ErrorTypeConfiguration {
		t.Fatalf("all missing: %+v %v", result, err)
	}
}

func TestProviderReviewWhatsAppAuthentication(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		retry  bool
	}{
		{"auth code", 401, `{"code":20003,"message":"secret credential upstream body"}`, false},
		{"auth error_code", 401, `{"error_code":20003,"error_message":"secret credential upstream body"}`, false},
		{"rate limit", 429, `{"code":20429,"message":"too many"}`, true},
		{"server unavailable", 503, `{"message":"temporary"}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); fmt.Fprint(w, tc.body) }))
			defer server.Close()
			p := newTestWhatsAppProvider(t, server.URL)
			m := NewManager(nil, nil, zap.NewNop())
			m.RegisterProvider(p)
			ctx := context.WithValue(context.Background(), WhatsAppConfigKey, &application.WhatsAppAppConfig{AccountSID: "customer", AuthToken: "secret", FromNumber: "+123"})
			for i := 0; i < 6; i++ {
				result, err := m.Send(ctx, &notification.Notification{Channel: notification.ChannelWhatsApp}, baseUser())
				var failure *DeliveryError
				if !errors.As(err, &failure) || failure.Retryable != tc.retry || failure.CredentialSource != CredSourceBYOC || result.Error != err {
					t.Fatalf("result=%+v err=%v", result, err)
				}
				if !tc.retry && failure.ErrorType != ErrorTypeAuth {
					t.Fatalf("auth category lost: %v", err)
				}
				if !tc.retry && (failure.Unwrap() == nil || !strings.Contains(failure.Unwrap().Error(), "20003")) {
					t.Fatalf("structured Twilio cause lost: %v", err)
				}
				if strings.Contains(SafeDeliveryErrorMessage(err), "secret") || strings.Contains(fmt.Sprint(DeliveryErrorMetadata(err)), "secret") {
					t.Fatal("unsafe history text")
				}
			}
			if m.breakers["whatsapp-whatsapp"].GetState() != StateClosed || m.breakers["whatsapp-whatsapp"].failures != 0 {
				t.Fatal("BYOC affected shared breaker")
			}
		})
	}
}
