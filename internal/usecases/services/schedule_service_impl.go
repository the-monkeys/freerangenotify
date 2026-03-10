package services

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"github.com/the-monkeys/freerangenotify/internal/domain/schedule"
	"github.com/the-monkeys/freerangenotify/internal/domain/topic"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/domain/workflow"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

type scheduleService struct {
	repo        schedule.Repository
	workflowSvc workflow.Service
	topicSvc    topic.Service
	userRepo    user.Repository
	logger      *zap.Logger
}

// NewScheduleService creates a new schedule service
func NewScheduleService(
	repo schedule.Repository,
	workflowSvc workflow.Service,
	userRepo user.Repository,
	logger *zap.Logger,
) schedule.Service {
	return &scheduleService{
		repo:        repo,
		workflowSvc: workflowSvc,
		userRepo:    userRepo,
		logger:      logger,
	}
}

// SetTopicService injects the optional topic service (for target_type=topic)
func (s *scheduleService) SetTopicService(ts topic.Service) {
	s.topicSvc = ts
}

func (s *scheduleService) Create(ctx context.Context, appID string, req *schedule.CreateRequest) (*schedule.WorkflowSchedule, error) {
	if req.TargetType == schedule.TargetTopic && req.TopicID == "" {
		return nil, errors.BadRequest("topic_id is required when target_type is topic")
	}
	if req.TargetType == schedule.TargetTopic && s.topicSvc == nil {
		return nil, errors.BadRequest("topics feature is not enabled")
	}

	// Validate cron
	if _, err := parseCron(req.Cron); err != nil {
		return nil, errors.BadRequest(fmt.Sprintf("invalid cron expression: %v", err))
	}

	sch := &schedule.WorkflowSchedule{
		ID:                uuid.New().String(),
		AppID:             appID,
		EnvironmentID:     req.EnvironmentID,
		Name:              req.Name,
		WorkflowTriggerID: req.WorkflowTriggerID,
		Cron:              req.Cron,
		TargetType:        req.TargetType,
		TopicID:           req.TopicID,
		Payload:           req.Payload,
		Status:            "active",
	}

	if err := s.repo.Create(ctx, sch); err != nil {
		return nil, fmt.Errorf("failed to create schedule: %w", err)
	}
	return sch, nil
}

func (s *scheduleService) Get(ctx context.Context, id, appID string) (*schedule.WorkflowSchedule, error) {
	sch, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, errors.NotFound("schedule", id)
	}
	if sch.AppID != appID {
		return nil, errors.NotFound("schedule", id)
	}
	return sch, nil
}

func (s *scheduleService) List(ctx context.Context, appID, environmentID string, limit, offset int) ([]*schedule.WorkflowSchedule, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return s.repo.List(ctx, appID, environmentID, limit, offset)
}

func (s *scheduleService) Update(ctx context.Context, id, appID string, req *schedule.UpdateRequest) (*schedule.WorkflowSchedule, error) {
	sch, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, errors.NotFound("schedule", id)
	}
	if sch.AppID != appID {
		return nil, errors.NotFound("schedule", id)
	}

	if req.Name != nil {
		sch.Name = *req.Name
	}
	if req.WorkflowTriggerID != nil {
		sch.WorkflowTriggerID = *req.WorkflowTriggerID
	}
	if req.Cron != nil {
		if _, err := parseCron(*req.Cron); err != nil {
			return nil, errors.BadRequest(fmt.Sprintf("invalid cron expression: %v", err))
		}
		sch.Cron = *req.Cron
	}
	if req.TargetType != nil {
		sch.TargetType = *req.TargetType
	}
	if req.TopicID != nil {
		sch.TopicID = *req.TopicID
	}
	if req.Payload != nil {
		sch.Payload = req.Payload
	}
	if req.Status != nil {
		sch.Status = *req.Status
	}

	if err := s.repo.Update(ctx, sch); err != nil {
		return nil, fmt.Errorf("failed to update schedule: %w", err)
	}
	return sch, nil
}

func (s *scheduleService) Delete(ctx context.Context, id, appID string) error {
	sch, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return errors.NotFound("schedule", id)
	}
	if sch.AppID != appID {
		return errors.NotFound("schedule", id)
	}
	return s.repo.Delete(ctx, id)
}

func parseCron(expr string) (cron.Schedule, error) {
	p := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	return p.Parse(expr)
}
