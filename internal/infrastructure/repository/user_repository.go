package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"go.uber.org/zap"
)

// UserRepository implements the user domain repository interface
type UserRepository struct {
	*BaseRepository
}

// NewUserRepository creates a new user repository
func NewUserRepository(client *elasticsearch.Client, logger *zap.Logger) user.Repository {
	return &UserRepository{
		BaseRepository: NewBaseRepository(client, "users", logger),
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
		return nil, fmt.Errorf("user not found")
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
	docs := make(map[string]interface{})
	for _, u := range users {
		docs[u.UserID] = u
	}
	return r.BaseRepository.BulkCreate(ctx, docs)
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

	if filter.AppID != "" {
		filters = append(filters, map[string]interface{}{
			"term": map[string]interface{}{
				"app_id": filter.AppID,
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
