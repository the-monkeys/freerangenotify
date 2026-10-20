package contracts

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	domainfile "github.com/the-monkeys/freerangenotify/internal/domain/file"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/filestore"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/providers"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"go.uber.org/zap"
)

func TestRichDocumentationContracts(t *testing.T) { runExamples(t, loadCases(t)) }
func verifyExampleSignature(raw []byte, signature, secret string) bool {
	if secret == "" {
		return false
	}
	received, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(raw)
	return hmac.Equal(mac.Sum(nil), received)
}
func TestRichDocumentationWebhookSignature(t *testing.T) {
	for _, secret := range []string{"fixture-secret", ""} {
		t.Run(secret, func(t *testing.T) {
			type capture struct {
				body      []byte
				signature string
			}
			requests := make(chan capture, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				requests <- capture{body, r.Header.Get("X-Webhook-Signature")}
				w.WriteHeader(204)
			}))
			defer server.Close()
			provider, err := providers.NewWebhookProvider(providers.WebhookConfig{Config: providers.Config{Timeout: time.Second}, Secret: secret}, zap.NewNop())
			require.NoError(t, err)
			n := &notification.Notification{NotificationID: "fixture-notification", AppID: "fixture-app", CreatedAt: time.Now().UTC(), Content: notification.Content{Title: "Update", Body: "Ready", Attachments: []notification.Attachment{{Type: "file", FileID: "file_fixture", Name: "report.pdf"}}}, Metadata: map[string]interface{}{"webhook_url": server.URL}}
			result, err := provider.Send(context.Background(), n, nil)
			require.NoError(t, err)
			require.True(t, result.Success)
			request := <-requests
			var body map[string]interface{}
			require.NoError(t, json.Unmarshal(request.body, &body))
			require.Equal(t, n.NotificationID, body["id"])
			require.Contains(t, body, "created_at")
			require.NotContains(t, body, "notification_id")
			content := body["content"].(map[string]interface{})
			file := content["attachments"].([]interface{})[0].(map[string]interface{})
			require.Equal(t, "file_fixture", file["file_id"])
			require.NotContains(t, file, "content_base64")
			if secret == "" {
				require.Empty(t, request.signature)
				require.False(t, verifyExampleSignature(request.body, request.signature, ""))
			} else {
				require.True(t, verifyExampleSignature(request.body, request.signature, secret))
				require.False(t, verifyExampleSignature(append(request.body, ' '), request.signature, secret))
				require.False(t, verifyExampleSignature(request.body, "00", secret))
			}
		})
	}
}
func TestRichDocumentationCustomFileLinks(t *testing.T) {
	for _, kind := range []string{"slack", "teams"} {
		t.Run(kind, func(t *testing.T) {
			requests := make(chan []byte, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				requests <- body
				w.WriteHeader(200)
			}))
			defer server.Close()
			signer, err := filestore.NewSigner(time.Minute, "fixture-key")
			require.NoError(t, err)
			ctx := context.WithValue(context.Background(), providers.FileDownloadURLKey, providers.FileDownloadURLFunc(func(app, id string) string {
				exp, sig := signer.Sign(app, id)
				return filestore.BuildSignedDownloadURL("https://notify.example/prefix", app, id, exp, sig)
			}))
			n := &notification.Notification{NotificationID: "fixture-id", AppID: "app&owner=other", Content: notification.Content{Title: "Document", Body: "Ready", Attachments: []notification.Attachment{{Type: "file", FileID: "file_fixture", Name: "report.pdf"}}}}
			provider := providers.NewCustomProvider("fixture", "webhook", kind, server.URL, "", "v1", nil, zap.NewNop())
			result, err := provider.Send(ctx, n, nil)
			require.NoError(t, err)
			require.True(t, result.Success)
			body := <-requests
			require.Contains(t, string(body), "/prefix/v1/files/download/file_fixture")
			link := n.Content.Attachments[0].URL
			u, err := url.Parse(link)
			require.NoError(t, err)
			require.Equal(t, n.AppID, u.Query().Get("app_id"))
			require.NoError(t, signer.VerifyQuery(n.AppID, "file_fixture", u.Query().Get("exp"), u.Query().Get("sig")))
		})
	}
}

type richFileMetadata struct {
	domainfile.Repository
	file *domainfile.FileObject
}

func (r richFileMetadata) GetByID(context.Context, string, string) (*domainfile.FileObject, error) {
	if r.file == nil {
		return nil, domainfile.ErrFileNotFound
	}
	return r.file, nil
}
func TestRichDocumentationFileErrors(t *testing.T) {
	for _, expired := range []bool{false, true} {
		t.Run(fmt.Sprint(expired), func(t *testing.T) {
			metadata := richFileMetadata{}
			expected := domainfile.ErrFileNotFound
			if expired {
				metadata.file = &domainfile.FileObject{FileID: "file_saved_report", AppID: "app-a", Name: "report.pdf", Size: 5, MIMEType: "application/pdf", ExpiresAt: time.Now().Add(-time.Minute)}
				expected = domainfile.ErrFileExpired
			}
			store, err := filestore.NewLocalStore(t.TempDir(), nil)
			require.NoError(t, err)
			files := services.NewFileService(store, metadata, services.FileServiceConfig{}, nil)
			resolver := services.NewAttachmentResolver(files, services.AttachmentResolverConfig{}, nil)
			_, err = resolver.Resolve(context.Background(), "app-a", notification.Attachment{Type: "file", FileID: "file_saved_report"})
			require.ErrorIs(t, err, expected)
		})
	}
}
