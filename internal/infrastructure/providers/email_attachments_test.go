package providers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/the-monkeys/freerangenotify/internal/domain/attachment"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
)

// fakeResolver returns a closure usable as AttachmentResolveFunc that yields
// a single in-memory resolved attachment matching the requested filename.
func fakeResolver(filename, mimeType, disposition, contentID string, payload []byte) AttachmentResolveFunc {
	return func(_ context.Context, atts []notification.Attachment) ([]*attachment.Resolved, error) {
		out := make([]*attachment.Resolved, 0, len(atts))
		for range atts {
			out = append(out, &attachment.Resolved{
				Filename:    filename,
				MIMEType:    mimeType,
				Disposition: disposition,
				ContentID:   contentID,
				Bytes:       payload,
				Size:        int64(len(payload)),
				Source:      attachment.SourceInline,
			})
		}
		return out, nil
	}
}

func testNotificationWithAttachment() *notification.Notification {
	n := &notification.Notification{
		NotificationID: "n-1",
		AppID:          "app-1",
	}
	n.Content.Title = "Hello"
	n.Content.Body = "World"
	n.Content.Attachments = []notification.Attachment{{Name: "x.pdf"}}
	return n
}

func testUser() *user.User {
	return &user.User{UserID: "u-1", Email: "to@example.com"}
}

// --- Postmark ---------------------------------------------------------------

func TestPostmarkProvider_Send_AttachmentInJSONBody(t *testing.T) {
	payload := []byte("PDF-BYTES")
	var captured map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"MessageID":"pm-test-1"}`))
	}))
	defer srv.Close()

	prev := postmarkAPIURL
	postmarkAPIURL = srv.URL
	defer func() { postmarkAPIURL = prev }()

	p, err := NewPostmarkProvider(PostmarkConfig{
		Config:      Config{Timeout: 5 * time.Second},
		ServerToken: "tok",
		FromEmail:   "from@example.com",
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewPostmarkProvider: %v", err)
	}

	ctx := context.WithValue(context.Background(), AttachmentResolverKey,
		fakeResolver("x.pdf", "application/pdf", "", "", payload))

	res, err := p.Send(ctx, testNotificationWithAttachment(), testUser())
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !res.Success {
		t.Fatalf("Send failed: %v", res.Error)
	}

	atts, ok := captured["Attachments"].([]interface{})
	if !ok || len(atts) != 1 {
		t.Fatalf("expected 1 attachment in body, got %#v", captured["Attachments"])
	}
	a := atts[0].(map[string]interface{})
	if a["Name"] != "x.pdf" {
		t.Errorf("Name = %v, want x.pdf", a["Name"])
	}
	if a["ContentType"] != "application/pdf" {
		t.Errorf("ContentType = %v, want application/pdf", a["ContentType"])
	}
	want := base64.StdEncoding.EncodeToString(payload)
	if a["Content"] != want {
		t.Errorf("Content base64 mismatch: got %v want %v", a["Content"], want)
	}
	if _, has := a["ContentID"]; has {
		t.Errorf("regular attachment should not have ContentID, got %v", a["ContentID"])
	}
}

func TestPostmarkProvider_Send_InlineAttachmentEmitsContentID(t *testing.T) {
	var captured map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		_, _ = w.Write([]byte(`{"MessageID":"pm-test-2"}`))
	}))
	defer srv.Close()
	prev := postmarkAPIURL
	postmarkAPIURL = srv.URL
	defer func() { postmarkAPIURL = prev }()

	p, _ := NewPostmarkProvider(PostmarkConfig{
		Config: Config{Timeout: 5 * time.Second}, ServerToken: "tok", FromEmail: "f@e.com",
	}, zap.NewNop())

	ctx := context.WithValue(context.Background(), AttachmentResolverKey,
		fakeResolver("logo.png", "image/png", "inline", "logo-cid", []byte("PNG")))

	if _, err := p.Send(ctx, testNotificationWithAttachment(), testUser()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	a := captured["Attachments"].([]interface{})[0].(map[string]interface{})
	if a["ContentID"] != "cid:logo-cid" {
		t.Errorf("inline ContentID = %v, want cid:logo-cid", a["ContentID"])
	}
}

// --- Resend -----------------------------------------------------------------

func TestResendProvider_Send_AttachmentInJSONBody(t *testing.T) {
	payload := []byte("RESEND-BYTES")
	var captured map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		_, _ = w.Write([]byte(`{"id":"rs-1"}`))
	}))
	defer srv.Close()
	prev := resendAPIURL
	resendAPIURL = srv.URL
	defer func() { resendAPIURL = prev }()

	p, err := NewResendProvider(ResendConfig{
		Config: Config{Timeout: 5 * time.Second}, APIKey: "key", FromEmail: "f@e.com",
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewResendProvider: %v", err)
	}
	ctx := context.WithValue(context.Background(), AttachmentResolverKey,
		fakeResolver("report.csv", "text/csv", "", "", payload))

	res, err := p.Send(ctx, testNotificationWithAttachment(), testUser())
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !res.Success {
		t.Fatalf("Send failed: %v", res.Error)
	}
	atts := captured["attachments"].([]interface{})
	if len(atts) != 1 {
		t.Fatalf("want 1 attachment, got %d", len(atts))
	}
	a := atts[0].(map[string]interface{})
	if a["filename"] != "report.csv" || a["content_type"] != "text/csv" {
		t.Errorf("bad metadata: %#v", a)
	}
	if a["content"] != base64.StdEncoding.EncodeToString(payload) {
		t.Errorf("base64 content mismatch")
	}
}

func TestResendProvider_Send_InlineDegradesToRegular(t *testing.T) {
	// Resend does not support inline; provider must warn and still ship.
	var captured map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&captured)
		_, _ = w.Write([]byte(`{"id":"rs-2"}`))
	}))
	defer srv.Close()
	prev := resendAPIURL
	resendAPIURL = srv.URL
	defer func() { resendAPIURL = prev }()

	p, _ := NewResendProvider(ResendConfig{
		Config: Config{Timeout: 5 * time.Second}, APIKey: "k", FromEmail: "f@e.com",
	}, zap.NewNop())
	ctx := context.WithValue(context.Background(), AttachmentResolverKey,
		fakeResolver("logo.png", "image/png", "inline", "logo-cid", []byte("PNG")))

	if _, err := p.Send(ctx, testNotificationWithAttachment(), testUser()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	atts := captured["attachments"].([]interface{})
	if len(atts) != 1 {
		t.Fatalf("want 1 attachment, got %d", len(atts))
	}
	// No inline-specific field is added — Resend has no equivalent.
	a := atts[0].(map[string]interface{})
	if a["filename"] != "logo.png" {
		t.Errorf("filename = %v", a["filename"])
	}
}

// --- Mailgun ----------------------------------------------------------------

func TestMailgunProvider_Send_AttachmentInMultipartForm(t *testing.T) {
	payload := []byte("MAILGUN-FILE-BYTES")
	var (
		capturedCT    string
		capturedField string
		capturedName  string
		capturedBytes []byte
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCT = r.Header.Get("Content-Type")
		_, params, _ := mime.ParseMediaType(capturedCT)
		mr := multipart.NewReader(r.Body, params["boundary"])
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("NextPart: %v", err)
			}
			if fn := part.FileName(); fn != "" {
				capturedField = part.FormName()
				capturedName = fn
				capturedBytes, _ = io.ReadAll(part)
			}
			_ = part.Close()
		}
		_, _ = w.Write([]byte(`{"id":"mg-1","message":"Queued"}`))
	}))
	defer srv.Close()
	prev := mailgunAPIBase
	mailgunAPIBase = srv.URL
	defer func() { mailgunAPIBase = prev }()

	p, err := NewMailgunProvider(MailgunConfig{
		Config:    Config{Timeout: 5 * time.Second},
		APIKey:    "key",
		Domain:    "mg.example.com",
		FromEmail: "f@e.com",
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("NewMailgunProvider: %v", err)
	}
	ctx := context.WithValue(context.Background(), AttachmentResolverKey,
		fakeResolver("report.pdf", "application/pdf", "", "", payload))

	res, err := p.Send(ctx, testNotificationWithAttachment(), testUser())
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !res.Success {
		t.Fatalf("Send failed: %v", res.Error)
	}
	if !strings.HasPrefix(capturedCT, "multipart/form-data") {
		t.Errorf("Content-Type = %q, want multipart/form-data; ...", capturedCT)
	}
	if capturedField != "attachment" {
		t.Errorf("field name = %q, want attachment", capturedField)
	}
	if capturedName != "report.pdf" {
		t.Errorf("filename = %q", capturedName)
	}
	if string(capturedBytes) != string(payload) {
		t.Errorf("bytes mismatch: got %q want %q", capturedBytes, payload)
	}
}

func TestMailgunProvider_Send_InlineUsesInlineField(t *testing.T) {
	var capturedField string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, params, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		mr := multipart.NewReader(r.Body, params["boundary"])
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("NextPart: %v", err)
			}
			if part.FileName() != "" {
				capturedField = part.FormName()
			}
			_ = part.Close()
		}
		_, _ = w.Write([]byte(`{"id":"mg-2"}`))
	}))
	defer srv.Close()
	prev := mailgunAPIBase
	mailgunAPIBase = srv.URL
	defer func() { mailgunAPIBase = prev }()

	p, _ := NewMailgunProvider(MailgunConfig{
		Config: Config{Timeout: 5 * time.Second}, APIKey: "k", Domain: "d.example", FromEmail: "f@e.com",
	}, zap.NewNop())
	ctx := context.WithValue(context.Background(), AttachmentResolverKey,
		fakeResolver("logo.png", "image/png", "inline", "logo-cid", []byte("PNG")))

	if _, err := p.Send(ctx, testNotificationWithAttachment(), testUser()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if capturedField != "inline" {
		t.Errorf("field name = %q, want inline", capturedField)
	}
}

func TestMailgunProvider_Send_NoAttachmentsKeepsURLEncodedFastPath(t *testing.T) {
	var capturedCT, capturedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCT = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		capturedBody = string(b)
		_, _ = w.Write([]byte(`{"id":"mg-3"}`))
	}))
	defer srv.Close()
	prev := mailgunAPIBase
	mailgunAPIBase = srv.URL
	defer func() { mailgunAPIBase = prev }()

	p, _ := NewMailgunProvider(MailgunConfig{
		Config: Config{Timeout: 5 * time.Second}, APIKey: "k", Domain: "d.example", FromEmail: "f@e.com",
	}, zap.NewNop())

	n := &notification.Notification{NotificationID: "n", AppID: "a"}
	n.Content.Title = "Subj"
	n.Content.Body = "Body"
	// No attachments → must keep urlencoded path.
	if _, err := p.Send(context.Background(), n, testUser()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if capturedCT != "application/x-www-form-urlencoded" {
		t.Errorf("Content-Type = %q, want application/x-www-form-urlencoded", capturedCT)
	}
	vals, err := url.ParseQuery(capturedBody)
	if err != nil {
		t.Fatalf("ParseQuery: %v", err)
	}
	if vals.Get("subject") != "Subj" {
		t.Errorf("subject = %q", vals.Get("subject"))
	}
}

// --- Shared helper ----------------------------------------------------------

func TestResolveEmailAttachments_NoResolverMarksDropped(t *testing.T) {
	n := testNotificationWithAttachment()
	resolved, dropped, err := resolveEmailAttachments(context.Background(), n, zap.NewNop(), "test")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if resolved != nil {
		t.Errorf("resolved = %v, want nil", resolved)
	}
	if !dropped {
		t.Errorf("dropped = false, want true (no resolver on ctx)")
	}
}

func TestResolveEmailAttachments_NoAttachmentsIsNoOp(t *testing.T) {
	n := &notification.Notification{}
	resolved, dropped, err := resolveEmailAttachments(context.Background(), n, zap.NewNop(), "test")
	if err != nil || resolved != nil || dropped {
		t.Errorf("expected clean no-op, got (%v, %v, %v)", resolved, dropped, err)
	}
}
