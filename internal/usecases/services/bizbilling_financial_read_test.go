package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"go.uber.org/zap"
)

func TestBizFinancialReadsRejectUncommittedSources(t *testing.T) {
	lifecycle, credit, ops, es, scope := financialFacadesFixture(t)
	ctx := context.Background()
	templates := &database.IndexTemplates{}
	ensureBillingIndex(t, es, bizbillingrepo.CreditNoteIndex, templates.GetBizCreditNotesTemplate())
	ensureBillingIndex(t, es, bizbillingrepo.RetainerIndex, templates.GetBizRetainersTemplate())
	_, err := ops.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandOpeningBalance, Scope: scope, OpeningBalance: &bizbilling.OpeningBalance{}})
	require.NoError(t, err)
	payment := &bizbilling.BizPayment{ID: "pending-payment-" + scope.UserID, AppID: scope.AppID, UserID: scope.UserID, Currency: scope.Currency, LastSequence: 2, AmountPaisa: 100}
	note := &bizbilling.CreditNote{ID: "pending-note-" + scope.UserID, AppID: scope.AppID, UserID: scope.UserID, Currency: scope.Currency, LastSequence: 2, AmountPaisa: 100, BalancePaisa: 100}
	retainer := &bizbilling.Retainer{ID: "pending-retainer-" + scope.UserID, AppID: scope.AppID, UserID: scope.UserID, Currency: scope.Currency, LastSequence: 2, AmountPaisa: 100, BalancePaisa: 100}
	require.NoError(t, bizbillingrepo.NewStore[bizbilling.BizPayment](es, "frn_biz_payments", zap.NewNop()).Create(ctx, payment.ID, payment))
	require.NoError(t, bizbillingrepo.NewStore[bizbilling.CreditNote](es, bizbillingrepo.CreditNoteIndex, zap.NewNop()).Create(ctx, note.ID, note))
	require.NoError(t, bizbillingrepo.NewStore[bizbilling.Retainer](es, bizbillingrepo.RetainerIndex, zap.NewNop()).Create(ctx, retainer.ID, retainer))
	for name, read := range map[string]func() error{
		"payment": func() error { _, err := lifecycle.GetPayment(ctx, scope.AppID, payment.ID); return err },
		"payments": func() error {
			_, _, err := lifecycle.ListPayments(ctx, bizbilling.PaymentFilter{AppID: scope.AppID})
			return err
		},
		"credit_note": func() error { _, err := credit.GetCreditNote(ctx, scope.AppID, note.ID); return err },
		"credit_notes": func() error {
			_, _, err := credit.ListCreditNotes(ctx, bizbilling.CreditNoteFilter{AppID: scope.AppID})
			return err
		},
		"retainer": func() error { _, err := credit.GetRetainer(ctx, scope.AppID, retainer.ID); return err },
		"retainers": func() error {
			_, _, err := credit.ListRetainers(ctx, bizbilling.RetainerFilter{AppID: scope.AppID})
			return err
		},
	} {
		t.Run(name, func(t *testing.T) { require.ErrorIs(t, read(), bizbilling.ErrReconciliationRequired) })
	}
}
