package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"go.uber.org/zap"
)

// NotificationRepository implements the notification domain repository interface
type NotificationRepository struct {
	*BaseRepository
}

// NewNotificationRepository creates a new notification repository
func NewNotificationRepository(client *elasticsearch.Client, logger *zap.Logger) notification.Repository {
	return &NotificationRepository{
		BaseRepository: NewBaseRepository(client, "notifications", logger),
	}
}

// Create creates a new notification
func (r *NotificationRepository) Create(ctx context.Context, n *notification.Notification) error {
	n.CreatedAt = time.Now()
	n.UpdatedAt = time.Now()
	return r.BaseRepository.Create(ctx, n.NotificationID, n)
}

// GetByID retrieves a notification by ID
func (r *NotificationRepository) GetByID(ctx context.Context, notificationID string) (*notification.Notification, error) {
	doc, err := r.BaseRepository.GetByID(ctx, notificationID)
	if err != nil {
		return nil, err
	}

	var n notification.Notification
	if err := mapToStruct(doc, &n); err != nil {
		return nil, fmt.Errorf("failed to map document to notification: %w", err)
	}

	return &n, nil
}

// Update updates an existing notification
func (r *NotificationRepository) Update(ctx context.Context, n *notification.Notification) error {
	n.UpdatedAt = time.Now()
	return r.BaseRepository.Update(ctx, n.NotificationID, n)
}

// UpdateStatus updates the status of a notification
func (r *NotificationRepository) UpdateStatus(ctx context.Context, notificationID string, status notification.Status) error {
	updateDoc := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}

	now := time.Now()
	// Set timestamp based on status
	switch status {
	case notification.StatusSent:
		updateDoc["sent_at"] = now
	case notification.StatusDelivered:
		updateDoc["delivered_at"] = now
	case notification.StatusRead:
		updateDoc["read_at"] = now
	case notification.StatusFailed:
		updateDoc["failed_at"] = now
	}

	return r.BaseRepository.Update(ctx, notificationID, updateDoc)
}

// Delete deletes a notification
func (r *NotificationRepository) Delete(ctx context.Context, notificationID string) error {
	return r.BaseRepository.Delete(ctx, notificationID)
}

// List lists notifications with pagination and filtering
func (r *NotificationRepository) List(ctx context.Context, filter *notification.NotificationFilter) ([]*notification.Notification, error) {
	query := r.buildNotificationQuery(filter)

	result, err := r.BaseRepository.Search(ctx, query)
	if err != nil {
		return nil, err
	}

	var notifications []*notification.Notification
	for _, hit := range result.Hits {
		var n notification.Notification
		if err := mapToStruct(hit, &n); err != nil {
			r.logger.Error("Failed to map document to notification", zap.Error(err))
			continue
		}
		notifications = append(notifications, &n)
	}

	return notifications, nil
}

// GetByUser retrieves notifications for a specific user
func (r *NotificationRepository) GetByUser(ctx context.Context, userID string, filter *notification.NotificationFilter) ([]*notification.Notification, error) {
	if filter == nil {
		filter = &notification.NotificationFilter{}
	}
	filter.UserID = userID

	return r.List(ctx, filter)
}

// GetByApp retrieves notifications for a specific application
func (r *NotificationRepository) GetByApp(ctx context.Context, appID string, filter *notification.NotificationFilter) ([]*notification.Notification, error) {
	if filter == nil {
		filter = &notification.NotificationFilter{}
	}
	filter.AppID = appID

	return r.List(ctx, filter)
}

// GetPending retrieves notifications that need to be sent.
// Excludes pending notifications with scheduled_at in the future (they stay in Redis scheduled queue).
func (r *NotificationRepository) GetPending(ctx context.Context) ([]*notification.Notification, error) {
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)

	// Exclude future-scheduled: either no scheduled_at, or scheduled_at <= now
	noScheduledAt := map[string]interface{}{
		"bool": map[string]interface{}{
			"must_not": []map[string]interface{}{
				{"exists": map[string]interface{}{"field": "scheduled_at"}},
			},
		},
	}
	scheduledInPast := map[string]interface{}{
		"range": map[string]interface{}{
			"scheduled_at": map[string]interface{}{"lte": nowStr},
		},
	}

	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{"term": map[string]interface{}{"status": notification.StatusPending}},
					{
						"bool": map[string]interface{}{
							"should": []map[string]interface{}{noScheduledAt, scheduledInPast},
							"minimum_should_match": 1,
						},
					},
				},
			},
		},
		"sort": []map[string]interface{}{
			{"created_at": map[string]interface{}{"order": "asc"}},
		},
	}

	result, err := r.BaseRepository.Search(ctx, query)
	if err != nil {
		return nil, err
	}

	var notifications []*notification.Notification
	for _, hit := range result.Hits {
		var n notification.Notification
		if err := mapToStruct(hit, &n); err != nil {
			r.logger.Error("Failed to map document to notification", zap.Error(err))
			continue
		}
		notifications = append(notifications, &n)
	}

	return notifications, nil
}

// GetRetryable retrieves notifications that can be retried
func (r *NotificationRepository) GetRetryable(ctx context.Context, maxRetries int) ([]*notification.Notification, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{
						"term": map[string]interface{}{
							"status": notification.StatusFailed,
						},
					},
					{
						"range": map[string]interface{}{
							"retry_count": map[string]interface{}{
								"lt": maxRetries,
							},
						},
					},
				},
			},
		},
		"sort": []map[string]interface{}{
			{
				"updated_at": map[string]interface{}{
					"order": "asc",
				},
			},
		},
	}

	result, err := r.BaseRepository.Search(ctx, query)
	if err != nil {
		return nil, err
	}

	var notifications []*notification.Notification
	for _, hit := range result.Hits {
		var n notification.Notification
		if err := mapToStruct(hit, &n); err != nil {
			r.logger.Error("Failed to map document to notification", zap.Error(err))
			continue
		}
		notifications = append(notifications, &n)
	}

	return notifications, nil
}

// IncrementRetryCount atomically increments the retry count of a notification.
func (r *NotificationRepository) IncrementRetryCount(ctx context.Context, notificationID string, errorMessage string) error {
	script := map[string]interface{}{
		"script": map[string]interface{}{
			"source": "ctx._source.retry_count += 1; ctx._source.error_message = params.error_message; ctx._source.updated_at = params.now",
			"lang":   "painless",
			"params": map[string]interface{}{
				"error_message": errorMessage,
				"now":           time.Now().Format(time.RFC3339),
			},
		},
	}
	return r.BaseRepository.ScriptUpdate(ctx, notificationID, script)
}

// BulkUpdateStatus updates the status of multiple notifications
func (r *NotificationRepository) BulkUpdateStatus(ctx context.Context, notificationIDs []string, status notification.Status) error {
	updateDoc := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}

	// Set timestamp based on status
	switch status {
	case notification.StatusSent:
		updateDoc["sent_at"] = time.Now()
	case notification.StatusDelivered:
		updateDoc["delivered_at"] = time.Now()
	case notification.StatusRead:
		updateDoc["read_at"] = time.Now()
	}

	documents := make(map[string]interface{})
	for _, id := range notificationIDs {
		documents[id] = updateDoc
	}

	return r.BaseRepository.BulkUpdate(ctx, documents)
}

// buildNotificationQuery builds Elasticsearch query from filter using ESQuery builder.
func (r *NotificationRepository) buildNotificationQuery(filter *notification.NotificationFilter) map[string]interface{} {
	if filter == nil {
		return NewQuery(50).Build()
	}

	qb := NewQuery(filter.PageSize)

	if filter.AppID != "" {
		qb.Term("app_id", filter.AppID)
	} else if len(filter.AppIDs) > 0 {
		qb.Terms("app_id", filter.AppIDs)
	}
	if filter.EnvironmentID != "" && filter.EnvironmentID != "default" {
		qb.Term("environment_id", filter.EnvironmentID)
	}
	if filter.UserID != "" {
		qb.Term("user_id", filter.UserID)
	}
	if filter.Channel != "" {
		qb.Term("channel", string(filter.Channel))
	}
	if filter.Status != "" {
		qb.Term("status", string(filter.Status))
	}
	if filter.Priority != "" {
		qb.Term("priority", string(filter.Priority))
	}
	if filter.TemplateID != "" {
		qb.Term("template_id", filter.TemplateID)
	}
	if filter.Category != "" {
		qb.Term("category", filter.Category)
	}
	if filter.DigestKey != "" {
		qb.Term("metadata.digest_key", filter.DigestKey)
	}
	if filter.FromDate != nil {
		qb.Range("created_at", map[string]interface{}{"gte": filter.FromDate.Format(time.RFC3339)})
	}
	if filter.ToDate != nil {
		qb.Range("created_at", map[string]interface{}{"lte": filter.ToDate.Format(time.RFC3339)})
	}

	sortField := filter.SortBy
	if sortField == "" {
		sortField = "created_at"
	}
	sortOrder := filter.SortOrder
	if sortOrder == "" {
		sortOrder = "desc"
	}
	qb.Sort(sortField, sortOrder)

	if filter.Cursor != "" {
		sortValues, err := DecodeCursor(filter.Cursor)
		if err == nil && len(sortValues) > 0 {
			qb.SearchAfter(sortValues)
		}
	} else {
		offset := (filter.Page - 1) * filter.PageSize
		if offset > 0 {
			qb.Offset(offset)
		}
	}

	return qb.Build()
}

// Count returns the number of notifications matching a filter
func (r *NotificationRepository) Count(ctx context.Context, filter *notification.NotificationFilter) (int64, error) {
	query := r.buildNotificationQuery(filter)
	// Remove pagination from count query
	delete(query, "from")
	delete(query, "size")
	delete(query, "sort")

	return r.BaseRepository.Count(ctx, query)
}

// ── Phase 5: Snooze, Archive, Mark-All-Read ────────────────────────

// UpdateSnooze updates a notification's status and snoozed_until field.
func (r *NotificationRepository) UpdateSnooze(ctx context.Context, id string, status notification.Status, snoozedUntil *time.Time) error {
	updateDoc := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}
	if snoozedUntil != nil {
		updateDoc["snoozed_until"] = *snoozedUntil
	} else {
		updateDoc["snoozed_until"] = nil
	}
	return r.BaseRepository.Update(ctx, id, updateDoc)
}

// BulkArchive sets multiple notifications to archived status.
func (r *NotificationRepository) BulkArchive(ctx context.Context, ids []string, archivedAt time.Time) error {
	updateDoc := map[string]interface{}{
		"status":      notification.StatusArchived,
		"archived_at": archivedAt,
		"updated_at":  time.Now(),
	}
	documents := make(map[string]interface{})
	for _, id := range ids {
		documents[id] = updateDoc
	}
	return r.BaseRepository.BulkUpdate(ctx, documents)
}

// MarkAllRead marks all unread (sent/delivered) notifications as read for a user
// using an atomic _update_by_query. If category is non-empty, only notifications
// matching that category are updated. Returns the number of notifications updated.
func (r *NotificationRepository) MarkAllRead(ctx context.Context, userID, appID, category string) (int, error) {
	filters := []map[string]interface{}{
		{"term": map[string]interface{}{"user_id": userID}},
		{"term": map[string]interface{}{"app_id": appID}},
		{"terms": map[string]interface{}{"status": []string{
			string(notification.StatusSent),
			string(notification.StatusDelivered),
		}}},
	}
	if category != "" {
		filters = append(filters, map[string]interface{}{
			"term": map[string]interface{}{"category": category},
		})
	}

	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{"filter": filters},
		},
		"script": map[string]interface{}{
			"source": "ctx._source.status = params.status; ctx._source.read_at = params.now; ctx._source.updated_at = params.now",
			"lang":   "painless",
			"params": map[string]interface{}{
				"status": string(notification.StatusRead),
				"now":    time.Now().Format(time.RFC3339),
			},
		},
	}

	updated, err := r.BaseRepository.UpdateByQuery(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("mark all read: %w", err)
	}
	return int(updated), nil
}

// ListSnoozedDue returns notifications whose snooze period has expired.
func (r *NotificationRepository) ListSnoozedDue(ctx context.Context, now time.Time) ([]*notification.Notification, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"filter": []map[string]interface{}{
					{"term": map[string]interface{}{"status": string(notification.StatusSnoozed)}},
					{"range": map[string]interface{}{
						"snoozed_until": map[string]interface{}{"lte": now.Format(time.RFC3339)},
					}},
				},
			},
		},
		"size": 100,
		"sort": []map[string]interface{}{
			{"snoozed_until": map[string]interface{}{"order": "asc"}},
		},
	}

	results, err := r.BaseRepository.Search(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("search snoozed due: %w", err)
	}

	var notifications []*notification.Notification
	for _, doc := range results.Hits {
		var n notification.Notification
		if err := mapToStruct(doc, &n); err != nil {
			r.logger.Warn("Failed to map snoozed notification", zap.Error(err))
			continue
		}
		notifications = append(notifications, &n)
	}
	return notifications, nil
}
