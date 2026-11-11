package services

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

func migrationFixture(t *testing.T, faults ...*lostBillingResponse) (*BizMigrationService, *BizOperationService, *elasticsearch.Client, bizbilling.Scope) {
	t.Helper()
	_, _, ops, es, scope := financialFacadesFixture(t, faults...)
	templates := &database.IndexTemplates{}
	for index, template := range map[string]map[string]interface{}{
		bizbillingrepo.LifecycleStateIndex:   templates.GetBizLifecycleStatesTemplate(),
		"applications":                       templates.GetApplicationsTemplate(),
		"frn_biz_products":                   templates.GetBizProductsTemplate(),
		"frn_biz_plans":                      templates.GetBizPlansTemplate(),
		"frn_biz_plan_addons":                templates.GetBizPlanAddonsTemplate(),
		"frn_biz_subscriptions":              templates.GetBizSubscriptionsTemplate(),
		bizbillingrepo.EstimateIndex:         templates.GetBizEstimatesTemplate(),
		bizbillingrepo.ExpenseIndex:          templates.GetBizExpensesTemplate(),
		bizbillingrepo.CouponIndex:           templates.GetBizCouponsTemplate(),
		bizbillingrepo.ContractIndex:         templates.GetBizContractsTemplate(),
		bizbillingrepo.UsageMeterIndex:       templates.GetBizUsageMetersTemplate(),
		bizbillingrepo.UsageEventIndex:       templates.GetBizUsageEventsTemplate(),
		bizbillingrepo.RevenueScheduleIndex:  templates.GetBizRevenueSchedulesTemplate(),
		bizbillingrepo.MigrationReviewIndex:  templates.GetBizMigrationReviewsTemplate(),
		bizbillingrepo.LegacyResolutionIndex: templates.GetBizLegacyResolutionsTemplate(),
	} {
		ensureBillingIndex(t, es, index, template)
	}
	ctx := context.Background()
	app := map[string]interface{}{"app_id": scope.AppID, "name": "Owned migration fixture"}
	require.NoError(t, bizbillingrepo.NewStore[map[string]interface{}](es, "applications", zap.NewNop()).Create(ctx, scope.AppID, &app))

	ops.SetLifecycleGate(NewBizLifecycleGate(bizbillingrepo.NewLifecycleStateRepo(es)))

	service := NewBizMigrationService(es, ops.gate, ops, zap.NewNop())
	service.SetNewCollectionEnabled(true)
	return service, ops, es, scope
}

func TestBizMigrationDryRunNoWrites(t *testing.T) {
	service, ops, _, scope := migrationFixture(t)
	ctx := context.Background()
	before, err := ops.gate.State(ctx, scope.AppID)
	require.NoError(t, err)
	first, err := service.Prepare(ctx, scope.AppID, MigrationOptions{DryRun: true, PrimaryCurrency: scope.Currency})
	require.NoError(t, err)
	second, err := service.Prepare(ctx, scope.AppID, MigrationOptions{DryRun: true, PrimaryCurrency: scope.Currency})
	require.NoError(t, err)
	_, reviewErr := service.reviews.Reports.Get(ctx, first.ID)
	require.True(t, apperrors.IsNotFound(reviewErr), "dry run must not persist even a review report")
	require.Equal(t, first.ID, second.ID)
	require.Equal(t, first.SourceHashes, second.SourceHashes)
	after, err := ops.gate.State(ctx, scope.AppID)
	require.NoError(t, err)
	require.Equal(t, before, after)
	require.Error(t, service.Activate(ctx, scope.AppID, first.ID))
}

func TestBizMigrationResumePreservesSources(t *testing.T) {
	service, ops, es, scope := migrationFixture(t)
	ctx := context.Background()
	profiles := bizbillingrepo.NewStore[user.User](es, "users", zap.NewNop())
	_, err := profiles.Update(ctx, scope.UserID, func(u *user.User) error { u.BalancePaisa = 400; u.BizBalanceCurrency = scope.Currency; return nil })
	require.NoError(t, err)
	dry, err := service.Prepare(ctx, scope.AppID, MigrationOptions{DryRun: true, PrimaryCurrency: scope.Currency})
	require.NoError(t, err)
	require.True(t, dry.Ready)
	reviewed, err := service.Prepare(ctx, scope.AppID, MigrationOptions{PrimaryCurrency: scope.Currency, ExpectedReportID: dry.ID, WritersUpgraded: true})
	require.NoError(t, err)
	require.NoError(t, service.Activate(ctx, scope.AppID, reviewed.ID))
	require.NoError(t, service.Activate(ctx, scope.AppID, reviewed.ID))
	state, err := ops.gate.State(ctx, scope.AppID)
	require.NoError(t, err)
	require.Equal(t, 2, state.LifecycleVersion)
	balance, err := ops.Balance(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, int64(400), balance.AvailablePaisa)
	require.Equal(t, int64(1), balance.LastSequence)
	after, err := service.Prepare(ctx, scope.AppID, MigrationOptions{DryRun: true, PrimaryCurrency: scope.Currency})
	require.NoError(t, err)
	require.Equal(t, dry.SourceHashes, after.SourceHashes)
}

func TestBizAmbiguousCurrencyNeedsReview(t *testing.T) {
	service, ops, es, scope := migrationFixture(t)
	ctx := context.Background()
	_, err := bizbillingrepo.NewStore[user.User](es, "users", zap.NewNop()).Update(ctx, scope.UserID, func(u *user.User) error { u.BalancePaisa = 400; return nil })
	require.NoError(t, err)
	dry, err := service.Prepare(ctx, scope.AppID, MigrationOptions{DryRun: true, PrimaryCurrency: scope.Currency})
	require.NoError(t, err)
	require.False(t, dry.Ready)
	require.NotEmpty(t, dry.Issues)
	reviewed, err := service.Prepare(ctx, scope.AppID, MigrationOptions{ExpectedReportID: dry.ID, PrimaryCurrency: scope.Currency, WritersUpgraded: true})
	require.NoError(t, err)
	require.Error(t, service.Activate(ctx, scope.AppID, reviewed.ID))
	issue := reviewed.Issues[0]
	require.NoError(t, service.Resolve(ctx, scope.AppID, reviewed.ID, LegacyResolution{IssueID: issue.ID, ExpectedSourceHash: issue.SourceHash, Currency: "INR", ActorID: "fixture-admin", Reason: "Reviewed original customer account"}))
	revised, err := service.Prepare(ctx, scope.AppID, MigrationOptions{ExpectedReportID: dry.ID, PrimaryCurrency: scope.Currency, WritersUpgraded: true})
	require.NoError(t, err)
	require.True(t, revised.Ready)
	require.NoError(t, service.Activate(ctx, scope.AppID, revised.ID))
	balance, err := ops.Balance(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, int64(400), balance.AvailablePaisa)
}

func TestBizCutoverFencesOldWriter(t *testing.T) {
	service, ops, es, scope := migrationFixture(t)
	ctx := context.Background()
	dry, err := service.Prepare(ctx, scope.AppID, MigrationOptions{DryRun: true, PrimaryCurrency: scope.Currency})
	require.NoError(t, err)
	reviewed, err := service.Prepare(ctx, scope.AppID, MigrationOptions{ExpectedReportID: dry.ID, PrimaryCurrency: scope.Currency, WritersUpgraded: true})
	require.NoError(t, err)
	writerCtx, release, err := BeginBizMutation(ctx, ops.gate, scope.AppID)
	require.NoError(t, err)
	require.NoError(t, ops.gate.store.Mutate(ctx, scope.AppID, func(s *bizbilling.LifecycleState) error {
		for id, p := range s.MutationPermits {
			p.ExpiresAt = time.Now().Add(-time.Hour)
			s.MutationPermits[id] = p
		}
		return nil
	}))
	require.ErrorIs(t, service.Activate(ctx, scope.AppID, reviewed.ID), bizbilling.ErrActiveLegacyWriter)
	credit := NewBizCreditService(ops.stores, ops.config, repository.NewUserRepository(es, zap.NewNop()), nil, zap.NewNop())
	credit.SetLifecycleGate(ops.gate)
	_, err = credit.AdjustBalance(writerCtx, scope.AppID, scope.UserID, 50)
	require.NoError(t, err)
	require.NoError(t, release())
	require.Error(t, service.Activate(ctx, scope.AppID, reviewed.ID))
	state, err := ops.gate.State(ctx, scope.AppID)
	require.NoError(t, err)
	require.Equal(t, 1, state.LifecycleVersion)
}

func TestBizMigrationInterruptedStagesPreserveSources(t *testing.T) {
	for _, stage := range []string{"journal", "balance", "activation"} {
		t.Run(stage, func(t *testing.T) {
			fault := &lostBillingResponse{}
			fault.match = func(r *http.Request, body []byte) bool {
				if r.Method != http.MethodPut {
					return false
				}
				switch stage {
				case "journal":
					return strings.Contains(r.URL.Path, "/"+bizbillingrepo.JournalIndex+"/_doc/")
				case "balance":
					return strings.Contains(r.URL.Path, "/"+bizbillingrepo.CustomerBalanceIndex+"/_doc/")
				case "activation":
					return strings.Contains(r.URL.Path, "/"+bizbillingrepo.LifecycleStateIndex+"/_doc/") && strings.Contains(string(body), `"lifecycle_version":2`)
				}
				return false
			}
			service, ops, es, scope := migrationFixture(t, fault)
			ctx := context.Background()
			_, err := bizbillingrepo.NewStore[user.User](es, "users", zap.NewNop()).Update(ctx, scope.UserID, func(u *user.User) error { u.BalancePaisa = 400; u.BizBalanceCurrency = scope.Currency; return nil })
			require.NoError(t, err)
			before, err := service.Prepare(ctx, scope.AppID, MigrationOptions{DryRun: true, PrimaryCurrency: scope.Currency})
			require.NoError(t, err)
			reviewed, err := service.Prepare(ctx, scope.AppID, MigrationOptions{PrimaryCurrency: scope.Currency, ExpectedReportID: before.ID, WritersUpgraded: true})
			require.NoError(t, err)
			fault.armed.Store(true)
			require.Error(t, service.Activate(ctx, scope.AppID, reviewed.ID))
			require.False(t, fault.armed.Load())
			state, err := ops.gate.State(ctx, scope.AppID)
			require.NoError(t, err)
			if state.LifecycleVersion == 1 {
				require.Equal(t, "preparing", state.Status)
				expireOperationLease(t, es, scope)
			}
			require.NoError(t, service.Activate(ctx, scope.AppID, reviewed.ID))
			balance, err := ops.Balance(ctx, scope)
			require.NoError(t, err)
			require.Equal(t, int64(400), balance.AvailablePaisa)
			require.Equal(t, int64(1), balance.LastSequence)
			after, err := service.Prepare(ctx, scope.AppID, MigrationOptions{DryRun: true, PrimaryCurrency: scope.Currency})
			require.NoError(t, err)
			require.Equal(t, before.SourceHashes, after.SourceHashes)
		})
	}
}

func TestBizMigrationCurrencySidecarPreservesCreditSource(t *testing.T) {
	service, ops, es, scope := migrationFixture(t)
	ctx := context.Background()
	note := &bizbilling.CreditNote{ID: "reviewed-note-" + scope.AppID, AppID: scope.AppID, UserID: scope.UserID, AmountPaisa: 500, BalancePaisa: 500, Status: bizbilling.CreditNoteStatusOpen}
	raw := bizbillingrepo.NewStore[bizbilling.CreditNote](es, bizbillingrepo.CreditNoteIndex, zap.NewNop())
	require.NoError(t, raw.Create(ctx, note.ID, note))
	dry, err := service.Prepare(ctx, scope.AppID, MigrationOptions{DryRun: true, PrimaryCurrency: scope.Currency})
	require.NoError(t, err)
	require.False(t, dry.Ready)
	reviewed, err := service.Prepare(ctx, scope.AppID, MigrationOptions{ExpectedReportID: dry.ID, PrimaryCurrency: scope.Currency, WritersUpgraded: true})
	require.NoError(t, err)
	for _, issue := range reviewed.Issues {
		require.Equal(t, "currency_required", issue.Code)
		require.NoError(t, service.Resolve(ctx, scope.AppID, reviewed.ID, LegacyResolution{IssueID: issue.ID, ExpectedSourceHash: issue.SourceHash, Currency: scope.Currency, ActorID: "reviewer", Reason: "Original credit note confirms currency"}))
	}
	ready, err := service.Prepare(ctx, scope.AppID, MigrationOptions{ExpectedReportID: dry.ID, PrimaryCurrency: scope.Currency, WritersUpgraded: true})
	require.NoError(t, err)
	require.True(t, ready.Ready)
	require.NoError(t, service.Activate(ctx, scope.AppID, ready.ID))
	original, err := raw.Get(ctx, note.ID)
	require.NoError(t, err)
	require.Equal(t, note, original)
	normalized, err := ops.stores.CreditNotes.Get(ctx, note.ID)
	require.NoError(t, err)
	require.Equal(t, scope.Currency, normalized.Currency)
}

func TestBizMigrationForeignOpeningDoesNotBlockPrimaryMoney(t *testing.T) {
	service, ops, es, scope := migrationFixture(t)
	ctx := context.Background()
	raw := bizbillingrepo.NewStore[user.User](es, "users", zap.NewNop())
	_, err := raw.Update(ctx, scope.UserID, func(u *user.User) error { u.BalancePaisa = 400; u.BizBalanceCurrency = "USD"; return nil })
	require.NoError(t, err)
	dry, err := service.Prepare(ctx, scope.AppID, MigrationOptions{DryRun: true, PrimaryCurrency: scope.Currency})
	require.NoError(t, err)
	require.True(t, dry.Ready)
	reviewed, err := service.Prepare(ctx, scope.AppID, MigrationOptions{ExpectedReportID: dry.ID, PrimaryCurrency: scope.Currency, WritersUpgraded: true})
	require.NoError(t, err)
	require.NoError(t, service.Activate(ctx, scope.AppID, reviewed.ID))
	_, err = ops.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandBalanceAdjustment, Scope: scope, BalanceAdjustment: &bizbilling.BalanceAdjustment{DeltaPaisa: 50, Reason: "New primary currency credit"}})
	require.NoError(t, err)
	primary, err := ops.Balance(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, int64(50), primary.AvailablePaisa)
	foreignScope := scope
	foreignScope.Currency = "USD"
	foreign, err := ops.Balance(ctx, foreignScope)
	require.NoError(t, err)
	require.Equal(t, int64(400), foreign.AvailablePaisa)
	profile, err := raw.Get(ctx, scope.UserID)
	require.NoError(t, err)
	require.Equal(t, int64(400), profile.BalancePaisa)
	require.Equal(t, "USD", profile.BizBalanceCurrency)
}

func TestBizMigrationActivationRequiresCollectionReadiness(t *testing.T) {
	service, ops, _, scope := migrationFixture(t)
	ctx := context.Background()
	service.SetNewCollectionEnabled(false)
	dry, err := service.Prepare(ctx, scope.AppID, MigrationOptions{DryRun: true, PrimaryCurrency: scope.Currency})
	require.NoError(t, err)
	reviewed, err := service.Prepare(ctx, scope.AppID, MigrationOptions{ExpectedReportID: dry.ID, PrimaryCurrency: scope.Currency, WritersUpgraded: true})
	require.NoError(t, err)
	require.Error(t, service.Activate(ctx, scope.AppID, reviewed.ID))
	state, err := ops.gate.State(ctx, scope.AppID)
	require.NoError(t, err)
	require.Equal(t, 1, state.LifecycleVersion)
	require.Equal(t, "legacy", state.Status)
}
