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
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"go.uber.org/zap"
)

// bytesReader converts byte slice to io.Reader
func bytesReader(data []byte) io.Reader {
	return bytes.NewReader(data)
}

const (
	authUsersIndex     = "auth_users"
	resetTokensIndex   = "password_reset_tokens"
	refreshTokensIndex = "refresh_tokens"
)

type authRepository struct {
	client *elasticsearch.Client
	logger *zap.Logger
}

// NewAuthRepository creates a new auth repository
func NewAuthRepository(client *elasticsearch.Client, logger *zap.Logger) auth.Repository {
	return &authRepository{
		client: client,
		logger: logger,
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

// GetUserByEmail retrieves a user by email
func (r *authRepository) GetUserByEmail(ctx context.Context, email string) (*auth.AdminUser, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{
				"email.keyword": email,
			},
		},
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

// CreateResetToken creates a password reset token
func (r *authRepository) CreateResetToken(ctx context.Context, token *auth.PasswordResetToken) error {
	if token.TokenID == "" {
		token.TokenID = uuid.New().String()
	}
	token.CreatedAt = time.Now()

	data, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("failed to marshal reset token: %w", err)
	}

	req := esapi.IndexRequest{
		Index:      resetTokensIndex,
		DocumentID: token.TokenID,
		Body:       bytesReader(data),
		Refresh:    "true",
	}

	res, err := req.Do(ctx, r.client)
	if err != nil {
		return fmt.Errorf("failed to index reset token: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("error indexing reset token: %s", res.String())
	}

	return nil
}

// GetResetToken retrieves a password reset token
func (r *authRepository) GetResetToken(ctx context.Context, token string) (*auth.PasswordResetToken, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{
						"term": map[string]interface{}{
							"token.keyword": token,
						},
					},
					{
						"term": map[string]interface{}{
							"used": false,
						},
					},
					{
						"range": map[string]interface{}{
							"expires_at": map[string]interface{}{
								"gte": time.Now().Format(time.RFC3339),
							},
						},
					},
				},
			},
		},
	}

	data, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	req := esapi.SearchRequest{
		Index: []string{resetTokensIndex},
		Body:  bytesReader(data),
	}

	res, err := req.Do(ctx, r.client)
	if err != nil {
		return nil, fmt.Errorf("failed to search reset token: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("error searching reset token: %s", res.String())
	}

	var result struct {
		Hits struct {
			Hits []struct {
				Source auth.PasswordResetToken `json:"_source"`
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

// MarkResetTokenUsed marks a reset token as used
func (r *authRepository) MarkResetTokenUsed(ctx context.Context, tokenID string) error {
	data, err := json.Marshal(map[string]interface{}{
		"doc": map[string]interface{}{
			"used": true,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to marshal token update: %w", err)
	}

	req := esapi.UpdateRequest{
		Index:      resetTokensIndex,
		DocumentID: tokenID,
		Body:       bytesReader(data),
		Refresh:    "true",
	}

	res, err := req.Do(ctx, r.client)
	if err != nil {
		return fmt.Errorf("failed to update reset token: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("error updating reset token: %s", res.String())
	}

	return nil
}

// CreateRefreshToken creates a refresh token
func (r *authRepository) CreateRefreshToken(ctx context.Context, token *auth.RefreshToken) error {
	if token.TokenID == "" {
		token.TokenID = uuid.New().String()
	}
	token.CreatedAt = time.Now()

	data, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("failed to marshal refresh token: %w", err)
	}

	req := esapi.IndexRequest{
		Index:      refreshTokensIndex,
		DocumentID: token.TokenID,
		Body:       bytesReader(data),
		Refresh:    "true",
	}

	res, err := req.Do(ctx, r.client)
	if err != nil {
		return fmt.Errorf("failed to index refresh token: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("error indexing refresh token: %s", res.String())
	}

	return nil
}

// GetRefreshToken retrieves a refresh token
func (r *authRepository) GetRefreshToken(ctx context.Context, token string) (*auth.RefreshToken, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{
						"term": map[string]interface{}{
							"token.keyword": token,
						},
					},
					{
						"term": map[string]interface{}{
							"revoked": false,
						},
					},
					{
						"range": map[string]interface{}{
							"expires_at": map[string]interface{}{
								"gte": time.Now().Format(time.RFC3339),
							},
						},
					},
				},
			},
		},
	}

	data, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query: %w", err)
	}

	req := esapi.SearchRequest{
		Index: []string{refreshTokensIndex},
		Body:  bytesReader(data),
	}

	res, err := req.Do(ctx, r.client)
	if err != nil {
		return nil, fmt.Errorf("failed to search refresh token: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return nil, fmt.Errorf("error searching refresh token: %s", res.String())
	}

	var result struct {
		Hits struct {
			Hits []struct {
				Source auth.RefreshToken `json:"_source"`
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

// RevokeRefreshToken revokes a refresh token
func (r *authRepository) RevokeRefreshToken(ctx context.Context, tokenID string) error {
	data, err := json.Marshal(map[string]interface{}{
		"doc": map[string]interface{}{
			"revoked": true,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to marshal token revocation: %w", err)
	}

	req := esapi.UpdateRequest{
		Index:      refreshTokensIndex,
		DocumentID: tokenID,
		Body:       bytesReader(data),
		Refresh:    "true",
	}

	res, err := req.Do(ctx, r.client)
	if err != nil {
		return fmt.Errorf("failed to revoke refresh token: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("error revoking refresh token: %s", res.String())
	}

	return nil
}

// RevokeAllUserTokens revokes all refresh tokens for a user
func (r *authRepository) RevokeAllUserTokens(ctx context.Context, userID string) error {
	query := map[string]interface{}{
		"script": map[string]interface{}{
			"source": "ctx._source.revoked = true",
			"lang":   "painless",
		},
		"query": map[string]interface{}{
			"term": map[string]interface{}{
				"user_id.keyword": userID,
			},
		},
	}

	data, err := json.Marshal(query)
	if err != nil {
		return fmt.Errorf("failed to marshal query: %w", err)
	}

	req := esapi.UpdateByQueryRequest{
		Index: []string{refreshTokensIndex},
		Body:  bytesReader(data),
	}

	res, err := req.Do(ctx, r.client)
	if err != nil {
		return fmt.Errorf("failed to revoke user tokens: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("error revoking user tokens: %s", res.String())
	}

	return nil
}
