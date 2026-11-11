package services

import (
	"context"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"go.uber.org/zap"
	"time"
)

// BizLifecycleGate prevents app cutover from overtaking in-flight writers.
// An expired permit is evidence requiring review, never proof of completion.
type BizLifecycleGate struct {
	store bizbilling.LifecycleStateStore
}

func NewBizLifecycleGate(store bizbilling.LifecycleStateStore) *BizLifecycleGate {
	return &BizLifecycleGate{store: store}
}
func normalizeLifecycle(s *bizbilling.LifecycleState, app string) {
	if s.AppID == "" {
		s.AppID = app
	}
	if s.LifecycleVersion == 0 {
		s.LifecycleVersion = 1
	}
	if s.Status == "" {
		s.Status = "legacy"
	}
	if s.MutationPermits == nil {
		s.MutationPermits = map[string]bizbilling.MutationPermit{}
	}
}
func (g *BizLifecycleGate) State(ctx context.Context, app string) (bizbilling.LifecycleState, error) {
	s, err := g.store.Load(ctx, app)
	if err != nil {
		return s, err
	}
	if s.AppID != "" && s.AppID != app {
		return s, bizbilling.ErrReconciliationRequired
	}
	normalizeLifecycle(&s, app)
	if recoverExpiredMerchantSwitch(&s) {
		if err := g.store.Mutate(ctx, app, func(current *bizbilling.LifecycleState) error {
			recoverExpiredMerchantSwitch(current)
			return nil
		}); err != nil {
			return s, err
		}
		return g.State(ctx, app)
	}
	return s, nil
}
func (g *BizLifecycleGate) AcquireMutation(ctx context.Context, app string) (bizbilling.MutationPermit, error) {
	p := bizbilling.MutationPermit{AppID: app, WriterID: uuid.NewString(), StartedAt: time.Now().UTC()}
	p.ExpiresAt = p.StartedAt.Add(30 * time.Second)
	if app == "" {
		return p, bizbilling.ErrStaleMutationPermit
	}
	err := g.store.Mutate(ctx, app, func(s *bizbilling.LifecycleState) error {
		normalizeLifecycle(s, app)
		recoverExpiredMerchantSwitch(s)
		if s.AppID != app || (s.LifecycleVersion != 1 && s.LifecycleVersion != 2) {
			return bizbilling.ErrReconciliationRequired
		}
		if s.Status != "legacy" && s.Status != "active" {
			return bizbilling.ErrActiveLegacyWriter
		}
		p.Epoch = s.Epoch
		s.MutationPermits[p.WriterID] = p
		return nil
	})
	return p, err
}
func (g *BizLifecycleGate) CheckMutation(ctx context.Context, p bizbilling.MutationPermit) error {
	s, err := g.State(ctx, p.AppID)
	if err != nil {
		return err
	}
	stored, ok := s.MutationPermits[p.WriterID]
	if !ok || stored.AppID != p.AppID || stored.Epoch != p.Epoch || s.Epoch != p.Epoch {
		return bizbilling.ErrStaleMutationPermit
	}
	return nil
}
func (g *BizLifecycleGate) ReleaseMutation(ctx context.Context, p bizbilling.MutationPermit) error {
	return g.store.Mutate(ctx, p.AppID, func(s *bizbilling.LifecycleState) error {
		stored, ok := s.MutationPermits[p.WriterID]
		if !ok {
			return nil
		}
		if stored.Epoch != p.Epoch || s.Epoch != p.Epoch {
			return bizbilling.ErrStaleMutationPermit
		}
		if s.LifecycleVersion == 2 {
			revision, err := bizbilling.CheckedAdd(s.WriteRevision, 1)
			if err != nil {
				return err
			}
			s.WriteRevision = revision
		}
		delete(s.MutationPermits, p.WriterID)
		return nil
	})
}

type billingMutationContextKey struct{}

// Nested billing facade calls share one durable permit and one release.
func BeginBizMutation(ctx context.Context, gate *BizLifecycleGate, app string) (context.Context, func() error, error) {
	noop := func() error { return nil }
	if gate == nil {
		return ctx, noop, nil
	}
	if handled, err := gate.migrationAllowed(ctx, app); handled {
		return ctx, noop, err
	}
	if p, ok := ctx.Value(billingMutationContextKey{}).(bizbilling.MutationPermit); ok {
		if p.AppID != app {
			return ctx, noop, bizbilling.ErrStaleMutationPermit
		}
		return ctx, noop, gate.CheckMutation(ctx, p)
	}
	p, err := gate.AcquireMutation(ctx, app)
	if err != nil {
		return ctx, noop, err
	}
	return context.WithValue(ctx, billingMutationContextKey{}, p), func() error {
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		return gate.ReleaseMutation(releaseCtx, p)
	}, nil
}

func releaseBizMutation(logger *zap.Logger, done func() error) {
	if err := done(); err != nil && logger != nil {
		logger.Warn("billing writer release requires reconciliation", zap.Error(err))
	}
}
