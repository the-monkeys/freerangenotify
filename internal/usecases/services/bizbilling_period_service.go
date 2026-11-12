package services

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

type BizPeriodService struct {
	es        *elasticsearch.Client
	lifecycle *BizLifecycleService
	recurring *BizRecurringService
	pager     *bizbillingrepo.SubscriptionPager
	logger    *zap.Logger
}

func NewBizPeriodService(es *elasticsearch.Client, lifecycle *BizLifecycleService, recurring *BizRecurringService, logger *zap.Logger) *BizPeriodService {
	return &BizPeriodService{es: es, lifecycle: lifecycle, recurring: recurring, pager: bizbillingrepo.NewSubscriptionPager(es), logger: logger}
}

func (s *BizLifecycleService) SetPeriodService(service *BizPeriodService) { s.periods = service }

func (s *BizPeriodService) EnsurePeriod(ctx context.Context, app, subscription string, start, end time.Time) (bizbilling.OperationResult, error) {
	return s.applyPeriod(ctx, app, subscription, start, end, "")
}
func (s *BizPeriodService) BindProviderInvoice(ctx context.Context, app, subscription string, start, end time.Time, providerInvoice string) (bizbilling.OperationResult, error) {
	if providerInvoice == "" {
		return bizbilling.OperationResult{}, bizbilling.ErrInvalidGatewayProof
	}
	return s.applyPeriod(ctx, app, subscription, start, end, providerInvoice)
}

func (s *BizPeriodService) applyPeriod(ctx context.Context, app, subscription string, start, end time.Time, providerInvoice string) (bizbilling.OperationResult, error) {
	if s.lifecycle.operations == nil {
		return bizbilling.OperationResult{}, bizbilling.ErrReconciliationRequired
	}
	sub, err := s.lifecycle.stores.Subscriptions.Get(ctx, subscription)
	if err != nil {
		return bizbilling.OperationResult{}, err
	}
	if sub.AppID != app {
		return bizbilling.OperationResult{}, apperrors.NotFound("subscription", subscription)
	}
	if sub.FinancialScope().Validate() != nil {
		return bizbilling.OperationResult{}, bizbilling.ErrLegacyReviewRequired
	}
	start, end = start.UTC(), end.UTC()
	id := bizbillingrepo.PeriodID(app, subscription, start)
	key := gatewayEffectID("period_ensure", id, end.Format(time.RFC3339Nano))
	if providerInvoice != "" {
		key = gatewayEffectID("period_provider", id, providerInvoice)
	}
	return s.lifecycle.operations.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandSubscriptionPeriod, Scope: sub.FinancialScope(), IdempotencyKey: key, ActorID: bizOperationActor(ctx), SubscriptionPeriod: &bizbilling.SubscriptionPeriod{SubscriptionID: subscription, Start: start, End: end, ProviderInvoiceID: providerInvoice}})
}

func (s *BizPeriodService) RunDue(ctx context.Context, limit int) (int, error) {
	return s.RunDueForApp(ctx, "", limit)
}

func (s *BizPeriodService) RunDueForApp(ctx context.Context, app string, limit int) (int, error) {
	if limit < 1 || limit > 100 {
		limit = 100
	}
	now := time.Now().UTC()
	count := 0
	var pending error
	for _, scan := range []struct {
		field    string
		statuses []string
	}{
		{"current_period_end", []string{"active", "past_due"}},
		{"trial_end", []string{"trialing"}},
	} {
		pit, err := s.pager.Open(ctx)
		if err != nil {
			return count, err
		}
		completed, scanErr := s.scanDue(ctx, app, scan.field, scan.statuses, pit, now, limit)
		count += completed
		if scanErr != nil {
			pending = scanErr
		}
		if ctx.Err() != nil {
			return count, ctx.Err()
		}
	}
	return count, pending
}

func (s *BizPeriodService) scanDue(ctx context.Context, app, field string, statuses []string, pit string, now time.Time, limit int) (int, error) {
	defer func() { s.pager.Close(pit) }()
	var after []json.RawMessage
	count, restarts := 0, 0
	var pending error
	for {
		if ctx.Err() != nil {
			return count, ctx.Err()
		}
		page, err := s.pager.Page(ctx, pit, app, field, statuses, now, after, limit)
		if stderrors.Is(err, bizbillingrepo.ErrSubscriptionPITExpired) && restarts < 3 {
			s.pager.Close(pit)
			pit, err = s.pager.Open(ctx)
			if err != nil {
				return count, err
			}
			after = nil
			restarts++
			continue
		}
		if err != nil {
			return count, err
		}
		pit = page.PITID
		restarts = 0
		if len(page.Rows) == 0 {
			return count, pending
		}
		after = page.After
		for _, hit := range page.Rows {
			processed, err := s.runDueSubscription(ctx, hit.AppID, hit.ID, field, now)
			count += processed
			if err != nil {
				pending = err
			}
			if ctx.Err() != nil {
				return count, ctx.Err()
			}
		}
	}
}

func (s *BizPeriodService) runDueSubscription(ctx context.Context, app, id, field string, now time.Time) (int, error) {
	state, err := s.lifecycle.gate.State(ctx, app)
	if err != nil {
		return 0, err
	}
	if state.LifecycleVersion == 1 {
		return 0, nil
	}
	if state.LifecycleVersion != 2 || state.Status != "active" {
		return 0, bizbilling.ErrReconciliationRequired
	}
	count := 0
	for {
		sub, err := s.lifecycle.GetSubscription(ctx, app, id)
		if err != nil {
			return count, err
		}
		start := sub.CurrentPeriodEnd
		if field == "trial_end" {
			if sub.Status != bizbilling.SubscriptionStatusTrialing || sub.TrialEnd == nil {
				return count, nil
			}
			start = *sub.TrialEnd
		} else if sub.Status != bizbilling.SubscriptionStatusActive && sub.Status != bizbilling.SubscriptionStatusPastDue {
			return count, nil
		}
		if start.IsZero() || start.After(now) {
			return count, nil
		}
		if sub.CancelAtPeriodEnd || sub.NonRenewing {
			_, err := s.lifecycle.CancelSubscription(WithBizOperationKey(ctx, gatewayEffectID("period_stop", sub.ID, start.Format(time.RFC3339Nano))), app, id, false)
			return count, err
		}
		if sub.EffectiveCollectionMode() == bizbilling.CollectionAutomatic {
			if s.recurring == nil {
				return count, bizbilling.ErrGatewayPending
			}
			processed, err := s.recurring.SyncProviderSubscription(ctx, app, id)
			return count + processed, err
		}
		price := sub.PriceSnapshot
		if sub.PendingChange != nil {
			price = &sub.PendingChange.Price
		}
		if price == nil {
			price, err = s.lifecycle.operations.subscriptionPrice(ctx, sub.FinancialScope(), sub, false)
			if err != nil {
				return count, err
			}
		}
		anchor, err := subscriptionAnchor(sub)
		if err != nil {
			return count, err
		}
		if field == "trial_end" {
			anchor = start.Day()
		}
		end, err := bizbilling.NextAnchoredPeriod(start, price.BillingCycle, price.CycleDays, anchor)
		if err != nil {
			return count, err
		}
		if _, err = s.EnsurePeriod(ctx, app, id, start, end); err != nil {
			return count, err
		}
		count++
		if field == "trial_end" {
			field = "current_period_end"
		}
	}
}
