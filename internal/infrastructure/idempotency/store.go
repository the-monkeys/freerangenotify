package idempotency

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

const (
	keyPrefix    = "frn:idemp:"
	defaultTTL   = 24 * time.Hour
	maxKeyLen    = 256
	idempHeader  = "Idempotency-Key"
	idempHeaderX = "X-Idempotency-Key"
)

var keySanitize = regexp.MustCompile(`[^a-zA-Z0-9\-_]`)

// CachedResponse holds a stored idempotent response
type CachedResponse struct {
	Status      int             `json:"status"`
	Body        json.RawMessage `json:"body"`
	RequestHash string          `json:"request_hash,omitempty"`
	Pending     bool            `json:"pending,omitempty"`
}

// BeginOperation claims before side effects. Pending operations never expire:
// an interrupted broadcast needs reconciliation, not an automatic second send.
func (s *Store) BeginOperation(ctx context.Context, appID, key, requestHash string) (*CachedResponse, bool, error) {
	pending := CachedResponse{Status: 409, Body: json.RawMessage(`{"error":"broadcast outcome pending; reconcile before retrying","retryable":false}`), RequestHash: requestHash, Pending: true}
	data, err := json.Marshal(pending)
	if err != nil {
		return nil, false, err
	}
	claimed, err := s.client.SetNX(ctx, keyPrefix+appID+":"+key, data, 0).Result()
	if err != nil {
		return nil, false, fmt.Errorf("idempotency claim unavailable: %w", err)
	}
	if claimed {
		return nil, true, nil
	}
	existing, err := s.Get(ctx, appID, key)
	if err != nil {
		return nil, false, err
	}
	if existing == nil {
		return nil, false, fmt.Errorf("idempotency claim disappeared")
	}
	return existing, false, nil
}

func (s *Store) CompleteOperation(ctx context.Context, appID, key, requestHash string, status int, body []byte) error {
	data, err := json.Marshal(CachedResponse{Status: status, Body: body, RequestHash: requestHash})
	if err != nil {
		return err
	}
	return s.client.Eval(ctx, `
local old=redis.call('GET',KEYS[1])
if not old then return redis.error_reply('operation receipt missing') end
local receipt=cjson.decode(old)
if receipt.request_hash~=ARGV[1] then return redis.error_reply('operation identity mismatch') end
redis.call('SET',KEYS[1],ARGV[2],'PX',ARGV[3])
return 1`, []string{keyPrefix + appID + ":" + key}, requestHash, data, s.ttl.Milliseconds()).Err()
}

// Store provides idempotency key storage (Redis-backed)
type Store struct {
	client *redis.Client
	ttl    time.Duration
	logger *zap.Logger
}

// NewStore creates a new idempotency store
func NewStore(client *redis.Client, logger *zap.Logger) *Store {
	return &Store{
		client: client,
		ttl:    defaultTTL,
		logger: logger,
	}
}

// GetIdempotencyKey extracts and sanitizes the idempotency key from request headers
func GetIdempotencyKey(c interface {
	Get(key string, defaultValue ...string) string
}) string {
	key := c.Get(idempHeader)
	if key == "" {
		key = c.Get(idempHeaderX)
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	key = keySanitize.ReplaceAllString(key, "")
	if len(key) > maxKeyLen {
		key = key[:maxKeyLen]
	}
	return key
}

// Get returns a cached response for the given app and key, or nil if not found
func (s *Store) Get(ctx context.Context, appID, key string) (*CachedResponse, error) {
	if appID == "" || key == "" {
		return nil, nil
	}
	redisKey := keyPrefix + appID + ":" + key
	data, err := s.client.Get(ctx, redisKey).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("idempotency get: %w", err)
	}
	var cached CachedResponse
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, fmt.Errorf("idempotency decode: %w", err)
	}
	return &cached, nil
}

// Set stores a response for the given app and key
func (s *Store) Set(ctx context.Context, appID, key string, status int, body []byte) error {
	if appID == "" || key == "" {
		return nil
	}
	redisKey := keyPrefix + appID + ":" + key
	cached := CachedResponse{Status: status, Body: body}
	data, err := json.Marshal(cached)
	if err != nil {
		return fmt.Errorf("idempotency encode: %w", err)
	}
	if err := s.client.Set(ctx, redisKey, data, s.ttl).Err(); err != nil {
		return fmt.Errorf("idempotency set: %w", err)
	}
	s.logger.Debug("Idempotency cached",
		zap.String("app_id", appID),
		zap.String("key", key),
		zap.Int("status", status))
	return nil
}
