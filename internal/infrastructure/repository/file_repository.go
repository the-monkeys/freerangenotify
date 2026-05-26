package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"go.uber.org/zap"

	domainfile "github.com/the-monkeys/freerangenotify/internal/domain/file"
	pkgerrors "github.com/the-monkeys/freerangenotify/pkg/errors"
)

// fileIndexName is the Elasticsearch index that holds FileObject metadata.
// Bytes live in a FileStore; this index is the listable/searchable view.
const fileIndexName = "files"

// FileRepository implements file.Repository on top of Elasticsearch.
//
// Tenant safety: every read/write filters on (app_id, file_id). GetByID
// returns ErrFileNotFound for cross-tenant hits so the API never leaks the
// existence of another tenant's file.
type FileRepository struct {
	*BaseRepository
}

// NewFileRepository constructs a FileRepository.
func NewFileRepository(client *elasticsearch.Client, logger *zap.Logger) domainfile.Repository {
	return &FileRepository{
		BaseRepository: NewBaseRepository(client, fileIndexName, logger, RefreshWaitFor),
	}
}

// Create inserts a new FileObject. CreatedAt is stamped if unset.
func (r *FileRepository) Create(ctx context.Context, f *domainfile.FileObject) error {
	if err := f.Validate(); err != nil {
		return err
	}
	if f.CreatedAt.IsZero() {
		f.CreatedAt = time.Now().UTC()
	}
	return r.BaseRepository.Create(ctx, f.FileID, f)
}

// GetByID returns the FileObject only if it belongs to appID. Cross-tenant
// lookups return ErrFileNotFound rather than ErrCrossTenantAccess so we do
// not disclose existence.
func (r *FileRepository) GetByID(ctx context.Context, appID, fileID string) (*domainfile.FileObject, error) {
	doc, err := r.BaseRepository.GetByID(ctx, fileID)
	if err != nil {
		var nf *pkgerrors.AppError
		if errors.As(err, &nf) && nf.Code == string(pkgerrors.ErrCodeNotFound) {
			return nil, domainfile.ErrFileNotFound
		}
		return nil, fmt.Errorf("file repo: get: %w", err)
	}

	var f domainfile.FileObject
	if err := mapToStruct(doc, &f); err != nil {
		return nil, fmt.Errorf("file repo: decode: %w", err)
	}
	if f.AppID != appID {
		// Tenant mismatch — opaque "not found".
		return nil, domainfile.ErrFileNotFound
	}
	return &f, nil
}

// Delete removes the metadata record for the tenant. Hits a tenancy-checked
// GetByID first so cross-tenant deletes cannot succeed.
func (r *FileRepository) Delete(ctx context.Context, appID, fileID string) error {
	if _, err := r.GetByID(ctx, appID, fileID); err != nil {
		return err
	}
	return r.BaseRepository.Delete(ctx, fileID)
}

// List returns the tenant's files, newest first.
func (r *FileRepository) List(ctx context.Context, appID string, limit, offset int) ([]*domainfile.FileObject, int64, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					// Use the keyword sub-field: the `files` index has no
					// explicit mapping, so ES dynamic-maps `app_id` as
					// `text` with a `.keyword` sub-field. A `term` against
					// the analyzed `text` field tokenises UUIDs on dashes
					// and never matches — list returned empty for every
					// tenant. Querying the keyword sub-field is exact.
					{"term": map[string]interface{}{"app_id.keyword": appID}},
				},
			},
		},
		"sort": []map[string]interface{}{
			{"created_at": map[string]interface{}{"order": "desc"}},
		},
		"from": offset,
		"size": limit,
	}
	result, err := r.BaseRepository.Search(ctx, query)
	if err != nil {
		return nil, 0, fmt.Errorf("file repo: list: %w", err)
	}

	files := make([]*domainfile.FileObject, 0, len(result.Hits))
	for _, hit := range result.Hits {
		var f domainfile.FileObject
		if err := mapToStruct(hit, &f); err != nil {
			r.BaseRepository.logger.Warn("file repo: skip undecodable hit", zap.Error(err))
			continue
		}
		files = append(files, &f)
	}
	return files, result.Total, nil
}
