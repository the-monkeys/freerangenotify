package file

import (
	"context"
	"io"
)

// FileStore is the storage backend for raw file bytes.
//
// Implementations may be backed by a local filesystem, S3, GCS, or any other
// blob store. The interface intentionally exposes streaming I/O so callers
// never have to hold a full file in memory.
//
// All implementations MUST be tenant-safe: callers pass an explicit appID and
// the implementation MUST guarantee that bytes uploaded under one appID can
// never be read or deleted by callers identifying as a different appID.
type FileStore interface {
	// Put stores the contents of r as a new object keyed by (appID, fileID).
	// size is the expected payload length in bytes; implementations may use it
	// to reject early or to allocate space. Returning a non-nil error means
	// no object was persisted (or any partial write has been cleaned up).
	Put(ctx context.Context, appID, fileID string, r io.Reader, size int64) error

	// Get returns a reader over the object's bytes. The caller MUST Close().
	// Returns ErrFileNotFound when the object does not exist for the tenant.
	Get(ctx context.Context, appID, fileID string) (io.ReadCloser, error)

	// Delete removes the object. Returns ErrFileNotFound when the object does
	// not exist; idempotent variants should be layered above this interface.
	Delete(ctx context.Context, appID, fileID string) error

	// Exists reports whether an object exists for the tenant without opening it.
	Exists(ctx context.Context, appID, fileID string) (bool, error)
}

// Repository persists FileObject metadata records.
//
// The bytes themselves live in a FileStore; this interface owns only the
// searchable, listable metadata index. Both must agree on (AppID, FileID).
type Repository interface {
	// Create inserts a new FileObject. Returns an error if a record with the
	// same FileID already exists.
	Create(ctx context.Context, f *FileObject) error

	// GetByID returns the FileObject if it belongs to appID, else
	// ErrFileNotFound (NOT ErrCrossTenantAccess — we do not leak existence).
	GetByID(ctx context.Context, appID, fileID string) (*FileObject, error)

	// Delete removes the metadata record. Returns ErrFileNotFound if the
	// record does not exist for the tenant.
	Delete(ctx context.Context, appID, fileID string) error

	// List returns up to limit FileObjects for the tenant, newest first.
	// offset is a simple integer-based paging cursor.
	List(ctx context.Context, appID string, limit, offset int) ([]*FileObject, int64, error)
}
