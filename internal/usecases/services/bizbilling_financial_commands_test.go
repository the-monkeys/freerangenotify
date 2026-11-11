package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"go.uber.org/zap"
)

func financialFixture(t *testing.T, faults ...*lostBillingResponse) (*BizOperationService, *elasticsearch.Client, bizbilling.Scope) {
	t.Helper()
	var fault *lostBillingResponse
	if len(faults) > 0 {
		fault = faults[0]
	}
	svc, es := operationFixture(t, fault)
	templates := &database.IndexTemplates{}
	scope := openingCommand().Scope
	for index, template := range map[string]map[string]interface{}{
		"users":                               templates.GetUsersTemplate(),
		bizbillingrepo.CreditNoteIndex:        templates.GetBizCreditNotesTemplate(),
		bizbillingrepo.RetainerIndex:          templates.GetBizRetainersTemplate(),
		"frn_biz_invoices":                    templates.GetBizInvoicesTemplate(),
		"frn_biz_payments":                    templates.GetBizPaymentsTemplate(),
		bizbillingrepo.CreditLotIndex:         templates.GetBizCreditLotsTemplate(),
		bizbillingrepo.CreditApplicationIndex: templates.GetBizCreditApplicationsTemplate(),
		bizbillingrepo.RefundReservationIndex: templates.GetBizRefundReservationsTemplate(),
	} {
		res, err := es.Indices.Exists([]string{index})
		require.NoError(t, err)
		exists := res.StatusCode == 200
		res.Body.Close()
		if exists {
			continue
		}
		data, err := json.Marshal(template)
		require.NoError(t, err)
		res, err = es.Indices.Create(index, es.Indices.Create.WithBody(bytes.NewReader(data)))
		require.NoError(t, err)
		require.False(t, res.IsError())
		res.Body.Close()
	}
	require.NoError(t, bizbillingrepo.NewStore[user.User](es, "users", zap.NewNop(), repository.RefreshFalse).Create(context.Background(), scope.UserID, &user.User{UserID: scope.UserID, AppID: scope.AppID, Email: "financial@example.invalid"}))
	return svc, es, scope
}

func TestBizRefundPrincipalAfterExcessReturned(t *testing.T) {
	svc, es, scope := financialFixture(t)
	ctx := context.Background()
	inv := seedFinancialInvoice(t, es, scope, 1000)
	inv.Status = bizbilling.InvoiceStatusPaid
	inv.AmountPaidPaisa = 1200
	inv.AmountDuePaisa = 0
	inv.ExcessPaidPaisa = 200
	inv.CashRefundedPaisa = 200
	require.NoError(t, svc.stores.Invoices.Replace(ctx, inv.ID, inv))
	payment := &bizbilling.BizPayment{CreditProvenanceKnown: true, ID: uuid.NewString(), AppID: scope.AppID, UserID: scope.UserID, Currency: scope.Currency, InvoiceIDs: []string{inv.ID}, AmountPaisa: 1200, ExcessCreditPaisa: 200, RefundedPaisa: 200, RefundedExcessPaisa: 200, Method: "cash", Status: bizbilling.PaymentStatusSuccess, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	require.NoError(t, svc.stores.Payments.Create(ctx, payment.ID, payment))
	require.NoError(t, svc.stores.CreditLots.Create(ctx, payment.ID+"-excess", &bizbilling.CreditLot{ID: payment.ID + "-excess", Scope: scope, SourcePaymentID: payment.ID, Currency: scope.Currency, OriginalPaisa: 200, RefundedPaisa: 200}))
	// Returning excess first must not consume the still-refundable principal.
	result, err := svc.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandRefundReservation, Scope: scope, RefundReservation: &bizbilling.RefundReservationRequest{PaymentID: payment.ID, AmountPaisa: 100, Reason: "partial principal refund"}})
	require.NoError(t, err)
	reservation, err := svc.stores.RefundReservations.Get(ctx, operationResource(t, result, "refund_reservation"))
	require.NoError(t, err)
	require.Equal(t, int64(100), reservation.AmountPaisa)
	require.Zero(t, reservation.ExcessPaisa)
}
func seedFinancialInvoice(t *testing.T, es *elasticsearch.Client, scope bizbilling.Scope, total int64) *bizbilling.BizInvoice {
	t.Helper()
	now := time.Now().UTC()
	invoice := &bizbilling.BizInvoice{ID: uuid.NewString(), AppID: scope.AppID, UserID: scope.UserID, Currency: scope.Currency, Status: bizbilling.InvoiceStatusOpen, InvoiceType: bizbilling.InvoiceTypeOneTime, InvoiceNumber: "FIXTURE-" + uuid.NewString(), SubtotalPaisa: total, TotalPaisa: total, AmountDuePaisa: total, LineItems: []bizbilling.InvoiceLineItem{{Description: "Service", Quantity: 1, UnitPrice: total, AmountPaisa: total}}, CreatedAt: now, UpdatedAt: now}
	require.NoError(t, bizbillingrepo.NewStore[bizbilling.BizInvoice](es, "frn_biz_invoices", zap.NewNop()).Create(context.Background(), invoice.ID, invoice))
	return invoice
}
func operationResource(t *testing.T, result bizbilling.OperationResult, kind string) string {
	t.Helper()
	for _, resource := range result.Resources {
		if resource.Kind == kind {
			return resource.ID
		}
	}
	t.Fatalf("missing %s result resource", kind)
	return ""
}

func TestBizCurrencylessCreditRequest(t *testing.T) {
	ops, es, scope := financialFixture(t)
	ctx := context.Background()
	templates := &database.IndexTemplates{}
	for index, template := range map[string]map[string]interface{}{"users": templates.GetUsersTemplate(), bizbillingrepo.LifecycleStateIndex: templates.GetBizLifecycleStatesTemplate()} {
		res, err := es.Indices.Exists([]string{index})
		require.NoError(t, err)
		exists := res.StatusCode == 200
		res.Body.Close()
		if exists {
			continue
		}
		data, err := json.Marshal(template)
		require.NoError(t, err)
		res, err = es.Indices.Create(index, es.Indices.Create.WithBody(bytes.NewReader(data)))
		require.NoError(t, err)
		require.False(t, res.IsError())
		res.Body.Close()
	}
	state := bizbilling.DefaultLifecycleState(scope.AppID)
	state.LifecycleVersion = 2
	state.Status = "active"
	state.PrimaryCurrency = "INR"
	require.NoError(t, bizbillingrepo.NewStore[bizbilling.LifecycleState](es, bizbillingrepo.LifecycleStateIndex, zap.NewNop()).Create(ctx, scope.AppID, &state))
	users := repository.NewUserRepository(es, zap.NewNop())
	require.NoError(t, users.Create(ctx, &user.User{UserID: scope.UserID, AppID: scope.AppID, Email: "fixture@example.invalid"}))
	gate := NewBizLifecycleGate(bizbillingrepo.NewLifecycleStateRepo(es))
	ops.SetLifecycleGate(gate)
	credit := NewBizCreditService(ops.stores, nil, users, nil, zap.NewNop())
	credit.SetLifecycleGate(gate)
	credit.SetOperationService(ops)
	ctx = WithBizOperationKey(ctx, "compatible-currencyless-adjustment")
	balance, err := credit.AdjustBalance(ctx, scope.AppID, scope.UserID, 100)
	require.NoError(t, err)
	require.Equal(t, int64(100), balance)
	// The unchanged legacy method can be retried safely when a key is supplied.
	balance, err = credit.AdjustBalance(ctx, scope.AppID, scope.UserID, 100)
	require.NoError(t, err)
	require.Equal(t, int64(100), balance)
	stored, err := ops.Balance(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, "INR", stored.Scope.Currency)
	require.Equal(t, int64(100), stored.AvailablePaisa)
	profile, err := users.GetByID(ctx, scope.UserID)
	require.NoError(t, err)
	require.Equal(t, int64(100), profile.BalancePaisa)
	require.Equal(t, "fixture@example.invalid", profile.Email)
}

func TestBizReadRequiresProjectionVerifier(t *testing.T) {
	gate := NewBizLifecycleGate(&gateMemoryStore{states: map[string]bizbilling.LifecycleState{"app": {AppID: "app", LifecycleVersion: 2, Status: "active", PrimaryCurrency: "INR"}}})
	lifecycle := &BizLifecycleService{gate: gate, repo: &retryInvoiceRepo{invoice: bizbilling.BizInvoice{ID: "invoice", AppID: "app", UserID: "customer", Currency: "INR", LastSequence: 2, Status: bizbilling.InvoiceStatusPaid}}, logger: zap.NewNop()}
	_, err := lifecycle.GetInvoice(context.Background(), "app", "invoice")
	require.ErrorIs(t, err, bizbilling.ErrReconciliationRequired, "an upgraded resource must not be exposed without checking its committed revision")
}

func TestBizInvoiceAccountingInvariant(t *testing.T) {
	svc, es, scope := financialFixture(t)
	ctx := context.Background()
	invoice := seedFinancialInvoice(t, es, scope, 1000)
	result, err := svc.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandManualPayment, Scope: scope, IdempotencyKey: "payment", ManualPayment: &bizbilling.ManualPayment{InvoiceID: invoice.ID, AmountPaisa: 1200, Method: "cash"}})
	require.NoError(t, err)
	paid, err := bizbillingrepo.NewStore[bizbilling.BizInvoice](es, "frn_biz_invoices", zap.NewNop()).Get(ctx, invoice.ID)
	require.NoError(t, err)
	require.Equal(t, bizbilling.InvoiceStatusPaid, paid.Status)
	require.Equal(t, int64(0), paid.AmountDuePaisa)
	require.Equal(t, int64(200), paid.ExcessPaidPaisa)
	require.Equal(t, paid.TotalPaisa, paid.AmountPaidPaisa+paid.TDSPaisa+paid.CreditsApplied+paid.AmountDuePaisa-paid.ExcessPaidPaisa)
	balance, err := svc.Balance(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, int64(200), balance.AvailablePaisa)
	payID := operationResource(t, result, "payment")
	payment, err := bizbillingrepo.NewStore[bizbilling.BizPayment](es, "frn_biz_payments", zap.NewNop()).Get(ctx, payID)
	require.NoError(t, err)
	require.Equal(t, int64(1200), payment.AmountPaisa)
	// Replaying the same payment cannot mint another 200 of customer credit.
	_, err = svc.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandManualPayment, Scope: scope, IdempotencyKey: "payment", ManualPayment: &bizbilling.ManualPayment{InvoiceID: invoice.ID, AmountPaisa: 1200, Method: "cash"}})
	require.NoError(t, err)
	balance, err = svc.Balance(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, int64(200), balance.AvailablePaisa)
}

func TestBizCrossCurrencyCreditRejected(t *testing.T) {
	svc, es, scope := financialFixture(t)
	invoice := seedFinancialInvoice(t, es, scope, 1000)
	wrong := scope
	wrong.Currency = "USD"
	command := bizbilling.Command{Kind: bizbilling.CommandManualPayment, Scope: wrong, IdempotencyKey: "wrong-currency", ManualPayment: &bizbilling.ManualPayment{InvoiceID: invoice.ID, AmountPaisa: 1000, Method: "cash"}}
	_, err := svc.Apply(context.Background(), command)
	require.ErrorIs(t, err, bizbilling.ErrCurrencyMismatch)
	_, err = svc.Apply(context.Background(), command)
	require.ErrorIs(t, err, bizbilling.ErrCurrencyMismatch, "replaying a rejected request must retain its original error category")
	stored, err := bizbillingrepo.NewStore[bizbilling.BizInvoice](es, "frn_biz_invoices", zap.NewNop()).Get(context.Background(), invoice.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1000), stored.AmountDuePaisa)
}

func TestBizCreditSpendRefundRace(t *testing.T) {
	svc, es, scope := financialFixture(t)
	ctx := context.Background()
	original := seedFinancialInvoice(t, es, scope, 1000)
	next := seedFinancialInvoice(t, es, scope, 200)
	paid, err := svc.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandManualPayment, Scope: scope, IdempotencyKey: "pay", ManualPayment: &bizbilling.ManualPayment{InvoiceID: original.ID, AmountPaisa: 1200, Method: "cash"}})
	require.NoError(t, err)
	payID := operationResource(t, paid, "payment")
	commands := []bizbilling.Command{
		{Kind: bizbilling.CommandBalanceApplication, Scope: scope, IdempotencyKey: "spend", BalanceApplication: &bizbilling.BalanceApplication{InvoiceID: next.ID}},
		{Kind: bizbilling.CommandRefundReservation, Scope: scope, IdempotencyKey: "refund", RefundReservation: &bizbilling.RefundReservationRequest{PaymentID: payID, AmountPaisa: 1200, Reason: "customer request"}},
	}
	var workers sync.WaitGroup
	outcomes := make(chan error, 2)
	for _, cmd := range commands {
		workers.Add(1)
		go func(cmd bizbilling.Command) { defer workers.Done(); _, err := svc.Apply(ctx, cmd); outcomes <- err }(cmd)
	}
	workers.Wait()
	close(outcomes)
	for err := range outcomes {
		if err != nil {
			require.True(t, errors.Is(err, bizbilling.ErrConflict) || errors.Is(err, bizbilling.ErrReconciliationRequired) || errors.Is(err, bizbilling.ErrInvalidMoney), err)
		}
	}
	// Resume the accepted head first; expiry never authorizes skipping it.
	for attempt := 0; attempt < 4; attempt++ {
		stream, err := svc.streams.Get(ctx, scope)
		require.NoError(t, err)
		if stream.ActiveOperationID != "" {
			expireOperationLease(t, es, scope)
			_, err = svc.Resume(ctx, scope.AppID, stream.ActiveOperationID)
			require.True(t, err == nil || errors.Is(err, bizbilling.ErrInvalidMoney), err)
		}
		for _, cmd := range commands {
			_, err = svc.Apply(ctx, cmd)
			require.True(t, err == nil || errors.Is(err, bizbilling.ErrInvalidMoney) || errors.Is(err, bizbilling.ErrConflict) || errors.Is(err, bizbilling.ErrReconciliationRequired), err)
		}
	}
	updated, err := bizbillingrepo.NewStore[bizbilling.BizInvoice](es, "frn_biz_invoices", zap.NewNop()).Get(ctx, next.ID)
	require.NoError(t, err)
	refundCmd := commands[1]
	if updated.CreditsApplied == 200 {
		_, err = svc.Apply(ctx, refundCmd)
		require.ErrorIs(t, err, bizbilling.ErrInvalidMoney)
		refundCmd.IdempotencyKey = "safe-refund"
		refundCmd.RefundReservation = &bizbilling.RefundReservationRequest{PaymentID: payID, AmountPaisa: 1000, Reason: "refund excludes spent excess"}
	}
	reserved, err := svc.Apply(ctx, refundCmd)
	require.NoError(t, err)
	reservationID := operationResource(t, reserved, "refund_reservation")
	_, err = svc.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandRefundSettlement, Scope: scope, IdempotencyKey: "settle", RefundSettlement: &bizbilling.RefundSettlement{ReservationID: reservationID, ConfirmationID: "offline-recorded"}})
	require.NoError(t, err)
	payment, err := bizbillingrepo.NewStore[bizbilling.BizPayment](es, "frn_biz_payments", zap.NewNop()).Get(ctx, payID)
	require.NoError(t, err)
	refundedExcess := max(int64(0), payment.RefundedPaisa-1000)
	require.LessOrEqual(t, updated.CreditsApplied+refundedExcess, int64(200), "spent credit and returned excess must never exceed its source")
	unchanged, err := bizbillingrepo.NewStore[bizbilling.BizInvoice](es, "frn_biz_invoices", zap.NewNop()).Get(ctx, original.ID)
	require.NoError(t, err)
	require.Equal(t, bizbilling.InvoiceStatusPaid, unchanged.Status)
	require.Zero(t, unchanged.AmountDuePaisa, "cash refund must not reopen a paid invoice")
}

func TestBizAllMoneyWritersUseJournal(t *testing.T) {
	svc, es, scope := financialFixture(t)
	ctx := context.Background()
	ensureBillingIndex(t, es, "frn_biz_configs", (&database.IndexTemplates{}).GetBizConfigsTemplate())
	ensureBillingIndex(t, es, "users", (&database.IndexTemplates{}).GetUsersTemplate())
	require.NoError(t, repository.NewUserRepository(es, zap.NewNop()).Create(ctx, &user.User{UserID: scope.UserID, AppID: scope.AppID, Email: "journal@example.invalid"}))
	// Creation, amendment and issuance are frozen financial facts, not direct writes.
	draft := bizbilling.Command{Kind: bizbilling.CommandInvoiceDraft, Scope: scope, IdempotencyKey: "draft", InvoiceDraft: &bizbilling.InvoiceDraft{Request: bizbilling.CreateInvoiceRequest{UserID: scope.UserID, Currency: scope.Currency, LineItems: []bizbilling.InvoiceLineItem{{Description: "Service", Quantity: 1, UnitPrice: 1000}}}}}
	result, err := svc.Apply(ctx, draft)
	require.NoError(t, err)
	id := operationResource(t, result, "invoice")
	replay, err := svc.Apply(ctx, draft)
	require.NoError(t, err)
	require.Equal(t, result, replay)
	changed := []struct {
		action string
		update *bizbilling.UpdateInvoiceRequest
	}{
		{"amend", &bizbilling.UpdateInvoiceRequest{LineItems: []bizbilling.InvoiceLineItem{{Description: "Amended service", Quantity: 1, UnitPrice: 2000}}}},
		{"issue", nil}, {"write_off", nil},
	}
	for _, action := range changed {
		outcome, err := svc.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandInvoiceChange, Scope: scope, IdempotencyKey: action.action, InvoiceChange: &bizbilling.InvoiceChange{InvoiceID: id, Action: action.action, Update: action.update}})
		require.NoError(t, err, action.action)
		op, err := svc.completedFinancialEvent(ctx, scope.AppID, outcome.OperationID)
		require.NoError(t, err)
		require.NoError(t, invoiceAccountingInvariant(op.InvoiceAfter))
	}
	inv, err := svc.stores.Invoices.Get(ctx, id)
	require.NoError(t, err)
	require.Equal(t, bizbilling.InvoiceStatusUncollectible, inv.Status)
	require.Equal(t, inv.TotalPaisa, inv.ForgivenPaisa)
	require.Zero(t, inv.AmountDuePaisa)
	require.Equal(t, int64(4), inv.LastSequence)
}

func ensureBillingIndex(t *testing.T, es *elasticsearch.Client, index string, template map[string]interface{}) {
	t.Helper()
	res, err := es.Indices.Exists([]string{index})
	require.NoError(t, err)
	exists := res.StatusCode == 200
	res.Body.Close()
	if exists {
		return
	}
	data, err := json.Marshal(template)
	require.NoError(t, err)
	res, err = es.Indices.Create(index, es.Indices.Create.WithBody(bytes.NewReader(data)))
	require.NoError(t, err)
	defer res.Body.Close()
	require.False(t, res.IsError())
}

func TestBizMoneyFacadePaymentReplay(t *testing.T) {
	ops, es, scope := financialFixture(t)
	ctx := context.Background()
	ensureBillingIndex(t, es, bizbillingrepo.LifecycleStateIndex, (&database.IndexTemplates{}).GetBizLifecycleStatesTemplate())
	state := bizbilling.DefaultLifecycleState(scope.AppID)
	state.LifecycleVersion = 2
	state.Status = "active"
	state.PrimaryCurrency = scope.Currency
	require.NoError(t, bizbillingrepo.NewStore[bizbilling.LifecycleState](es, bizbillingrepo.LifecycleStateIndex, zap.NewNop()).Create(ctx, scope.AppID, &state))
	gate := NewBizLifecycleGate(bizbillingrepo.NewLifecycleStateRepo(es))
	ops.SetLifecycleGate(gate)
	ensureBillingIndex(t, es, "users", (&database.IndexTemplates{}).GetUsersTemplate())
	require.NoError(t, repository.NewUserRepository(es, zap.NewNop()).Create(ctx, &user.User{UserID: scope.UserID, AppID: scope.AppID, Email: "facade@example.invalid"}))
	ensureBillingIndex(t, es, "frn_biz_configs", (&database.IndexTemplates{}).GetBizConfigsTemplate())
	lifecycle := &BizLifecycleService{configSvc: ops.config, repo: bizbillingrepo.NewLifecycleRepo(es, zap.NewNop()), stores: ops.stores, gate: gate, operations: ops, logger: zap.NewNop()}
	inv := seedFinancialInvoice(t, es, scope, 1000)
	keyCtx := WithBizOperationKey(ctx, "manual-cash")
	first, err := lifecycle.RecordPayment(keyCtx, scope.AppID, inv.ID, &bizbilling.RecordPaymentRequest{Method: "cash", AmountPaisa: 1000})
	require.NoError(t, err)
	second, err := lifecycle.RecordPayment(keyCtx, scope.AppID, inv.ID, &bizbilling.RecordPaymentRequest{Method: "cash", AmountPaisa: 1000})
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID)
	require.Positive(t, first.LastSequence)
}

func TestBizCreditSourcesUseJournal(t *testing.T) {
	svc, es, scope := financialFixture(t)
	ctx := context.Background()
	templates := &database.IndexTemplates{}
	ensureBillingIndex(t, es, "frn_biz_configs", templates.GetBizConfigsTemplate())
	ensureBillingIndex(t, es, bizbillingrepo.CreditNoteIndex, templates.GetBizCreditNotesTemplate())
	ensureBillingIndex(t, es, bizbillingrepo.RetainerIndex, templates.GetBizRetainersTemplate())
	ensureBillingIndex(t, es, bizbillingrepo.CreditApplicationIndex, templates.GetBizCreditApplicationsTemplate())
	inv := seedFinancialInvoice(t, es, scope, 1000)
	for _, kind := range []string{"credit_note", "retainer"} {
		result, err := svc.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandCreditSourceCreate, Scope: scope, IdempotencyKey: kind, CreditSourceCreate: &bizbilling.CreditSourceCreate{Type: kind, AmountPaisa: 100, Reason: "account correction"}})
		require.NoError(t, err)
		id := operationResource(t, result, kind)
		if kind == "retainer" {
			_, err = svc.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandCreditSourceChange, Scope: scope, CreditSourceChange: &bizbilling.CreditSourceChange{Type: kind, SourceID: id, Action: "paid"}})
			require.NoError(t, err)
		}
		applied, err := svc.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandCreditSourceChange, Scope: scope, IdempotencyKey: kind + "-apply", CreditSourceChange: &bizbilling.CreditSourceChange{Type: kind, SourceID: id, Action: "apply", InvoiceID: inv.ID}})
		require.NoError(t, err)
		applyID := operationResource(t, applied, "credit_application")
		restored, err := svc.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandCreditReversal, Scope: scope, IdempotencyKey: kind + "-reverse", CreditReversal: &bizbilling.CreditReversal{ApplicationID: applyID, Reason: "incorrect application"}})
		require.NoError(t, err)
		snapshot, err := svc.completedFinancialEvent(ctx, scope.AppID, restored.OperationID)
		require.NoError(t, err)
		require.Zero(t, snapshot.InvoiceAfter.CreditsApplied)
		require.Equal(t, int64(1000), snapshot.InvoiceAfter.AmountDuePaisa)
		_, err = svc.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandCreditReversal, Scope: scope, CreditReversal: &bizbilling.CreditReversal{ApplicationID: applyID, Reason: "second reversal"}})
		require.Error(t, err, "a different key must not restore the same credit twice")
	}
}

func TestBizCreditSourceFacadeReplay(t *testing.T) {
	ops, es, scope := financialFixture(t)
	ctx := context.Background()
	templates := &database.IndexTemplates{}
	ensureBillingIndex(t, es, "frn_biz_configs", templates.GetBizConfigsTemplate())
	ensureBillingIndex(t, es, bizbillingrepo.CreditNoteIndex, templates.GetBizCreditNotesTemplate())
	ensureBillingIndex(t, es, bizbillingrepo.RetainerIndex, templates.GetBizRetainersTemplate())
	state := bizbilling.DefaultLifecycleState(scope.AppID)
	state.LifecycleVersion = 2
	state.Status = "active"
	state.PrimaryCurrency = scope.Currency
	gate := NewBizLifecycleGate(&gateMemoryStore{states: map[string]bizbilling.LifecycleState{scope.AppID: state}})
	ops.SetLifecycleGate(gate)
	credit := NewBizCreditService(ops.stores, ops.config, nil, nil, zap.NewNop())
	credit.SetLifecycleGate(gate)
	credit.SetOperationService(ops)
	request := &bizbilling.CreateCreditNoteRequest{UserID: scope.UserID, AmountPaisa: 100, Reason: "fixture correction"}
	keyCtx := WithBizOperationKey(ctx, "create-credit-note")
	first, err := credit.CreateCreditNote(keyCtx, scope.AppID, request)
	require.NoError(t, err)
	second, err := credit.CreateCreditNote(keyCtx, scope.AppID, request)
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID)
	require.Equal(t, scope.Currency, first.Currency)
	require.Positive(t, first.LastSequence)
}

func TestBizGenericStoreCannotBypassJournal(t *testing.T) {
	ops, es, scope := financialFixture(t)
	ctx := context.Background()
	state := bizbilling.DefaultLifecycleState(scope.AppID)
	state.LifecycleVersion = 2
	state.Status = "active"
	state.PrimaryCurrency = scope.Currency
	gate := NewBizLifecycleGate(&gateMemoryStore{states: map[string]bizbilling.LifecycleState{scope.AppID: state}})
	ops.SetLifecycleGate(gate)
	inv := seedFinancialInvoice(t, es, scope, 1000)
	_, err := ops.stores.Invoices.Update(ctx, inv.ID, func(v *bizbilling.BizInvoice) error { v.AmountDuePaisa = 0; return nil })
	require.ErrorIs(t, err, bizbilling.ErrReconciliationRequired)
	require.ErrorIs(t, ops.stores.Invoices.Delete(ctx, inv.ID), bizbilling.ErrReconciliationRequired)
	repo := bizbillingrepo.NewLifecycleRepo(es, zap.NewNop())
	lifecycle := &BizLifecycleService{repo: repo, logger: zap.NewNop()}
	lifecycle.SetOperationService(ops)
	lifecycle.SetLifecycleGate(gate)
	err = repo.UpdateInvoice(ctx, inv.ID, func(v *bizbilling.BizInvoice) error { v.AmountDuePaisa = 0; return nil })
	require.ErrorIs(t, err, bizbilling.ErrReconciliationRequired)
	// The trusted writer still checks the stream lease and can save all facts.
	_, err = ops.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandManualPayment, Scope: scope, ManualPayment: &bizbilling.ManualPayment{InvoiceID: inv.ID, AmountPaisa: 1000, Method: "cash"}})
	require.NoError(t, err)
}

func TestBizCouponGlobalLimitAcrossCustomers(t *testing.T) {
	svc, es, scope := financialFixture(t)
	ctx := context.Background()
	templates := &database.IndexTemplates{}
	ensureBillingIndex(t, es, bizbillingrepo.CouponIndex, templates.GetBizCouponsTemplate())
	ensureBillingIndex(t, es, bizbillingrepo.CouponRedemptionIndex, templates.GetBizCouponRedemptionsTemplate())
	ensureBillingIndex(t, es, bizbillingrepo.CouponCustomerUsageIndex, templates.GetBizCouponCustomerUsageTemplate())
	coupon := &bizbilling.Coupon{ID: uuid.NewString(), AppID: scope.AppID, Code: uuid.NewString(), Active: true, MaxUses: 1, DiscountType: bizbilling.DiscountTypeFixed, DiscountValue: 100, CreatedAt: time.Now().UTC()}
	require.NoError(t, svc.stores.Coupons.Create(ctx, coupon.ID, coupon))
	require.NoError(t, svc.stores.Coupons.Refresh(ctx))
	scopes := []bizbilling.Scope{scope, {AppID: scope.AppID, UserID: uuid.NewString(), Currency: scope.Currency}}
	commands := make([]bizbilling.Command, 2)
	for i, customer := range scopes {
		inv := seedFinancialInvoice(t, es, customer, 1000)
		commands[i] = bizbilling.Command{Kind: bizbilling.CommandCouponApply, Scope: customer, IdempotencyKey: "coupon", CouponApply: &bizbilling.CouponApply{InvoiceID: inv.ID, Code: coupon.Code}}
		inv.Status = bizbilling.InvoiceStatusDraft
		require.NoError(t, svc.stores.Invoices.Replace(ctx, inv.ID, inv))
	}
	var wg sync.WaitGroup
	failures := make([]error, 2)
	results := make([]bizbilling.OperationResult, 2)
	for i := range commands {
		wg.Add(1)
		go func(i int) { defer wg.Done(); results[i], failures[i] = svc.Apply(ctx, commands[i]) }(i)
	}
	wg.Wait()
	success := 0
	for i := range commands {
		if failures[i] != nil && results[i].OperationID != "" {
			expireOperationLease(t, es, commands[i].Scope)
			results[i], failures[i] = svc.Resume(ctx, scope.AppID, results[i].OperationID)
		}
		if failures[i] == nil {
			success++
			replay, err := svc.Apply(ctx, commands[i])
			require.NoError(t, err)
			require.Equal(t, results[i], replay)
		}
	}
	require.Equal(t, 1, success, "global redemption limit must hold across independent customer streams")
	stored, err := svc.stores.Coupons.Get(ctx, coupon.ID)
	require.NoError(t, err)
	require.Equal(t, 1, stored.CurrentUses)
	require.Nil(t, stored.PendingCapacity, "transient capacity claim must drain")
}

func TestBizRefundReleaseRestoresAvailableCredit(t *testing.T) {
	svc, es, scope := financialFixture(t)
	ctx := context.Background()
	inv := seedFinancialInvoice(t, es, scope, 1000)
	paid, err := svc.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandManualPayment, Scope: scope, ManualPayment: &bizbilling.ManualPayment{InvoiceID: inv.ID, AmountPaisa: 1200, Method: "cash"}})
	require.NoError(t, err)
	reservation, err := svc.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandRefundReservation, Scope: scope, RefundReservation: &bizbilling.RefundReservationRequest{PaymentID: operationResource(t, paid, "payment"), AmountPaisa: 1200, Reason: "full refund"}})
	require.NoError(t, err)
	before, err := svc.Balance(ctx, scope)
	require.NoError(t, err)
	require.Zero(t, before.AvailablePaisa)
	require.Equal(t, int64(200), before.ReservedPaisa)
	command := bizbilling.Command{Kind: bizbilling.CommandRefundRelease, Scope: scope, IdempotencyKey: "release", RefundRelease: &bizbilling.RefundRelease{ReservationID: operationResource(t, reservation, "refund_reservation"), Reason: "offline refund cancelled", DefinitiveFailure: true}}
	_, err = svc.Apply(ctx, command)
	require.NoError(t, err)
	_, err = svc.Apply(ctx, command)
	require.NoError(t, err)
	after, err := svc.Balance(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, int64(200), after.AvailablePaisa)
	require.Zero(t, after.ReservedPaisa)
	row, err := svc.stores.Payments.Get(ctx, operationResource(t, paid, "payment"))
	require.NoError(t, err)
	require.Zero(t, row.RefundReservedPaisa)
	require.Zero(t, row.RefundedPaisa)
}

func TestBizCouponInterruptedReservationAndVoid(t *testing.T) {
	for _, boundary := range []string{"customer", "coupon", "receipt"} {
		t.Run(boundary, func(t *testing.T) {
			fault := &lostBillingResponse{match: func(r *http.Request, body []byte) bool {
				if r.Method != http.MethodPut {
					return false
				}
				switch boundary {
				case "customer":
					return strings.Contains(r.URL.Path, bizbillingrepo.CouponCustomerUsageIndex) && bytes.Contains(body, []byte(`"last_marker":"reserve:`))
				case "coupon":
					return strings.Contains(r.URL.Path, bizbillingrepo.CouponIndex+"/") && bytes.Contains(body, []byte(`"stage":"counted"`))
				default:
					return strings.Contains(r.URL.Path, bizbillingrepo.CouponRedemptionIndex) && bytes.Contains(body, []byte(`"state":"reserved"`))
				}
			}}
			svc, es, scope := financialFixture(t, fault)
			ctx := context.Background()
			templates := &database.IndexTemplates{}
			ensureBillingIndex(t, es, bizbillingrepo.CouponIndex, templates.GetBizCouponsTemplate())
			ensureBillingIndex(t, es, bizbillingrepo.CouponRedemptionIndex, templates.GetBizCouponRedemptionsTemplate())
			ensureBillingIndex(t, es, bizbillingrepo.CouponCustomerUsageIndex, templates.GetBizCouponCustomerUsageTemplate())
			coupon := &bizbilling.Coupon{ID: uuid.NewString(), AppID: scope.AppID, Code: uuid.NewString(), Active: true, MaxUses: 1, MaxUsesPerUser: 1, DiscountType: bizbilling.DiscountTypeFixed, DiscountValue: 100, CreatedAt: time.Now().UTC()}
			require.NoError(t, svc.stores.Coupons.Create(ctx, coupon.ID, coupon))
			require.NoError(t, svc.stores.Coupons.Refresh(ctx))
			inv := seedFinancialInvoice(t, es, scope, 1000)
			inv.Status = bizbilling.InvoiceStatusDraft
			require.NoError(t, svc.stores.Invoices.Replace(ctx, inv.ID, inv))
			command := bizbilling.Command{Kind: bizbilling.CommandCouponApply, Scope: scope, IdempotencyKey: "coupon", CouponApply: &bizbilling.CouponApply{InvoiceID: inv.ID, Code: coupon.Code}}
			fault.armed.Store(true)
			first, err := svc.Apply(ctx, command)
			require.Error(t, err)
			require.False(t, fault.armed.Load(), "requested crash boundary must be reached")
			expireOperationLease(t, es, scope)
			restarted := NewBizOperationService(es, zap.NewNop())
			_, err = restarted.Resume(ctx, scope.AppID, first.OperationID)
			require.NoError(t, err)
			row, err := svc.stores.Coupons.Get(ctx, coupon.ID)
			require.NoError(t, err)
			require.Equal(t, 1, row.CurrentUses)
			require.Nil(t, row.PendingCapacity)
			_, err = restarted.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandInvoiceChange, Scope: scope, IdempotencyKey: "void", InvoiceChange: &bizbilling.InvoiceChange{InvoiceID: inv.ID, Action: "void"}})
			require.NoError(t, err)
			row, err = svc.stores.Coupons.Get(ctx, coupon.ID)
			require.NoError(t, err)
			require.Zero(t, row.CurrentUses, "voiding an unpaid draft must release reserved coupon capacity")
			require.Nil(t, row.PendingCapacity)
		})
	}
}
