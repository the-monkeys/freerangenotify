package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// GetByIDs uses realtime mget in bounded batches rather than one HTTP call per recipient.
func (r *UserRepository) GetByIDs(ctx context.Context, ids []string) (map[string]*user.User, error) {
	users := make(map[string]*user.User, len(ids))
	for start := 0; start < len(ids); start += 500 {
		end := start + 500
		if end > len(ids) {
			end = len(ids)
		}
		body, err := json.Marshal(map[string]any{"ids": ids[start:end]})
		if err != nil {
			return nil, err
		}
		res, err := r.client.Mget(strings.NewReader(string(body)), r.client.Mget.WithContext(ctx), r.client.Mget.WithIndex(r.indexName))
		if err != nil {
			return nil, err
		}
		var result struct {
			Docs []struct {
				ID     string          `json:"_id"`
				Found  bool            `json:"found"`
				Source user.User       `json:"_source"`
				Error  json.RawMessage `json:"error"`
			} `json:"docs"`
		}
		decodeErr := json.NewDecoder(res.Body).Decode(&result)
		res.Body.Close()
		if res.IsError() || decodeErr != nil {
			return nil, fmt.Errorf("failed to load recipient batch")
		}
		if len(result.Docs) != end-start {
			return nil, fmt.Errorf("incomplete recipient batch")
		}
		for i, doc := range result.Docs {
			if doc.ID != ids[start+i] || len(doc.Error) > 0 {
				return nil, fmt.Errorf("recipient lookup failed")
			}
			if doc.Found {
				if doc.Source.UserID != doc.ID {
					return nil, fmt.Errorf("recipient identity mismatch")
				}
				u := doc.Source
				users[u.UserID] = &u
			}
		}
	}
	return users, nil
}

// UserRepository implements the user domain repository interface
type UserRepository struct {
	*BaseRepository
}

// NewUserRepository creates a new user repository
func NewUserRepository(client *elasticsearch.Client, logger *zap.Logger) user.Repository {
	return &UserRepository{
		BaseRepository: NewBaseRepository(client, "users", logger, RefreshWaitFor),
	}
}

// Create creates a new user
func (r *UserRepository) Create(ctx context.Context, u *user.User) error {
	u.CreatedAt = time.Now()
	u.UpdatedAt = time.Now()
	return r.BaseRepository.Create(ctx, u.UserID, u)
}

// GetByID retrieves a user by ID
func (r *UserRepository) GetByID(ctx context.Context, userID string) (*user.User, error) {
	doc, err := r.BaseRepository.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	var u user.User
	if err := mapToStruct(doc, &u); err != nil {
		return nil, fmt.Errorf("failed to map document to user: %w", err)
	}

	return &u, nil
}

// GetByEmail retrieves a user by email within an app
func (r *UserRepository) GetByEmail(ctx context.Context, appID, email string) (*user.User, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{
						"term": map[string]interface{}{
							"app_id": appID,
						},
					},
					{
						"term": map[string]interface{}{
							"email": email,
						},
					},
				},
			},
		},
	}

	result, err := r.BaseRepository.Search(ctx, query)
	if err != nil {
		return nil, err
	}

	if result.Total == 0 {
		return nil, apperrors.NotFound("User", "lookup")
	}

	var u user.User
	if err := mapToStruct(result.Hits[0], &u); err != nil {
		return nil, fmt.Errorf("failed to map document to user: %w", err)
	}

	return &u, nil
}

// GetByExternalID retrieves a user by external_id within an app
func (r *UserRepository) GetByExternalID(ctx context.Context, appID, externalID string) (*user.User, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{
						"term": map[string]interface{}{
							"app_id": appID,
						},
					},
					{
						"term": map[string]interface{}{
							"external_id": externalID,
						},
					},
				},
			},
		},
	}

	result, err := r.BaseRepository.Search(ctx, query)
	if err != nil {
		return nil, err
	}

	if result.Total == 0 {
		return nil, apperrors.NotFound("User", "lookup")
	}

	var u user.User
	if err := mapToStruct(result.Hits[0], &u); err != nil {
		return nil, fmt.Errorf("failed to map document to user: %w", err)
	}

	return &u, nil
}

// Update updates an existing user
func (r *UserRepository) Update(ctx context.Context, u *user.User) error {
	u.UpdatedAt = time.Now()
	return r.BaseRepository.Update(ctx, u.UserID, u)
}

// Delete deletes a user
func (r *UserRepository) Delete(ctx context.Context, userID string) error {
	return r.BaseRepository.Delete(ctx, userID)
}

// List lists users with pagination and filtering
func (r *UserRepository) List(ctx context.Context, filter user.UserFilter) ([]*user.User, error) {
	query := r.buildUserQuery(filter)

	result, err := r.BaseRepository.Search(ctx, query)
	if err != nil {
		return nil, err
	}

	var users []*user.User
	for _, hit := range result.Hits {
		var u user.User
		if err := mapToStruct(hit, &u); err != nil {
			r.logger.Error("Failed to map document to user", zap.Error(err))
			continue
		}
		users = append(users, &u)
	}

	return users, nil
}

// Count counts users matching the filter
func (r *UserRepository) Count(ctx context.Context, filter user.UserFilter) (int64, error) {
	query := r.buildUserQuery(filter)
	result, err := r.BaseRepository.Search(ctx, query)
	if err != nil {
		return 0, err
	}
	return result.Total, nil
}

// BulkCreate creates multiple users at once
func (r *UserRepository) BulkCreate(ctx context.Context, users []*user.User) error {
	outcomes, err := r.BulkCreateResults(ctx, users)
	if err != nil {
		return err
	}
	for _, itemErr := range outcomes {
		if itemErr != nil {
			return itemErr
		}
	}
	return nil
}
func (r *UserRepository) BulkCreateResults(ctx context.Context, users []*user.User) ([]error, error) {
	docs := make([]BulkDocument, len(users))
	for i, u := range users {
		docs[i] = BulkDocument{ID: u.UserID, Document: u}
	}
	return r.BaseRepository.BulkCreateResults(ctx, docs)
}

// LookupEmails batches exact, app-scoped identities without changing email case policy.
func (r *UserRepository) LookupEmails(ctx context.Context, appID, environmentID string, emails []string) (map[string]*user.User, error) {
	found := make(map[string]*user.User)
	if len(emails) == 0 {
		return found, nil
	}
	filters := []map[string]any{{"term": map[string]any{"app_id": appID}}, {"terms": map[string]any{"email": emails}}}
	if environmentID != "" && environmentID != "default" {
		filters = append(filters, map[string]any{"term": map[string]any{"environment_id": environmentID}})
	}
	for offset := 0; ; {
		result, err := r.Search(ctx, map[string]any{"query": map[string]any{"bool": map[string]any{"filter": filters}}, "size": 1000, "from": offset, "sort": []any{map[string]any{"user_id": "asc"}}})
		if err != nil {
			return nil, err
		}
		for _, hit := range result.Hits {
			var u user.User
			if err := mapToStruct(hit, &u); err != nil {
				return nil, err
			}
			if found[u.Email] == nil {
				found[u.Email] = &u
			}
		}
		offset += len(result.Hits)
		if int64(offset) >= result.Total {
			return found, nil
		}
		if len(result.Hits) == 0 || offset >= 10000 {
			return nil, fmt.Errorf("email lookup incomplete; no users written")
		}
	}
}

// GetUsersByApp retrieves all users for a specific application
func (r *UserRepository) GetUsersByApp(ctx context.Context, appID string, filter user.UserFilter) ([]*user.User, error) {
	// Override or ensure appID filter
	filter.AppID = appID
	return r.List(ctx, filter)
}

// AddDevice adds a device to a user
func (r *UserRepository) AddDevice(ctx context.Context, userID string, device user.Device) error {
	// First get the user
	u, err := r.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	// Add or update device
	deviceExists := false
	for i, d := range u.Devices {
		if d.DeviceID == device.DeviceID {
			u.Devices[i] = device
			deviceExists = true
			break
		}
	}

	if !deviceExists {
		u.Devices = append(u.Devices, device)
	}

	// Update user
	return r.Update(ctx, u)
}

// RemoveDevice removes a device from a user
func (r *UserRepository) RemoveDevice(ctx context.Context, userID, deviceID string) error {
	// First get the user
	u, err := r.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	// Remove device
	for i, d := range u.Devices {
		if d.DeviceID == deviceID {
			u.Devices = append(u.Devices[:i], u.Devices[i+1:]...)
			break
		}
	}

	// Update user
	return r.Update(ctx, u)
}

// UpdatePreferences updates user preferences
func (r *UserRepository) UpdatePreferences(ctx context.Context, userID string, preferences user.Preferences) error {
	// First get the user
	u, err := r.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	// Update preferences
	u.Preferences = preferences

	// Update user
	return r.Update(ctx, u)
}

// buildUserQuery builds Elasticsearch query from filter
func (r *UserRepository) buildUserQuery(filter user.UserFilter) map[string]interface{} {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"match_all": map[string]interface{}{},
		},
	}

	var filters []map[string]interface{}

	if len(filter.LinkedUserIDs) > 0 && filter.AppID != "" {
		// Show the app's own users plus specifically-linked users
		filters = append(filters, map[string]interface{}{
			"bool": map[string]interface{}{
				"should": []map[string]interface{}{
					{"term": map[string]interface{}{"app_id": filter.AppID}},
					{"terms": map[string]interface{}{"user_id": filter.LinkedUserIDs}},
				},
				"minimum_should_match": 1,
			},
		})
	} else if filter.AppID != "" {
		filters = append(filters, map[string]interface{}{
			"term": map[string]interface{}{
				"app_id": filter.AppID,
			},
		})
	} else if len(filter.AppIDs) > 0 {
		filters = append(filters, map[string]interface{}{
			"terms": map[string]interface{}{
				"app_id": filter.AppIDs,
			},
		})
	}

	if filter.EnvironmentID != "" && filter.EnvironmentID != "default" {
		filters = append(filters, map[string]interface{}{
			"term": map[string]interface{}{
				"environment_id": filter.EnvironmentID,
			},
		})
	}

	if filter.Email != "" {
		filters = append(filters, map[string]interface{}{
			"term": map[string]interface{}{
				"email": filter.Email,
			},
		})
	}

	if filter.ExternalID != "" {
		filters = append(filters, map[string]interface{}{
			"term": map[string]interface{}{
				"external_id": filter.ExternalID,
			},
		})
	}

	if filter.Timezone != "" {
		filters = append(filters, map[string]interface{}{
			"term": map[string]interface{}{
				"timezone": filter.Timezone,
			},
		})
	}

	if filter.Language != "" {
		filters = append(filters, map[string]interface{}{
			"term": map[string]interface{}{
				"language": filter.Language,
			},
		})
	}

	if filter.Search != "" {
		filters = append(filters, buildUserSearchClause(filter.Search))
	}

	if len(filters) > 0 {
		query["query"] = map[string]interface{}{
			"bool": map[string]interface{}{
				"must": filters,
			},
		}
	}

	// Add pagination
	if filter.Offset > 0 {
		query["from"] = filter.Offset
	}
	if filter.Limit > 0 {
		query["size"] = filter.Limit
	}

	// Add sorting
	query["sort"] = []map[string]interface{}{
		{
			"created_at": map[string]interface{}{
				"order": "desc",
			},
		},
	}

	return query
}

// buildUserSearchClause matches partial text across common user identifier fields.
func buildUserSearchClause(search string) map[string]interface{} {
	pattern := "*" + escapeElasticsearchWildcard(search) + "*"
	searchFields := []string{"email", "full_name", "user_id", "external_id", "phone"}
	should := make([]map[string]interface{}, 0, len(searchFields))
	for _, field := range searchFields {
		should = append(should, map[string]interface{}{
			"wildcard": map[string]interface{}{
				field: map[string]interface{}{
					"value":            pattern,
					"case_insensitive": true,
				},
			},
		})
	}
	return map[string]interface{}{
		"bool": map[string]interface{}{
			"should":               should,
			"minimum_should_match": 1,
		},
	}
}

func escapeElasticsearchWildcard(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `*`, `\*`)
	s = strings.ReplaceAll(s, `?`, `\?`)
	return s
}
