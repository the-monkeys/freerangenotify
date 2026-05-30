// Package file defines the domain model for a stored binary object that
// notifications can reference by opaque file_id.
//
// FileObject is intentionally storage-agnostic: the actual bytes live in
// whatever FileStore the deployment is wired to (local FS, S3, GCS). This
// package owns only the metadata, validation, and error sentinels.
package file

import (
	"errors"
	"strings"
	"time"
)

// Domain error sentinels — handlers/services map these to HTTP statuses.
var (
	ErrInvalidFileID         = errors.New("invalid file id")
	ErrInvalidAppID          = errors.New("invalid app id")
	ErrInvalidFileName       = errors.New("invalid file name")
	ErrInvalidMIMEType       = errors.New("invalid mime type")
	ErrInvalidSize           = errors.New("invalid file size")
	ErrFileNotFound          = errors.New("file not found")
	ErrFileExpired           = errors.New("file expired")
	ErrUnsupportedMIMEType   = errors.New("mime type not in allowlist")
	ErrFileTooLarge          = errors.New("file exceeds the maximum allowed size")
	ErrCrossTenantAccess     = errors.New("file does not belong to the requesting tenant")
)

// FileObject is the persisted metadata record for an uploaded file.
//
// The opaque FileID is what callers reference from Attachment.FileID; FRN
// never exposes storage paths or backend-specific identifiers across the API
// boundary.
type FileObject struct {
	FileID    string    `json:"file_id"            es:"file_id"`
	AppID     string    `json:"app_id"             es:"app_id"`
	Name      string    `json:"name"               es:"name"`
	Size      int64     `json:"size"               es:"size"`
	MIMEType  string    `json:"mime_type"          es:"mime_type"`
	SHA256    string    `json:"sha256"             es:"sha256"`
	ExpiresAt time.Time `json:"expires_at,omitempty" es:"expires_at"`
	CreatedAt time.Time `json:"created_at"         es:"created_at"`
}

// Validate enforces the invariants required of every persisted FileObject.
// It does not check tenant ownership — that is the caller's responsibility,
// because cross-tenant lookups are a service-layer concern.
func (f *FileObject) Validate() error {
	if strings.TrimSpace(f.FileID) == "" {
		return ErrInvalidFileID
	}
	if strings.TrimSpace(f.AppID) == "" {
		return ErrInvalidAppID
	}
	if strings.TrimSpace(f.Name) == "" {
		return ErrInvalidFileName
	}
	if strings.TrimSpace(f.MIMEType) == "" {
		return ErrInvalidMIMEType
	}
	if f.Size <= 0 {
		return ErrInvalidSize
	}
	return nil
}

// IsExpired reports whether the file's retention window has elapsed.
// Files without an expiry are never considered expired.
func (f *FileObject) IsExpired(now time.Time) bool {
	if f.ExpiresAt.IsZero() {
		return false
	}
	return now.After(f.ExpiresAt)
}
