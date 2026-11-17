package services

import (
	"context"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
)

func (s *BizCreditService) tryFinancialSourceCreate(ctx context.Context, appID, userID, currency string, input bizbilling.CreditSourceCreate) (*bizbilling.JournalEvent, bool, error) {
	scope, upgraded, err := financialScopeFor(ctx, s.gate, appID, userID, currency)
	if err == nil && !upgraded && currency != "" {
		return nil, false, apperrors.Conflict("Currency-specific credit requires a reviewed billing upgrade")
	}

	if err != nil || !upgraded {
		return nil, upgraded, err
	}
	if input.InvoiceID != "" {
		invoice, err := s.stores.Invoices.Get(ctx, input.InvoiceID)
		if err != nil {
			return nil, true, err
		}
		if invoice.AppID != appID || invoice.UserID != userID {
			return nil, true, apperrors.NotFound("invoice", input.InvoiceID)
		}
		if invoice.Currency != scope.Currency {
			return nil, true, bizbilling.ErrCurrencyMismatch
		}
		scope.Mode = invoice.FinancialScope().Mode
	}
	event, err := applyRequestedFinancialCommand(ctx, s.operations, bizbilling.Command{Kind: bizbilling.CommandCreditSourceCreate, Scope: scope, CreditSourceCreate: &input})
	return event, true, err
}

func (s *BizCreditService) sourceFinancialScope(ctx context.Context, appID, kind, id string) (bizbilling.Scope, bool, error) {
	var actual bizbilling.Scope
	switch kind {
	case "credit_note":
		note, err := s.stores.CreditNotes.Get(ctx, id)
		if err != nil {
			return actual, false, err
		}
		actual = note.FinancialScope()
	case "retainer":
		ret, err := s.stores.Retainers.Get(ctx, id)
		if err != nil {
			return actual, false, err
		}
		actual = ret.FinancialScope()
	default:
		return actual, false, bizbilling.ErrInvalidMoney
	}
	if actual.AppID != appID {
		return actual, false, apperrors.NotFound("billing credit", id)
	}
	scope, upgraded, err := financialScopeFor(ctx, s.gate, appID, actual.UserID, actual.Currency)
	scope.Mode = actual.Mode
	if upgraded && actual.Currency == "" {
		return scope, true, bizbilling.ErrReconciliationRequired
	}
	return scope, upgraded, err
}

func (s *BizCreditService) tryFinancialSourceChange(ctx context.Context, appID string, input bizbilling.CreditSourceChange) (*bizbilling.JournalEvent, bool, error) {
	scope, upgraded, err := s.sourceFinancialScope(ctx, appID, input.Type, input.SourceID)
	if err != nil || !upgraded {
		return nil, upgraded, err
	}
	event, err := applyRequestedFinancialCommand(ctx, s.operations, bizbilling.Command{Kind: bizbilling.CommandCreditSourceChange, Scope: scope, CreditSourceChange: &input})
	return event, true, err
}

func (s *BizLifecycleService) tryFinancialSourceApplication(ctx context.Context, appID, invoiceID, kind, sourceID string) (*bizbilling.BizInvoice, bool, error) {
	scope, upgraded, err := s.financialInvoiceScope(ctx, appID, invoiceID)
	if err != nil || !upgraded {
		return nil, upgraded, err
	}
	event, err := s.applyFinancialCommand(ctx, bizbilling.Command{Kind: bizbilling.CommandCreditSourceChange, Scope: scope, CreditSourceChange: &bizbilling.CreditSourceChange{Type: kind, SourceID: sourceID, InvoiceID: invoiceID, Action: "apply"}})
	if err != nil {
		return nil, true, err
	}
	return event.InvoiceAfter, true, nil
}

// SetUserBalance preserves explicit balance_paisa semantics through the same
// customer stream as payments, spending and refunds. It never edits a profile.
func (s *BizCreditService) SetUserBalance(ctx context.Context, appID string, profile *user.User, target int64, reason, fingerprint string) (*user.BillingBalanceReceipt, error) {
	if profile.AppID != appID {
		return nil, apperrors.NotFound("customer", profile.UserID)
	}
	scope, upgraded, err := financialScopeFor(ctx, s.gate, appID, profile.UserID, "")
	if err != nil || !upgraded {
		return nil, err
	}
	event, err := applyRequestedFinancialCommand(ctx, s.operations, bizbilling.Command{Kind: bizbilling.CommandBalanceSet, Scope: scope, BalanceSet: &bizbilling.BalanceSet{Initialize: reason == "customer opening balance", TargetPaisa: target, Reason: reason, RequestFingerprint: fingerprint}})
	if err != nil {
		return nil, err
	}
	return &user.BillingBalanceReceipt{BalancePaisa: event.BalanceAfter.AvailablePaisa, Sequence: event.Sequence, Currency: scope.Currency}, nil
}

func (s *BizCreditService) VerifyUserBalanceReceipt(ctx context.Context, profile *user.User) error {
	if s.gate == nil || s.operations == nil || profile.BizBalanceSequence <= 0 || profile.BizBalanceCurrency == "" {
		return bizbilling.ErrReconciliationRequired
	}
	state, err := s.gate.State(ctx, profile.AppID)
	if err != nil {
		return err
	}
	if profile.BizBalanceCurrency != state.PrimaryCurrency {
		return bizbilling.ErrCurrencyMismatch
	}

	scope, upgraded, err := financialScopeFor(ctx, s.gate, profile.AppID, profile.UserID, profile.BizBalanceCurrency)
	if err != nil {
		return err
	}
	if !upgraded {
		return bizbilling.ErrReconciliationRequired
	}
	balance, err := s.operations.Balance(ctx, scope)
	if err != nil {
		return err
	}
	if balance.LastSequence != profile.BizBalanceSequence || balance.AvailablePaisa != profile.BalancePaisa {
		return bizbilling.ErrReconciliationRequired
	}
	return nil
}
