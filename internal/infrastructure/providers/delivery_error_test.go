package providers

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/smtp"
	"net/textproto"
	"strings"
	"testing"

	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"go.uber.org/zap"
)

func TestDeliveryErrorSafeWrappedMetadata(t *testing.T) {
	cause := errors.New("password=hunter2 token=secret raw upstream body")
	err := fmt.Errorf("outer: %w", &DeliveryError{Provider: "smtp", ErrorType: ErrorTypeAuth, Stage: "auth", CredentialSource: CredSourceBYOC, Retryable: false, Err: cause})
	if !errors.Is(err, cause) {
		t.Fatal("cause lost")
	}
	md := DeliveryErrorMetadata(err)
	for k, v := range map[string]interface{}{"failure_code": "authentication", "failure_stage": "auth", "provider": "smtp", "credential_source": "byoc", "retryable": false} {
		if md[k] != v {
			t.Errorf("%s=%v want %v", k, md[k], v)
		}
	}
	text := SafeDeliveryErrorMessage(err)
	if strings.Contains(text, "hunter2") || strings.Contains(text, "secret") || strings.Contains(text, "raw upstream") || !strings.Contains(text, "authentication") {
		t.Fatal(text)
	}
	malicious := &DeliveryError{Provider: "token=secret", ErrorType: "raw body", Stage: "password=secret", CredentialSource: "secret", Err: cause}
	if strings.Contains(malicious.Error(), "secret") || strings.Contains(fmt.Sprint(DeliveryErrorMetadata(malicious)), "secret") {
		t.Fatal("unsafe fields exposed")
	}
	if DeliveryErrorMetadata(cause) != nil || SafeDeliveryErrorMessage(nil) != "" {
		t.Fatal("optional metadata/nil contract")
	}
}

func TestDeliveryErrorClassification(t *testing.T) {
	for _, tc := range []struct {
		name     string
		cause    error
		category string
		retry    bool
	}{
		{"smtp auth", &textproto.Error{Code: 535, Msg: "bad credentials"}, ErrorTypeAuth, false},
		{"smtp temporary", &textproto.Error{Code: 451, Msg: "try later"}, ErrorTypeProviderAPI, true},
		{"smtp invalid recipient", &textproto.Error{Code: 550, Msg: "no mailbox"}, ErrorTypeInvalid, false},
		{"dns servfail", &net.DNSError{Err: "server misbehaving", Name: "smtp.test", IsTemporary: true}, ErrorTypeNetwork, true},
		{"timeout", context.DeadlineExceeded, ErrorTypeTimeout, true},
		{"unknown", errors.New("unknown"), ErrorTypeUnknown, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newDeliveryError("smtp", "send", CredSourceBYOC, "", tc.cause)
			if e.ErrorType != tc.category || e.Retryable != tc.retry || !errors.Is(e, tc.cause) {
				t.Fatalf("%+v", e)
			}
		})
	}
}

func TestDeliveryErrorHTTPFailuresRemainRetryable(t *testing.T) {
	for _, code := range []int{429, 500, 503} {
		err := newDeliveryError("http", "send", CredSourceSystem, ErrorTypeProviderAPI, fmt.Errorf("%d upstream unavailable", code))
		if !err.Retryable {
			t.Fatalf("HTTP %d became terminal: %v", code, err)
		}
	}
}

func TestDeliveryErrorPreservesOuterErrorChain(t *testing.T) {
	sentinel := errors.New("outer sentinel")
	typed := &DeliveryError{Provider: "smtp", ErrorType: ErrorTypeAuth, Stage: "auth", Retryable: false, Err: errors.New("535 auth")}
	chain := errors.Join(typed, sentinel)
	wrapped := newDeliveryError("smtp", "send", CredSourceSystem, ErrorTypeProviderAPI, chain)
	if !errors.Is(wrapped, sentinel) || !errors.Is(wrapped, typed) || !errors.Is(wrapped, chain) {
		t.Fatal("typed wrapping discarded part of the cause chain")
	}
}

func TestManagerSMTPBYOCDoesNotUseSystemBreaker(t *testing.T) {
	port, _ := smtpFixture(t)
	raw, _ := NewSMTPProvider(SMTPConfig{Host: "127.0.0.1", Port: port}, zap.NewNop())
	p := raw.(*SMTPProvider)
	cause := errors.New("535 Authentication Failed")
	p.sender = func(string, smtp.Auth, string, []string, []byte) error { return cause }
	m := NewManager(nil, nil, zap.NewNop())
	m.RegisterProvider(p)
	m.breakers["smtp-email"].recordFailure()
	for i := 0; i < 6; i++ {
		_, err := m.Send(appSMTPContext(port), &notification.Notification{Channel: notification.ChannelEmail}, &user.User{Email: "to@example.test"})
		var de *DeliveryError
		if !errors.As(err, &de) || de.Retryable || !errors.Is(err, cause) {
			t.Fatalf("cause/category lost: %v", err)
		}
	}
	if m.breakers["smtp-email"].GetState() != StateClosed || m.breakers["smtp-email"].failures != 1 {
		t.Fatal("BYOC changed system breaker")
	}
	p.sender = func(string, smtp.Auth, string, []string, []byte) error { return nil }
	if _, err := m.Send(appSMTPContext(port), &notification.Notification{Channel: notification.ChannelEmail}, &user.User{Email: "to@example.test"}); err != nil {
		t.Fatal(err)
	}
	if m.breakers["smtp-email"].failures != 1 {
		t.Fatal("BYOC success reset system breaker")
	}
	m.breakers["smtp-email"].state = StateOpen
	if _, err := m.Send(appSMTPContext(port), &notification.Notification{Channel: notification.ChannelEmail}, &user.User{Email: "to@example.test"}); err != nil {
		t.Fatalf("system breaker blocked customer: %v", err)
	}
}

func TestManagerTwilioAuthCategoryAndIsolation(t *testing.T) {
	m := NewManager(nil, nil, zap.NewNop())
	p := &deliveryFixture{name: "twilio", channel: notification.ChannelSMS, result: (&TwilioProvider{}).handleError(errors.New("Twilio 20003 Authentication Error secret"))}
	m.RegisterProvider(p)
	ctx := context.WithValue(context.Background(), SMSConfigKey, &application.SMSAppConfig{AccountSID: "customer", AuthToken: "secret"})
	for i := 0; i < 6; i++ {
		result, err := m.Send(ctx, &notification.Notification{Channel: notification.ChannelSMS}, &user.User{})
		var de *DeliveryError
		if !errors.As(err, &de) || de.ErrorType != ErrorTypeAuth || de.Retryable || de.CredentialSource != CredSourceBYOC || result.Error != err {
			t.Fatalf("result=%+v err=%v", result, err)
		}
	}
	if m.breakers["twilio-sms"].GetState() != StateClosed {
		t.Fatal("customer auth opened shared breaker")
	}
	p.result = nil
	result, err := m.Send(context.Background(), &notification.Notification{Channel: notification.ChannelSMS}, &user.User{})
	if err != nil || !result.Success {
		t.Fatalf("other app rejected: %v", err)
	}
}

func TestManagerCustomBYOCNameDoesNotCountSharedFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "secret upstream body")
	}))
	defer server.Close()
	p := NewCustomProvider("tenant-delivery", "custom", "generic", server.URL, "", "", nil, zap.NewNop())
	m := NewManager(nil, nil, zap.NewNop())
	m.RegisterProvider(p)
	for i := 0; i < 6; i++ {
		_, err := m.Send(context.Background(), &notification.Notification{Channel: notification.Channel("custom")}, &user.User{})
		var de *DeliveryError
		if !errors.As(err, &de) || de.CredentialSource != CredSourceBYOC || !de.Retryable {
			t.Fatalf("%v", err)
		}
	}
	if m.breakers["tenant-delivery-custom"].GetState() != StateClosed {
		t.Fatal("BYOC custom name counted failures")
	}
}
