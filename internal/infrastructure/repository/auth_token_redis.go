package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"go.uber.org/zap"
)

const (
	refreshTokenPrefix = "frn:token:"        // frn:token:{token_id} -> JSON
	refreshTokenLookup = "frn:token:lookup:"  // frn:token:lookup:{token_string} -> token_id
	userTokensPrefix   = "frn:user_tokens:"   // frn:user_tokens:{user_id} -> SET of token_ids
	resetTokenPrefix   = "frn:reset:"         // frn:reset:{token_id} -> JSON
	resetTokenLookup   = "frn:reset:lookup:"  // frn:reset:lookup:{token_string} -> token_id
)

// redisTokenStore implements the token-related methods of auth.Repository.
type redisTokenStore struct {
	client *redis.Client
	logger *zap.Logger
}

func newRedisTokenStore(client *redis.Client, logger *zap.Logger) *redisTokenStore {
	return &redisTokenStore{client: client, logger: logger}
}

// --- Refresh Token Operations ---

func (s *redisTokenStore) CreateRefreshToken(ctx context.Context, token *auth.RefreshToken) error {
	if token.TokenID == "" {
		token.TokenID = uuid.New().String()
	}
	token.CreatedAt = time.Now()

	data, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("failed to marshal refresh token: %w", err)
	}

	ttl := time.Until(token.ExpiresAt)
	if ttl <= 0 {
		return fmt.Errorf("refresh token already expired")
	}

	pipe := s.client.Pipeline()
	pipe.Set(ctx, refreshTokenPrefix+token.TokenID, data, ttl)
	pipe.Set(ctx, refreshTokenLookup+token.Token, token.TokenID, ttl)
	pipe.SAdd(ctx, userTokensPrefix+token.UserID, token.TokenID)
	pipe.Expire(ctx, userTokensPrefix+token.UserID, ttl)
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to store refresh token: %w", err)
	}
	return nil
}

func (s *redisTokenStore) GetRefreshToken(ctx context.Context, tokenStr string) (*auth.RefreshToken, error) {
	tokenID, err := s.client.Get(ctx, refreshTokenLookup+tokenStr).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to lookup refresh token: %w", err)
	}

	data, err := s.client.Get(ctx, refreshTokenPrefix+tokenID).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get refresh token: %w", err)
	}

	var token auth.RefreshToken
	if err := json.Unmarshal([]byte(data), &token); err != nil {
		return nil, fmt.Errorf("failed to unmarshal refresh token: %w", err)
	}

	if token.Revoked || time.Now().After(token.ExpiresAt) {
		return nil, nil
	}

	return &token, nil
}

func (s *redisTokenStore) RevokeRefreshToken(ctx context.Context, tokenID string) error {
	data, err := s.client.Get(ctx, refreshTokenPrefix+tokenID).Result()
	if err == redis.Nil {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to get refresh token for revocation: %w", err)
	}

	var token auth.RefreshToken
	if err := json.Unmarshal([]byte(data), &token); err != nil {
		return fmt.Errorf("failed to unmarshal refresh token: %w", err)
	}

	pipe := s.client.Pipeline()
	pipe.Del(ctx, refreshTokenPrefix+tokenID)
	pipe.Del(ctx, refreshTokenLookup+token.Token)
	pipe.SRem(ctx, userTokensPrefix+token.UserID, tokenID)
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to revoke refresh token: %w", err)
	}
	return nil
}

func (s *redisTokenStore) RevokeAllUserTokens(ctx context.Context, userID string) error {
	tokenIDs, err := s.client.SMembers(ctx, userTokensPrefix+userID).Result()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("failed to get user tokens: %w", err)
	}

	if len(tokenIDs) == 0 {
		return nil
	}

	pipe := s.client.Pipeline()
	for _, tokenID := range tokenIDs {
		data, err := s.client.Get(ctx, refreshTokenPrefix+tokenID).Result()
		if err == nil {
			var token auth.RefreshToken
			if json.Unmarshal([]byte(data), &token) == nil {
				pipe.Del(ctx, refreshTokenLookup+token.Token)
			}
		}
		pipe.Del(ctx, refreshTokenPrefix+tokenID)
	}
	pipe.Del(ctx, userTokensPrefix+userID)
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to revoke all user tokens: %w", err)
	}
	return nil
}

// --- Password Reset Token Operations ---

func (s *redisTokenStore) CreateResetToken(ctx context.Context, token *auth.PasswordResetToken) error {
	if token.TokenID == "" {
		token.TokenID = uuid.New().String()
	}
	token.CreatedAt = time.Now()

	data, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("failed to marshal reset token: %w", err)
	}

	ttl := time.Until(token.ExpiresAt)
	if ttl <= 0 {
		ttl = time.Hour
	}

	pipe := s.client.Pipeline()
	pipe.Set(ctx, resetTokenPrefix+token.TokenID, data, ttl)
	pipe.Set(ctx, resetTokenLookup+token.Token, token.TokenID, ttl)
	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to store reset token: %w", err)
	}
	return nil
}

func (s *redisTokenStore) GetResetToken(ctx context.Context, tokenStr string) (*auth.PasswordResetToken, error) {
	tokenID, err := s.client.Get(ctx, resetTokenLookup+tokenStr).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to lookup reset token: %w", err)
	}

	data, err := s.client.Get(ctx, resetTokenPrefix+tokenID).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get reset token: %w", err)
	}

	var token auth.PasswordResetToken
	if err := json.Unmarshal([]byte(data), &token); err != nil {
		return nil, fmt.Errorf("failed to unmarshal reset token: %w", err)
	}

	if token.Used || time.Now().After(token.ExpiresAt) {
		return nil, nil
	}

	return &token, nil
}

func (s *redisTokenStore) MarkResetTokenUsed(ctx context.Context, tokenID string) error {
	pipe := s.client.Pipeline()
	pipe.Del(ctx, resetTokenPrefix+tokenID)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to mark reset token used: %w", err)
	}
	return nil
}
