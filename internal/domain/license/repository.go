package license

import (
	"context"
	"time"
)

// Repository defines persistence operations for hosted subscriptions.
type Repository interface {
	Create(ctx context.Context, sub *Subscription) error
	GetByID(ctx context.Context, id string) (*Subscription, error)
	Update(ctx context.Context, sub *Subscription) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, filter SubscriptionFilter) ([]*Subscription, error)

	// Resolve active subscription with app-level precedence, then tenant fallback.
	GetActiveSubscription(ctx context.Context, tenantID, appID string, now time.Time) (*Subscription, error)
}
