package user

import (
	"context"
	"time"
)

// Presence represents a user's active connection/availability
type Presence struct {
	UserID     string    `json:"user_id"`
	AppID      string    `json:"app_id"`
	DynamicURL string    `json:"dynamic_url"` // Optional specific endpoint for this session
	LastSeen   time.Time `json:"last_seen"`
	Status     string    `json:"status"` // "active", "away", etc.
}

// PresenceRepository defines operations for tracking user presence
type PresenceRepository interface {
	Set(ctx context.Context, presence *Presence, ttl time.Duration) error
	Get(ctx context.Context, userID string) (*Presence, error)
	Delete(ctx context.Context, userID string) error
	IsAvailable(ctx context.Context, userID string) (bool, string, error) // Returns (isAvailable, dynamicURL, error)
}
