package services

import (
	"context"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
)

func (s *BizOperationService) buildEstimateConversion(ctx context.Context, command bizbilling.Command, event *bizbilling.JournalEvent) error {
	input := command.EstimateConvert
	est, err := s.stores.Estimates.Get(ctx, input.EstimateID)
	if err != nil {
		return err
	}
	if err = financialScopeMatches(est.FinancialScope(), command.Scope); err != nil {
		return err
	}
	if est.Status != bizbilling.EstimateStatusAccepted || est.ConvertedInvoiceID != "" {
		return apperrors.Conflict("Only an accepted, unconverted estimate can be invoiced")
	}
	items := append([]bizbilling.InvoiceLineItem(nil), est.LineItems...)
	if len(input.LineIndexes) > 0 {
		items = nil
		seen := map[int]bool{}
		for _, index := range input.LineIndexes {
			if index < 0 || index >= len(est.LineItems) || seen[index] {
				return bizbilling.ErrInvalidMoney
			}
			seen[index] = true
			items = append(items, est.LineItems[index])
		}
	}
	draft := command
	draft.InvoiceDraft = &bizbilling.InvoiceDraft{Request: bizbilling.CreateInvoiceRequest{UserID: est.UserID, Currency: est.Currency, LineItems: items, Notes: "Converted from estimate " + est.EstimateNumber}, EstimateID: est.ID}
	if err = s.buildFinancialInvoiceDraft(ctx, draft, event); err != nil {
		return err
	}
	issueFinancialInvoice(event, event.InvoiceAfter)
	est.Status = bizbilling.EstimateStatusConverted
	est.ConvertedInvoiceID = event.InvoiceAfter.ID
	est.LastSequence = event.Sequence
	est.UpdatedAt = event.CreatedAt
	event.EstimateAfter = est
	event.Resources = append(event.Resources, bizbilling.ResourceRef{Kind: "estimate", ID: est.ID})
	return nil
}

func issueFinancialInvoice(event *bizbilling.JournalEvent, inv *bizbilling.BizInvoice) {
	inv.Status, inv.IssuedAt = bizbilling.InvoiceStatusOpen, &event.CreatedAt
	if inv.AmountDuePaisa == 0 {
		inv.Status, inv.PaidAt = bizbilling.InvoiceStatusPaid, &event.CreatedAt
	}
	event.Notifications = append(event.Notifications, bizbilling.NotificationIntent{ID: event.OperationID + "-invoice-issued", AppID: inv.AppID, UserID: inv.UserID, TemplateID: "biz_invoice_issued", InvoiceID: inv.ID, EventID: event.ID, Channel: "email"})
}
