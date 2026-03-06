package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/digest"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

type digestService struct {
	repo   digest.Repository
	logger *zap.Logger
}

// NewDigestService creates a new digest service.
func NewDigestService(repo digest.Repository, logger *zap.Logger) digest.Service {
	return &digestService{
		repo:   repo,
		logger: logger,
	}
}

func (s *digestService) Create(ctx context.Context, appID string, req *digest.CreateRequest) (*digest.DigestRule, error) {
	s.logger.Info("Creating digest rule",
		zap.String("app_id", appID),
		zap.String("name", req.Name),
		zap.String("digest_key", req.DigestKey))

	// Validate window duration
	if _, err := time.ParseDuration(req.Window); err != nil {
		return nil, errors.BadRequest(fmt.Sprintf("invalid window duration '%s': must be a valid Go duration (e.g. 1h, 30m, 24h)", req.Window))
	}

	rule := &digest.DigestRule{
		ID:            uuid.New().String(),
		AppID:         appID,
		EnvironmentID: req.EnvironmentID,
		Name:          req.Name,
		DigestKey:     req.DigestKey,
		Window:        req.Window,
		Channel:       req.Channel,
		TemplateID:    req.TemplateID,
		MaxBatch:      req.MaxBatch,
		Status:        "active",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := s.repo.Create(ctx, rule); err != nil {
		return nil, fmt.Errorf("failed to create digest rule: %w", err)
	}

	return rule, nil
}

func (s *digestService) Get(ctx context.Context, id, appID string) (*digest.DigestRule, error) {
	rule, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, errors.NotFound("digest_rule", id)
	}
	if rule.AppID != appID {
		return nil, errors.NotFound("digest_rule", id)
	}
	return rule, nil
}

func (s *digestService) List(ctx context.Context, appID, environmentID string, limit, offset int) ([]*digest.DigestRule, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return s.repo.List(ctx, appID, environmentID, limit, offset)
}

func (s *digestService) Update(ctx context.Context, id, appID string, req *digest.UpdateRequest) (*digest.DigestRule, error) {
	rule, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, errors.NotFound("digest_rule", id)
	}
	if rule.AppID != appID {
		return nil, errors.NotFound("digest_rule", id)
	}

	if req.Name != nil {
		rule.Name = *req.Name
	}
	if req.Window != nil {
		if _, err := time.ParseDuration(*req.Window); err != nil {
			return nil, errors.BadRequest(fmt.Sprintf("invalid window duration '%s'", *req.Window))
		}
		rule.Window = *req.Window
	}
	if req.Channel != nil {
		rule.Channel = *req.Channel
	}
	if req.TemplateID != nil {
		rule.TemplateID = *req.TemplateID
	}
	if req.MaxBatch != nil {
		rule.MaxBatch = *req.MaxBatch
	}
	if req.Status != nil {
		rule.Status = *req.Status
	}

	rule.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, rule); err != nil {
		return nil, fmt.Errorf("failed to update digest rule: %w", err)
	}

	return rule, nil
}

func (s *digestService) Delete(ctx context.Context, id, appID string) error {
	rule, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return errors.NotFound("digest_rule", id)
	}
	if rule.AppID != appID {
		return errors.NotFound("digest_rule", id)
	}

	return s.repo.Delete(ctx, id)
}
