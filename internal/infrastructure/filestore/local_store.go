package filestore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.uber.org/zap"

	domainfile "github.com/the-monkeys/freerangenotify/internal/domain/file"
)

// safeIDPattern restricts appID/fileID to characters that cannot escape the
// store root via path traversal. Anything else is rejected by validateID,
// providing defense-in-depth on top of filepath.Clean.
var safeIDPattern = regexp.MustCompile(`^[A-Za-z0-9._\-]+$`)

// LocalStore is a tenant-segmented filesystem FileStore.
//
// Layout:
//
//	<root>/<appID>/<fileID>
//
// We never expose `root` outside the process. All Get/Put/Delete operations
// reject inputs whose IDs would resolve outside the per-tenant directory.
type LocalStore struct {
	root   string
	logger *zap.Logger
}

// NewLocalStore creates the root directory if missing and returns a ready
// store. Returns an error if the root cannot be created or is not a directory.
func NewLocalStore(root string, logger *zap.Logger) (*LocalStore, error) {
	if strings.TrimSpace(root) == "" {
		return nil, errors.New("filestore: root path is required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("filestore: resolve root: %w", err)
	}
	if err := os.MkdirAll(abs, 0o750); err != nil {
		return nil, fmt.Errorf("filestore: mkdir root: %w", err)
	}
	st, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("filestore: stat root: %w", err)
	}
	if !st.IsDir() {
		return nil, fmt.Errorf("filestore: root %q is not a directory", abs)
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &LocalStore{root: abs, logger: logger}, nil
}

// Put streams r into the per-tenant file. Existing files are overwritten —
// the higher-level service enforces unique fileIDs.
func (s *LocalStore) Put(ctx context.Context, appID, fileID string, r io.Reader, size int64) error {
	if err := validateID(appID); err != nil {
		return fmt.Errorf("appID: %w", err)
	}
	if err := validateID(fileID); err != nil {
		return fmt.Errorf("fileID: %w", err)
	}
	if r == nil {
		return errors.New("filestore: reader is nil")
	}

	target, err := s.resolve(appID, fileID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return fmt.Errorf("filestore: mkdir tenant dir: %w", err)
	}

	// Write atomically: tmp file in same dir then rename.
	tmp, err := os.CreateTemp(filepath.Dir(target), ".upload-*.tmp")
	if err != nil {
		return fmt.Errorf("filestore: create temp: %w", err)
	}
	tmpName := tmp.Name()
	// Honor context cancellation by copying through a context-aware reader.
	written, copyErr := io.Copy(tmp, &ctxReader{ctx: ctx, r: r})
	closeErr := tmp.Close()
	if copyErr != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("filestore: write: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("filestore: close temp: %w", closeErr)
	}
	if size > 0 && written != size {
		_ = os.Remove(tmpName)
		return fmt.Errorf("filestore: size mismatch: wrote %d, declared %d", written, size)
	}
	if err := os.Chmod(tmpName, 0o640); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("filestore: chmod: %w", err)
	}
	if err := os.Rename(tmpName, target); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("filestore: rename: %w", err)
	}
	s.logger.Debug("filestore put",
		zap.String("app_id", appID), zap.String("file_id", fileID), zap.Int64("size", written))
	return nil
}

// Get opens the file for reading.
func (s *LocalStore) Get(ctx context.Context, appID, fileID string) (io.ReadCloser, error) {
	if err := validateID(appID); err != nil {
		return nil, fmt.Errorf("appID: %w", err)
	}
	if err := validateID(fileID); err != nil {
		return nil, fmt.Errorf("fileID: %w", err)
	}
	target, err := s.resolve(appID, fileID)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(target)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, domainfile.ErrFileNotFound
		}
		return nil, fmt.Errorf("filestore: open: %w", err)
	}
	return f, nil
}

// Delete removes the file. Returns ErrFileNotFound if missing.
func (s *LocalStore) Delete(ctx context.Context, appID, fileID string) error {
	if err := validateID(appID); err != nil {
		return fmt.Errorf("appID: %w", err)
	}
	if err := validateID(fileID); err != nil {
		return fmt.Errorf("fileID: %w", err)
	}
	target, err := s.resolve(appID, fileID)
	if err != nil {
		return err
	}
	if err := os.Remove(target); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return domainfile.ErrFileNotFound
		}
		return fmt.Errorf("filestore: remove: %w", err)
	}
	return nil
}

// Exists reports whether the file is present.
func (s *LocalStore) Exists(ctx context.Context, appID, fileID string) (bool, error) {
	if err := validateID(appID); err != nil {
		return false, fmt.Errorf("appID: %w", err)
	}
	if err := validateID(fileID); err != nil {
		return false, fmt.Errorf("fileID: %w", err)
	}
	target, err := s.resolve(appID, fileID)
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(target); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("filestore: stat: %w", err)
	}
	return true, nil
}

// Root returns the absolute root path. Intended for tests / diagnostics only.
func (s *LocalStore) Root() string { return s.root }

// resolve returns the absolute path and asserts it lives under root.
func (s *LocalStore) resolve(appID, fileID string) (string, error) {
	joined := filepath.Join(s.root, appID, fileID)
	clean := filepath.Clean(joined)
	// Defense-in-depth: ensure the final path is rooted in s.root.
	rel, err := filepath.Rel(s.root, clean)
	if err != nil || strings.HasPrefix(rel, "..") || rel == ".." {
		return "", fmt.Errorf("filestore: path escapes root")
	}
	return clean, nil
}

// validateID rejects ids that are empty, too long, or contain characters that
// could enable path traversal even after Clean.
func validateID(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("empty")
	}
	if len(id) > 128 {
		return errors.New("too long")
	}
	if !safeIDPattern.MatchString(id) {
		return errors.New("contains forbidden characters")
	}
	if id == "." || id == ".." {
		return errors.New("reserved")
	}
	return nil
}

// ctxReader wraps an io.Reader to surface context cancellation as a read error.
type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (c *ctxReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}
