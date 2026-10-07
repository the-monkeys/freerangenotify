package providers

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/smtp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"go.uber.org/zap"
)

// A real SMTP fixture: health connections and full unauthenticated deliveries.
func smtpFixture(t *testing.T) (int, <-chan string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	messages := make(chan string, 16)
	var wg sync.WaitGroup
	wg.Add(1)
	t.Cleanup(func() { ln.Close(); wg.Wait() })
	go func() {
		defer wg.Done()
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer c.Close()
				c.SetDeadline(time.Now().Add(2 * time.Second))
				fmt.Fprint(c, "220 localhost ESMTP\r\n")
				r := bufio.NewReader(c)
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					switch {
					case strings.HasPrefix(line, "EHLO"), strings.HasPrefix(line, "HELO"):
						fmt.Fprint(c, "250 localhost\r\n")
					case strings.HasPrefix(line, "DATA"):
						fmt.Fprint(c, "354 send message\r\n")
						var msg strings.Builder
						for {
							line, err = r.ReadString('\n')
							if err != nil {
								return
							}
							if line == ".\r\n" {
								break
							}
							msg.WriteString(line)
						}
						messages <- msg.String()
						fmt.Fprint(c, "250 accepted\r\n")
					case strings.HasPrefix(line, "QUIT"):
						fmt.Fprint(c, "221 bye\r\n")
						return
					default:
						fmt.Fprint(c, "250 ok\r\n")
					}
				}
			}()
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port, messages
}

func closedSMTPPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

func appSMTPContext(port int) context.Context {
	return context.WithValue(context.Background(), EmailConfigKey, &application.EmailConfig{ProviderType: "smtp", SMTP: &application.SMTPConfig{Host: "127.0.0.1", Port: port, FromEmail: "customer@example.test", FromName: "Customer"}})
}

func TestSMTPProviderHealthUsesRequestEndpoint(t *testing.T) {
	port, _ := smtpFixture(t)
	for _, tc := range []struct {
		name             string
		system, customer int
		healthy          bool
	}{
		{"system down customer healthy", closedSMTPPort(t), port, true},
		{"system healthy customer down", port, closedSMTPPort(t), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := NewSMTPProvider(SMTPConfig{Host: "127.0.0.1", Port: tc.system, Config: Config{Timeout: 200 * time.Millisecond}}, zap.NewNop())
			if err != nil {
				t.Fatal(err)
			}
			if got := p.IsHealthy(appSMTPContext(tc.customer)); got != tc.healthy {
				t.Fatalf("health = %v, want %v", got, tc.healthy)
			}
		})
	}
}

func TestSMTPProviderCustomPortAndSender(t *testing.T) {
	raw, _ := NewSMTPProvider(SMTPConfig{Host: "system.test", Port: 25}, zap.NewNop())
	p := raw.(*SMTPProvider)
	p.sender = func(addr string, _ smtp.Auth, from string, _ []string, msg []byte) error {
		if addr != "127.0.0.1:587" || from != "customer@example.test" || !strings.Contains(string(msg), "Customer") {
			t.Errorf("wrong custom endpoint/sender: %s %s %s", addr, from, msg)
		}
		return nil
	}
	result, err := p.Send(appSMTPContext(0), &notification.Notification{}, &user.User{Email: "to@example.test"})
	if err != nil || !result.Success || result.Metadata["from_email"] != "customer@example.test" {
		t.Fatalf("send = %+v, %v", result, err)
	}
}

func TestSMTPProviderAuthFailureIsTerminal(t *testing.T) {
	raw, _ := NewSMTPProvider(SMTPConfig{Host: "localhost", Config: Config{MaxRetries: 3}}, zap.NewNop())
	p := raw.(*SMTPProvider)
	attempts := 0
	p.sender = func(string, smtp.Auth, string, []string, []byte) error {
		attempts++
		return errors.New("535 Authentication Failed secret upstream body")
	}
	result, err := p.Send(context.Background(), &notification.Notification{}, &user.User{Email: "to@example.test"})
	if err != nil || attempts != 1 || result.ErrorType != ErrorTypeAuth {
		t.Fatalf("attempts=%d result=%+v err=%v", attempts, result, err)
	}
}

func TestSMTPProviderCanceledSendAndWait(t *testing.T) {
	for _, before := range []bool{true, false} {
		t.Run(fmt.Sprint(before), func(t *testing.T) {
			raw, _ := NewSMTPProvider(SMTPConfig{Host: "localhost", Config: Config{MaxRetries: 3, RetryDelay: time.Second}}, zap.NewNop())
			p := raw.(*SMTPProvider)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if before {
				cancel()
			}
			attempts := 0
			p.sender = func(string, smtp.Auth, string, []string, []byte) error {
				attempts++
				cancel()
				return io.ErrUnexpectedEOF
			}
			start := time.Now()
			result, _ := p.Send(ctx, &notification.Notification{}, &user.User{Email: "to@example.test"})
			if !errors.Is(result.Error, context.Canceled) || time.Since(start) > 300*time.Millisecond || (before && attempts != 0) || (!before && attempts != 1) {
				t.Fatalf("attempts=%d elapsed=%s err=%v", attempts, time.Since(start), result.Error)
			}
		})
	}
}

func TestManagerCustomSMTPWithoutSystem(t *testing.T) {
	for _, withDefault := range []bool{false, true} {
		t.Run(fmt.Sprint(withDefault), func(t *testing.T) {
			port, messages := smtpFixture(t)
			m := NewManager(nil, nil, zap.NewNop())
			defaultP := &deliveryFixture{name: "default", channel: notification.ChannelEmail}
			if withDefault {
				m.RegisterProvider(defaultP)
			}
			result, err := m.Send(appSMTPContext(port), &notification.Notification{NotificationID: "custom", Channel: notification.ChannelEmail, Content: notification.Content{Title: "Custom subject"}}, &user.User{Email: "recipient@example.test"})
			if err != nil || result == nil || !result.Success {
				t.Fatalf("custom SMTP: %+v %v", result, err)
			}
			select {
			case msg := <-messages:
				if !strings.Contains(msg, "Subject: Custom subject") {
					t.Fatal(msg)
				}
			case <-time.After(time.Second):
				t.Fatal("custom SMTP did not send")
			}
			if defaultP.calls != 0 {
				t.Fatal("silently used default")
			}
			if len(m.namedProviders) != 0 && !withDefault {
				t.Fatal("request registered globally")
			}
			if withDefault {
				p, _ := m.GetProvider(notification.ChannelEmail)
				if p != defaultP {
					t.Fatal("default changed")
				}
			}
		})
	}
}

type deliveryFixture struct {
	name    string
	channel notification.Channel
	result  *Result
	calls   int
}

func (p *deliveryFixture) Send(context.Context, *notification.Notification, *user.User) (*Result, error) {
	p.calls++
	if p.result != nil {
		return p.result, nil
	}
	return NewResult("fixture", 0), nil
}
func (p *deliveryFixture) GetName() string                           { return p.name }
func (p *deliveryFixture) GetSupportedChannel() notification.Channel { return p.channel }
func (p *deliveryFixture) IsHealthy(context.Context) bool            { return true }
func (p *deliveryFixture) Close() error                              { return nil }

func TestManagerSMTPFailedResultsCountForBreaker(t *testing.T) {
	m := NewManager(nil, nil, zap.NewNop())
	p := &deliveryFixture{name: "fixture", channel: notification.ChannelEmail, result: NewErrorResult(errors.New("upstream failure"), ErrorTypeProviderAPI)}
	m.RegisterProvider(p)
	for i := 0; i < 5; i++ {
		_, err := m.Send(context.Background(), &notification.Notification{Channel: notification.ChannelEmail}, &user.User{})
		if err == nil {
			t.Fatal("failed result lost")
		}
	}
	if m.breakers["fixture-email"].GetState() != StateOpen {
		t.Fatal("failed Result with nil Go error counted as success")
	}
}

func TestManagerSMTPDefaultAndIncompletePaths(t *testing.T) {
	m := NewManager(nil, nil, zap.NewNop())
	first := &deliveryFixture{name: "first", channel: notification.ChannelEmail}
	second := &deliveryFixture{name: "second", channel: notification.ChannelEmail}
	m.RegisterProvider(first)
	m.RegisterProvider(second)
	for _, cfg := range []*application.EmailConfig{nil, {ProviderType: "system"}, {ProviderType: "smtp"}, {ProviderType: "sendgrid"}} {
		ctx := context.Background()
		if cfg != nil {
			ctx = context.WithValue(ctx, EmailConfigKey, cfg)
		}
		result, err := m.Send(ctx, &notification.Notification{Channel: notification.ChannelEmail}, &user.User{})
		if err != nil || !result.Success {
			t.Fatalf("cfg=%+v error=%v", cfg, err)
		}
	}
	if first.calls != 4 || second.calls != 0 {
		t.Fatalf("default ordering changed: first=%d second=%d", first.calls, second.calls)
	}
}

func TestManagerSMTPDoesNotReplaceMissingExplicitProvider(t *testing.T) {
	for _, cfg := range []*application.EmailConfig{
		{ProviderType: "sendgrid", SendGrid: &application.SendGridConfig{APIKey: "customer-secret", FromEmail: "from@example.test"}},
		{ProviderType: "unsupported"},
	} {
		t.Run(cfg.ProviderType, func(t *testing.T) {
			m := NewManager(nil, nil, zap.NewNop())
			fallback := &deliveryFixture{name: "smtp", channel: notification.ChannelEmail}
			m.RegisterProvider(fallback)
			ctx := context.WithValue(context.Background(), EmailConfigKey, cfg)
			result, err := m.Send(ctx, &notification.Notification{Channel: notification.ChannelEmail}, &user.User{})
			var de *DeliveryError
			if !errors.As(err, &de) || de.ErrorType != ErrorTypeConfiguration || de.Retryable || de.Provider != cfg.ProviderType || fallback.calls != 0 {
				t.Fatalf("result=%+v error=%v default calls=%d", result, err, fallback.calls)
			}
		})
	}
}

func TestManagerSMTPConcurrentApplications(t *testing.T) {
	firstPort, firstMessages := smtpFixture(t)
	secondPort, secondMessages := smtpFixture(t)
	m := NewManager(nil, nil, zap.NewNop())
	raw, _ := NewSMTPProvider(SMTPConfig{Host: "127.0.0.1", Port: closedSMTPPort(t)}, zap.NewNop())
	m.RegisterProvider(raw)
	var wg sync.WaitGroup
	for _, tc := range []struct {
		port  int
		title string
	}{{firstPort, "first app"}, {secondPort, "second app"}} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := m.Send(appSMTPContext(tc.port), &notification.Notification{Channel: notification.ChannelEmail, Content: notification.Content{Title: tc.title}}, &user.User{Email: "to@example.test"})
			if err != nil || result == nil || !result.Success {
				t.Errorf("%s: %+v %v", tc.title, result, err)
			}
		}()
	}
	wg.Wait()
	for _, tc := range []struct {
		messages <-chan string
		title    string
	}{{firstMessages, "first app"}, {secondMessages, "second app"}} {
		select {
		case msg := <-tc.messages:
			if !strings.Contains(msg, "Subject: "+tc.title) {
				t.Fatal(msg)
			}
		case <-time.After(time.Second):
			t.Fatalf("no message for %s", tc.title)
		}
	}
	if raw.(*SMTPProvider).port == firstPort || raw.(*SMTPProvider).port == secondPort {
		t.Fatal("shared provider mutated")
	}
}

func TestManagerSMTPConfigurationFailureIsTerminal(t *testing.T) {
	for _, registered := range []bool{false, true} {
		t.Run(fmt.Sprint(registered), func(t *testing.T) {
			m := NewManager(nil, nil, zap.NewNop())
			if registered {
				p, _ := NewSMTPProvider(SMTPConfig{Host: "system.test"}, zap.NewNop())
				m.RegisterProvider(p)
			}
			ctx := context.WithValue(context.Background(), EmailConfigKey, &application.EmailConfig{ProviderType: "smtp", SMTP: &application.SMTPConfig{}})
			result, err := m.Send(ctx, &notification.Notification{Channel: notification.ChannelEmail}, &user.User{Email: "to@example.test"})
			var de *DeliveryError
			if !errors.As(err, &de) || de.Retryable || de.ErrorType != ErrorTypeConfiguration || de.CredentialSource != CredSourceBYOC || result.Metadata["credential_source"] != CredSourceBYOC {
				t.Fatalf("%+v %v", result, err)
			}
		})
	}
}

func TestManagerSMTPExplicitFallback(t *testing.T) {
	port, messages := smtpFixture(t)
	m := NewManager(nil, nil, zap.NewNop())
	first := &deliveryFixture{name: "first", channel: notification.ChannelEmail, result: NewErrorResult(errors.New("credentials rejected"), ErrorTypeAuth)}
	m.RegisterProvider(first)
	result, err := m.SendWithFallback(appSMTPContext(port), &notification.Notification{Channel: notification.ChannelEmail, Content: notification.Content{Title: "fallback"}}, &user.User{Email: "to@example.test"}, []string{"first", "smtp"})
	if err != nil || !result.Success || first.calls != 1 {
		t.Fatalf("%+v %v first=%d", result, err, first.calls)
	}
	select {
	case <-messages:
	case <-time.After(time.Second):
		t.Fatal("missing explicit SMTP fallback")
	}
	result, err = m.SendWithFallback(context.Background(), &notification.Notification{Channel: notification.ChannelEmail}, &user.User{}, []string{"first"})
	var de *DeliveryError
	if !errors.As(err, &de) || de.Retryable || de.ErrorType != ErrorTypeAuth || result.Metadata["failure_code"] != ErrorTypeAuth || !strings.Contains(err.Error(), "all fallback providers failed") {
		t.Fatalf("%+v %v", result, err)
	}
}

func TestManagerSMTPHealthRetainsNetworkCause(t *testing.T) {
	m := NewManager(nil, nil, zap.NewNop())
	p, _ := NewSMTPProvider(SMTPConfig{Host: "127.0.0.1", Port: closedSMTPPort(t), Config: Config{Timeout: 100 * time.Millisecond}}, zap.NewNop())
	m.RegisterProvider(p)
	result, err := m.Send(context.Background(), &notification.Notification{Channel: notification.ChannelEmail}, &user.User{Email: "to@example.test"})
	var de *DeliveryError
	var op *net.OpError
	if !errors.As(err, &de) || !errors.As(err, &op) || de.ErrorType != ErrorTypeNetwork || !de.Retryable || result.Metadata["failure_stage"] != "connect" {
		t.Fatalf("%+v %v", result, err)
	}
}
