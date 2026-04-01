package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/topic"
	"github.com/the-monkeys/freerangenotify/internal/domain/workflow"
	pkgerrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// topicService implements topic.Service.
type topicService struct {
	repo        topic.Repository
	workflowSvc workflow.Service // Phase 4: optional, for on-subscribe trigger
	logger      *zap.Logger
}

// NewTopicService creates a new topic service.
func NewTopicService(repo topic.Repository, logger *zap.Logger) topic.Service {
	return &topicService{
		repo:   repo,
		logger: logger,
	}
}

// SetWorkflowService injects the optional workflow service for on-subscribe triggers.
func (s *topicService) SetWorkflowService(ws workflow.Service) {
	s.workflowSvc = ws
}

func (s *topicService) Create(ctx context.Context, appID string, req *topic.CreateRequest) (*topic.Topic, error) {
	// Check for duplicate key within the app
	existing, err := s.repo.GetByKey(ctx, appID, req.Key)
	if err == nil && existing != nil {
		return nil, pkgerrors.New(pkgerrors.ErrCodeConflict,
			fmt.Sprintf("topic with key '%s' already exists in this app", req.Key))
	}

	t := &topic.Topic{
		ID:                   uuid.New().String(),
		AppID:                appID,
		EnvironmentID:        req.EnvironmentID,
		Name:                 req.Name,
		Key:                  req.Key,
		Description:          req.Description,
		OnSubscribeTriggerID: req.OnSubscribeTriggerID,
		CreatedAt:            time.Now().UTC(),
		UpdatedAt:            time.Now().UTC(),
	}

	if err := s.repo.Create(ctx, t); err != nil {
		s.logger.Error("Failed to create topic", zap.Error(err))
		return nil, pkgerrors.New(pkgerrors.ErrCodeInternal, "failed to create topic")
	}

	s.logger.Info("Topic created",
		zap.String("topic_id", t.ID),
		zap.String("app_id", appID),
		zap.String("key", t.Key))
	return t, nil
}

func (s *topicService) Get(ctx context.Context, id, appID string) (*topic.Topic, error) {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, pkgerrors.New(pkgerrors.ErrCodeNotFound, "topic not found")
	}
	if t.AppID != appID {
		return nil, pkgerrors.New(pkgerrors.ErrCodeNotFound, "topic not found")
	}
	return t, nil
}

func (s *topicService) GetByKey(ctx context.Context, appID, key string) (*topic.Topic, error) {
	t, err := s.repo.GetByKey(ctx, appID, key)
	if err != nil {
		return nil, pkgerrors.New(pkgerrors.ErrCodeNotFound, "topic not found")
	}
	return t, nil
}

func (s *topicService) List(ctx context.Context, appID, environmentID string, linkedIDs []string, limit, offset int) ([]*topic.Topic, int64, error) {
	topics, total, err := s.repo.List(ctx, appID, environmentID, linkedIDs, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	// Enrich each topic with subscriber count
	for _, t := range topics {
		count, countErr := s.repo.GetSubscriberCount(ctx, t.ID)
		if countErr == nil {
			t.SubscriberCount = count
		}
	}
	return topics, total, nil
}

func (s *topicService) Delete(ctx context.Context, id, appID string) error {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return pkgerrors.New(pkgerrors.ErrCodeNotFound, "topic not found")
	}
	if t.AppID != appID {
		return pkgerrors.New(pkgerrors.ErrCodeNotFound, "topic not found")
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		s.logger.Error("Failed to delete topic", zap.Error(err))
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "failed to delete topic")
	}

	s.logger.Info("Topic deleted",
		zap.String("topic_id", id),
		zap.String("app_id", appID))
	return nil
}

func (s *topicService) Update(ctx context.Context, id, appID string, req *topic.UpdateRequest) (*topic.Topic, error) {
	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, pkgerrors.New(pkgerrors.ErrCodeNotFound, "topic not found")
	}
	if t.AppID != appID {
		return nil, pkgerrors.New(pkgerrors.ErrCodeNotFound, "topic not found")
	}

	if req.Name != nil {
		t.Name = *req.Name
	}
	if req.Description != nil {
		t.Description = *req.Description
	}
	t.UpdatedAt = time.Now().UTC()

	if err := s.repo.Update(ctx, t); err != nil {
		s.logger.Error("Failed to update topic", zap.Error(err))
		return nil, pkgerrors.New(pkgerrors.ErrCodeInternal, "failed to update topic")
	}

	s.logger.Info("Topic updated",
		zap.String("topic_id", id),
		zap.String("app_id", appID))
	return t, nil
}

func (s *topicService) AddSubscribers(ctx context.Context, topicID, appID string, userIDs []string) error {
	// Verify topic exists and belongs to app
	t, err := s.repo.GetByID(ctx, topicID)
	if err != nil {
		return pkgerrors.New(pkgerrors.ErrCodeNotFound, "topic not found")
	}
	if t.AppID != appID {
		return pkgerrors.New(pkgerrors.ErrCodeNotFound, "topic not found")
	}

	if err := s.repo.AddSubscribers(ctx, topicID, appID, userIDs); err != nil {
		s.logger.Error("Failed to add subscribers", zap.Error(err))
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "failed to add subscribers")
	}

	// Phase 4: Trigger workflow for each new subscriber (fire-and-forget)
	if t.OnSubscribeTriggerID != "" && s.workflowSvc != nil && len(userIDs) > 0 {
		payload := map[string]any{"topic_id": topicID, "topic_key": t.Key}
		for _, userID := range userIDs {
			_, triggerErr := s.workflowSvc.Trigger(ctx, appID, &workflow.TriggerRequest{
				TriggerID: t.OnSubscribeTriggerID,
				UserID:    userID,
				Payload:   payload,
			})
			if triggerErr != nil {
				s.logger.Warn("On-subscribe workflow trigger failed (non-fatal)",
					zap.String("topic_id", topicID),
					zap.String("user_id", userID),
					zap.String("trigger_id", t.OnSubscribeTriggerID),
					zap.Error(triggerErr))
			}
		}
	}

	s.logger.Info("Subscribers added to topic",
		zap.String("topic_id", topicID),
		zap.Int("count", len(userIDs)))
	return nil
}

func (s *topicService) RemoveSubscribers(ctx context.Context, topicID, appID string, userIDs []string) error {
	// Verify topic exists and belongs to app
	t, err := s.repo.GetByID(ctx, topicID)
	if err != nil {
		return pkgerrors.New(pkgerrors.ErrCodeNotFound, "topic not found")
	}
	if t.AppID != appID {
		return pkgerrors.New(pkgerrors.ErrCodeNotFound, "topic not found")
	}

	if err := s.repo.RemoveSubscribers(ctx, topicID, userIDs); err != nil {
		s.logger.Error("Failed to remove subscribers", zap.Error(err))
		return pkgerrors.New(pkgerrors.ErrCodeInternal, "failed to remove subscribers")
	}

	s.logger.Info("Subscribers removed from topic",
		zap.String("topic_id", topicID),
		zap.Int("count", len(userIDs)))
	return nil
}

func (s *topicService) GetSubscribers(ctx context.Context, topicID, appID string, limit, offset int) ([]topic.TopicSubscription, int64, error) {
	// Verify topic exists and belongs to app
	t, err := s.repo.GetByID(ctx, topicID)
	if err != nil {
		return nil, 0, pkgerrors.New(pkgerrors.ErrCodeNotFound, "topic not found")
	}
	if t.AppID != appID {
		return nil, 0, pkgerrors.New(pkgerrors.ErrCodeNotFound, "topic not found")
	}

	return s.repo.GetSubscribers(ctx, topicID, limit, offset)
}

func (s *topicService) GetSubscriberUserIDs(ctx context.Context, topicID, appID string) ([]string, error) {
	// Verify topic exists and belongs to app
	t, err := s.repo.GetByID(ctx, topicID)
	if err != nil {
		return nil, pkgerrors.New(pkgerrors.ErrCodeNotFound, "topic not found")
	}
	if t.AppID != appID {
		return nil, pkgerrors.New(pkgerrors.ErrCodeNotFound, "topic not found")
	}

	// Page through all subscribers
	const batchSize = 500
	var allUserIDs []string
	offset := 0

	for {
		subs, total, err := s.repo.GetSubscribers(ctx, topicID, batchSize, offset)
		if err != nil {
			return nil, err
		}

		for _, sub := range subs {
			allUserIDs = append(allUserIDs, sub.UserID)
		}

		offset += batchSize
		if int64(offset) >= total {
			break
		}
	}

	return allUserIDs, nil
}
