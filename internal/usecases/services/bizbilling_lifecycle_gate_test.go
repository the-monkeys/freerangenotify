package services

import (
	"context"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"go.uber.org/zap"
	"sync"
	"testing"
)

type gateMemoryStore struct {
	mu     sync.Mutex
	states map[string]bizbilling.LifecycleState
}

func TestBizLegacyMoneyWriterRespectsCutover(t *testing.T) {
	store := &gateMemoryStore{states: map[string]bizbilling.LifecycleState{"app-a": {AppID: "app-a", Status: "preparing", LifecycleVersion: 1}}}
	gate := NewBizLifecycleGate(store)
	lifecycle := &BizLifecycleService{logger: zap.NewNop()}
	lifecycle.SetLifecycleGate(gate)
	credit := &BizCreditService{logger: zap.NewNop()}
	credit.SetLifecycleGate(gate)
	require.NotPanics(t, func() {
		_, err := lifecycle.RecordPayment(context.Background(), "app-a", "invoice", &bizbilling.RecordPaymentRequest{Method: "cash"})
		require.ErrorIs(t, err, bizbilling.ErrActiveLegacyWriter)
	})
	require.NotPanics(t, func() {
		_, err := credit.AdjustBalance(context.Background(), "app-a", "user", 1)
		require.ErrorIs(t, err, bizbilling.ErrActiveLegacyWriter)
	})
}

func (m *gateMemoryStore) Load(_ context.Context, app string) (bizbilling.LifecycleState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.states[app], nil
}
func (m *gateMemoryStore) Mutate(_ context.Context, app string, fn func(*bizbilling.LifecycleState) error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.states[app]
	if err := fn(&s); err != nil {
		return err
	}
	m.states[app] = s
	return nil
}
func TestBizLifecycleMissingStateUsesLegacy(t *testing.T) {
	g := NewBizLifecycleGate(&gateMemoryStore{states: map[string]bizbilling.LifecycleState{}})
	state, err := g.State(context.Background(), "app-a")
	require.NoError(t, err)
	require.Equal(t, 1, state.LifecycleVersion)
	require.Equal(t, "legacy", state.Status)
	p, err := g.AcquireMutation(context.Background(), "app-a")
	require.NoError(t, err)
	require.NoError(t, g.CheckMutation(context.Background(), p))
	require.NoError(t, g.ReleaseMutation(context.Background(), p))
	require.Error(t, g.CheckMutation(context.Background(), p))
}
func TestBizLifecyclePreparingRejectsNewWriters(t *testing.T) {
	store := &gateMemoryStore{states: map[string]bizbilling.LifecycleState{"app-a": {AppID: "app-a", Status: "preparing", LifecycleVersion: 1}}}
	g := NewBizLifecycleGate(store)
	_, err := g.AcquireMutation(context.Background(), "app-a")
	require.ErrorIs(t, err, bizbilling.ErrActiveLegacyWriter)
	_, err = g.AcquireMutation(context.Background(), "app-b")
	require.NoError(t, err)
}
func TestBizLifecycleRejectsMismatchedOrFutureState(t *testing.T) {
	for _, state := range []bizbilling.LifecycleState{{AppID: "other", Status: "legacy", LifecycleVersion: 1}, {AppID: "app-a", Status: "active", LifecycleVersion: 3}} {
		store := &gateMemoryStore{states: map[string]bizbilling.LifecycleState{"app-a": state}}
		_, err := NewBizLifecycleGate(store).AcquireMutation(context.Background(), "app-a")
		require.ErrorIs(t, err, bizbilling.ErrReconciliationRequired)
	}
}
func TestBizLifecycleExpiredPermitCannotBeAssumedDrained(t *testing.T) {
	store := &gateMemoryStore{states: map[string]bizbilling.LifecycleState{}}
	g := NewBizLifecycleGate(store)
	ctx := context.Background()
	p, err := g.AcquireMutation(ctx, "app-a")
	require.NoError(t, err)
	require.NoError(t, store.Mutate(ctx, "app-a", func(s *bizbilling.LifecycleState) error { s.Epoch++; return nil }))
	require.ErrorIs(t, g.CheckMutation(ctx, p), bizbilling.ErrStaleMutationPermit)
	require.Len(t, store.states["app-a"].MutationPermits, 1)
}
