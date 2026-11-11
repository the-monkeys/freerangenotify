package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
)

type writerReviewContextKey struct{}
type writerReviewToken struct{ appID, id, owner string }

func sameReviewedPermits(state *bizbilling.LifecycleState, record *bizbilling.WriterPermitReviewRecord) bool {
	if state.Epoch != record.Resolution.ExpectedEpoch || len(state.MutationPermits) != len(record.Permits) {
		return false
	}
	for _, expected := range record.Permits {
		current, ok := state.MutationPermits[expected.WriterID]
		if !ok || bizbillingrepo.MigrationDigest(current) != bizbillingrepo.MigrationDigest(expected) {
			return false
		}
	}
	return true
}

func (g *BizLifecycleGate) beginWriterReview(ctx context.Context, record *bizbilling.WriterPermitReviewRecord) (context.Context, func(), error) {
	token := writerReviewToken{appID: record.AppID, id: record.ID, owner: uuid.NewString()}
	noop := func() {}
	err := g.store.Mutate(ctx, record.AppID, func(state *bizbilling.LifecycleState) error {
		normalizeLifecycle(state, record.AppID)
		if state.AppID != record.AppID || !sameReviewedPermits(state, record) {
			return bizbilling.ErrActiveLegacyWriter
		}
		prior := state.Status
		if prior == "recovering" {
			if state.PermitReview == nil || state.PermitReview.ID != record.ID || (state.PermitReview.Owner != "" && state.PermitReview.LeaseUntil.After(time.Now().UTC())) {
				return bizbilling.ErrConflict
			}
			prior = state.PermitReview.PriorStatus
		}
		if (state.LifecycleVersion != 1 || prior != "legacy") && (state.LifecycleVersion != 2 || prior != "active") {
			return bizbilling.ErrActiveLegacyWriter
		}
		state.Status = "recovering"
		state.PermitReview = &bizbilling.WriterPermitReviewFence{ID: record.ID, Owner: token.owner, PriorStatus: prior, LeaseUntil: time.Now().UTC().Add(2 * time.Minute)}
		return nil
	})
	if err != nil {
		return ctx, noop, err
	}
	ctx = context.WithValue(ctx, writerReviewContextKey{}, token)
	release := func() {
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		_ = g.store.Mutate(releaseCtx, record.AppID, func(state *bizbilling.LifecycleState) error {
			if state.Status == "recovering" && state.PermitReview != nil && state.PermitReview.ID == token.id && state.PermitReview.Owner == token.owner {
				state.Status = state.PermitReview.PriorStatus
				state.PermitReview = nil
			}
			return nil
		})
	}
	return ctx, release, nil
}

func writerReviewMatches(ctx context.Context, state *bizbilling.LifecycleState) bool {
	token, ok := ctx.Value(writerReviewContextKey{}).(writerReviewToken)
	return ok && token.appID == state.AppID && state.Status == "recovering" && state.PermitReview != nil && state.PermitReview.ID == token.id && state.PermitReview.Owner == token.owner && state.PermitReview.LeaseUntil.After(time.Now().UTC())
}
func (g *BizLifecycleGate) renewWriterReview(ctx context.Context, appID string) error {
	return g.store.Mutate(ctx, appID, func(state *bizbilling.LifecycleState) error {
		if !writerReviewMatches(ctx, state) {
			return bizbilling.ErrStaleMutationPermit
		}
		state.PermitReview.LeaseUntil = time.Now().UTC().Add(2 * time.Minute)
		return nil
	})
}
func (g *BizLifecycleGate) completeWriterReview(ctx context.Context, record *bizbilling.WriterPermitReviewRecord) error {
	return g.store.Mutate(ctx, record.AppID, func(state *bizbilling.LifecycleState) error {
		if !writerReviewMatches(ctx, state) || !sameReviewedPermits(state, record) {
			return bizbilling.ErrStaleMutationPermit
		}
		epoch, err := bizbilling.CheckedAdd(state.Epoch, 1)
		if err != nil {
			return err
		}
		if state.LifecycleVersion == 2 {
			revision, err := bizbilling.CheckedAdd(state.WriteRevision, 1)
			if err != nil {
				return err
			}
			state.WriteRevision = revision
		}
		for _, permit := range record.Permits {
			delete(state.MutationPermits, permit.WriterID)
		}
		state.Epoch = epoch
		state.Status = state.PermitReview.PriorStatus
		state.PermitReview = nil
		state.LastWriterReviewID = record.ID
		return nil
	})
}
