package services

import (
	"context"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"go.uber.org/zap"
)

func (s *BizRecurringService) Reconcile(ctx context.Context, limit int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit < 1 || limit > 100 {
		limit = 100
	}
	if err := s.effects.Refresh(ctx); err != nil {
		return 0, err
	}
	query := bizbillingrepo.NewQuery("").Terms("kind", []string{"subscription_authorize", "subscription_cancel", "subscription_native_pause", "subscription_native_resume"}).Terms("state", []string{"pending", "provider_created", "provider_confirmed", "authorized", "active"})
	body := query.Body("id", limit, 0)
	body["sort"] = []map[string]any{{"id": "asc"}}
	if s.cursor != "" {
		body["search_after"] = []any{s.cursor}
	}
	rows, _, err := s.effects.Find(ctx, body)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		s.cursor = ""
		return 0, nil
	}
	count := 0
	var pending error
	for _, effect := range rows {
		if ctx.Err() != nil {
			return count, ctx.Err()
		}
		s.cursor = effect.ID
		run, done, err := BeginBizMutation(ctx, s.lifecycle.gate, effect.Scope.AppID)
		if err != nil {
			pending = bizbilling.ErrGatewayPending
			continue
		}
		sub, err := s.lifecycle.GetSubscription(run, effect.Scope.AppID, effect.SubscriptionID)
		if err == nil {
			if effect.Kind == "subscription_native_pause" || effect.Kind == "subscription_native_resume" {
				_, err = s.resumeNative(run, sub, effect)
			} else if effect.StopRequested && effect.Kind == "subscription_authorize" {
				_, err = s.stopAuthorization(run, sub, effect, "recover-stop-"+effect.ID)
			} else if effect.Kind == "subscription_cancel" {
				_, err = s.resumeCancelEffect(run, sub, effect)
			} else {
				_, err = s.resumeAuthorization(run, sub, effect)
				if err == nil && (effect.State == "authorized" || effect.State == "active") {
					_, err = s.syncProviderEffect(run, sub, effect)
				}
			}
		}
		releaseBizMutation(s.logger, done)
		if err != nil {
			pending = bizbilling.ErrGatewayPending
		} else {
			count++
		}
	}
	return count, pending
}

func (s *BizRecurringService) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				run, cancel := context.WithTimeout(ctx, 25*time.Second)
				_, err := s.Reconcile(run, 100)
				cancel()
				if err != nil && ctx.Err() == nil {
					s.logger.Warn("Billing authorization recovery pending", zap.String("failure_code", "subscription_recovery_pending"))
				}
			}
		}
	}()
}
