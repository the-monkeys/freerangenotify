package providers

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/smtp"
	"net/textproto"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/attachment"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"go.uber.org/zap"
)

func tlsSMTPFixture(t *testing.T, implicit bool, authDisconnects ...int) (int, *tls.Config, <-chan string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, _ := x509.MarshalPKCS8PrivateKey(key)
	cert, err := tls.X509KeyPair(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	parsed, _ := x509.ParseCertificate(der)
	pool.AddCert(parsed)
	addr := "127.0.0.1:0"
	if implicit {
		addr = "127.0.0.1:465"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		if implicit {
			t.Skipf("465 unavailable: %v", err)
		}
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	messages := make(chan string, 4)
	var authAttempts atomic.Int32
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				conn.SetDeadline(time.Now().Add(2 * time.Second))
				secure := implicit
				serverTLS := &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
				if implicit {
					conn = tls.Server(conn, serverTLS)
				}
				fmt.Fprint(conn, "220 localhost ESMTP\r\n")
				r := bufio.NewReader(conn)
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					switch {
					case strings.HasPrefix(line, "EHLO"):
						if secure {
							fmt.Fprint(conn, "250-localhost\r\n250 AUTH PLAIN\r\n")
						} else {
							fmt.Fprint(conn, "250-localhost\r\n250 STARTTLS\r\n")
						}
					case strings.HasPrefix(line, "STARTTLS"):
						fmt.Fprint(conn, "220 start TLS\r\n")
						conn = tls.Server(conn, serverTLS)
						r = bufio.NewReader(conn)
						secure = true
					case strings.HasPrefix(line, "AUTH"):
						if len(authDisconnects) > 0 && int(authAttempts.Add(1)) <= authDisconnects[0] {
							return
						}
						if !secure {
							fmt.Fprint(conn, "535 insecure auth\r\n")
						} else {
							fmt.Fprint(conn, "235 authenticated\r\n")
						}
					case strings.HasPrefix(line, "DATA"):
						fmt.Fprint(conn, "354 send data\r\n")
						var b strings.Builder
						for {
							line, err = r.ReadString('\n')
							if err != nil {
								return
							}
							if line == ".\r\n" {
								break
							}
							b.WriteString(line)
						}
						messages <- b.String()
						fmt.Fprint(conn, "250 accepted\r\n")
					case strings.HasPrefix(line, "QUIT"):
						fmt.Fprint(conn, "221 bye\r\n")
						return
					default:
						fmt.Fprint(conn, "250 ok\r\n")
					}
				}
			}()
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port, &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}, messages
}

func TestProviderReviewSMTPAuthEOF(t *testing.T) {
	t.Run("reconnect succeeds", func(t *testing.T) {
		port, tlsConfig, messages := tlsSMTPFixture(t, false, 1)
		p, _ := NewSMTPProvider(SMTPConfig{Config: Config{Timeout: time.Second, MaxRetries: 1}, Host: "127.0.0.1", Port: port, Username: "customer", Password: "secret", FromEmail: "from@example.test", TLSConfig: tlsConfig}, zap.NewNop())
		result, err := p.Send(context.Background(), &notification.Notification{}, &user.User{Email: "to@example.test"})
		if err != nil || !result.Success {
			t.Fatalf("AUTH disconnect prevented reconnect: %+v %v", result, err)
		}
		select {
		case <-messages:
		case <-time.After(time.Second):
			t.Fatal("no delivered message after reconnect")
		}
	})
	t.Run("exhausted attempts retain cause", func(t *testing.T) {
		port, tlsConfig, _ := tlsSMTPFixture(t, false, 3)
		p, _ := NewSMTPProvider(SMTPConfig{Config: Config{Timeout: time.Second, MaxRetries: 1}, Host: "127.0.0.1", Port: port, Username: "customer", Password: "secret", TLSConfig: tlsConfig}, zap.NewNop())
		result, _ := p.Send(context.Background(), &notification.Notification{}, &user.User{Email: "to@example.test"})
		var failure *DeliveryError
		if !errors.As(result.Error, &failure) || !failure.Retryable || failure.Stage != "auth" || !errors.Is(result.Error, io.EOF) {
			t.Fatalf("AUTH EOF became terminal or lost: %+v", result)
		}
	})
	for _, cause := range []error{io.EOF, io.ErrUnexpectedEOF} {
		failure := smtpOperationError(context.Background(), "auth", CredSourceBYOC, cause)
		var typed *DeliveryError
		if !errors.As(failure, &typed) || !typed.Retryable || !errors.Is(failure, cause) {
			t.Fatalf("connection loss became terminal: %v", failure)
		}
	}
}

func TestSMTPProviderTLSModes(t *testing.T) {
	for _, implicit := range []bool{false, true} {
		t.Run(fmt.Sprint(implicit), func(t *testing.T) {
			port, tlsConfig, messages := tlsSMTPFixture(t, implicit)
			p, err := NewSMTPProvider(SMTPConfig{Config: Config{Timeout: time.Second}, Host: "127.0.0.1", Port: port, Username: "customer", Password: "secret", FromEmail: "from@example.test", TLSConfig: tlsConfig}, zap.NewNop())
			if err != nil {
				t.Fatal(err)
			}
			if !p.IsHealthy(context.Background()) {
				t.Fatal("TLS SMTP unhealthy")
			}
			result, err := p.Send(context.Background(), &notification.Notification{Content: notification.Content{Title: "TLS subject"}}, &user.User{Email: "to@example.test"})
			if err != nil || !result.Success {
				t.Fatalf("send=%+v err=%v", result, err)
			}
			select {
			case msg := <-messages:
				if !strings.Contains(msg, "TLS subject") {
					t.Fatal(msg)
				}
			case <-time.After(time.Second):
				t.Fatal("no TLS delivery")
			}
		})
	}
}

func TestSMTPProviderHealthAndSendBounded(t *testing.T) {
	for _, send := range []bool{false, true} {
		for _, cancel := range []bool{false, true} {
			t.Run(fmt.Sprintf("send=%v/cancel=%v", send, cancel), func(t *testing.T) {
				ln, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				defer ln.Close()
				accepted := make(chan net.Conn, 1)
				go func() {
					c, err := ln.Accept()
					if err == nil {
						accepted <- c
					}
				}()
				p, _ := NewSMTPProvider(SMTPConfig{Config: Config{Timeout: 80 * time.Millisecond}, Host: "127.0.0.1", Port: ln.Addr().(*net.TCPAddr).Port}, zap.NewNop())
				ctx, cancelFn := context.WithCancel(context.Background())
				defer cancelFn()
				if cancel {
					time.AfterFunc(20*time.Millisecond, cancelFn)
				}
				start := time.Now()
				var failure error
				if send {
					result, _ := p.Send(ctx, &notification.Notification{}, &user.User{Email: "to@example.test"})
					failure = result.Error
				} else {
					failure = p.(HealthChecker).CheckHealth(ctx)
				}
				c := <-accepted
				c.Close()
				want := context.DeadlineExceeded
				if cancel {
					want = context.Canceled
				}
				if !errors.Is(failure, want) || time.Since(start) > 400*time.Millisecond {
					t.Fatalf("elapsed=%s error=%v", time.Since(start), failure)
				}
			})
		}
	}
}

func TestSMTPProviderTemporaryErrorsRetry(t *testing.T) {
	p, _ := NewSMTPProvider(SMTPConfig{Host: "localhost", Config: Config{MaxRetries: 2}}, zap.NewNop())
	attempts := 0
	p.(*SMTPProvider).sender = func(string, smtp.Auth, string, []string, []byte) error {
		attempts++
		return &textproto.Error{Code: 451, Msg: "temporary"}
	}
	result, _ := p.Send(context.Background(), &notification.Notification{}, &user.User{Email: "to@example.test"})
	if result.Success || attempts != 3 {
		t.Fatalf("attempts=%d result=%+v", attempts, result)
	}
}

func TestSMTPProviderTransientFailThenSuccess(t *testing.T) {
	raw, _ := NewSMTPProvider(SMTPConfig{Host: "localhost", Config: Config{MaxRetries: 2, RetryDelay: time.Millisecond}}, zap.NewNop())
	p := raw.(*SMTPProvider)
	attempts := 0
	p.sender = func(string, smtp.Auth, string, []string, []byte) error {
		attempts++
		if attempts == 1 {
			return &textproto.Error{Code: 451, Msg: "try later"}
		}
		return nil
	}
	result, _ := p.Send(context.Background(), &notification.Notification{}, &user.User{Email: "to@example.test"})
	if !result.Success || attempts != 2 {
		t.Fatalf("result=%+v attempts=%d", result, attempts)
	}
}

func TestSMTPProviderAttachmentErrorsRetainRetryPolicy(t *testing.T) {
	for _, tc := range []struct {
		name  string
		cause error
		retry bool
	}{{"remote timeout", context.DeadlineExceeded, true}, {"remote temporary", errors.New("temporary upstream fetch failure"), true}, {"permanent", notification.ErrInvalidAttachment, false}} {
		t.Run(tc.name, func(t *testing.T) {
			raw, _ := NewSMTPProvider(SMTPConfig{Host: "localhost"}, zap.NewNop())
			p := raw.(*SMTPProvider)
			p.sender = func(string, smtp.Auth, string, []string, []byte) error {
				t.Error("sent despite unresolved attachments")
				return nil
			}
			ctx := context.WithValue(context.Background(), AttachmentResolverKey, AttachmentResolveFunc(func(context.Context, []notification.Attachment) ([]*attachment.Resolved, error) {
				return nil, fmt.Errorf("resolve: %w", tc.cause)
			}))
			result, _ := p.Send(ctx, testNotificationWithAttachment(), testUser())
			var de *DeliveryError
			if !errors.As(result.Error, &de) || de.Retryable != tc.retry || !errors.Is(result.Error, tc.cause) {
				t.Fatalf("result=%+v", result)
			}
		})
	}
}

type failedSMTPAttachmentReader struct{}

func (failedSMTPAttachmentReader) Read([]byte) (int, error) { return 0, context.DeadlineExceeded }

func TestSMTPProviderStreamingAttachmentTimeoutRetainsCause(t *testing.T) {
	raw, _ := NewSMTPProvider(SMTPConfig{Host: "localhost"}, zap.NewNop())
	p := raw.(*SMTPProvider)
	ctx := context.WithValue(context.Background(), AttachmentResolverKey, AttachmentResolveFunc(func(context.Context, []notification.Attachment) ([]*attachment.Resolved, error) {
		return []*attachment.Resolved{{Filename: "remote.txt", MIMEType: "text/plain", Reader: io.NopCloser(failedSMTPAttachmentReader{})}}, nil
	}))
	result, _ := p.Send(ctx, testNotificationWithAttachment(), testUser())
	var de *DeliveryError
	if !errors.As(result.Error, &de) || !de.Retryable || !errors.Is(result.Error, context.DeadlineExceeded) || !errors.Is(result.Error, ErrSMTPAttachmentReadFailed) {
		t.Fatalf("result=%+v", result)
	}
}
