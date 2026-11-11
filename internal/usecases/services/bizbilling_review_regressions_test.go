package services

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"go.uber.org/zap"
	"testing"
	"time"
)

func TestBizReviewedLegacyExpenseConversion(t *testing.T) {
	service, ops, es, scope := migrationFixture(t)
	ctx := context.Background()
	expense := &bizbilling.Expense{ID: uuid.NewString(), AppID: scope.AppID, BilledToUser: scope.UserID, Billable: true, AmountPaisa: 1000, Date: time.Now().UTC()}
	require.NoError(t, bizbillingrepo.NewStore[bizbilling.Expense](es, bizbillingrepo.ExpenseIndex, zap.NewNop()).Create(ctx, expense.ID, expense))
	preview, err := service.Prepare(ctx, scope.AppID, MigrationOptions{DryRun: true, PrimaryCurrency: scope.Currency})
	require.NoError(t, err)
	reviewed, err := service.Prepare(ctx, scope.AppID, MigrationOptions{PrimaryCurrency: scope.Currency, ExpectedReportID: preview.ID, WritersUpgraded: true})
	require.NoError(t, err)
	require.Len(t, reviewed.Issues, 1)
	issue := reviewed.Issues[0]
	require.NoError(t, service.Resolve(ctx, scope.AppID, reviewed.ID, LegacyResolution{IssueID: issue.ID, ExpectedSourceHash: issue.SourceHash, Currency: scope.Currency, ActorID: "reviewer", Reason: "Original expense confirmed"}))
	ready, err := service.Prepare(ctx, scope.AppID, MigrationOptions{PrimaryCurrency: scope.Currency, ExpectedReportID: preview.ID, WritersUpgraded: true})
	require.NoError(t, err)
	require.NoError(t, service.Activate(ctx, scope.AppID, ready.ID))
	lifecycle := &BizLifecycleService{repo: bizbillingrepo.NewLifecycleRepo(es, zap.NewNop()), stores: ops.stores, configSvc: ops.config, logger: zap.NewNop()}
	lifecycle.SetOperationService(ops)
	lifecycle.SetLifecycleGate(ops.gate)
	converted, err := NewBizExpenseService(ops.stores, lifecycle, zap.NewNop()).ConvertToInvoice(ctx, scope.AppID, scope.UserID, []string{expense.ID})
	require.NoError(t, err)
	require.NotEmpty(t, converted.ID)
}

type writerRecovery interface {
	RecoverWriter(context.Context, string, string, bizbilling.WriterPermitResolution) (string, error)
}

func TestBizAbandonedWriterRequiresFencedReview(t *testing.T) {
	service, ops, _, scope := migrationFixture(t)
	ctx := context.Background()
	require.NoError(t, ops.gate.store.Mutate(ctx, scope.AppID, func(s *bizbilling.LifecycleState) error {
		s.LifecycleVersion = 2
		s.Status = "active"
		s.PrimaryCurrency = scope.Currency
		return nil
	}))
	permit, err := ops.gate.AcquireMutation(ctx, scope.AppID)
	require.NoError(t, err)
	require.NoError(t, ops.gate.store.Mutate(ctx, scope.AppID, func(s *bizbilling.LifecycleState) error {
		p := s.MutationPermits[permit.WriterID]
		p.ExpiresAt = time.Now().Add(-time.Hour)
		s.MutationPermits[p.WriterID] = p
		return nil
	}))
	_, err = beginFinancialReport(ctx, ops.gate, scope.AppID)
	require.ErrorIs(t, err, bizbilling.ErrConflict)
	recovery, ok := any(service).(writerRecovery)
	require.True(t, ok, "abandoned permits need an explicit durable recovery path")
	preview, err := service.Prepare(ctx, scope.AppID, MigrationOptions{DryRun: true, PrimaryCurrency: scope.Currency})
	require.NoError(t, err)
	decision := bizbilling.WriterPermitResolution{WriterID: permit.WriterID, ExpectedEpoch: permit.Epoch, PrimaryCurrency: scope.Currency, ActorID: "operator", Reason: "Origin stopped and outstanding storage requests drained"}
	_, err = recovery.RecoverWriter(ctx, scope.AppID, preview.ID, decision)
	require.Error(t, err)
	state, err := ops.gate.State(ctx, scope.AppID)
	require.NoError(t, err)
	require.Contains(t, state.MutationPermits, permit.WriterID)
	decision.ConfirmedDrained = true
	id, err := recovery.RecoverWriter(ctx, scope.AppID, preview.ID, decision)
	require.NoError(t, err)
	require.NotEmpty(t, id)
	replay, err := recovery.RecoverWriter(ctx, scope.AppID, preview.ID, decision)
	require.NoError(t, err)
	require.Equal(t, id, replay)
	_, err = beginFinancialReport(ctx, ops.gate, scope.AppID)
	require.NoError(t, err)
	require.ErrorIs(t, ops.gate.CheckMutation(ctx, permit), bizbilling.ErrStaleMutationPermit)
	after, err := service.Prepare(ctx, scope.AppID, MigrationOptions{DryRun: true, PrimaryCurrency: scope.Currency})
	require.NoError(t, err)
	require.Equal(t, preview.SourceHashes, after.SourceHashes)
}
func TestBizAbandonedWriterRejectsStaleSourceReview(t *testing.T) {
	service, ops, es, scope := migrationFixture(t)
	ctx := context.Background()
	permit, err := ops.gate.AcquireMutation(ctx, scope.AppID)
	require.NoError(t, err)
	recovery, ok := any(service).(writerRecovery)
	require.True(t, ok)
	preview, err := service.Prepare(ctx, scope.AppID, MigrationOptions{DryRun: true, PrimaryCurrency: scope.Currency})
	require.NoError(t, err)
	_, err = bizbillingrepo.NewStore[user.User](es, "users", zap.NewNop()).Update(ctx, scope.UserID, func(u *user.User) error { u.FullName = "Changed after review"; return nil })
	require.NoError(t, err)
	_, err = recovery.RecoverWriter(ctx, scope.AppID, preview.ID, bizbilling.WriterPermitResolution{WriterID: permit.WriterID, ExpectedEpoch: permit.Epoch, PrimaryCurrency: scope.Currency, ActorID: "operator", Reason: "Origin drained", ConfirmedDrained: true})
	require.Error(t, err)
	state, err := ops.gate.State(ctx, scope.AppID)
	require.NoError(t, err)
	require.Contains(t, state.MutationPermits, permit.WriterID)
	require.Equal(t, int64(0), state.Epoch)
}

func TestBizAbandonedWriterBatchAndPendingOperations(t *testing.T) {
	service, ops, _, scope := migrationFixture(t)
	ctx := context.Background()
	require.NoError(t, ops.gate.store.Mutate(ctx, scope.AppID, func(s *bizbilling.LifecycleState) error {
		s.LifecycleVersion = 2
		s.Status = "active"
		s.PrimaryCurrency = scope.Currency
		return nil
	}))
	first, err := ops.gate.AcquireMutation(ctx, scope.AppID)
	require.NoError(t, err)
	second, err := ops.gate.AcquireMutation(ctx, scope.AppID)
	require.NoError(t, err)
	preview, err := service.Prepare(ctx, scope.AppID, MigrationOptions{DryRun: true, PrimaryCurrency: scope.Currency})
	require.NoError(t, err)
	decision := bizbilling.WriterPermitResolution{WriterID: first.WriterID, ExpectedEpoch: first.Epoch, PrimaryCurrency: scope.Currency, ActorID: "operator", Reason: "All selected origins and store requests drained", ConfirmedDrained: true}
	_, err = service.RecoverWriter(ctx, scope.AppID, preview.ID, decision)
	require.ErrorIs(t, err, bizbilling.ErrActiveLegacyWriter)
	decision.WriterID = ""
	decision.WriterIDs = []string{second.WriterID, first.WriterID}
	pending, _, err := ops.operations.Begin(ctx, bizbilling.Command{Kind: bizbilling.CommandBalanceAdjustment, Scope: scope, IdempotencyKey: "must-recover-first", BalanceAdjustment: &bizbilling.BalanceAdjustment{DeltaPaisa: 1, Reason: "Accepted operation"}})
	require.NoError(t, err)
	_, err = service.RecoverWriter(ctx, scope.AppID, preview.ID, decision)
	require.ErrorIs(t, err, bizbilling.ErrReconciliationRequired)
	state, err := ops.gate.State(ctx, scope.AppID)
	require.NoError(t, err)
	require.Len(t, state.MutationPermits, 2)
	require.Equal(t, "active", state.Status)
	// This saved intent never started a stream; definitive rejection is safe.
	require.NoError(t, ops.operations.Reject(ctx, pending.ID, "invalid_financial_intent"))
	require.NoError(t, ops.operations.FinishRejected(ctx, pending.ID))
	_, err = service.RecoverWriter(ctx, scope.AppID, preview.ID, decision)
	require.NoError(t, err)
	state, err = ops.gate.State(ctx, scope.AppID)
	require.NoError(t, err)
	require.Empty(t, state.MutationPermits)
	require.Equal(t, int64(1), state.Epoch)
}
