package services

import (
	"context"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
)

func (s *BizOperationService) buildCreditSourceCreate(ctx context.Context, command bizbilling.Command, event *bizbilling.JournalEvent) error {
	if _, err := s.billingCustomerState(ctx, command.Scope); err != nil {
		return err
	}
	input := command.CreditSourceCreate
	if input.InvoiceID != "" {
		invoice, err := s.financialInvoice(ctx, command.Scope, input.InvoiceID)
		if err != nil {
			return err
		}
		if invoice.CollectionOwner != "" {
			return bizbilling.ErrAutomaticCollectionInProgress
		}
	}
	scope, now := command.Scope, event.CreatedAt
	switch input.Type {
	case "credit_note":
		number, err := s.config.NextNumber(ctx, scope.AppID, NumberKindCreditNote)
		if err != nil {
			return err
		}
		event.CreditNoteAfter = &bizbilling.CreditNote{ID: event.OperationID + "-credit-note", AppID: scope.AppID, UserID: scope.UserID, Currency: scope.Currency, LastSequence: event.Sequence, InvoiceID: input.InvoiceID, CreditNoteNumber: number, AmountPaisa: input.AmountPaisa, BalancePaisa: input.AmountPaisa, Reason: input.Reason, Status: bizbilling.CreditNoteStatusOpen, CreatedAt: now, UpdatedAt: now}
		if scope.Mode == bizbilling.GatewayTest {
			event.CreditNoteAfter.GatewayMode = bizbilling.GatewayTest
		}
		event.Resources = []bizbilling.ResourceRef{{Kind: "credit_note", ID: event.CreditNoteAfter.ID}}
	case "retainer":
		event.RetainerAfter = &bizbilling.Retainer{ID: event.OperationID + "-retainer", AppID: scope.AppID, UserID: scope.UserID, Currency: scope.Currency, LastSequence: event.Sequence, AmountPaisa: input.AmountPaisa, BalancePaisa: input.AmountPaisa, Notes: input.Notes, Status: bizbilling.RetainerStatusOpen, CreatedAt: now, UpdatedAt: now}
		event.Resources = []bizbilling.ResourceRef{{Kind: "retainer", ID: event.RetainerAfter.ID}}
	default:
		return bizbilling.ErrInvalidMoney
	}
	return nil
}

func validCreditSource(original, available, refunded int64) error {
	if original <= 0 || available < 0 || refunded < 0 {
		return bizbilling.ErrReconciliationRequired
	}
	total, err := bizbilling.CheckedAdd(available, refunded)
	if err != nil {
		return err
	}
	if total > original {
		return bizbilling.ErrReconciliationRequired
	}
	return nil
}

func (s *BizOperationService) buildCreditSourceChange(ctx context.Context, command bizbilling.Command, event *bizbilling.JournalEvent) error {
	input := command.CreditSourceChange
	var available int64
	switch input.Type {
	case "credit_note":
		note, err := s.stores.CreditNotes.Get(ctx, input.SourceID)
		if err != nil {
			return err
		}
		if err = financialScopeMatches(note.FinancialScope(), command.Scope); err != nil {
			return err
		}
		if err = validCreditSource(note.AmountPaisa, note.BalancePaisa, note.RefundedPaisa); err != nil {
			return err
		}
		if note.Status != bizbilling.CreditNoteStatusOpen {
			return bizbilling.ErrInvalidMoney
		}
		event.CreditNoteAfter = note
		available = note.BalancePaisa
	case "retainer":
		ret, err := s.stores.Retainers.Get(ctx, input.SourceID)
		if err != nil {
			return err
		}
		if err = financialScopeMatches(ret.FinancialScope(), command.Scope); err != nil {
			return err
		}
		if err = validCreditSource(ret.AmountPaisa, ret.BalancePaisa, ret.RefundedPaisa); err != nil {
			return err
		}
		if input.Action == "paid" {
			if ret.Status != bizbilling.RetainerStatusOpen {
				return bizbilling.ErrInvalidMoney
			}
			ret.Status = bizbilling.RetainerStatusPaid
		} else if ret.Status != bizbilling.RetainerStatusPaid {
			return bizbilling.ErrInvalidMoney
		}
		event.RetainerAfter = ret
		available = ret.BalancePaisa
	default:
		return bizbilling.ErrInvalidMoney
	}
	if input.Action == "apply" {
		inv, err := s.financialInvoice(ctx, command.Scope, input.InvoiceID)
		if err != nil {
			return err
		}
		amount := min(available, inv.AmountDuePaisa)
		if input.MaxPaisa > 0 {
			amount = min(amount, input.MaxPaisa)
		}
		if err = applyFinancialCredit(inv, amount, event); err != nil {
			return err
		}
		application := newCreditApplication(event, inv.ID, input.Type, input.SourceID, amount)
		event.CreditApplicationAfter = application
		event.Resources = append(event.Resources, bizbilling.ResourceRef{Kind: "credit_application", ID: application.ID}, bizbilling.ResourceRef{Kind: "invoice", ID: inv.ID})
		if event.CreditNoteAfter != nil {
			note := event.CreditNoteAfter
			note.BalancePaisa -= amount
			note.AppliedInvoices = appendInvoiceID(note.AppliedInvoices, inv.ID)
			if note.BalancePaisa == 0 {
				note.Status = bizbilling.CreditNoteStatusApplied
			}
		} else {
			ret := event.RetainerAfter
			ret.BalancePaisa -= amount
			ret.AppliedInvoices = appendInvoiceID(ret.AppliedInvoices, inv.ID)
			if ret.BalancePaisa == 0 {
				ret.Status = bizbilling.RetainerStatusApplied
			}
		}
	} else if input.Action == "refund" {
		note := event.CreditNoteAfter
		if note == nil || note.BalancePaisa <= 0 {
			return bizbilling.ErrInvalidMoney
		}
		var err error
		note.RefundedPaisa, err = bizbilling.CheckedAdd(note.RefundedPaisa, note.BalancePaisa)
		if err != nil {
			return err
		}
		note.BalancePaisa = 0
		note.Status = bizbilling.CreditNoteStatusRefunded
	}
	if note := event.CreditNoteAfter; note != nil {
		note.LastSequence = event.Sequence
		note.UpdatedAt = event.CreatedAt
		event.Resources = append(event.Resources, bizbilling.ResourceRef{Kind: "credit_note", ID: note.ID})
	}
	if ret := event.RetainerAfter; ret != nil {
		ret.LastSequence = event.Sequence
		ret.UpdatedAt = event.CreatedAt
		event.Resources = append(event.Resources, bizbilling.ResourceRef{Kind: "retainer", ID: ret.ID})
	}
	return nil
}

func appendInvoiceID(ids []string, id string) []string {
	for _, v := range ids {
		if v == id {
			return ids
		}
	}
	return append(ids, id)
}
func newCreditApplication(event *bizbilling.JournalEvent, invoiceID, sourceType, sourceID string, amount int64) *bizbilling.CreditApplication {
	return &bizbilling.CreditApplication{ID: event.OperationID + "-application", Scope: event.Scope, InvoiceID: invoiceID, SourceType: sourceType, SourceID: sourceID, AmountPaisa: amount, State: "applied", LastSequence: event.Sequence}
}
func applyFinancialCredit(inv *bizbilling.BizInvoice, amount int64, event *bizbilling.JournalEvent) error {
	if inv.CollectionOwner != "" {
		return bizbilling.ErrAutomaticCollectionInProgress
	}
	if (!bizbilling.PayableStatuses[inv.Status] && inv.Status != bizbilling.InvoiceStatusDraft) || amount <= 0 || amount > inv.AmountDuePaisa {
		return bizbilling.ErrInvalidMoney
	}
	var err error
	inv.CreditsApplied, err = bizbilling.CheckedAdd(inv.CreditsApplied, amount)
	if err != nil {
		return err
	}
	inv.AmountDuePaisa -= amount
	inv.LastSequence = event.Sequence
	inv.UpdatedAt = event.CreatedAt
	if inv.AmountDuePaisa == 0 && inv.Status != bizbilling.InvoiceStatusDraft {
		inv.Status = bizbilling.InvoiceStatusPaid
		inv.PaidAt = &event.CreatedAt
	}
	event.InvoiceAfter = inv
	return nil
}

func (s *BizOperationService) buildCreditReversal(ctx context.Context, command bizbilling.Command, event *bizbilling.JournalEvent, balance *bizbilling.CustomerBalance) error {
	application, err := s.stores.CreditApplications.Get(ctx, command.CreditReversal.ApplicationID)
	if err != nil {
		return err
	}
	if err = financialScopeMatches(application.Scope, command.Scope); err != nil {
		return err
	}
	if application.State != "applied" || application.AmountPaisa <= 0 {
		return bizbilling.ErrInvalidMoney
	}
	inv, err := s.financialInvoice(ctx, command.Scope, application.InvoiceID)
	if err != nil {
		return err
	}
	if inv.CollectionOwner != "" {
		return bizbilling.ErrAutomaticCollectionInProgress
	}
	if inv.Status == bizbilling.InvoiceStatusVoid || inv.Status == bizbilling.InvoiceStatusUncollectible || inv.CreditsApplied < application.AmountPaisa {
		return bizbilling.ErrInvalidMoney
	}
	switch application.SourceType {
	case "balance":
		total := application.NonRefundablePaisa
		if total < 0 {
			return bizbilling.ErrReconciliationRequired
		}
		for _, debit := range application.LotDebits {
			if debit.AmountPaisa <= 0 {
				return bizbilling.ErrReconciliationRequired
			}
			lot, err := s.stores.CreditLots.Get(ctx, debit.LotID)
			if err != nil {
				return err
			}
			if err = financialScopeMatches(lot.Scope, command.Scope); err != nil {
				return err
			}
			lot.AvailablePaisa, err = bizbilling.CheckedAdd(lot.AvailablePaisa, debit.AmountPaisa)
			if err != nil {
				return err
			}
			if err = validCreditLot(lot); err != nil {
				return err
			}
			lot.LastSequence = event.Sequence
			event.CreditLotsAfter = append(event.CreditLotsAfter, *lot)
			total, err = bizbilling.CheckedAdd(total, debit.AmountPaisa)
			if err != nil {
				return err
			}
		}
		if total != application.AmountPaisa {
			return bizbilling.ErrReconciliationRequired
		}
		balance.AvailablePaisa, err = bizbilling.CheckedAdd(balance.AvailablePaisa, total)
		if err != nil {
			return err
		}
		balance.NonRefundablePaisa, err = bizbilling.CheckedAdd(balance.NonRefundablePaisa, application.NonRefundablePaisa)
		if err != nil {
			return err
		}
	case "credit_note":
		note, err := s.stores.CreditNotes.Get(ctx, application.SourceID)
		if err != nil {
			return err
		}
		if err = financialScopeMatches(note.FinancialScope(), command.Scope); err != nil {
			return err
		}
		if note.Status == bizbilling.CreditNoteStatusVoid || note.Status == bizbilling.CreditNoteStatusRefunded {
			return bizbilling.ErrInvalidMoney
		}
		note.BalancePaisa, err = bizbilling.CheckedAdd(note.BalancePaisa, application.AmountPaisa)
		if err != nil {
			return err
		}
		if err = validCreditSource(note.AmountPaisa, note.BalancePaisa, note.RefundedPaisa); err != nil {
			return err
		}
		note.Status = bizbilling.CreditNoteStatusOpen
		note.LastSequence = event.Sequence
		note.UpdatedAt = event.CreatedAt
		event.CreditNoteAfter = note
	case "retainer":
		ret, err := s.stores.Retainers.Get(ctx, application.SourceID)
		if err != nil {
			return err
		}
		if err = financialScopeMatches(ret.FinancialScope(), command.Scope); err != nil {
			return err
		}
		if ret.Status != bizbilling.RetainerStatusPaid && ret.Status != bizbilling.RetainerStatusApplied {
			return bizbilling.ErrInvalidMoney
		}
		ret.BalancePaisa, err = bizbilling.CheckedAdd(ret.BalancePaisa, application.AmountPaisa)
		if err != nil {
			return err
		}
		if err = validCreditSource(ret.AmountPaisa, ret.BalancePaisa, ret.RefundedPaisa); err != nil {
			return err
		}
		ret.Status = bizbilling.RetainerStatusPaid
		ret.LastSequence = event.Sequence
		ret.UpdatedAt = event.CreatedAt
		event.RetainerAfter = ret
	default:
		return bizbilling.ErrReconciliationRequired
	}
	inv.CreditsApplied -= application.AmountPaisa
	inv.AmountDuePaisa, err = bizbilling.CheckedAdd(inv.AmountDuePaisa, application.AmountPaisa)
	if err != nil {
		return err
	}
	if inv.Status == bizbilling.InvoiceStatusPaid {
		inv.Status = bizbilling.InvoiceStatusOpen
		inv.PaidAt = nil
	}
	inv.LastSequence = event.Sequence
	inv.UpdatedAt = event.CreatedAt
	event.InvoiceAfter = inv
	application.State = "reversed"
	application.LastSequence = event.Sequence
	event.CreditApplicationAfter = application
	event.Resources = []bizbilling.ResourceRef{{Kind: "invoice", ID: inv.ID}, {Kind: "credit_application", ID: application.ID}}
	return nil
}
