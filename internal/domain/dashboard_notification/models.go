package dashboard_notification

import (
	"context"
	"time"
)

// DashboardNotification is a platform-level in-app notification for dashboard users (auth users).
// Stored separately from app-scoped notifications.
type DashboardNotification struct {
	ID        string     `json:"id" es:"id"`
	UserID    string     `json:"user_id" es:"user_id"`       // auth user ID
	Title     string     `json:"title" es:"title"`
	Body      string     `json:"body" es:"body"`
	Category  string     `json:"category" es:"category"`    // e.g. "org_invite", "system"
	Data      map[string]interface{} `json:"data,omitempty" es:"data"`
	ReadAt    *time.Time `json:"read_at,omitempty" es:"read_at"`
	CreatedAt time.Time  `json:"created_at" es:"created_at"`
}

// Repository defines persistence for dashboard notifications.
type Repository interface {
	Create(ctx context.Context, n *DashboardNotification) error
	GetByID(ctx context.Context, id string) (*DashboardNotification, error)
	ListByUser(ctx context.Context, userID string, limit, offset int) ([]*DashboardNotification, int, error)
	MarkRead(ctx context.Context, userID string, ids []string) (int, error)
	GetUnreadCount(ctx context.Context, userID string) (int64, error)
}

// Notifier sends a dashboard notification and publishes to SSE for real-time delivery.
type Notifier interface {
	NotifyUser(ctx context.Context, userID string, title, body, category string, data map[string]interface{}) error
}
