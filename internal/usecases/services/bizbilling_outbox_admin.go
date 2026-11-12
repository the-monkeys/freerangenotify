package services

import (
	"context"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

func (s *BizOutboxService) ListPage(ctx context.Context, app, status string, limit, offset int) ([]*bizbilling.NotificationDelivery, int64, error) {
	if app == "" {
		return nil, 0, apperrors.Unauthorized("Application not authenticated")
	}
	switch status {
	case "", "pending", "accepted", "delivered", "failed", "unknown":
	default:
		return nil, 0, apperrors.BadRequest("Invalid delivery status")
	}
	if offset < 0 || offset > 9900 {
		return nil, 0, apperrors.BadRequest("Invalid delivery offset")
	}
	if err := s.records.Refresh(ctx); err != nil {
		return nil, 0, err
	}
	query := bizbillingrepo.NewQuery(app).Term("status", status).Body("created_at", limit, offset)
	query["sort"] = []any{map[string]any{"created_at": "desc"}, map[string]any{"id": "asc"}}
	rows, total, err := s.records.Find(ctx, query)
	if err != nil {
		return nil, 0, err
	}
	for i, r := range rows {
		rows[i] = publicDelivery(r)
	}
	return rows, total, nil
}

// Retry changes delivery admission only. Uncertainty can never authorize a
// second send, and the lifetime attempt count remains available for audit.
func (s *BizOutboxService) Retry(ctx context.Context, app, id string) (*bizbilling.NotificationDelivery, error) {
	row, err := s.records.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if app == "" || row.AppID != app {
		return nil, apperrors.NotFound("notification delivery", id)
	}
	row, err = s.records.Update(ctx, id, func(r *bizbilling.NotificationDelivery) error {
		if r.AppID != app {
			return apperrors.NotFound("notification delivery", id)
		}
		if r.Scope.Mode == bizbilling.GatewayTest || r.MayHaveDispatched || r.LeaseUntil.After(time.Now()) {
			return bizbilling.ErrConflict
		}
		if r.Status != "pending" && (r.Status != "failed" || (r.FailureCode != "admission_rejected" && r.FailureCode != "document_unavailable")) {
			return bizbilling.ErrConflict
		}
		r.Status, r.FailureCode = "pending", ""
		r.RetryAfterAttempt = r.Attempts
		r.NextAttemptAt, r.UpdatedAt = time.Now().UTC(), time.Now().UTC()
		r.LeaseOwner, r.LeaseUntil = "", time.Time{}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return publicDelivery(row), nil
}

func (s *BizOutboxService) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				run, cancel := context.WithTimeout(ctx, 25*time.Second)
				_, observeErr := s.RefreshDelivery(run, 100)
				_, dispatchErr := s.DispatchDue(run, 100)
				cancel()
				if (observeErr != nil || dispatchErr != nil) && ctx.Err() == nil {
					s.logger.Warn("billing notification recovery incomplete", zap.String("failure_code", "delivery_recovery_pending"))
				}
			}
		}
	}()
}
