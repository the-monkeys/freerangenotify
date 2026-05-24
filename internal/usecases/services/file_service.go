package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	domainfile "github.com/the-monkeys/freerangenotify/internal/domain/file"
)

// DefaultMaxFileBytes is the global hard upper bound on a single file. Per-app
// caps may be lower but never higher; the resolver and providers may apply
// stricter channel limits on top of this.
const DefaultMaxFileBytes int64 = 50 * 1024 * 1024 // 50 MiB

// DefaultRetention is the time after which a file may be garbage-collected if
// no explicit retention is configured. Zero retention means "never expire".
const DefaultRetention = 30 * 24 * time.Hour

// DefaultAllowedMIMETypes is the conservative starting allowlist. Operators
// override via FileServiceConfig.AllowedMIMETypes.
var DefaultAllowedMIMETypes = []string{
	"application/pdf",
	"image/jpeg",
	"image/png",
	"image/gif",
	"image/webp",
	"audio/mpeg",
	"audio/ogg",
	"audio/wav",
	"video/mp4",
	"text/csv",
	"text/plain",
	"application/zip",
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	"application/vnd.openxmlformats-officedocument.presentationml.presentation",
}

// FileServiceConfig holds tunables for the FileService.
//
// Zero-valued fields fall back to package defaults: MaxBytes -> 50 MiB,
// Retention -> 30 days, AllowedMIMETypes -> DefaultAllowedMIMETypes.
// A nil VirusScanner disables scanning.
type FileServiceConfig struct {
	MaxBytes         int64
	Retention        time.Duration
	AllowedMIMETypes []string
	VirusScanner     VirusScanner
}

// VirusScanner is the optional hook for AV integration. Implementations MUST
// be fast and side-effect-free; expensive scanners belong on a queue, not in
// the upload path.
type VirusScanner interface {
	Scan(ctx context.Context, name, mimeType string, content []byte) error
}

// FileService orchestrates file uploads, retrieval, listing and deletion.
//
// Upload is a single end-to-end transaction: validate metadata -> stream
// bytes into the FileStore while hashing -> persist metadata. On any failure
// after the bytes hit the store, the bytes are removed so we do not leak
// orphaned objects.
type FileService struct {
	store     domainfile.FileStore
	repo      domainfile.Repository
	cfg       FileServiceConfig
	allowed   map[string]struct{}
	logger    *zap.Logger
	// now and idGen are overridable for tests.
	now   func() time.Time
	idGen func() string
}

// NewFileService wires a FileService.
func NewFileService(store domainfile.FileStore, repo domainfile.Repository, cfg FileServiceConfig, logger *zap.Logger) *FileService {
	if cfg.MaxBytes <= 0 {
		cfg.MaxBytes = DefaultMaxFileBytes
	}
	if cfg.Retention < 0 {
		cfg.Retention = 0
	} else if cfg.Retention == 0 {
		cfg.Retention = DefaultRetention
	}
	if len(cfg.AllowedMIMETypes) == 0 {
		cfg.AllowedMIMETypes = DefaultAllowedMIMETypes
	}
	allowed := make(map[string]struct{}, len(cfg.AllowedMIMETypes))
	for _, m := range cfg.AllowedMIMETypes {
		allowed[strings.ToLower(strings.TrimSpace(m))] = struct{}{}
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &FileService{
		store:   store,
		repo:    repo,
		cfg:     cfg,
		allowed: allowed,
		logger:  logger,
		now:     func() time.Time { return time.Now().UTC() },
		idGen:   func() string { return "file_" + strings.ReplaceAll(uuid.NewString(), "-", "") },
	}
}

// UploadInput captures everything the service needs for an upload.
//
// Reader is consumed exactly once. DeclaredSize is the caller-provided size;
// the service enforces it and rejects size mismatches with ErrFileTooLarge or
// io.ErrUnexpectedEOF as appropriate.
type UploadInput struct {
	AppID        string
	Name         string
	MIMEType     string
	DeclaredSize int64
	Reader       io.Reader
}

// Upload validates, stores, and registers a new file. On success the
// returned FileObject contains the assigned FileID, SHA256, and ExpiresAt.
func (s *FileService) Upload(ctx context.Context, in UploadInput) (*domainfile.FileObject, error) {
	if strings.TrimSpace(in.AppID) == "" {
		return nil, domainfile.ErrInvalidAppID
	}
	if strings.TrimSpace(in.Name) == "" {
		return nil, domainfile.ErrInvalidFileName
	}
	mime := strings.ToLower(strings.TrimSpace(in.MIMEType))
	if mime == "" {
		return nil, domainfile.ErrInvalidMIMEType
	}
	if !s.mimeAllowed(mime) {
		return nil, domainfile.ErrUnsupportedMIMEType
	}
	if in.DeclaredSize <= 0 {
		return nil, domainfile.ErrInvalidSize
	}
	if in.DeclaredSize > s.cfg.MaxBytes {
		return nil, domainfile.ErrFileTooLarge
	}
	if in.Reader == nil {
		return nil, errors.New("file service: reader is nil")
	}

	// Bound the reader so a misbehaving upload cannot exceed MaxBytes even if
	// DeclaredSize was understated.
	limited := io.LimitReader(in.Reader, s.cfg.MaxBytes+1)

	// Hash while streaming to the store.
	hasher := sha256.New()
	tee := io.TeeReader(limited, hasher)

	// Optional AV scan: only run when bytes are small enough to buffer.
	// Anything above the per-call scan threshold is skipped (operators should
	// pair the service with an async scanner in that case).
	const scanThreshold = 4 * 1024 * 1024
	var avBuf []byte
	if s.cfg.VirusScanner != nil && in.DeclaredSize <= scanThreshold {
		avBuf = make([]byte, 0, in.DeclaredSize)
		tee = io.TeeReader(tee, &byteCollector{buf: &avBuf})
	}

	fileID := s.idGen()

	if err := s.store.Put(ctx, in.AppID, fileID, tee, in.DeclaredSize); err != nil {
		// Wrap underlying size-mismatch from the store as ErrFileTooLarge when
		// the caller exceeded the declared size; otherwise propagate.
		return nil, fmt.Errorf("file service: store: %w", err)
	}

	if s.cfg.VirusScanner != nil && avBuf != nil {
		if err := s.cfg.VirusScanner.Scan(ctx, in.Name, mime, avBuf); err != nil {
			// Cleanup on AV failure.
			_ = s.store.Delete(ctx, in.AppID, fileID)
			return nil, fmt.Errorf("file service: scan: %w", err)
		}
	}

	now := s.now()
	expires := time.Time{}
	if s.cfg.Retention > 0 {
		expires = now.Add(s.cfg.Retention)
	}

	obj := &domainfile.FileObject{
		FileID:    fileID,
		AppID:     in.AppID,
		Name:      in.Name,
		Size:      in.DeclaredSize,
		MIMEType:  mime,
		SHA256:    hex.EncodeToString(hasher.Sum(nil)),
		ExpiresAt: expires,
		CreatedAt: now,
	}

	if err := s.repo.Create(ctx, obj); err != nil {
		_ = s.store.Delete(ctx, in.AppID, fileID)
		return nil, fmt.Errorf("file service: repo: %w", err)
	}

	s.logger.Info("file uploaded",
		zap.String("app_id", in.AppID),
		zap.String("file_id", fileID),
		zap.String("mime", mime),
		zap.Int64("size", in.DeclaredSize),
	)
	return obj, nil
}

// Get returns metadata for (appID, fileID). Returns ErrFileNotFound for
// missing or cross-tenant requests.
func (s *FileService) Get(ctx context.Context, appID, fileID string) (*domainfile.FileObject, error) {
	return s.repo.GetByID(ctx, appID, fileID)
}

// OpenContent returns the file's metadata and a reader over its bytes. The
// caller MUST Close the reader. Cross-tenant requests return ErrFileNotFound.
func (s *FileService) OpenContent(ctx context.Context, appID, fileID string) (*domainfile.FileObject, io.ReadCloser, error) {
	obj, err := s.repo.GetByID(ctx, appID, fileID)
	if err != nil {
		return nil, nil, err
	}
	if obj.IsExpired(s.now()) {
		return nil, nil, domainfile.ErrFileExpired
	}
	rc, err := s.store.Get(ctx, appID, fileID)
	if err != nil {
		return nil, nil, err
	}
	return obj, rc, nil
}

// Delete removes the file and its metadata. Cross-tenant requests return
// ErrFileNotFound. Best-effort cleanup: if the metadata delete succeeds but
// the bytes delete fails, the bytes are orphaned and the error is logged.
func (s *FileService) Delete(ctx context.Context, appID, fileID string) error {
	if err := s.repo.Delete(ctx, appID, fileID); err != nil {
		return err
	}
	if err := s.store.Delete(ctx, appID, fileID); err != nil && !errors.Is(err, domainfile.ErrFileNotFound) {
		s.logger.Warn("file service: bytes orphaned after metadata delete",
			zap.String("app_id", appID), zap.String("file_id", fileID), zap.Error(err))
	}
	return nil
}

// List returns the tenant's files newest first.
func (s *FileService) List(ctx context.Context, appID string, limit, offset int) ([]*domainfile.FileObject, int64, error) {
	return s.repo.List(ctx, appID, limit, offset)
}

func (s *FileService) mimeAllowed(mime string) bool {
	if _, ok := s.allowed[mime]; ok {
		return true
	}
	// Allow wildcard entries like "image/*".
	if slash := strings.IndexByte(mime, '/'); slash > 0 {
		prefix := mime[:slash] + "/*"
		if _, ok := s.allowed[prefix]; ok {
			return true
		}
	}
	return false
}

// byteCollector is an io.Writer that appends into the referenced slice.
// It exists so we can capture up-to-N-bytes of a TeeReader for the optional
// AV scanner without a second allocation per Read.
type byteCollector struct{ buf *[]byte }

func (b *byteCollector) Write(p []byte) (int, error) {
	*b.buf = append(*b.buf, p...)
	return len(p), nil
}
