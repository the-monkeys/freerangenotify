package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
)

type merchantSwitchKey struct{}
type merchantSwitchToken struct {
	app, owner string
	mode       bizbilling.GatewayMode
	revision   int64
}

// This lease guards metadata publication only. No financial effect is admitted
// under it, and an old publisher must still present its current owner token.
// Financial writer permits are deliberately never expired by this recovery.
func recoverExpiredMerchantSwitch(state *bizbilling.LifecycleState) bool {
	fence := state.MerchantSwitch
	if state.Status != "merchant_switching" || fence == nil || fence.LeaseUntil.After(time.Now().UTC()) || len(state.MutationPermits) != 0 {
		return false
	}
	if (state.LifecycleVersion != 1 || fence.PriorStatus != "legacy") && (state.LifecycleVersion != 2 || fence.PriorStatus != "active") {
		return false
	}
	state.Status, state.MerchantSwitch = fence.PriorStatus, nil
	return true
}

// Merchant changes drain registered writers. This metadata-only fence grants no
// bypass of financial admission; accepted orders retain their immutable refs.
func (g *BizLifecycleGate) beginMerchantSwitch(ctx context.Context, app string, mode bizbilling.GatewayMode, revision int64) (context.Context, func(), error) {
	token := merchantSwitchToken{app: app, owner: uuid.NewString(), mode: mode, revision: revision}
	noop := func() {}
	err := g.store.Mutate(ctx, app, func(state *bizbilling.LifecycleState) error {
		normalizeLifecycle(state, app)
		if state.AppID != app || state.MerchantSlotRevisions[mode] != revision {
			return bizbilling.ErrConflict
		}
		if len(state.MutationPermits) != 0 {
			return bizbilling.ErrActiveLegacyWriter
		}
		prior := state.Status
		if prior == "merchant_switching" {
			fence := state.MerchantSwitch
			if fence == nil || fence.Mode != mode || fence.ExpectedRevision != revision || fence.LeaseUntil.After(time.Now().UTC()) {
				return bizbilling.ErrConflict
			}
			prior = fence.PriorStatus
		}
		if (state.LifecycleVersion != 1 || prior != "legacy") && (state.LifecycleVersion != 2 || prior != "active") {
			return bizbilling.ErrActiveLegacyWriter
		}
		state.Status = "merchant_switching"
		state.MerchantSwitch = &bizbilling.MerchantSwitchFence{Owner: token.owner, Mode: mode, ExpectedRevision: revision, PriorStatus: prior, LeaseUntil: time.Now().UTC().Add(2 * time.Minute)}
		return nil
	})
	if err != nil {
		return ctx, noop, err
	}
	ctx = context.WithValue(ctx, merchantSwitchKey{}, token)
	release := func() {
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		_ = g.store.Mutate(releaseCtx, app, func(state *bizbilling.LifecycleState) error {
			if state.Status == "merchant_switching" && state.MerchantSwitch != nil && state.MerchantSwitch.Owner == token.owner {
				state.Status, state.MerchantSwitch = state.MerchantSwitch.PriorStatus, nil
			}
			return nil
		})
	}
	return ctx, release, nil
}

func merchantSwitchMatches(ctx context.Context, state *bizbilling.LifecycleState) bool {
	token, ok := ctx.Value(merchantSwitchKey{}).(merchantSwitchToken)
	return ok && state.AppID == token.app && state.Status == "merchant_switching" && state.MerchantSwitch != nil && state.MerchantSwitch.Owner == token.owner && state.MerchantSwitch.Mode == token.mode && state.MerchantSwitch.ExpectedRevision == token.revision && state.MerchantSlotRevisions[token.mode] == token.revision && state.MerchantSwitch.LeaseUntil.After(time.Now().UTC())
}

func (g *BizLifecycleGate) renewMerchantSwitch(ctx context.Context, app string) error {
	return g.store.Mutate(ctx, app, func(state *bizbilling.LifecycleState) error {
		if !merchantSwitchMatches(ctx, state) {
			return bizbilling.ErrStaleMutationPermit
		}
		state.MerchantSwitch.LeaseUntil = time.Now().UTC().Add(2 * time.Minute)
		return nil
	})
}

func (g *BizLifecycleGate) publishMerchantSwitch(ctx context.Context, app, connectionID string) error {
	return g.store.Mutate(ctx, app, func(state *bizbilling.LifecycleState) error {
		if !merchantSwitchMatches(ctx, state) || len(state.MutationPermits) != 0 {
			return bizbilling.ErrStaleMutationPermit
		}
		fence := state.MerchantSwitch
		revision, err := bizbilling.CheckedAdd(fence.ExpectedRevision, 1)
		if err != nil {
			return err
		}
		epoch, err := bizbilling.CheckedAdd(state.Epoch, 1)
		if err != nil {
			return err
		}
		if state.LifecycleVersion == 2 {
			updated, err := bizbilling.CheckedAdd(state.WriteRevision, 1)
			if err != nil {
				return err
			}
			state.WriteRevision = updated
		}
		if state.MerchantSlots == nil {
			state.MerchantSlots = map[bizbilling.GatewayMode]string{}
		}
		if state.MerchantSlotRevisions == nil {
			state.MerchantSlotRevisions = map[bizbilling.GatewayMode]int64{}
		}
		if connectionID == "" {
			delete(state.MerchantSlots, fence.Mode)
		} else {
			state.MerchantSlots[fence.Mode] = connectionID
		}
		state.MerchantSlotRevisions[fence.Mode] = revision
		state.Epoch, state.Status, state.MerchantSwitch = epoch, fence.PriorStatus, nil
		return nil
	})
}
