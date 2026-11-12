package services

import (
	"context"
	"errors"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/secrets"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

type billingNotificationDispatcher interface {
	Dispatch(context.Context, bizbilling.NotificationIntent, string, map[string]interface{}) (*notification.Notification, error)
	Get(context.Context, string, string) (*notification.Notification, error)
}

type BizOutboxService struct {
	es         *elasticsearch.Client
	operations *BizOperationService
	sender     billingNotificationDispatcher
	portal     *BizPortalService
	codec      *secrets.ConnectionCodec
	records    *bizbillingrepo.Store[bizbilling.NotificationDelivery]
	logger     *zap.Logger
}

func NewBizOutboxService(es *elasticsearch.Client, operations *BizOperationService, sender billingNotificationDispatcher, portal *BizPortalService, codec *secrets.ConnectionCodec, logger *zap.Logger) *BizOutboxService {
	return &BizOutboxService{es: es, operations: operations, sender: sender, portal: portal, codec: codec, records: bizbillingrepo.NewStore[bizbilling.NotificationDelivery](es, bizbillingrepo.NotificationOutboxIndex, logger, repository.RefreshFalse), logger: logger}
}

func publicDelivery(row *bizbilling.NotificationDelivery) *bizbilling.NotificationDelivery {
	copy := *row
	copy.SensitivePayload, copy.LeaseOwner = nil, ""
	return &copy
}
func (s *BizOutboxService) Get(ctx context.Context, app, id string) (*bizbilling.NotificationDelivery, error) {
	row, err := s.records.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if row.AppID != app {
		return nil, apperrors.NotFound("notification delivery", id)
	}
	return publicDelivery(row), nil
}
func (s *BizOutboxService) List(ctx context.Context, app string, limit int) ([]*bizbilling.NotificationDelivery, error) {
	rows, _, err := s.ListPage(ctx, app, "", limit, 0)
	return rows, err
}

func (s *BizOutboxService) DispatchDue(ctx context.Context, limit int) (int, error) {
	return s.DispatchDueForApp(ctx, "", limit)
}
func (s *BizOutboxService) DispatchDueForApp(ctx context.Context, app string, limit int) (int, error) {
	if err := s.records.Refresh(ctx); err != nil {
		return 0, err
	}
	now := time.Now().UTC()
	query := bizbillingrepo.NewQuery(app).Term("status", "pending").Range("next_attempt_at", nil, now).Range("lease_until", nil, now).Body("next_attempt_at", limit, 0)
	query["sort"] = []any{map[string]any{"next_attempt_at": "asc"}, map[string]any{"id": "asc"}}
	rows, _, err := s.records.Find(ctx, query)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, row := range rows {
		if ctx.Err() != nil {
			return count, ctx.Err()
		}
		if row.NextAttemptAt.After(time.Now()) || row.LeaseUntil.After(time.Now()) {
			continue
		}
		claimed, err := s.claim(ctx, row.ID)
		if err != nil {
			return count, err
		}
		if claimed == nil {
			continue
		}
		if err = s.dispatch(ctx, claimed); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (s *BizOutboxService) claim(ctx context.Context, id string) (*bizbilling.NotificationDelivery, error) {
	owner := uuid.NewString()
	won := false
	row, err := s.records.Update(ctx, id, func(r *bizbilling.NotificationDelivery) error {
		won = false
		now := time.Now().UTC()
		if r.Status != "pending" || r.NextAttemptAt.After(now) || r.LeaseUntil.After(now) {
			return nil
		}
		if r.MayHaveDispatched {
			r.Status, r.FailureCode = "unknown", "admission_unconfirmed"
			return nil
		}
		r.LeaseOwner, r.LeaseUntil, r.UpdatedAt = owner, now.Add(30*time.Second), now
		won = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !won {
		return nil, nil
	}
	return row, nil
}

func (s *BizOutboxService) finish(ctx context.Context, row *bizbilling.NotificationDelivery, status, code string, uncertain bool) error {
	_, err := s.records.Update(ctx, row.ID, func(r *bizbilling.NotificationDelivery) error {
		if r.LeaseOwner != row.LeaseOwner {
			return bizbilling.ErrConflict
		}
		r.Status, r.FailureCode, r.MayHaveDispatched = status, code, uncertain
		r.LeaseOwner, r.LeaseUntil, r.UpdatedAt = "", time.Time{}, time.Now().UTC()
		if status == "pending" {
			r.NextAttemptAt = time.Now().UTC().Add(time.Duration(1<<min(max(r.Attempts-r.RetryAfterAttempt, 0), 7)) * 5 * time.Second)
		}
		return nil
	})
	return err
}

func (s *BizOutboxService) dispatch(ctx context.Context, row *bizbilling.NotificationDelivery) error {
	if row.Scope.Mode == bizbilling.GatewayTest {
		return s.finish(ctx, row, "failed", "test_mode_suppressed", false)
	}
	head, err := committedFinancialSequence(ctx, s.operations.gate, s.operations, row.Scope)
	if err != nil || head < row.Sequence {
		return s.finish(ctx, row, "pending", "financial_revision_pending", false)
	}
	if row.Attempts > 0 && s.sender != nil {
		existing, lookupErr := s.sender.Get(ctx, row.NotificationID, row.AppID)
		if lookupErr == nil && existing != nil && existing.Status != notification.StatusFailed {
			if existing.AppID != row.AppID || existing.UserID != row.Scope.UserID || existing.NotificationID != row.NotificationID {
				return s.finish(ctx, row, "unknown", "admission_unverified", true)
			}
			status, code := deliveryObservation(existing, "unknown")
			return s.finish(ctx, row, status, code, true)
		}
		if lookupErr != nil && !apperrors.IsNotFound(lookupErr) {
			return s.finish(ctx, row, "pending", "notification_lookup_pending", false)
		}
	}
	row, err = s.records.Update(ctx, row.ID, func(r *bizbilling.NotificationDelivery) error {
		if r.LeaseOwner != row.LeaseOwner || !r.LeaseUntil.After(time.Now()) {
			return bizbilling.ErrConflict
		}
		r.Attempts++
		r.UpdatedAt = time.Now().UTC()
		return nil
	})
	if err != nil {
		return err
	}
	data, err := s.preparePayload(ctx, row)
	if err != nil {
		return s.preparationFailure(ctx, row)
	}
	if data == nil {
		return s.finish(ctx, row, "failed", "reminder_no_longer_due", false)
	}
	row, err = s.records.Update(ctx, row.ID, func(r *bizbilling.NotificationDelivery) error {
		if r.LeaseOwner != row.LeaseOwner || !r.LeaseUntil.After(time.Now()) {
			return bizbilling.ErrConflict
		}
		r.MayHaveDispatched = true
		r.UpdatedAt = time.Now().UTC()
		return nil
	})
	if err != nil {
		return err
	}
	if s.sender == nil {
		return s.admissionFailure(ctx, row)
	}
	run, cancel := context.WithTimeout(ctx, 15*time.Second)
	n, sendErr := s.sender.Dispatch(run, row.Intent, row.NotificationID, data)
	cancel()
	if sendErr != nil {
		var uncertain *notification.DispatchError
		if errors.As(sendErr, &uncertain) && uncertain.MayBeEnqueued {
			return s.finish(ctx, row, "unknown", "admission_unconfirmed", true)
		}
		return s.admissionFailure(ctx, row)
	}
	if n == nil || n.NotificationID != row.NotificationID || n.AppID != row.AppID || n.UserID != row.Scope.UserID {
		return s.finish(ctx, row, "unknown", "admission_unverified", true)
	}
	status, code := deliveryObservation(n, "accepted")
	return s.finish(ctx, row, status, code, true)
}

func (s *BizOutboxService) admissionFailure(ctx context.Context, row *bizbilling.NotificationDelivery) error {
	status := "pending"
	if row.Attempts-row.RetryAfterAttempt >= 6 {
		status = "failed"
	}
	return s.finish(ctx, row, status, "admission_rejected", false)
}

func deliveryObservation(n *notification.Notification, prior string) (string, string) {
	switch n.Status {
	case notification.StatusDelivered, notification.StatusRead:
		return "delivered", ""
	case notification.StatusProcessing, notification.StatusSent:
		return "accepted", ""
	case notification.StatusQueued:
		if prior == "accepted" {
			return "accepted", ""
		}
		return "unknown", "admission_unconfirmed"
	case notification.StatusCancelled:
		return "failed", "notification_cancelled"
	case notification.StatusFailed:
		return "unknown", "provider_outcome_needs_review"
	default:
		return "unknown", "delivery_unconfirmed"
	}
}

func (s *BizOutboxService) RefreshDelivery(ctx context.Context, limit int) (int, error) {
	return s.RefreshDeliveryForApp(ctx, "", limit)
}
func (s *BizOutboxService) RefreshDeliveryForApp(ctx context.Context, app string, limit int) (int, error) {
	if s.sender == nil {
		return 0, nil
	}
	if err := s.records.Refresh(ctx); err != nil {
		return 0, err
	}
	query := bizbillingrepo.NewQuery(app).Terms("status", []string{"accepted", "unknown"}).Body("updated_at", limit, 0)
	query["sort"] = []any{map[string]any{"updated_at": "asc"}, map[string]any{"id": "asc"}}
	rows, _, err := s.records.Find(ctx, query)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, row := range rows {
		if ctx.Err() != nil {
			return count, ctx.Err()
		}
		n, lookupErr := s.sender.Get(ctx, row.NotificationID, row.AppID)
		status, code := "unknown", "notification_unavailable"
		if !apperrors.IsNotFound(lookupErr) {
			if lookupErr != nil {
				return count, lookupErr
			}
			if n == nil || n.AppID != row.AppID || n.UserID != row.Scope.UserID || n.NotificationID != row.NotificationID {
				return count, bizbilling.ErrReconciliationRequired
			}
			status, code = deliveryObservation(n, row.Status)
		}

		_, err = s.records.Update(ctx, row.ID, func(r *bizbilling.NotificationDelivery) error {
			if (r.Status != "accepted" && r.Status != "unknown") || r.Status != row.Status || r.UpdatedAt.After(row.UpdatedAt) {
				return nil
			}
			r.Status, r.FailureCode, r.UpdatedAt = status, code, time.Now().UTC()
			return nil
		})
		if err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (s *BizOutboxService) preparationFailure(ctx context.Context, row *bizbilling.NotificationDelivery) error {
	status := "pending"
	if row.Attempts-row.RetryAfterAttempt >= 6 {
		status = "failed"
	}
	return s.finish(ctx, row, status, "document_unavailable", false)
}
