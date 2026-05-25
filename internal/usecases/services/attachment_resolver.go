package services

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
)

// AttachmentSource identifies how a ResolvedAttachment's bytes were obtained.
type AttachmentSource string

const (
	AttachmentSourceURL    AttachmentSource = "url"
	AttachmentSourceInline AttachmentSource = "inline"
	AttachmentSourceFileID AttachmentSource = "file_id"
)

// DefaultURLFetchMaxBytes caps the size of an attachment pulled from a remote
// URL. Larger payloads MUST go through POST /v1/files so the platform tracks
// them.
const DefaultURLFetchMaxBytes int64 = 25 * 1024 * 1024 // 25 MiB

// DefaultURLFetchTimeout bounds a single attachment download.
const DefaultURLFetchTimeout = 20 * time.Second

// ResolvedAttachment is the channel-agnostic, byte-ready view of an inbound
// notification.Attachment. Providers consume this instead of the raw spec so
// URL / base64 / file_id are unified at one boundary.
//
// Exactly one of Bytes or Reader is populated. The caller MUST invoke Close
// to release any underlying file or HTTP body, even when consuming Bytes —
// Close is a safe no-op when there is nothing to release.
type ResolvedAttachment struct {
	Filename    string
	MIMEType    string
	Disposition string // "attachment" (default) | "inline"
	ContentID   string // RFC 2392 token for inline HTML email embed
	Bytes       []byte
	Reader      io.ReadCloser
	Size        int64
	Source      AttachmentSource
	SHA256      string // populated for file_id source; empty otherwise
}

// Close releases any retained resources. It is safe to call multiple times.
func (r *ResolvedAttachment) Close() error {
	if r == nil || r.Reader == nil {
		return nil
	}
	err := r.Reader.Close()
	r.Reader = nil
	return err
}

// AttachmentResolverConfig holds resolver tunables. Zero values fall back to
// package defaults.
type AttachmentResolverConfig struct {
	HTTPClient       *http.Client
	URLFetchMaxBytes int64
	URLFetchTimeout  time.Duration
}

// AttachmentResolver materialises a notification.Attachment into a
// ResolvedAttachment regardless of source. It is the single boundary between
// the abstract attachment shape and the byte buffers providers need.
type AttachmentResolver struct {
	files       *FileService
	httpClient  *http.Client
	maxURLBytes int64
	logger      *zap.Logger
}

// NewAttachmentResolver wires the resolver. The FileService is required when
// callers may submit file_id attachments; pass nil to disable that source.
func NewAttachmentResolver(files *FileService, cfg AttachmentResolverConfig, logger *zap.Logger) *AttachmentResolver {
	if logger == nil {
		logger = zap.NewNop()
	}
	timeout := cfg.URLFetchTimeout
	if timeout <= 0 {
		timeout = DefaultURLFetchTimeout
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: timeout}
	}
	maxBytes := cfg.URLFetchMaxBytes
	if maxBytes <= 0 {
		maxBytes = DefaultURLFetchMaxBytes
	}
	return &AttachmentResolver{
		files:       files,
		httpClient:  httpClient,
		maxURLBytes: maxBytes,
		logger:      logger,
	}
}

// ErrFileSourceUnavailable is returned when a notification carries a file_id
// attachment but the resolver was not wired with a FileService.
var ErrFileSourceUnavailable = errors.New("attachment resolver: file_id source is not configured")

// ErrAttachmentURLOversize is returned when a remote URL response exceeds the
// configured cap. The caller should either raise the cap (operator policy)
// or stage the file via POST /v1/files first.
var ErrAttachmentURLOversize = errors.New("attachment resolver: remote URL response exceeds max bytes")

// Resolve materialises a single attachment. The caller is responsible for
// calling Close on the returned value to release any open resources.
func (r *AttachmentResolver) Resolve(ctx context.Context, appID string, a notification.Attachment) (*ResolvedAttachment, error) {
	// Determine source — Validate() on the parent Content enforces
	// exactly-one, but we are defensive in case Resolve is called directly.
	sources := 0
	if a.URL != "" {
		sources++
	}
	if a.ContentBase64 != "" {
		sources++
	}
	if a.FileID != "" {
		sources++
	}
	switch sources {
	case 0:
		return nil, notification.ErrAttachmentMissingSource
	case 1:
		// ok
	default:
		return nil, notification.ErrAmbiguousAttachmentSource
	}

	disp := strings.ToLower(strings.TrimSpace(a.Disposition))
	if disp == "" {
		disp = "attachment"
	}

	switch {
	case a.FileID != "":
		return r.resolveFile(ctx, appID, a, disp)
	case a.ContentBase64 != "":
		return r.resolveInline(a, disp)
	case a.URL != "":
		return r.resolveURL(ctx, a, disp)
	}
	// Unreachable.
	return nil, notification.ErrAttachmentMissingSource
}

func (r *AttachmentResolver) resolveFile(ctx context.Context, appID string, a notification.Attachment, disp string) (*ResolvedAttachment, error) {
	if r.files == nil {
		return nil, ErrFileSourceUnavailable
	}
	obj, rc, err := r.files.OpenContent(ctx, appID, a.FileID)
	if err != nil {
		return nil, fmt.Errorf("attachment resolver: file %s: %w", a.FileID, err)
	}
	name := pickNonEmpty(a.Name, obj.Name)
	mime := pickNonEmpty(a.MimeType, obj.MIMEType)
	return &ResolvedAttachment{
		Filename:    name,
		MIMEType:    mime,
		Disposition: disp,
		ContentID:   a.ContentID,
		Reader:      rc,
		Size:        obj.Size,
		Source:      AttachmentSourceFileID,
		SHA256:      obj.SHA256,
	}, nil
}

func (r *AttachmentResolver) resolveInline(a notification.Attachment, disp string) (*ResolvedAttachment, error) {
	// Strip a data-URL prefix if the caller pasted one verbatim.
	payload := a.ContentBase64
	if i := strings.Index(payload, ","); i >= 0 && strings.HasPrefix(payload, "data:") {
		payload = payload[i+1:]
	}
	bytes, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		// Tolerate URL-safe base64 too.
		bytes, err = base64.RawURLEncoding.DecodeString(payload)
		if err != nil {
			return nil, fmt.Errorf("attachment resolver: invalid base64: %w", err)
		}
	}
	mime := a.MimeType
	if mime == "" {
		mime = "application/octet-stream"
	}
	return &ResolvedAttachment{
		Filename:    a.Name,
		MIMEType:    mime,
		Disposition: disp,
		ContentID:   a.ContentID,
		Bytes:       bytes,
		Size:        int64(len(bytes)),
		Source:      AttachmentSourceInline,
	}, nil
}

func (r *AttachmentResolver) resolveURL(ctx context.Context, a notification.Attachment, disp string) (*ResolvedAttachment, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("attachment resolver: build request: %w", err)
	}
	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("attachment resolver: fetch %s: %w", a.URL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("attachment resolver: fetch %s: HTTP %d", a.URL, resp.StatusCode)
	}

	// Bound the response so an attacker cannot exhaust memory by serving an
	// open-ended body. +1 lets us detect overflow precisely.
	limited := io.LimitReader(resp.Body, r.maxURLBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("attachment resolver: read body: %w", err)
	}
	if int64(len(body)) > r.maxURLBytes {
		return nil, ErrAttachmentURLOversize
	}

	mime := pickNonEmpty(a.MimeType, resp.Header.Get("Content-Type"), "application/octet-stream")
	name := a.Name
	if name == "" {
		name = filenameFromURL(a.URL)
	}
	return &ResolvedAttachment{
		Filename:    name,
		MIMEType:    mime,
		Disposition: disp,
		ContentID:   a.ContentID,
		Bytes:       body,
		Size:        int64(len(body)),
		Source:      AttachmentSourceURL,
	}, nil
}

// ResolveAll resolves a slice of attachments and rolls back on the first
// failure by closing any successfully-resolved entries. This avoids leaking
// open file handles or HTTP bodies when one attachment fails mid-batch.
func (r *AttachmentResolver) ResolveAll(ctx context.Context, appID string, in []notification.Attachment) ([]*ResolvedAttachment, error) {
	out := make([]*ResolvedAttachment, 0, len(in))
	for i := range in {
		ra, err := r.Resolve(ctx, appID, in[i])
		if err != nil {
			for _, prev := range out {
				_ = prev.Close()
			}
			return nil, err
		}
		out = append(out, ra)
	}
	return out, nil
}

func pickNonEmpty(s ...string) string {
	for _, v := range s {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// filenameFromURL returns the last path segment, falling back to "download"
// when the URL has no useful tail.
func filenameFromURL(rawURL string) string {
	cut := rawURL
	if i := strings.IndexAny(cut, "?#"); i >= 0 {
		cut = cut[:i]
	}
	if i := strings.LastIndex(cut, "/"); i >= 0 && i+1 < len(cut) {
		return cut[i+1:]
	}
	return "download"
}
