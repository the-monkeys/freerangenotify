package dto

import (
	"time"

	domainfile "github.com/the-monkeys/freerangenotify/internal/domain/file"
)

// FileResponse is the JSON view of a stored file's metadata.
//
// SHA256 lets clients verify integrity. ExpiresAt is omitted when the file is
// pinned (never-expire). URLs are not embedded here — callers either stream
// via /v1/files/:id/content or mint a signed URL via /v1/files/:id/download-url.
type FileResponse struct {
	FileID    string    `json:"file_id"`
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	MIMEType  string    `json:"mime_type"`
	SHA256    string    `json:"sha256"`
	ExpiresAt *string   `json:"expires_at,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// NewFileResponse maps a domain file to its JSON view.
func NewFileResponse(f *domainfile.FileObject) FileResponse {
	resp := FileResponse{
		FileID:    f.FileID,
		Name:      f.Name,
		Size:      f.Size,
		MIMEType:  f.MIMEType,
		SHA256:    f.SHA256,
		CreatedAt: f.CreatedAt,
	}
	if !f.ExpiresAt.IsZero() {
		s := f.ExpiresAt.UTC().Format(time.RFC3339)
		resp.ExpiresAt = &s
	}
	return resp
}

// FileListResponse wraps a page of files with the total count for the tenant.
type FileListResponse struct {
	Files []FileResponse `json:"files"`
	Total int64          `json:"total"`
}

// SignedURLResponse is returned by the download-url endpoint and carries a
// short-lived URL the caller may hand to an end-user or external system.
type SignedURLResponse struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}
