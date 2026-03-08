package services

import (
	"context"

	"github.com/the-monkeys/freerangenotify/internal/domain/audit"
	"go.uber.org/zap"
)

type auditService struct {
	repo   audit.Repository
	logger *zap.Logger
}

// NewAuditService creates a new audit.Service backed by the given repository.
func NewAuditService(repo audit.Repository, logger *zap.Logger) audit.Service {
	return &auditService{repo: repo, logger: logger}
}

func (s *auditService) Record(ctx context.Context, log *audit.AuditLog) error {
	if err := s.repo.Create(ctx, log); err != nil {
		s.logger.Error("Failed to record audit log",
			zap.String("action", log.Action),
			zap.String("resource", log.Resource),
			zap.Error(err))
		return err
	}
	return nil
}

func (s *auditService) Get(ctx context.Context, id string) (*audit.AuditLog, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *auditService) List(ctx context.Context, filter audit.Filter) ([]*audit.AuditLog, error) {
	return s.repo.List(ctx, filter)
}
