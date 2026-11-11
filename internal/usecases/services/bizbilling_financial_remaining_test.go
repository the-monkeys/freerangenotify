package services

import (
	"context"
	stderrors "errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

func TestBizExpenseConversionUsesSingleJournalFact(t *testing.T) {
	lifecycle, _, ops, es, scope := financialFacadesFixture(t)
	ctx := context.Background()
	ensureBillingIndex(t, es, bizbillingrepo.ExpenseIndex, (&database.IndexTemplates{}).GetBizExpensesTemplate())
	id := uuid.NewString()
	source := struct {
		bizbilling.Expense
		Currency string `json:"currency"`
	}{Expense: bizbilling.Expense{ID: id, AppID: scope.AppID, Category: "Services", AmountPaisa: 1000, Date: time.Now().UTC(), Billable: true, BilledToUser: scope.UserID}, Currency: scope.Currency}
	require.NoError(t, bizbillingrepo.NewStore[struct {
		bizbilling.Expense
		Currency string `json:"currency"`
	}](es, bizbillingrepo.ExpenseIndex, zap.NewNop()).Create(ctx, id, &source))
	service := NewBizExpenseService(ops.stores, lifecycle, zap.NewNop())
	first, err := service.ConvertToInvoice(ctx, scope.AppID, scope.UserID, []string{id})
	if err != nil {
		var failure *bizbilling.OperationFailure
		if stderrors.As(err, &failure) {
			op, e := ops.operations.Get(ctx, failure.Result.OperationID)
			t.Logf("operation diagnostic: %#v load=%v cause=%v", op, e, failure.Cause)
		}
		current, e := ops.stores.Expenses.Get(ctx, id)
		t.Logf("source diagnostic: %#v load=%v scope=%#v", current, e, scope)
	}
	require.NoError(t, err)
	head, err := ops.streams.Get(ctx, scope)
	require.NoError(t, err)
	fact, err := ops.journal.Get(ctx, scope, head.CommittedSequence)
	require.NoError(t, err)
	require.Equal(t, "expense_convert", string(fact.Kind))
	replay, err := service.ConvertToInvoice(ctx, scope.AppID, scope.UserID, []string{id})
	require.NoError(t, err)
	require.Equal(t, first.ID, replay.ID)
	stored, err := service.Get(ctx, scope.AppID, id)
	require.NoError(t, err)
	require.Equal(t, first.ID, stored.InvoiceID)
}

func TestBizDunningProgressUsesJournal(t *testing.T) {
	lifecycle, _, ops, es, scope := financialFacadesFixture(t)
	ctx := context.Background()
	invoice := seedFinancialInvoice(t, es, scope, 1000)
	past := time.Now().UTC().AddDate(0, 0, -5)
	invoice.DueDate = &past
	invoice.Status = bizbilling.InvoiceStatusOverdue
	require.NoError(t, bizbillingrepo.NewStore[bizbilling.BizInvoice](es, "frn_biz_invoices", zap.NewNop()).Replace(ctx, invoice.ID, invoice))
	scheduler := &BizBillingScheduler{lifecycleSvc: lifecycle, logger: zap.NewNop()}
	config := &bizbilling.DunningConfig{Enabled: true, Steps: []bizbilling.DunningStep{{DayOffset: 1, Channel: "email", Template: "biz_reminder_gentle"}}}
	scheduler.runDunningSteps(ctx, invoice, config, 5)
	stored, err := lifecycle.GetInvoice(ctx, scope.AppID, invoice.ID)
	require.NoError(t, err)
	require.NotNil(t, stored.Metadata)
	require.EqualValues(t, 1, stored.Metadata["dunning_last_step"])
	head, err := ops.streams.Get(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, int64(1), head.CommittedSequence)
	fact, err := ops.journal.Get(ctx, scope, head.CommittedSequence)
	require.NoError(t, err)
	require.Equal(t, "invoice_reminder", string(fact.Kind))
	require.Len(t, fact.Notifications, 1)
	scheduler.runDunningSteps(ctx, invoice, config, 5)
	head, err = ops.streams.Get(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, int64(1), head.CommittedSequence)
}

func TestBizAnalyticsUsesCurrencyAndExactMinorUnits(t *testing.T) {
	lifecycle, _, ops, es, scope := financialFacadesFixture(t)
	ctx := context.Background()
	templates := &database.IndexTemplates{}
	ensureBillingIndex(t, es, "frn_biz_plans", templates.GetBizPlansTemplate())
	ensureBillingIndex(t, es, "frn_biz_subscriptions", templates.GetBizSubscriptionsTemplate())
	ensureBillingIndex(t, es, bizbillingrepo.RevenueScheduleIndex, templates.GetBizRevenueSchedulesTemplate())
	plans := NewBizPlanService(bizbillingrepo.NewPlanRepo(es, zap.NewNop()), bizbillingrepo.NewPlanAddonRepo(es, zap.NewNop()), bizbillingrepo.NewProductRepo(es, zap.NewNop()), zap.NewNop())
	analytics := NewBizAnalyticsService(ops.stores, plans, zap.NewNop(), ops)
	const large = int64(9007199254740993)
	past := time.Now().UTC().AddDate(0, 0, -45)
	paid := seedFinancialInvoice(t, es, scope, large)
	paid.DueDate = &past
	require.NoError(t, bizbillingrepo.NewStore[bizbilling.BizInvoice](es, "frn_biz_invoices", zap.NewNop()).Replace(ctx, paid.ID, paid))
	_, err := lifecycle.RecordPayment(ctx, scope.AppID, paid.ID, &bizbilling.RecordPaymentRequest{Method: "cash", AmountPaisa: large})
	require.NoError(t, err)
	due := seedFinancialInvoice(t, es, scope, 1234)
	due.DueDate = &past
	require.NoError(t, bizbillingrepo.NewStore[bizbilling.BizInvoice](es, "frn_biz_invoices", zap.NewNop()).Replace(ctx, due.ID, due))
	usd := scope
	usd.Currency = "USD"
	foreign := seedFinancialInvoice(t, es, usd, 2100)
	foreign.DueDate = &past
	foreign.SubtotalPaisa = 2000
	foreign.TaxAmountPaisa = 100
	foreign.TaxBreakdown.IGSTPaisa = 100
	require.NoError(t, bizbillingrepo.NewStore[bizbilling.BizInvoice](es, "frn_biz_invoices", zap.NewNop()).Replace(ctx, foreign.ID, foreign))
	_, err = lifecycle.RecordPayment(ctx, scope.AppID, foreign.ID, &bizbilling.RecordPaymentRequest{Method: "cash", AmountPaisa: 2100})
	require.NoError(t, err)
	schedule := &bizbilling.RevenueSchedule{ID: uuid.NewString(), AppID: scope.AppID, InvoiceID: paid.ID, TotalPaisa: large, RecognizedPaisa: large - 100, DeferredPaisa: 100}
	require.NoError(t, ops.stores.RevenueSchedules.Create(ctx, schedule.ID, schedule))
	require.NoError(t, ops.stores.RevenueSchedules.Refresh(ctx))
	t.Run("revenue", func(t *testing.T) {
		value, err := analytics.Revenue(ctx, scope.AppID)
		require.NoError(t, err)
		require.EqualValues(t, large, value["total_collected_paisa"])
	})
	t.Run("invoices", func(t *testing.T) {
		value, err := analytics.Invoices(ctx, scope.AppID)
		require.NoError(t, err)
		require.EqualValues(t, large+1234, value["total_billed_paisa"])
		require.EqualValues(t, large, value["total_paid_paisa"])
	})
	t.Run("customers", func(t *testing.T) {
		value, err := analytics.Customers(ctx, scope.AppID, 10)
		require.NoError(t, err)
		require.Len(t, value, 1)
		require.EqualValues(t, large, value[0]["ltv_paisa"])
	})
	t.Run("aging", func(t *testing.T) {
		value, err := analytics.Aging(ctx, scope.AppID)
		require.NoError(t, err)
		bucket := value["31_60"].(map[string]interface{})
		require.EqualValues(t, 1234, bucket["amount_due_paisa"])
	})
	t.Run("dunning", func(t *testing.T) {
		value, err := analytics.Dunning(ctx, scope.AppID)
		require.NoError(t, err)
		require.EqualValues(t, large, value["recovered_paisa"])
		require.EqualValues(t, 1234, value["outstanding_paisa"])
	})
	t.Run("tax", func(t *testing.T) {
		value, err := analytics.TaxSummary(ctx, scope.AppID)
		require.NoError(t, err)
		require.EqualValues(t, 0, value["tax_total_paisa"])
	})
	t.Run("recognition", func(t *testing.T) {
		value, err := analytics.RevenueRecognition(ctx, scope.AppID)
		require.NoError(t, err)
		require.EqualValues(t, large-100, value["recognized_paisa"])
	})
}

func TestBizUnreadySubscriptionWritesHaveNoPartialEffects(t *testing.T) {
	lifecycle, _, ops, es, scope := financialFacadesFixture(t)
	ctx := context.Background()
	ensureBillingIndex(t, es, "frn_biz_subscriptions", (&database.IndexTemplates{}).GetBizSubscriptionsTemplate())
	sub := &bizbilling.BizSubscription{ID: uuid.NewString(), AppID: scope.AppID, UserID: scope.UserID, PlanID: "not-yet-configured", Status: bizbilling.SubscriptionStatusActive, Quantity: 1}
	require.NoError(t, bizbillingrepo.NewStore[bizbilling.BizSubscription](es, "frn_biz_subscriptions", zap.NewNop()).Create(ctx, sub.ID, sub))
	assertBlocked := func(err error) {
		require.Error(t, err)
		appErr, ok := apperrors.AsAppError(err)
		require.True(t, ok)
		require.Equal(t, 409, appErr.GetHTTPStatus())
	}
	_, err := lifecycle.CreateSubscription(ctx, scope.AppID, &bizbilling.CreateSubscriptionRequest{UserID: scope.UserID, PlanID: sub.PlanID})
	assertBlocked(err)
	assertBlocked(lifecycle.RenewSubscription(ctx, sub))
	assertBlocked(lifecycle.ConvertTrial(ctx, sub))
	_, err = lifecycle.ChangePlan(ctx, scope.AppID, sub.ID, "other-plan", nil)
	assertBlocked(err)
	head, err := ops.streams.Get(ctx, scope)
	require.True(t, apperrors.IsNotFound(err))
	require.Nil(t, head)
	stored, err := ops.stores.Subscriptions.Get(ctx, sub.ID)
	require.NoError(t, err)
	require.Equal(t, sub, stored)
}

func TestBizSlowFinancialBuildKeepsClaim(t *testing.T) {
	fault := &lostBillingResponse{}
	fault.match = func(r *http.Request, _ []byte) bool {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/users/_doc/") && fault.armed.CompareAndSwap(true, false) {
			time.Sleep(31 * time.Second)
		}
		return false
	}
	_, _, ops, _, scope := financialFacadesFixture(t, fault)
	fault.armed.Store(true)
	_, err := ops.Apply(context.Background(), bizbilling.Command{Kind: bizbilling.CommandBalanceAdjustment, Scope: scope, BalanceAdjustment: &bizbilling.BalanceAdjustment{DeltaPaisa: 100, Reason: "slow database verification"}})
	require.NoError(t, err)
	balance, err := ops.Balance(context.Background(), scope)
	require.NoError(t, err)
	require.Equal(t, int64(100), balance.AvailablePaisa)
}

func TestBizRevenueRecognitionWriterInvalidatesReport(t *testing.T) {
	_, _, ops, es, scope := financialFacadesFixture(t)
	ctx := context.Background()
	ensureBillingIndex(t, es, bizbillingrepo.RevenueScheduleIndex, (&database.IndexTemplates{}).GetBizRevenueSchedulesTemplate())
	schedule := &bizbilling.RevenueSchedule{ID: uuid.NewString(), AppID: scope.AppID, InvoiceID: "source", TotalPaisa: 1000, DeferredPaisa: 1000}
	require.NoError(t, bizbillingrepo.NewStore[bizbilling.RevenueSchedule](es, bizbillingrepo.RevenueScheduleIndex, zap.NewNop()).Create(ctx, schedule.ID, schedule))
	revision, err := beginFinancialReport(ctx, ops.gate, scope.AppID)
	require.NoError(t, err)
	_, err = ops.stores.RevenueSchedules.Update(ctx, schedule.ID, func(r *bizbilling.RevenueSchedule) error { r.RecognizedPaisa = 500; r.DeferredPaisa = 500; return nil })
	require.NoError(t, err)
	require.ErrorIs(t, revision.verify(ctx, ops.gate), bizbilling.ErrConflict)
}
