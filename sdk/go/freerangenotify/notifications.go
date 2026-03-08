package freerangenotify

import (
	"context"
	"net/url"
	"strconv"
)

// NotificationsClient handles notification operations.
type NotificationsClient struct {
	client *Client
}

// ── Quick-Send ──

// QuickSend delivers a notification via the Quick-Send endpoint.
func (n *NotificationsClient) QuickSend(ctx context.Context, params SendParams) (*SendResult, error) {
	var result SendResult
	if err := n.client.do(ctx, "POST", "/quick-send", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ── Standard Send ──

// Send creates a standard notification for a specific user.
func (n *NotificationsClient) Send(ctx context.Context, params NotificationSendParams) (*NotificationResponse, error) {
	var result NotificationResponse
	if err := n.client.do(ctx, "POST", "/notifications/", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ── Bulk Send ──

// SendBulk sends a notification to multiple users at once.
func (n *NotificationsClient) SendBulk(ctx context.Context, params BulkSendParams) (*BulkSendResult, error) {
	var result BulkSendResult
	if err := n.client.do(ctx, "POST", "/notifications/bulk", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ── Batch Send ──

// SendBatch sends multiple distinct notifications in a single request.
func (n *NotificationsClient) SendBatch(ctx context.Context, notifications []NotificationSendParams) (*BulkSendResult, error) {
	var result BulkSendResult
	payload := map[string]interface{}{"notifications": notifications}
	if err := n.client.do(ctx, "POST", "/notifications/batch", payload, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ── Broadcast ──

// Broadcast sends a notification to all users in the application.
func (n *NotificationsClient) Broadcast(ctx context.Context, params BroadcastParams) (*BroadcastResult, error) {
	var result BroadcastResult
	if err := n.client.do(ctx, "POST", "/notifications/broadcast", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ── List ──

// ListNotificationsOptions configures the notification list query.
type ListNotificationsOptions struct {
	UserID     string
	AppID      string
	Channel    string
	Status     string
	Category   string
	Priority   string
	Page       int
	PageSize   int
	UnreadOnly bool
}

// List returns a paginated list of notifications.
func (n *NotificationsClient) List(ctx context.Context, opts ListNotificationsOptions) (*NotificationListResponse, error) {
	q := url.Values{}
	if opts.UserID != "" {
		q.Set("user_id", opts.UserID)
	}
	if opts.AppID != "" {
		q.Set("app_id", opts.AppID)
	}
	if opts.Channel != "" {
		q.Set("channel", opts.Channel)
	}
	if opts.Status != "" {
		q.Set("status", opts.Status)
	}
	if opts.Category != "" {
		q.Set("category", opts.Category)
	}
	if opts.Priority != "" {
		q.Set("priority", opts.Priority)
	}
	if opts.Page > 0 {
		q.Set("page", strconv.Itoa(opts.Page))
	}
	if opts.PageSize > 0 {
		q.Set("page_size", strconv.Itoa(opts.PageSize))
	}
	if opts.UnreadOnly {
		q.Set("unread_only", "true")
	}

	var result NotificationListResponse
	if err := n.client.doWithQuery(ctx, "GET", "/notifications/", q, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ── Get ──

// Get retrieves a single notification by ID.
func (n *NotificationsClient) Get(ctx context.Context, notificationID string) (*NotificationResponse, error) {
	var result NotificationResponse
	if err := n.client.do(ctx, "GET", "/notifications/"+notificationID, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ── Unread Count ──

// GetUnreadCount returns the number of unread notifications for a user.
func (n *NotificationsClient) GetUnreadCount(ctx context.Context, userID string) (int, error) {
	q := url.Values{}
	if userID != "" {
		q.Set("user_id", userID)
	}
	var result struct {
		Count int `json:"count"`
	}
	if err := n.client.doWithQuery(ctx, "GET", "/notifications/unread/count", q, &result); err != nil {
		return 0, err
	}
	return result.Count, nil
}

// ── List Unread ──

// ListUnread returns a paginated list of unread notifications for a user.
func (n *NotificationsClient) ListUnread(ctx context.Context, userID string, page, pageSize int) (*NotificationListResponse, error) {
	q := url.Values{}
	if userID != "" {
		q.Set("user_id", userID)
	}
	if page > 0 {
		q.Set("page", strconv.Itoa(page))
	}
	if pageSize > 0 {
		q.Set("page_size", strconv.Itoa(pageSize))
	}

	var result NotificationListResponse
	if err := n.client.doWithQuery(ctx, "GET", "/notifications/unread", q, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ── Mark Read ──

// MarkRead marks specific notifications as read.
func (n *NotificationsClient) MarkRead(ctx context.Context, userID string, notificationIDs []string) error {
	payload := map[string]interface{}{
		"user_id":          userID,
		"notification_ids": notificationIDs,
	}
	return n.client.do(ctx, "POST", "/notifications/read", payload, nil)
}

// ── Mark All Read ──

// MarkAllRead marks all notifications as read for a user, optionally filtered by category.
func (n *NotificationsClient) MarkAllRead(ctx context.Context, userID, category string) error {
	payload := map[string]interface{}{
		"user_id": userID,
	}
	if category != "" {
		payload["category"] = category
	}
	return n.client.do(ctx, "POST", "/notifications/read-all", payload, nil)
}

// ── Update Status ──

// UpdateStatus updates the status of a notification (e.g. delivered, failed).
func (n *NotificationsClient) UpdateStatus(ctx context.Context, notificationID, status, errorMessage string) error {
	payload := map[string]interface{}{
		"status": status,
	}
	if errorMessage != "" {
		payload["error_message"] = errorMessage
	}
	return n.client.do(ctx, "PUT", "/notifications/"+notificationID+"/status", payload, nil)
}

// ── Cancel ──

// Cancel cancels a pending notification.
func (n *NotificationsClient) Cancel(ctx context.Context, notificationID string) error {
	return n.client.do(ctx, "DELETE", "/notifications/"+notificationID, nil, nil)
}

// ── Cancel Batch ──

// CancelBatch cancels multiple pending notifications at once.
func (n *NotificationsClient) CancelBatch(ctx context.Context, notificationIDs []string) error {
	payload := map[string]interface{}{"notification_ids": notificationIDs}
	return n.client.do(ctx, "DELETE", "/notifications/batch", payload, nil)
}

// ── Retry ──

// Retry requeues a failed notification for redelivery.
func (n *NotificationsClient) Retry(ctx context.Context, notificationID string) (*NotificationResponse, error) {
	var result NotificationResponse
	if err := n.client.do(ctx, "POST", "/notifications/"+notificationID+"/retry", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ── Snooze ──

// Snooze temporarily suppresses a notification for the given duration (e.g. "1h", "30m").
func (n *NotificationsClient) Snooze(ctx context.Context, notificationID, duration string) error {
	payload := map[string]interface{}{"duration": duration}
	return n.client.do(ctx, "POST", "/notifications/"+notificationID+"/snooze", payload, nil)
}

// ── Unsnooze ──

// Unsnooze removes the snooze from a snoozed notification, transitioning it back to sent.
func (n *NotificationsClient) Unsnooze(ctx context.Context, notificationID string) error {
	return n.client.do(ctx, "POST", "/notifications/"+notificationID+"/unsnooze", nil, nil)
}

// ── Archive ──

// Archive bulk-archives notifications for a user.
func (n *NotificationsClient) Archive(ctx context.Context, userID string, notificationIDs []string) error {
	payload := map[string]interface{}{
		"notification_ids": notificationIDs,
		"user_id":          userID,
	}
	return n.client.do(ctx, "PATCH", "/notifications/bulk/archive", payload, nil)
}
