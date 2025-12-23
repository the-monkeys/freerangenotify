package usecases

import (
	"context"

	"github.com/the-monkeys/freerangenotify/internal/domain/user"
)

// PresenceService defines the interface for handling user availability and smart delivery
type PresenceService interface {
	CheckIn(ctx context.Context, userID, appID, dynamicURL string) error
	GetPresence(ctx context.Context, userID string) (*user.Presence, error)
	IsUserAvailable(ctx context.Context, userID string) (bool, string, error)
}
