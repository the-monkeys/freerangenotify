package services

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"go.uber.org/zap"
)

func TestBizHistoricalRefundNeedsProvenanceReview(t *testing.T) {
	ops, es, scope := financialFixture(t)
	ctx := context.Background()
	inv := seedFinancialInvoice(t, es, scope, 1000)
	payment := &bizbilling.BizPayment{ID: uuid.NewString(), AppID: scope.AppID, UserID: scope.UserID, InvoiceIDs: []string{inv.ID}, AmountPaisa: 1000, Currency: scope.Currency, Method: "cash", Status: bizbilling.PaymentStatusSuccess, CreatedAt: time.Now().UTC()}
	require.NoError(t, ops.stores.Payments.Create(ctx, payment.ID, payment))
	_, err := ops.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandRefundReservation, Scope: scope, RefundReservation: &bizbilling.RefundReservationRequest{PaymentID: payment.ID, Reason: "historical refund"}})
	require.ErrorIs(t, err, bizbilling.ErrReconciliationRequired, "the old amount alone does not prove no excess credit was spent")
	// A review rejection must not hold the customer's stream indefinitely.
	_, err = ops.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandManualPayment, Scope: scope, ManualPayment: &bizbilling.ManualPayment{InvoiceID: inv.ID, AmountPaisa: 1000, Method: "cash"}})
	require.NoError(t, err)
}

func TestBizNegativeCheckpointCannotBypassReadCache(t *testing.T) {
	_, _, ops, _, scope := financialFacadesFixture(t)
	ctx := context.Background()
	_, err := ops.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandOpeningBalance, Scope: scope, OpeningBalance: &bizbilling.OpeningBalance{}})
	require.NoError(t, err)
	rows := []*bizbilling.BizPayment{{ID: "valid", AppID: scope.AppID, UserID: scope.UserID, Currency: scope.Currency, LastSequence: 1}, {ID: "corrupt", AppID: scope.AppID, UserID: scope.UserID, Currency: scope.Currency, LastSequence: -1}}
	require.ErrorIs(t, guardFinancialRows(ctx, ops.gate, ops, rows), bizbilling.ErrReconciliationRequired)
}

func TestBizDraftExplicitZeroTax(t *testing.T) {
	lifecycle, _, _, _, scope := financialFacadesFixture(t)
	zero := 0
	ctx := WithBizOperationKey(context.Background(), "exempt-draft")
	request := &bizbilling.CreateInvoiceRequest{UserID: scope.UserID, Currency: scope.Currency, LineItems: []bizbilling.InvoiceLineItem{{Description: "Exempt service", Quantity: 1, UnitPrice: 1000, TaxRateOverride: &zero}}}
	first, err := lifecycle.CreateOneTimeInvoice(ctx, scope.AppID, request)
	require.NoError(t, err)
	require.Equal(t, int64(1000), first.TotalPaisa)
	require.Zero(t, first.TaxAmountPaisa)
	replay, err := lifecycle.CreateOneTimeInvoice(ctx, scope.AppID, request)
	require.NoError(t, err)
	require.Equal(t, first.ID, replay.ID)
	require.Equal(t, first.TotalPaisa, replay.TotalPaisa)
}

func TestBizOpeningCreditCannotResetExistingCustomer(t *testing.T) {
	_, credit, ops, _, scope := financialFacadesFixture(t)
	ctx := context.Background()
	_, err := credit.SetUserBalance(WithBizOperationKey(ctx, "unexpected-opening"), scope.AppID, &user.User{UserID: scope.UserID, AppID: scope.AppID}, 500, "customer opening balance", "create-request")
	require.Error(t, err, "creating an already-existing customer cannot reset its balance before failing")
	balance, err := ops.Balance(ctx, scope)
	require.NoError(t, err)
	require.Zero(t, balance.AvailablePaisa)
}

func TestBizOpeningCreditCannotRunTwiceWithoutProfile(t *testing.T) {
	_, credit, ops, _, scope := financialFacadesFixture(t)
	ctx := context.Background()
	profile := &user.User{UserID: uuid.NewString(), AppID: scope.AppID}
	_, err := credit.SetUserBalance(WithBizOperationKey(ctx, "opening-one"), scope.AppID, profile, 500, "customer opening balance", "original")
	require.NoError(t, err)
	_, err = credit.SetUserBalance(WithBizOperationKey(ctx, "opening-two"), scope.AppID, profile, 100, "customer opening balance", "competing")
	require.Error(t, err)
	balance, err := ops.Balance(ctx, bizbilling.Scope{AppID: scope.AppID, UserID: profile.UserID, Currency: scope.Currency})
	require.NoError(t, err)
	require.Equal(t, int64(500), balance.AvailablePaisa)
}

func TestBizLegacyBalanceConsumerUsesJournal(t *testing.T) {
	_, credit, ops, _, scope := financialFacadesFixture(t)
	ctx := context.Background()
	_, err := credit.AdjustBalance(ctx, scope.AppID, scope.UserID, 100)
	require.NoError(t, err)
	keyed := WithBizOperationKey(ctx, "consume")
	first, err := credit.ConsumeBalance(keyed, scope.AppID, scope.UserID, 40)
	require.NoError(t, err)
	require.Equal(t, int64(40), first)
	balance, err := ops.Balance(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, int64(60), balance.AvailablePaisa, "the exported compatibility consumer must not only edit the profile mirror")
	replay, err := credit.ConsumeBalance(keyed, scope.AppID, scope.UserID, 40)
	require.NoError(t, err)
	require.Equal(t, first, replay)
	balance, err = ops.Balance(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, int64(60), balance.AvailablePaisa)
}
func TestBizAutomaticOwnerRejectsLinkedCreditNote(t *testing.T) {
	_, credit, _, es, scope := financialFacadesFixture(t)
	ctx := context.Background()
	invoice := seedFinancialInvoice(t, es, scope, 1000)
	invoice.CollectionOwner = "automatic-collection"
	require.NoError(t, bizbillingrepo.NewStore[bizbilling.BizInvoice](es, "frn_biz_invoices", zap.NewNop()).Replace(ctx, invoice.ID, invoice))
	_, err := credit.CreateCreditNote(ctx, scope.AppID, &bizbilling.CreateCreditNoteRequest{UserID: scope.UserID, InvoiceID: invoice.ID, AmountPaisa: 100, Reason: "invoice adjustment"})
	require.ErrorIs(t, err, bizbilling.ErrAutomaticCollectionInProgress)
}
