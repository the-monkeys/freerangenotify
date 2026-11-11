package services

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
)

func TestBizStatementKeepsPaidHistoryAndCurrency(t *testing.T) {
	lifecycle, credit, _, es, scope := financialFacadesFixture(t)
	ctx := context.Background()
	templates := &database.IndexTemplates{}
	ensureBillingIndex(t, es, bizbillingrepo.CreditNoteIndex, templates.GetBizCreditNotesTemplate())
	ensureBillingIndex(t, es, bizbillingrepo.RetainerIndex, templates.GetBizRetainersTemplate())
	invoice := seedFinancialInvoice(t, es, scope, 1000)
	payment, err := lifecycle.RecordPayment(ctx, scope.AppID, invoice.ID, &bizbilling.RecordPaymentRequest{Method: "cash", AmountPaisa: 1000})
	require.NoError(t, err)
	_, err = lifecycle.RefundPayment(ctx, scope.AppID, payment.ID, &bizbilling.RefundPaymentRequest{Reason: "refund already made"})
	require.NoError(t, err)
	usd := scope
	usd.Currency = "USD"
	other := seedFinancialInvoice(t, es, usd, 2000)
	_, err = lifecycle.RecordPayment(ctx, scope.AppID, other.ID, &bizbilling.RecordPaymentRequest{Method: "cash", AmountPaisa: 2000})
	require.NoError(t, err)
	statement, err := credit.Statement(ctx, scope.AppID, scope.UserID)
	require.NoError(t, err)
	require.Equal(t, int64(1000), statement.TotalBilled, "never merge USD into an INR statement")
	require.Equal(t, int64(1000), statement.TotalPaid, "refunds are separate from the historical payment")
	require.Zero(t, statement.BalanceOwed, "returning cash does not reopen a paid invoice")
	require.Zero(t, statement.CreditPaisa)
}

func TestBizStatementWaitsForActiveWriter(t *testing.T) {
	lifecycle, credit, _, es, scope := financialFacadesFixture(t)
	ctx := context.Background()
	ensureBillingIndex(t, es, bizbillingrepo.CreditNoteIndex, (&database.IndexTemplates{}).GetBizCreditNotesTemplate())
	permit, err := lifecycle.gate.AcquireMutation(ctx, scope.AppID)
	require.NoError(t, err)
	defer lifecycle.gate.ReleaseMutation(ctx, permit)
	_, err = credit.Statement(ctx, scope.AppID, scope.UserID)
	require.ErrorIs(t, err, bizbilling.ErrConflict)
}

func TestBizStatementRejectsChangesDuringRead(t *testing.T) {
	var armed atomic.Bool
	var action func() error
	var actionErr error
	fault := &lostBillingResponse{match: func(r *http.Request, _ []byte) bool {
		if strings.Contains(r.URL.Path, "frn_biz_invoices/_search") && armed.CompareAndSwap(true, false) {
			actionErr = action()
		}
		return false
	}}
	lifecycle, credit, _, es, scope := financialFacadesFixture(t, fault)
	ctx := context.Background()
	ensureBillingIndex(t, es, bizbillingrepo.CreditNoteIndex, (&database.IndexTemplates{}).GetBizCreditNotesTemplate())
	invoice := seedFinancialInvoice(t, es, scope, 1000)
	action = func() error {
		_, err := lifecycle.RecordPayment(ctx, scope.AppID, invoice.ID, &bizbilling.RecordPaymentRequest{Method: "cash", AmountPaisa: 1000})
		return err
	}
	armed.Store(true)
	_, err := credit.Statement(ctx, scope.AppID, scope.UserID)
	require.NoError(t, actionErr)
	require.False(t, armed.Load(), "a real report query must trigger the concurrent mutation")
	require.ErrorIs(t, err, bizbilling.ErrConflict)
}
