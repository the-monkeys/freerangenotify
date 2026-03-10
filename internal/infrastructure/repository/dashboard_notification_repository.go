package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/dashboard_notification"
	"go.uber.org/zap"
)

const dashboardNotificationsIndex = "dashboard_notifications"

// DashboardNotificationRepository implements dashboard_notification.Repository.
type DashboardNotificationRepository struct {
	client *elasticsearch.Client
	logger *zap.Logger
}

// NewDashboardNotificationRepository creates a new repository.
func NewDashboardNotificationRepository(client *elasticsearch.Client, logger *zap.Logger) dashboard_notification.Repository {
	return &DashboardNotificationRepository{
		client: client,
		logger: logger,
	}
}

// Create stores a new dashboard notification.
func (r *DashboardNotificationRepository) Create(ctx context.Context, n *dashboard_notification.DashboardNotification) error {
	if n.ID == "" {
		n.ID = uuid.New().String()
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now().UTC()
	}
	base := NewBaseRepository(r.client, dashboardNotificationsIndex, r.logger, RefreshWaitFor)
	return base.Create(ctx, n.ID, n)
}

// GetByID retrieves a notification by ID.
func (r *DashboardNotificationRepository) GetByID(ctx context.Context, id string) (*dashboard_notification.DashboardNotification, error) {
	base := NewBaseRepository(r.client, dashboardNotificationsIndex, r.logger)
	doc, err := base.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	var n dashboard_notification.DashboardNotification
	if err := mapToStruct(doc, &n); err != nil {
		return nil, fmt.Errorf("map notification: %w", err)
	}
	return &n, nil
}

// ListByUser returns notifications for a user with pagination.
func (r *DashboardNotificationRepository) ListByUser(ctx context.Context, userID string, limit, offset int) ([]*dashboard_notification.DashboardNotification, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	base := NewBaseRepository(r.client, dashboardNotificationsIndex, r.logger)
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{"user_id": userID},
		},
		"sort": []map[string]interface{}{{"created_at": "desc"}},
		"size": limit,
		"from": offset,
	}
	result, err := base.Search(ctx, query)
	if err != nil {
		return nil, 0, err
	}
	list := make([]*dashboard_notification.DashboardNotification, 0, len(result.Hits))
	for _, hit := range result.Hits {
		var n dashboard_notification.DashboardNotification
		if err := mapToStruct(hit, &n); err != nil {
			continue
		}
		list = append(list, &n)
	}
	return list, int(result.Total), nil
}

// MarkRead marks notifications as read for a user.
func (r *DashboardNotificationRepository) MarkRead(ctx context.Context, userID string, ids []string) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	base := NewBaseRepository(r.client, dashboardNotificationsIndex, r.logger)
	now := time.Now().UTC()
	updated := 0
	for _, id := range ids {
		doc, err := base.GetByID(ctx, id)
		if err != nil || doc == nil {
			continue
		}
		uid, _ := doc["user_id"].(string)
		if uid != userID {
			continue
		}
		doc["read_at"] = now.Format(time.RFC3339)
		if err := base.Update(ctx, id, doc); err != nil {
			r.logger.Warn("Failed to mark notification read", zap.String("id", id), zap.Error(err))
			continue
		}
		updated++
	}
	return updated, nil
}

// GetUnreadCount returns the count of unread notifications for a user.
func (r *DashboardNotificationRepository) GetUnreadCount(ctx context.Context, userID string) (int64, error) {
	base := NewBaseRepository(r.client, dashboardNotificationsIndex, r.logger)
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{"term": map[string]interface{}{"user_id": userID}},
				},
				"must_not": []map[string]interface{}{
					{"exists": map[string]interface{}{"field": "read_at"}},
				},
			},
		},
		"size": 0,
	}
	result, err := base.Search(ctx, query)
	if err != nil {
		return 0, err
	}
	return result.Total, nil
}
