package infrastructure

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/dashboard_notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/sse"
	"go.uber.org/zap"
)

// DashboardNotifier creates dashboard notifications and publishes to SSE.
type DashboardNotifier struct {
	repo     dashboard_notification.Repository
	broadcaster *sse.Broadcaster
	redis    *redis.Client
	logger   *zap.Logger
}

// NewDashboardNotifier creates a new DashboardNotifier.
func NewDashboardNotifier(
	repo dashboard_notification.Repository,
	broadcaster *sse.Broadcaster,
	redisClient *redis.Client,
	logger *zap.Logger,
) dashboard_notification.Notifier {
	return &DashboardNotifier{
		repo:       repo,
		broadcaster: broadcaster,
		redis:      redisClient,
		logger:     logger,
	}
}

// NotifyUser creates a dashboard notification and publishes it to SSE.
func (n *DashboardNotifier) NotifyUser(ctx context.Context, userID string, title, body, category string, data map[string]interface{}) error {
	dn := &dashboard_notification.DashboardNotification{
		ID:        uuid.New().String(),
		UserID:    userID,
		Title:     title,
		Body:      body,
		Category:  category,
		Data:      data,
		CreatedAt: time.Now().UTC(),
	}
	if err := n.repo.Create(ctx, dn); err != nil {
		n.logger.Error("Failed to create dashboard notification", zap.Error(err))
		return err
	}

	// Publish to SSE for real-time delivery.
	notif := &notification.Notification{
		NotificationID: dn.ID,
		AppID:         "dashboard",
		UserID:        userID,
		Channel:       notification.ChannelSSE,
		Content:       notification.Content{Title: title, Body: body, Data: data},
		Category:      category,
		Status:        notification.StatusDelivered,
		CreatedAt:     dn.CreatedAt,
	}
	msg := &sse.SSEMessage{
		Type:         "notification",
		UserID:       userID,
		Notification: notif,
	}
	if n.broadcaster != nil {
		if err := n.broadcaster.PublishMessage(msg); err != nil {
			n.logger.Warn("Failed to publish SSE for dashboard notification", zap.Error(err))
		}
	} else if n.redis != nil {
		// Fallback: publish directly to Redis if broadcaster not set
		payload, _ := json.Marshal(msg)
		if err := n.redis.Publish(ctx, "sse:notifications", payload).Err(); err != nil {
			n.logger.Warn("Failed to publish dashboard notification to Redis", zap.Error(err))
		}
	}

	return nil
}
