package services

import (
	"context"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/usecases"
	"go.uber.org/zap"
)

type presenceService struct {
	presenceRepo user.PresenceRepository
	notifService notification.Service
	logger       *zap.Logger
}

func NewPresenceService(
	presenceRepo user.PresenceRepository,
	notifService notification.Service,
	logger *zap.Logger,
) usecases.PresenceService {
	return &presenceService{
		presenceRepo: presenceRepo,
		notifService: notifService,
		logger:       logger,
	}
}

func (s *presenceService) CheckIn(ctx context.Context, userID, appID, dynamicURL string) error {
	p := &user.Presence{
		UserID:     userID,
		AppID:      appID,
		DynamicURL: dynamicURL,
		LastSeen:   time.Now(),
		Status:     "active",
	}

	// 1. Register presence (5 minute TTL for heartbeats)
	if err := s.presenceRepo.Set(ctx, p, 5*time.Minute); err != nil {
		s.logger.Error("Failed to set user presence", zap.String("user_id", userID), zap.Error(err))
		return err
	}

	s.logger.Info("User checked in", zap.String("user_id", userID), zap.String("url", dynamicURL))

	// 2. Trigger instant flush of queued notifications
	// We do this in a goroutine to not block the check-in response (or just call it if quick)
	// Given it's a "Jump the line" feature, we want it reactive.
	go func() {
		// Use a fresh background context as the request context might be cancelled
		if err := s.notifService.FlushQueued(context.Background(), userID); err != nil {
			s.logger.Error("Failed to flush queued notifications on check-in",
				zap.String("user_id", userID), zap.Error(err))
		}
	}()

	return nil
}

func (s *presenceService) GetPresence(ctx context.Context, userID string) (*user.Presence, error) {
	return s.presenceRepo.Get(ctx, userID)
}

func (s *presenceService) IsUserAvailable(ctx context.Context, userID string) (bool, string, error) {
	return s.presenceRepo.IsAvailable(ctx, userID)
}
