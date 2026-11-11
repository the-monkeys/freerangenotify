package providers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/smtp"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/attachment"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"go.uber.org/zap"
)

type attachmentCapture struct {
	JSON  map[string]interface{}
	Parts map[string][]string
	MIME  string
}

func attachmentContractProvider(t *testing.T, name string) (Provider, <-chan attachmentCapture) {
	t.Helper()
	captured := make(chan attachmentCapture, 2)
	logger := zap.NewNop()
	if name == "smtp" {
		p, err := NewSMTPProvider(SMTPConfig{Config: Config{Timeout: time.Second}, Host: "fixture", FromEmail: "from@example.test"}, logger)
		require.NoError(t, err)
		p.(*SMTPProvider).sender = func(_ string, _ smtp.Auth, _ string, _ []string, body []byte) error {
			captured <- attachmentCapture{MIME: string(body)}
			return nil
		}
		return p, captured
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capture := attachmentCapture{}
		if name == "mailgun" {
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				http.Error(w, "invalid multipart", 400)
				return
			}
			defer r.MultipartForm.RemoveAll()
			capture.Parts = map[string][]string{}
			for field, files := range r.MultipartForm.File {
				for _, file := range files {
					reader, err := file.Open()
					if err != nil {
						http.Error(w, "unreadable part", 400)
						return
					}
					body, _ := io.ReadAll(reader)
					reader.Close()
					capture.Parts[field] = append(capture.Parts[field], file.Filename+":"+string(body))
				}
			}
		} else if err := json.NewDecoder(r.Body).Decode(&capture.JSON); err != nil {
			http.Error(w, "invalid JSON", 400)
			return
		}
		captured <- capture
		w.Header().Set("Content-Type", "application/json")
		switch name {
		case "ses":
			fmt.Fprint(w, `{"MessageId":"fixture-id"}`)
		case "sendgrid":
			w.WriteHeader(202)
		case "postmark":
			fmt.Fprint(w, `{"MessageID":"fixture-id"}`)
		default:
			fmt.Fprint(w, `{"id":"fixture-id","message":"Queued"}`)
		}
	}))
	t.Cleanup(server.Close)
	switch name {
	case "ses":
		return &SESProvider{config: SESConfig{FromEmail: "from@example.test"}, logger: logger, client: sesv2.New(sesv2.Options{Region: "us-east-1", Credentials: credentials.NewStaticCredentialsProvider("fixture-key", "fixture-secret", ""), BaseEndpoint: aws.String(server.URL), HTTPClient: server.Client()})}, captured
	case "sendgrid":
		p, err := NewSendGridProvider(SendGridConfig{APIKey: "fixture-key", FromEmail: "from@example.test"}, logger)
		require.NoError(t, err)
		p.(*SendGridProvider).client.BaseURL = server.URL
		return p, captured
	case "postmark":
		old := postmarkAPIURL
		postmarkAPIURL = server.URL
		t.Cleanup(func() { postmarkAPIURL = old })
		p, err := NewPostmarkProvider(PostmarkConfig{ServerToken: "fixture-token", FromEmail: "from@example.test"}, logger)
		require.NoError(t, err)
		return p, captured
	case "resend":
		old := resendAPIURL
		resendAPIURL = server.URL
		t.Cleanup(func() { resendAPIURL = old })
		p, err := NewResendProvider(ResendConfig{APIKey: "fixture-key", FromEmail: "from@example.test"}, logger)
		require.NoError(t, err)
		return p, captured
	case "mailgun":
		old := mailgunAPIBase
		mailgunAPIBase = server.URL
		t.Cleanup(func() { mailgunAPIBase = old })
		p, err := NewMailgunProvider(MailgunConfig{APIKey: "fixture-key", Domain: "example.test", FromEmail: "from@example.test"}, logger)
		require.NoError(t, err)
		return p, captured
	default:
		t.Fatal("unknown provider")
		return nil, nil
	}
}

type attachmentTrackingReader struct {
	io.ReadCloser
	closed *int
}

func (r *attachmentTrackingReader) Close() error { *r.closed++; return r.ReadCloser.Close() }

func TestEmailAttachmentContract(t *testing.T) {
	for _, name := range []string{"smtp", "ses", "sendgrid", "mailgun", "postmark", "resend"} {
		t.Run(name, func(t *testing.T) {
			provider, captured := attachmentContractProvider(t, name)
			source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "URL-BYTES") }))
			defer source.Close()
			file := filepath.Join(t.TempDir(), "document.pdf")
			require.NoError(t, os.WriteFile(file, []byte("FILE-BYTES"), 0600))
			n := testNotificationWithAttachment()
			n.Content.Body = `<img src="cid:logo-image">`
			n.Content.Attachments = []notification.Attachment{
				{Type: "image", Name: "logo.png", ContentBase64: base64.StdEncoding.EncodeToString([]byte("INLINE-BYTES")), ContentID: "logo-image", Disposition: "inline", MimeType: "image/png"},
				{Type: "file", Name: "report.csv", URL: source.URL, MimeType: "text/csv"},
				{Type: "file", Name: "document.pdf", FileID: "file_fixture", MimeType: "application/pdf"},
			}
			closed := 0
			resolve := AttachmentResolveFunc(func(ctx context.Context, specs []notification.Attachment) ([]*attachment.Resolved, error) {
				out := []*attachment.Resolved{}
				for _, spec := range specs {
					resolved := &attachment.Resolved{Filename: spec.Name, MIMEType: spec.MimeType, Disposition: spec.Disposition, ContentID: spec.ContentID}
					switch {
					case spec.ContentBase64 != "":
						body, err := base64.StdEncoding.DecodeString(spec.ContentBase64)
						if err != nil {
							return nil, err
						}
						resolved.Bytes = body
						resolved.Source = attachment.SourceInline
					case spec.URL != "":
						response, err := http.Get(spec.URL)
						if err != nil {
							return nil, err
						}
						resolved.Reader = &attachmentTrackingReader{ReadCloser: response.Body, closed: &closed}
						resolved.Source = attachment.SourceURL
					case spec.FileID == "file_fixture":
						reader, err := os.Open(file)
						if err != nil {
							return nil, err
						}
						resolved.Reader = &attachmentTrackingReader{ReadCloser: reader, closed: &closed}
						resolved.Source = attachment.SourceFileID
					default:
						return nil, fmt.Errorf("unexpected source")
					}
					out = append(out, resolved)
				}
				return out, nil
			})
			result, err := provider.Send(context.WithValue(context.Background(), AttachmentResolverKey, resolve), n, testUser())
			require.NoError(t, err)
			require.True(t, result.Success, "%v", result.Error)
			require.Equal(t, 2, closed, "URL and file streams must close")
			var wire attachmentCapture
			select {
			case wire = <-captured:
			case <-time.After(time.Second):
				t.Fatal("no captured provider request")
			}
			if name == "ses" {
				encoded := wire.JSON["Content"].(map[string]interface{})["Raw"].(map[string]interface{})["Data"].(string)
				raw, err := base64.StdEncoding.DecodeString(encoded)
				require.NoError(t, err)
				wire.MIME = string(raw)
			}
			switch name {
			case "smtp", "ses":
				require.Contains(t, strings.ToLower(wire.MIME), "content-id: <logo-image>")
				for _, body := range []string{"INLINE-BYTES", "URL-BYTES", "FILE-BYTES"} {
					require.Contains(t, wire.MIME, base64.StdEncoding.EncodeToString([]byte(body)))
				}
			case "mailgun":
				require.Equal(t, []string{"logo-image:INLINE-BYTES"}, wire.Parts["inline"])
				require.ElementsMatch(t, []string{"report.csv:URL-BYTES", "document.pdf:FILE-BYTES"}, wire.Parts["attachment"])
			default:
				field := "attachments"
				content := "content"
				cid := "content_id"
				expectedCID := "logo-image"
				if name == "postmark" {
					field = "Attachments"
					content = "Content"
					cid = "ContentID"
					expectedCID = "cid:logo-image"
				}
				list := wire.JSON[field].([]interface{})
				require.Len(t, list, 3)
				for index, body := range []string{"INLINE-BYTES", "URL-BYTES", "FILE-BYTES"} {
					item := list[index].(map[string]interface{})
					require.Equal(t, base64.StdEncoding.EncodeToString([]byte(body)), item[content])
					if index == 0 {
						require.Equal(t, expectedCID, item[cid])
					} else {
						require.NotContains(t, item, cid)
					}
				}
			}
			// Resolver failure must never become a provider call or a successful send.
			cause := errors.New("fixture source unavailable")
			fail := AttachmentResolveFunc(func(context.Context, []notification.Attachment) ([]*attachment.Resolved, error) { return nil, cause })
			result, err = provider.Send(context.WithValue(context.Background(), AttachmentResolverKey, fail), n, testUser())
			require.NoError(t, err)
			require.False(t, result.Success)
			require.ErrorIs(t, result.Error, cause)
			require.Empty(t, captured)
		})
	}
}
