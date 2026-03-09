package repository

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"go.uber.org/zap"
)

// bytesReader converts byte slice to io.Reader
func bytesReader(data []byte) io.Reader {
	return bytes.NewReader(data)
}

const (
	authUsersIndex = "auth_users"
)

type authRepository struct {
	client     *elasticsearch.Client
	logger     *zap.Logger
	tokenStore *redisTokenStore
}

// NewAuthRepository creates a new auth repository
func NewAuthRepository(esClient *elasticsearch.Client, redisClient *redis.Client, logger *zap.Logger) auth.Repository {
	return &authRepository{
		client:     esClient,
		logger:     logger,
		tokenStore: newRedisTokenStore(redisClient, logger),
	}
}

// CreateUser creates a new admin user
func (r *authRepository) CreateUser(ctx context.Context, user *auth.AdminUser) error {
	if user.UserID == "" {
		user.UserID = uuid.New().String()
	}
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()

	data, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("failed to marshal user: %w", err)
	}

	req := esapi.IndexRequest{
		Index:      authUsersIndex,
		DocumentID: user.UserID,
		Body:       bytesReader(data),
		Refresh:    "true",
	}

	res, err := req.Do(ctx, r.client)
	if err != nil {
		return fmt.Errorf("failed to index user: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("error indexing user: %s", res.String())
	}

	return nil
}

// GetUserByID retrieves a user by ID
func (r *authRepository) GetUserByID(ctx context.Context, userID string) (*auth.AdminUser, error) {
	req := esapi.GetRequest{
		Index:      authUsersIndex,
		DocumentID: userID,
	}

	res, err := req.Do(ctx, r.client)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		if res.StatusCode == 404 {
			return nil, nil
		}
		return nil, fmt.Errorf("error getting user: %s", res.String())
	}

	var result struct {
		Source auth.AdminUser `json:"_source"`
	}

	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode user: %w", err)
	}

	return &result.Source, nil
}

// DeleteUser deletes a user by ID
func (r *authRepository) DeleteUser(ctx context.Context, userID string) error {
	req := esapi.DeleteRequest{
		Index:      authUsersIndex,
		DocumentID: userID,
		Refresh:    "true",
	}

	res, err := req.Do(ctx, r.client)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("error deleting user: %s", res.String())
	}

	return nil
}

// GetUserByEmail retrieves a user by email.
// When multiple users share the same email (race-condition duplicate),
// the oldest account (by created_at) is returned deterministically.
func (r *authRepository) GetUserByEmail(ctx context.Context, email string) (*auth.AdminUser, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{
				"email.keyword": email,
			},
		},
		"sort": []map[string]interface{}{
			{"created_at": map[string]interface{}{"order": "asc"}},
		},
		"size": 1,
	}

	data, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	req := esapi.SearchRequest{
		Index: []string{authUsersIndex},
		Body:  bytesReader(data),
	}

	res, err := req.Do(ctx, r.client)
	if err != nil {
		return nil, fmt.Errorf("failed to search user: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("error searching user: %s", res.String())
	}

	var result struct {
		Hits struct {
			Hits []struct {
				Source auth.AdminUser `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode search results: %w", err)
	}

	if len(result.Hits.Hits) == 0 {
		return nil, nil
	}

	return &result.Hits.Hits[0].Source, nil
}

// UpdateUser updates a user
func (r *authRepository) UpdateUser(ctx context.Context, user *auth.AdminUser) error {
	user.UpdatedAt = time.Now()

	data, err := json.Marshal(map[string]interface{}{
		"doc": user,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal user update: %w", err)
	}

	req := esapi.UpdateRequest{
		Index:      authUsersIndex,
		DocumentID: user.UserID,
		Body:       bytesReader(data),
		Refresh:    "true",
	}

	res, err := req.Do(ctx, r.client)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("error updating user: %s", res.String())
	}

	return nil
}

// UpdateLastLogin updates the last login time for a user
func (r *authRepository) UpdateLastLogin(ctx context.Context, userID string, loginTime time.Time) error {
	data, err := json.Marshal(map[string]interface{}{
		"doc": map[string]interface{}{
			"last_login_at": loginTime,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to marshal last login update: %w", err)
	}

	req := esapi.UpdateRequest{
		Index:      authUsersIndex,
		DocumentID: userID,
		Body:       bytesReader(data),
		Refresh:    "true",
	}

	res, err := req.Do(ctx, r.client)
	if err != nil {
		return fmt.Errorf("failed to update last login: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("error updating last login: %s", res.String())
	}

	return nil
}

func (r *authRepository) CreateResetToken(ctx context.Context, token *auth.PasswordResetToken) error {
	return r.tokenStore.CreateResetToken(ctx, token)
}

func (r *authRepository) GetResetToken(ctx context.Context, token string) (*auth.PasswordResetToken, error) {
	return r.tokenStore.GetResetToken(ctx, token)
}

func (r *authRepository) MarkResetTokenUsed(ctx context.Context, tokenID string) error {
	return r.tokenStore.MarkResetTokenUsed(ctx, tokenID)
}

func (r *authRepository) CreateRefreshToken(ctx context.Context, token *auth.RefreshToken) error {
	return r.tokenStore.CreateRefreshToken(ctx, token)
}

func (r *authRepository) GetRefreshToken(ctx context.Context, token string) (*auth.RefreshToken, error) {
	return r.tokenStore.GetRefreshToken(ctx, token)
}

func (r *authRepository) RevokeRefreshToken(ctx context.Context, tokenID string) error {
	return r.tokenStore.RevokeRefreshToken(ctx, tokenID)
}

func (r *authRepository) RevokeAllUserTokens(ctx context.Context, userID string) error {
	return r.tokenStore.RevokeAllUserTokens(ctx, userID)
}
