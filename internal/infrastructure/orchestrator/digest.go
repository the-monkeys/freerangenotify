package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/digest"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"go.uber.org/zap"
)

// Redis key patterns for digest
const (
	digestKeyPrefix = "frn:digest:"
	digestFlushKey  = "frn:digest:flush"
)

// DigestManager manages notification digesting using Redis sorted sets.
type DigestManager struct {
	digestRepo   digest.Repository
	notifService notification.Service
	redisClient  *redis.Client
	logger       *zap.Logger

	wg       sync.WaitGroup
	stopChan chan struct{}
}

// NewDigestManager creates a new digest manager.
func NewDigestManager(
	digestRepo digest.Repository,
	notifService notification.Service,
	redisClient *redis.Client,
	logger *zap.Logger,
) *DigestManager {
	return &DigestManager{
		digestRepo:   digestRepo,
		notifService: notifService,
		redisClient:  redisClient,
		logger:       logger,
		stopChan:     make(chan struct{}),
	}
}

// MatchesDigestRule checks if a notification should be digested.
// Returns the matching rule or nil if no rule matches.
func (dm *DigestManager) MatchesDigestRule(ctx context.Context, notif *notification.Notification) (*digest.DigestRule, string) {
	if notif.Metadata == nil {
		return nil, ""
	}
	digestKey, ok := notif.Metadata["digest_key"]
	if !ok {
		return nil, ""
	}

	keyStr, ok := digestKey.(string)
	if !ok || keyStr == "" {
		return nil, ""
	}

	rule, err := dm.digestRepo.GetActiveByKey(ctx, notif.AppID, keyStr)
	if err != nil || rule == nil {
		return nil, ""
	}

	return rule, keyStr
}

// Accumulate adds a notification payload to the digest accumulator.
func (dm *DigestManager) Accumulate(ctx context.Context, notif *notification.Notification, rule *digest.DigestRule, digestKeyValue string) error {
	// Determine the grouping value from the notification metadata
	groupVal := ""
	if v, ok := notif.Metadata[rule.DigestKey]; ok {
		groupVal = fmt.Sprintf("%v", v)
	}

	redisKey := fmt.Sprintf("%s%s:%s:%s", digestKeyPrefix, notif.AppID, notif.UserID, groupVal)

	// Serialize notification payload for accumulation
	payload := map[string]interface{}{
		"notification_id": notif.NotificationID,
		"title":           notif.Content.Title,
		"body":            notif.Content.Body,
		"data":            notif.Content.Data,
		"category":        notif.Category,
		"created_at":      notif.CreatedAt.Format(time.RFC3339),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal digest payload: %w", err)
	}

	// Add to sorted set (score = timestamp)
	if err := dm.redisClient.ZAdd(ctx, redisKey, &redis.Z{
		Score:  float64(time.Now().Unix()),
		Member: string(data),
	}).Err(); err != nil {
		return fmt.Errorf("failed to accumulate digest event: %w", err)
	}

	// Schedule flush if not already scheduled
	exists, _ := dm.redisClient.ZScore(ctx, digestFlushKey, redisKey).Result()
	if exists == 0 {
		window, err := time.ParseDuration(rule.Window)
		if err != nil {
			window = 1 * time.Hour // Safe default
		}
		flushAt := time.Now().Add(window)
		dm.redisClient.ZAdd(ctx, digestFlushKey, &redis.Z{
			Score:  float64(flushAt.Unix()),
			Member: redisKey,
		})
	}

	dm.logger.Info("Notification accumulated into digest",
		zap.String("notification_id", notif.NotificationID),
		zap.String("digest_key", redisKey))

	return nil
}

// StartFlushPoller starts the background goroutine that flushes mature digests.
func (dm *DigestManager) StartFlushPoller(ctx context.Context) {
	dm.wg.Add(1)
	go dm.flushPoller(ctx)
}

func (dm *DigestManager) flushPoller(ctx context.Context) {
	defer dm.wg.Done()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-dm.stopChan:
			return
		case <-ticker.C:
			dm.flushReady(ctx)
		}
	}
}

func (dm *DigestManager) flushReady(ctx context.Context) {
	now := float64(time.Now().Unix())
	results, err := dm.redisClient.ZRangeByScore(ctx, digestFlushKey, &redis.ZRangeBy{
		Min:   "-inf",
		Max:   fmt.Sprintf("%f", now),
		Count: 50,
	}).Result()
	if err != nil {
		return
	}

	for _, redisKey := range results {
		if err := dm.flushOneDigest(ctx, redisKey); err != nil {
			dm.logger.Error("Failed to flush digest",
				zap.String("key", redisKey),
				zap.Error(err))
			continue
		}
		// Remove from flush schedule
		dm.redisClient.ZRem(ctx, digestFlushKey, redisKey)
	}
}

func (dm *DigestManager) flushOneDigest(ctx context.Context, redisKey string) error {
	// Get all accumulated events
	results, err := dm.redisClient.ZRangeByScore(ctx, redisKey, &redis.ZRangeBy{
		Min: "-inf",
		Max: "+inf",
	}).Result()
	if err != nil {
		return err
	}

	if len(results) == 0 {
		return nil
	}

	// Parse events
	var events []map[string]interface{}
	for _, r := range results {
		var event map[string]interface{}
		if err := json.Unmarshal([]byte(r), &event); err != nil {
			continue
		}
		events = append(events, event)
	}

	// Parse the redis key to extract app_id, user_id, groupVal
	// Key format: frn:digest:{app_id}:{user_id}:{group_val}
	parts := parseDigestKey(redisKey)
	if parts == nil {
		dm.logger.Error("Failed to parse digest key", zap.String("key", redisKey))
		dm.redisClient.Del(ctx, redisKey)
		return nil
	}

	// Send a consolidated digest notification
	req := notification.SendRequest{
		AppID:    parts.appID,
		UserID:   parts.userID,
		Channel:  notification.ChannelInApp,
		Priority: notification.PriorityNormal,
		Title:    fmt.Sprintf("You have %d new notifications", len(events)),
		Body:     fmt.Sprintf("%d notifications have been batched for you.", len(events)),
		Data: map[string]interface{}{
			"events":       events,
			"event_count":  len(events),
			"digest_group": parts.groupVal,
		},
	}

	if _, err := dm.notifService.Send(ctx, req); err != nil {
		return fmt.Errorf("failed to send digest notification: %w", err)
	}

	dm.logger.Info("Digest flushed",
		zap.String("key", redisKey),
		zap.Int("event_count", len(events)))

	// Clear the accumulator
	dm.redisClient.Del(ctx, redisKey)

	return nil
}

// Shutdown gracefully stops the digest manager.
func (dm *DigestManager) Shutdown() {
	close(dm.stopChan)
	dm.wg.Wait()
}

// digestKeyParts holds parsed components of a digest Redis key.
type digestKeyParts struct {
	appID    string
	userID   string
	groupVal string
}

// parseDigestKey extracts app_id, user_id, and group_val from a digest Redis key.
// Key format: frn:digest:{app_id}:{user_id}:{group_val}
func parseDigestKey(key string) *digestKeyParts {
	// Remove prefix "frn:digest:"
	prefix := digestKeyPrefix
	if len(key) <= len(prefix) {
		return nil
	}
	remainder := key[len(prefix):]
	// Split into at most 3 parts: app_id, user_id, group_val
	parts := splitN(remainder, ":", 3)
	if len(parts) < 2 {
		return nil
	}
	result := &digestKeyParts{
		appID:  parts[0],
		userID: parts[1],
	}
	if len(parts) == 3 {
		result.groupVal = parts[2]
	}
	return result
}

// splitN splits a string into at most n parts by sep.
func splitN(s, sep string, n int) []string {
	result := make([]string, 0, n)
	for i := 0; i < n-1; i++ {
		idx := indexOf(s, sep)
		if idx < 0 {
			break
		}
		result = append(result, s[:idx])
		s = s[idx+len(sep):]
	}
	result = append(result, s)
	return result
}

// indexOf returns the index of sep in s, or -1 if not found.
func indexOf(s, sep string) int {
	for i := 0; i <= len(s)-len(sep); i++ {
		if s[i:i+len(sep)] == sep {
			return i
		}
	}
	return -1
}
