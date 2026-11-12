package services

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"go.uber.org/zap"
)

func TestBizMigrationSubscriptionReviewPreservesDatesAndIDs(t *testing.T) {
	service, ops, es, scope := migrationFixture(t)
	ctx := context.Background()
	plan := &bizbilling.Plan{ID: uuid.NewString(), AppID: scope.AppID, Name: "Legacy monthly", Currency: scope.Currency, AmountPaisa: 1000, BillingCycle: bizbilling.BillingCycleMonthly}
	require.NoError(t, ops.stores.Plans.Create(ctx, plan.ID, plan))
	end := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	start := end.AddDate(0, -1, 0)
	sub := &bizbilling.BizSubscription{ID: uuid.NewString(), AppID: scope.AppID, UserID: scope.UserID, PlanID: plan.ID, Status: bizbilling.SubscriptionStatusActive, Quantity: 1, CurrentPeriodStart: start, CurrentPeriodEnd: end, AnchorDay: end.Day(), CreatedAt: start, UpdatedAt: start}
	raw := bizbillingrepo.NewStore[bizbilling.BizSubscription](es, "frn_biz_subscriptions", zap.NewNop())
	require.NoError(t, raw.Create(ctx, sub.ID, sub))
	dry, err := service.Prepare(ctx, scope.AppID, MigrationOptions{DryRun: true, PrimaryCurrency: scope.Currency})
	require.NoError(t, err)
	require.False(t, dry.Ready)
	reviewed, err := service.Prepare(ctx, scope.AppID, MigrationOptions{ExpectedReportID: dry.ID, PrimaryCurrency: scope.Currency, WritersUpgraded: true})
	require.NoError(t, err)
	for _, issue := range reviewed.Issues {
		require.Equal(t, "currency_required", issue.Code, "supported manual subscriptions must be reviewable")
		require.NoError(t, service.Resolve(ctx, scope.AppID, reviewed.ID, LegacyResolution{IssueID: issue.ID, ExpectedSourceHash: issue.SourceHash, Currency: scope.Currency, ActorID: "reviewer", Reason: "Original customer agreement confirms INR"}))
	}
	ready, err := service.Prepare(ctx, scope.AppID, MigrationOptions{ExpectedReportID: dry.ID, PrimaryCurrency: scope.Currency, WritersUpgraded: true})
	require.NoError(t, err)
	require.True(t, ready.Ready)
	require.NoError(t, service.Activate(ctx, scope.AppID, ready.ID))
	original, err := raw.Get(ctx, sub.ID)
	require.NoError(t, err)
	require.Equal(t, sub, original, "activating a reviewed app must not rewrite the subscription")
	normalized, err := ops.stores.Subscriptions.Get(ctx, sub.ID)
	require.NoError(t, err)
	require.Equal(t, scope.Currency, normalized.Currency)
	ensureBillingIndex(t, es, bizbillingrepo.BillingPeriodIndex, (&database.IndexTemplates{}).GetBizBillingPeriodsTemplate())
	lifecycle := &BizLifecycleService{repo: bizbillingrepo.NewLifecycleRepo(es, zap.NewNop()), stores: ops.stores, configSvc: ops.config, logger: zap.NewNop()}
	lifecycle.SetOperationService(ops)
	lifecycle.SetLifecycleGate(ops.gate)
	periods := NewBizPeriodService(es, lifecycle, nil, zap.NewNop())
	lifecycle.SetPeriodService(periods)
	count, err := periods.RunDueForApp(ctx, scope.AppID, 100)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	stored, err := lifecycle.GetSubscription(ctx, scope.AppID, sub.ID)
	require.NoError(t, err)
	require.Equal(t, sub.ID, stored.ID)
	require.True(t, stored.CurrentPeriodStart.Equal(end))
}
