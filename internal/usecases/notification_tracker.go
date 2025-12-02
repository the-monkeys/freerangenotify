package usecases

import (
	"context"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"go.uber.org/zap"
)

// NotificationTracker tracks notification delivery and emits events
type NotificationTracker struct {
	repo   notification.Repository
	logger *zap.Logger
}

// NewNotificationTracker creates a new notification tracker
func NewNotificationTracker(repo notification.Repository, logger *zap.Logger) *NotificationTracker {
	return &NotificationTracker{
		repo:   repo,
		logger: logger,
	}
}

// UpdateStatus updates the notification status and logs the event
func (t *NotificationTracker) UpdateStatus(ctx context.Context, notificationID string, status notification.Status, errorMessage string) error {
	err := t.repo.UpdateStatus(ctx, notificationID, status)
	if err != nil {
		t.logger.Error("Failed to update notification status",
			zap.String("notification_id", notificationID),
			zap.String("status", string(status)),
			zap.Error(err))
		return err
	}

	// If there's an error message, update it separately
	if errorMessage != "" {
		notif, err := t.repo.GetByID(ctx, notificationID)
		if err == nil {
			notif.ErrorMessage = errorMessage
			t.repo.Update(ctx, notif)
		}
	}

	t.logger.Info("Notification status updated",
		zap.String("notification_id", notificationID),
		zap.String("status", string(status)))

	// Emit event for analytics
	t.EmitEvent(ctx, "notification.status_changed", map[string]interface{}{
		"notification_id": notificationID,
		"status":          string(status),
		"timestamp":       time.Now(),
		"error_message":   errorMessage,
	})

	return nil
}

// EmitEvent emits an analytics event (placeholder for now)
func (t *NotificationTracker) EmitEvent(ctx context.Context, eventType string, data map[string]interface{}) {
	// TODO: Implement actual analytics event emission
	// This could publish to a message queue, write to analytics DB, or send to analytics service
	t.logger.Debug("Analytics event emitted",
		zap.String("event_type", eventType),
		zap.Any("data", data))
}

// TrackDeliveryMetrics tracks delivery metrics for a notification
func (t *NotificationTracker) TrackDeliveryMetrics(ctx context.Context, notif *notification.Notification, deliveryTime time.Duration) {
	metrics := map[string]interface{}{
		"notification_id":  notif.NotificationID,
		"app_id":           notif.AppID,
		"user_id":          notif.UserID,
		"channel":          string(notif.Channel),
		"priority":         string(notif.Priority),
		"delivery_time_ms": deliveryTime.Milliseconds(),
		"retry_count":      notif.RetryCount,
		"timestamp":        time.Now(),
	}

	t.EmitEvent(ctx, "notification.delivered", metrics)

	t.logger.Info("Delivery metrics tracked",
		zap.String("notification_id", notif.NotificationID),
		zap.Duration("delivery_time", deliveryTime))
}

// GenerateDeliveryReport generates a delivery report for a given period
func (t *NotificationTracker) GenerateDeliveryReport(ctx context.Context, filter notification.NotificationFilter) (*DeliveryReport, error) {
	// Get notifications for the period
	notifications, err := t.repo.List(ctx, &filter)
	if err != nil {
		return nil, err
	}

	// Get total count
	total, err := t.repo.Count(ctx, &filter)
	if err != nil {
		return nil, err
	}

	// Calculate metrics
	report := &DeliveryReport{
		TotalNotifications: total,
		Period: Period{
			From: filter.FromDate,
			To:   filter.ToDate,
		},
		ByStatus:  make(map[string]int64),
		ByChannel: make(map[string]int64),
	}

	for _, n := range notifications {
		report.ByStatus[string(n.Status)]++
		report.ByChannel[string(n.Channel)]++

		if n.Status == notification.StatusDelivered && n.SentAt != nil && n.DeliveredAt != nil {
			deliveryTime := n.DeliveredAt.Sub(*n.SentAt)
			report.TotalDeliveryTime += deliveryTime
			report.DeliveredCount++
		}

		if n.Status == notification.StatusFailed {
			report.FailedCount++
		}

		report.TotalRetries += int64(n.RetryCount)
	}

	if report.DeliveredCount > 0 {
		report.AverageDeliveryTime = report.TotalDeliveryTime / time.Duration(report.DeliveredCount)
	}

	if total > 0 {
		report.SuccessRate = float64(report.DeliveredCount) / float64(total) * 100
	}

	return report, nil
}

// DeliveryReport represents a delivery metrics report
type DeliveryReport struct {
	TotalNotifications  int64            `json:"total_notifications"`
	DeliveredCount      int64            `json:"delivered_count"`
	FailedCount         int64            `json:"failed_count"`
	TotalRetries        int64            `json:"total_retries"`
	TotalDeliveryTime   time.Duration    `json:"total_delivery_time"`
	AverageDeliveryTime time.Duration    `json:"average_delivery_time"`
	SuccessRate         float64          `json:"success_rate"`
	Period              Period           `json:"period"`
	ByStatus            map[string]int64 `json:"by_status"`
	ByChannel           map[string]int64 `json:"by_channel"`
}

// Period represents a time period
type Period struct {
	From *time.Time `json:"from"`
	To   *time.Time `json:"to"`
}
