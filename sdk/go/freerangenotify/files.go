package freerangenotify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// FilesClient is the sub-client for the /v1/files API.
//
// Files uploaded here can be referenced from notification attachments via
// FileID, which is the recommended path for any payload larger than ~10 MB or
// for files reused across many notifications (e.g. invoices, contracts).
//
//	f, _ := os.Open("invoice-2026-05.pdf")
//	defer f.Close()
//	obj, err := client.Files.Upload(ctx, freerangenotify.UploadFileParams{
//	    Name:     "invoice-2026-05.pdf",
//	    MIMEType: "application/pdf",
//	    Reader:   f,
//	})
//	// ... then later:
//	client.Notifications.Send(ctx, freerangenotify.SendParams{
//	    UserID:   userID,
//	    Channel:  "email",
//	    Template: "invoice_email",
//	    Content: &freerangenotify.Content{
//	        Attachments: []freerangenotify.ContentAttachment{{
//	            Type:   "file",
//	            FileID: obj.FileID,
//	            Name:   obj.Name,
//	        }},
//	    },
//	})
type FilesClient struct{ client *Client }

// File is the JSON shape returned by /v1/files endpoints.
type File struct {
	FileID    string    `json:"file_id"`
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	MIMEType  string    `json:"mime_type"`
	SHA256    string    `json:"sha256"`
	ExpiresAt *string   `json:"expires_at,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// FileListResponse pairs the page of files with the tenant total.
type FileListResponse struct {
	Files []File `json:"files"`
	Total int64  `json:"total"`
}

// SignedURL is the short-lived public download URL minted by the server.
type SignedURL struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

// UploadFileParams configures a multipart upload. Reader is required; Name
// and MIMEType default to "file" and "application/octet-stream" respectively
// when omitted.
type UploadFileParams struct {
	// Name is the user-visible filename. Strongly recommended (defaults to
	// "file" when empty). Server-side strips path components.
	Name string
	// MIMEType is sent as the part's Content-Type. The server enforces a
	// configurable allowlist (default: common image/document types).
	MIMEType string
	// Reader supplies the file bytes. The server caps uploads at 50 MiB by
	// default; callers should not pre-buffer large streams.
	Reader io.Reader
}

// ListFilesOptions paginates List(). Limit defaults server-side to 50 and is
// capped at 200; Offset defaults to 0.
type ListFilesOptions struct {
	Limit  int
	Offset int
}

// Upload streams the file bytes via multipart/form-data and returns the
// stored object's metadata (FileID, SHA256, ExpiresAt, ...).
func (f *FilesClient) Upload(ctx context.Context, params UploadFileParams) (*File, error) {
	if params.Reader == nil {
		return nil, fmt.Errorf("freerangenotify: Upload requires a Reader")
	}
	name := strings.TrimSpace(params.Name)
	if name == "" {
		name = "file"
	}
	mimeType := strings.TrimSpace(params.MIMEType)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	hdr := make(textproto.MIMEHeader)
	hdr.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="file"; filename="%s"`, escapeQuotes(filepath.Base(name))))
	hdr.Set("Content-Type", mimeType)
	part, err := mw.CreatePart(hdr)
	if err != nil {
		return nil, fmt.Errorf("freerangenotify: build multipart: %w", err)
	}
	if _, err := io.Copy(part, params.Reader); err != nil {
		return nil, fmt.Errorf("freerangenotify: copy file bytes: %w", err)
	}
	if err := mw.Close(); err != nil {
		return nil, fmt.Errorf("freerangenotify: close multipart: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.client.baseURL+"/files", &buf)
	if err != nil {
		return nil, fmt.Errorf("freerangenotify: create upload request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+f.client.apiKey)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := f.client.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("freerangenotify: upload request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, &APIError{StatusCode: resp.StatusCode, Body: string(body)}
	}
	var out File
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("freerangenotify: decode upload response: %w", err)
	}
	return &out, nil
}

// Get returns the file metadata.
func (f *FilesClient) Get(ctx context.Context, fileID string) (*File, error) {
	var out File
	if err := f.client.do(ctx, http.MethodGet, "/files/"+url.PathEscape(fileID), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// List returns the tenant's files newest first.
func (f *FilesClient) List(ctx context.Context, opts ListFilesOptions) (*FileListResponse, error) {
	q := url.Values{}
	if opts.Limit > 0 {
		q.Set("limit", strconv.Itoa(opts.Limit))
	}
	if opts.Offset > 0 {
		q.Set("offset", strconv.Itoa(opts.Offset))
	}
	var out FileListResponse
	if err := f.client.doWithQuery(ctx, http.MethodGet, "/files", q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete removes the file's bytes and metadata. Returns nil on success even
// if the file was already gone (server returns 204 / 404 — caller should treat
// 404 as a precondition error via APIError).
func (f *FilesClient) Delete(ctx context.Context, fileID string) error {
	return f.client.do(ctx, http.MethodDelete, "/files/"+url.PathEscape(fileID), nil, nil)
}

// Content streams the file bytes back to the caller. The returned ReadCloser
// must be closed by the caller. Use this when your backend needs to relay the
// file; for direct end-user delivery prefer DownloadURL.
func (f *FilesClient) Content(ctx context.Context, fileID string) (io.ReadCloser, *http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		f.client.baseURL+"/files/"+url.PathEscape(fileID)+"/content", nil)
	if err != nil {
		return nil, nil, fmt.Errorf("freerangenotify: create content request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+f.client.apiKey)
	resp, err := f.client.http.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("freerangenotify: content request failed: %w", err)
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, resp, &APIError{StatusCode: resp.StatusCode, Body: string(body)}
	}
	return resp.Body, resp, nil
}

// DownloadURL mints a short-lived signed URL the caller may hand to an
// end-user, third party, or background process for unauthenticated download.
// The TTL is configured server-side (default 15 minutes).
func (f *FilesClient) DownloadURL(ctx context.Context, fileID string) (*SignedURL, error) {
	var out SignedURL
	if err := f.client.do(ctx, http.MethodGet,
		"/files/"+url.PathEscape(fileID)+"/download-url", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// escapeQuotes is the minimal Content-Disposition filename escape used by the
// stdlib's multipart writer for form fields. Inlined to avoid a private dep.
func escapeQuotes(s string) string {
	return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s)
}
