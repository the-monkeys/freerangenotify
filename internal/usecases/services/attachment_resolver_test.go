package services

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
)

func TestAttachmentResolver_Inline_Base64(t *testing.T) {
	r := NewAttachmentResolver(nil, AttachmentResolverConfig{}, nil)
	raw := []byte("hello world")
	enc := base64.StdEncoding.EncodeToString(raw)

	ra, err := r.Resolve(context.Background(), "app1", notification.Attachment{
		Type:          "file",
		ContentBase64: enc,
		MimeType:      "text/plain",
		Name:          "hello.txt",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	defer ra.Close()
	if ra.Source != AttachmentSourceInline {
		t.Errorf("Source = %q", ra.Source)
	}
	if string(ra.Bytes) != "hello world" {
		t.Errorf("bytes = %q", ra.Bytes)
	}
	if ra.Size != int64(len(raw)) {
		t.Errorf("size = %d", ra.Size)
	}
	if ra.Disposition != "attachment" {
		t.Errorf("default disposition = %q", ra.Disposition)
	}
}

func TestAttachmentResolver_Inline_DataURLPrefix(t *testing.T) {
	r := NewAttachmentResolver(nil, AttachmentResolverConfig{}, nil)
	raw := []byte("PNGDATA")
	enc := "data:image/png;base64," + base64.StdEncoding.EncodeToString(raw)
	ra, err := r.Resolve(context.Background(), "app1", notification.Attachment{
		Type:          "image",
		ContentBase64: enc,
		MimeType:      "image/png",
		Disposition:   "inline",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	defer ra.Close()
	if string(ra.Bytes) != "PNGDATA" {
		t.Errorf("bytes = %q", ra.Bytes)
	}
	if ra.Disposition != "inline" {
		t.Errorf("disposition = %q", ra.Disposition)
	}
}

func TestAttachmentResolver_Inline_InvalidBase64(t *testing.T) {
	r := NewAttachmentResolver(nil, AttachmentResolverConfig{}, nil)
	_, err := r.Resolve(context.Background(), "app1", notification.Attachment{
		Type:          "file",
		ContentBase64: "!!!not base64!!!",
	})
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestAttachmentResolver_FileID_RoundTrip(t *testing.T) {
	svc, _, _ := newSvc(t, FileServiceConfig{})
	ctx := context.Background()
	obj, err := svc.Upload(ctx, UploadInput{
		AppID: "app1", Name: "doc.pdf", MIMEType: "application/pdf",
		DeclaredSize: 11, Reader: strings.NewReader("hello world"),
	})
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}

	r := NewAttachmentResolver(svc, AttachmentResolverConfig{}, nil)
	ra, err := r.Resolve(ctx, "app1", notification.Attachment{
		Type:   "file",
		FileID: obj.FileID,
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	defer ra.Close()
	if ra.Source != AttachmentSourceFileID {
		t.Errorf("Source = %q", ra.Source)
	}
	if ra.SHA256 != obj.SHA256 {
		t.Errorf("SHA256 = %q want %q", ra.SHA256, obj.SHA256)
	}
	if ra.Reader == nil {
		t.Fatal("Reader is nil")
	}
	body, _ := io.ReadAll(ra.Reader)
	if string(body) != "hello world" {
		t.Errorf("bytes = %q", body)
	}
	if ra.Filename != "doc.pdf" {
		t.Errorf("filename = %q", ra.Filename)
	}
}

func TestAttachmentResolver_FileID_CrossTenant(t *testing.T) {
	svc, _, _ := newSvc(t, FileServiceConfig{})
	ctx := context.Background()
	obj, _ := svc.Upload(ctx, UploadInput{
		AppID: "app1", Name: "x.pdf", MIMEType: "application/pdf",
		DeclaredSize: 3, Reader: strings.NewReader("xyz"),
	})
	r := NewAttachmentResolver(svc, AttachmentResolverConfig{}, nil)
	_, err := r.Resolve(ctx, "app2", notification.Attachment{
		Type: "file", FileID: obj.FileID,
	})
	if err == nil {
		t.Fatal("cross-tenant resolve must fail")
	}
}

func TestAttachmentResolver_FileID_NotConfigured(t *testing.T) {
	r := NewAttachmentResolver(nil, AttachmentResolverConfig{}, nil)
	_, err := r.Resolve(context.Background(), "app1", notification.Attachment{
		Type: "file", FileID: "file_x",
	})
	if !errors.Is(err, ErrFileSourceUnavailable) {
		t.Errorf("want ErrFileSourceUnavailable, got %v", err)
	}
}

func TestAttachmentResolver_URL_Fetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("PNGBYTES"))
	}))
	defer srv.Close()

	r := NewAttachmentResolver(nil, AttachmentResolverConfig{HTTPClient: srv.Client()}, nil)
	ra, err := r.Resolve(context.Background(), "app1", notification.Attachment{
		Type: "image", URL: srv.URL + "/path/banner.png",
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	defer ra.Close()
	if ra.Source != AttachmentSourceURL {
		t.Errorf("Source = %q", ra.Source)
	}
	if string(ra.Bytes) != "PNGBYTES" {
		t.Errorf("bytes = %q", ra.Bytes)
	}
	if ra.MIMEType != "image/png" {
		t.Errorf("mime = %q", ra.MIMEType)
	}
	if ra.Filename != "banner.png" {
		t.Errorf("filename = %q", ra.Filename)
	}
}

func TestAttachmentResolver_URL_Oversize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(make([]byte, 1024))
	}))
	defer srv.Close()
	r := NewAttachmentResolver(nil, AttachmentResolverConfig{
		HTTPClient:       srv.Client(),
		URLFetchMaxBytes: 128,
	}, nil)
	_, err := r.Resolve(context.Background(), "app1", notification.Attachment{
		Type: "file", URL: srv.URL,
	})
	if !errors.Is(err, ErrAttachmentURLOversize) {
		t.Errorf("want ErrAttachmentURLOversize, got %v", err)
	}
}

func TestAttachmentResolver_URL_Non2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer srv.Close()
	r := NewAttachmentResolver(nil, AttachmentResolverConfig{HTTPClient: srv.Client()}, nil)
	_, err := r.Resolve(context.Background(), "app1", notification.Attachment{
		Type: "file", URL: srv.URL,
	})
	if err == nil {
		t.Fatal("expected error on 404")
	}
}

func TestAttachmentResolver_NoSource(t *testing.T) {
	r := NewAttachmentResolver(nil, AttachmentResolverConfig{}, nil)
	_, err := r.Resolve(context.Background(), "app1", notification.Attachment{Type: "file"})
	if !errors.Is(err, notification.ErrAttachmentMissingSource) {
		t.Errorf("want ErrAttachmentMissingSource, got %v", err)
	}
}

func TestAttachmentResolver_AmbiguousSource(t *testing.T) {
	r := NewAttachmentResolver(nil, AttachmentResolverConfig{}, nil)
	_, err := r.Resolve(context.Background(), "app1", notification.Attachment{
		Type:          "file",
		URL:           "https://example.com/x",
		ContentBase64: base64.StdEncoding.EncodeToString([]byte("x")),
	})
	if !errors.Is(err, notification.ErrAmbiguousAttachmentSource) {
		t.Errorf("want ErrAmbiguousAttachmentSource, got %v", err)
	}
}

func TestAttachmentResolver_ResolveAll_RollsBackOnError(t *testing.T) {
	svc, _, _ := newSvc(t, FileServiceConfig{})
	ctx := context.Background()
	obj, _ := svc.Upload(ctx, UploadInput{
		AppID: "app1", Name: "x.pdf", MIMEType: "application/pdf",
		DeclaredSize: 3, Reader: strings.NewReader("xyz"),
	})
	r := NewAttachmentResolver(svc, AttachmentResolverConfig{}, nil)

	_, err := r.ResolveAll(ctx, "app1", []notification.Attachment{
		{Type: "file", FileID: obj.FileID},
		{Type: "file"}, // invalid — triggers rollback of the first.
	})
	if err == nil {
		t.Fatal("expected error from second entry")
	}
}
