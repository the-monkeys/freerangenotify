package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
)

type migrationContextKey struct{}
type migrationPermit struct{ AppID, ReportID, Owner string }

func (g *BizLifecycleGate) migrationAllowed(ctx context.Context, appID string) (bool, error) {
	p, ok := ctx.Value(migrationContextKey{}).(migrationPermit)
	if !ok {
		return false, nil
	}
	if p.AppID != appID {
		return true, bizbilling.ErrStaleMutationPermit
	}
	s, err := g.State(ctx, appID)
	if err != nil {
		return true, err
	}
	if s.Status != "preparing" || s.LifecycleVersion != 1 || s.MigrationOwner != p.Owner || s.MigrationReportID != p.ReportID || !s.MigrationLeaseUntil.After(time.Now().UTC()) || len(s.MutationPermits) != 0 {
		return true, bizbilling.ErrStaleMutationPermit
	}
	return true, nil
}

func (g *BizLifecycleGate) beginMigration(ctx context.Context, appID, reportID string) (context.Context, func(), error) {
	p := migrationPermit{AppID: appID, ReportID: reportID, Owner: uuid.NewString()}
	noop := func() {}
	err := g.store.Mutate(ctx, appID, func(s *bizbilling.LifecycleState) error {
		normalizeLifecycle(s, appID)
		if s.AppID != appID || s.LifecycleVersion != 1 || (s.Status != "legacy" && s.Status != "preparing") {
			return bizbilling.ErrConflict
		}
		if s.Status == "preparing" && (s.MigrationReportID != reportID || (s.MigrationOwner != "" && s.MigrationLeaseUntil.After(time.Now().UTC()))) {
			return bizbilling.ErrConflict
		}
		s.Status = "preparing"
		s.MigrationReportID = reportID
		s.MigrationOwner = p.Owner
		s.MigrationLeaseUntil = time.Now().UTC().Add(2 * time.Minute)
		return nil
	})
	if err != nil {
		return ctx, noop, err
	}
	ctx = context.WithValue(ctx, migrationContextKey{}, p)
	release := func() {
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		_ = g.store.Mutate(releaseCtx, appID, func(s *bizbilling.LifecycleState) error {
			if s.Status != "preparing" || s.MigrationReportID != reportID || s.MigrationOwner != p.Owner {
				return nil
			}
			s.MigrationOwner = ""
			s.MigrationLeaseUntil = time.Time{}
			if !s.MigrationFactsStarted {
				s.Status = "legacy"
				s.MigrationReportID = ""
			}
			return nil
		})
	}
	state, err := g.State(ctx, appID)
	if err != nil {
		release()
		return ctx, noop, err
	}
	if len(state.MutationPermits) != 0 {
		release()
		return ctx, noop, bizbilling.ErrActiveLegacyWriter
	}
	return ctx, release, nil
}
func (g *BizLifecycleGate) renewMigration(ctx context.Context, appID string) error {
	p, ok := ctx.Value(migrationContextKey{}).(migrationPermit)
	if !ok || p.AppID != appID {
		return bizbilling.ErrStaleMutationPermit
	}
	return g.store.Mutate(ctx, appID, func(s *bizbilling.LifecycleState) error {
		if s.Status != "preparing" || s.MigrationOwner != p.Owner || s.MigrationReportID != p.ReportID || !s.MigrationLeaseUntil.After(time.Now().UTC()) || len(s.MutationPermits) != 0 {
			return bizbilling.ErrStaleMutationPermit
		}
		s.MigrationLeaseUntil = time.Now().UTC().Add(2 * time.Minute)
		return nil
	})
}
func (g *BizLifecycleGate) startMigrationFacts(ctx context.Context, appID string) error {
	if _, err := g.migrationAllowed(ctx, appID); err != nil {
		return err
	}
	p, ok := ctx.Value(migrationContextKey{}).(migrationPermit)
	if !ok {
		return bizbilling.ErrStaleMutationPermit
	}
	return g.store.Mutate(ctx, appID, func(s *bizbilling.LifecycleState) error {
		if s.MigrationOwner != p.Owner || s.MigrationReportID != p.ReportID || s.Status != "preparing" || len(s.MutationPermits) != 0 {
			return bizbilling.ErrStaleMutationPermit
		}
		s.MigrationFactsStarted = true
		return nil
	})
}
func (g *BizLifecycleGate) completeMigration(ctx context.Context, appID, currency string) error {
	p, ok := ctx.Value(migrationContextKey{}).(migrationPermit)
	if !ok {
		return bizbilling.ErrStaleMutationPermit
	}
	return g.store.Mutate(ctx, appID, func(s *bizbilling.LifecycleState) error {
		if s.Status != "preparing" || s.MigrationOwner != p.Owner || s.MigrationReportID != p.ReportID || !s.MigrationFactsStarted || !s.MigrationLeaseUntil.After(time.Now().UTC()) || len(s.MutationPermits) != 0 {
			return bizbilling.ErrStaleMutationPermit
		}
		epoch, err := bizbilling.CheckedAdd(s.Epoch, 1)
		if err != nil {
			return err
		}
		s.Epoch = epoch
		s.LifecycleVersion = 2
		s.Status = "active"
		s.PrimaryCurrency = currency
		s.MigrationOwner = ""
		s.MigrationLeaseUntil = time.Time{}
		return nil
	})
}
