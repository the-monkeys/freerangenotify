package services

import (
	"context"
	"sort"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
)

func (s *BizCreditService) StatementInCurrency(ctx context.Context, appID, userID, currency string) (*bizbilling.Statement, error) {
	scope, upgraded, err := financialScopeFor(ctx, s.gate, appID, userID, currency)
	if err != nil {
		return nil, err
	}
	if !upgraded {
		if currency != "" {
			return nil, apperrors.Conflict("Currency-specific statements require a reviewed billing upgrade")
		}
		return s.legacyStatement(ctx, appID, userID)
	}
	if s.operations == nil {
		return nil, bizbilling.ErrReconciliationRequired
	}
	revision, err := beginFinancialReport(ctx, s.gate, appID)
	if err != nil {
		return nil, err
	}
	statement := &bizbilling.Statement{UserID: userID, Currency: scope.Currency, GeneratedAt: time.Now().UTC(), Entries: []bizbilling.StatementEntry{}}
	err = walkFinancialRows(ctx, s.stores.Invoices, s.gate, s.operations, appID, userID, func(invoice *bizbilling.BizInvoice) error {
		matches, err := reportCurrencyMatches(invoice.Currency, scope.Currency)
		if err != nil {
			return err
		}
		if !matches {
			return nil
		}
		if invoice.Status == bizbilling.InvoiceStatusDraft || invoice.Status == bizbilling.InvoiceStatusVoid {
			return nil
		}
		if err = invoiceAccountingInvariant(invoice); err != nil {
			return err
		}
		for _, amount := range []struct {
			target *int64
			value  int64
		}{{&statement.TotalBilled, invoice.TotalPaisa}, {&statement.BalanceOwed, invoice.AmountDuePaisa}, {&statement.TotalForgivenPaisa, invoice.ForgivenPaisa}, {&statement.TotalCreditsAppliedPaisa, invoice.CreditsApplied}} {
			if err = addReportAmount(amount.target, amount.value); err != nil {
				return err
			}
		}
		statement.Entries = append(statement.Entries, bizbilling.StatementEntry{Date: invoice.CreatedAt, Type: "invoice", Reference: invoice.InvoiceNumber, Description: string(invoice.Status), DebitPaisa: invoice.TotalPaisa})
		return nil
	})
	if err != nil {
		return nil, err
	}
	err = walkFinancialRows(ctx, s.stores.Payments, s.gate, s.operations, appID, userID, func(payment *bizbilling.BizPayment) error {
		matches, err := reportCurrencyMatches(payment.Currency, scope.Currency)
		if err != nil {
			return err
		}
		if !matches {
			return nil
		}
		if payment.Status != bizbilling.PaymentStatusSuccess && payment.Status != bizbilling.PaymentStatusRefunded {
			return nil
		}
		if err = validFinancialPayment(payment); err != nil {
			return err
		}
		for _, amount := range []struct {
			target *int64
			value  int64
		}{{&statement.TotalPaid, payment.AmountPaisa}, {&statement.TotalRefundedPaisa, payment.RefundedPaisa}, {&statement.TotalTDSPaisa, payment.TDSPaisa}} {
			if err = addReportAmount(amount.target, amount.value); err != nil {
				return err
			}
		}
		statement.Entries = append(statement.Entries, bizbilling.StatementEntry{Date: payment.CreatedAt, Type: "payment", Reference: payment.ID, Description: payment.Method, CreditPaisa: payment.AmountPaisa})
		if payment.RefundedPaisa > 0 {
			statement.Entries = append(statement.Entries, bizbilling.StatementEntry{Date: payment.UpdatedAt, Type: "refund", Reference: payment.ID, DebitPaisa: payment.RefundedPaisa})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	err = walkFinancialRows(ctx, s.stores.CreditNotes, s.gate, s.operations, appID, userID, func(note *bizbilling.CreditNote) error {
		matches, err := reportCurrencyMatches(note.Currency, scope.Currency)
		if err != nil {
			return err
		}
		if !matches || note.Status == bizbilling.CreditNoteStatusVoid {
			return nil
		}
		if err = validCreditSource(note.AmountPaisa, note.BalancePaisa, note.RefundedPaisa); err != nil {
			return err
		}
		if err = addReportAmount(&statement.CreditNoteAvailablePaisa, note.BalancePaisa); err != nil {
			return err
		}
		statement.Entries = append(statement.Entries, bizbilling.StatementEntry{Date: note.CreatedAt, Type: "credit_note", Reference: note.CreditNoteNumber, Description: note.Reason, CreditPaisa: note.AmountPaisa})
		return nil
	})
	if err != nil {
		return nil, err
	}
	err = walkFinancialRows(ctx, s.stores.Retainers, s.gate, s.operations, appID, userID, func(retainer *bizbilling.Retainer) error {
		matches, err := reportCurrencyMatches(retainer.Currency, scope.Currency)
		if err != nil {
			return err
		}
		if !matches {
			return nil
		}
		if retainer.Status != bizbilling.RetainerStatusPaid && retainer.Status != bizbilling.RetainerStatusApplied {
			return nil
		}
		if err = validCreditSource(retainer.AmountPaisa, retainer.BalancePaisa, retainer.RefundedPaisa); err != nil {
			return err
		}
		if err = addReportAmount(&statement.RetainerAvailablePaisa, retainer.BalancePaisa); err != nil {
			return err
		}
		if err = addReportAmount(&statement.TotalPaid, retainer.AmountPaisa); err != nil {
			return err
		}
		statement.Entries = append(statement.Entries, bizbilling.StatementEntry{Date: retainer.CreatedAt, Type: "retainer", Reference: retainer.ID, Description: retainer.Notes, CreditPaisa: retainer.AmountPaisa})
		return nil
	})
	if err != nil {
		return nil, err
	}
	balance, err := s.operations.Balance(ctx, scope)
	if err != nil {
		return nil, err
	}
	if balance.UnresolvedLegacy {
		return nil, bizbilling.ErrLegacyReviewRequired
	}
	statement.CreditPaisa, statement.ReservedCreditPaisa = balance.AvailablePaisa, balance.ReservedPaisa
	sort.SliceStable(statement.Entries, func(i, j int) bool {
		a, b := statement.Entries[i], statement.Entries[j]
		if a.Date.Equal(b.Date) {
			if a.Reference == b.Reference {
				return a.Type < b.Type
			}
			return a.Reference < b.Reference
		}
		return a.Date.After(b.Date)
	})
	if err = revision.verify(ctx, s.gate); err != nil {
		return nil, err
	}
	return statement, nil
}
