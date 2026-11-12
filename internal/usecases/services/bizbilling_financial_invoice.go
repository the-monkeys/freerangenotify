package services

import (
	"context"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
)

func (s *BizOperationService) billingCustomerState(ctx context.Context, scope bizbilling.Scope) (string, error) {
	profile, err := s.profiles.Get(ctx, scope.UserID)
	if err != nil {
		return "", err
	}
	if profile.AppID != scope.AppID {
		return "", apperrors.NotFound("customer", scope.UserID)
	}
	if profile.BillingAddress != nil {
		return profile.BillingAddress.State, nil
	}
	return "", nil
}

func (s *BizOperationService) invoiceTotals(ctx context.Context, scope bizbilling.Scope, items []bizbilling.InvoiceLineItem) (bizbilling.InvoiceTotals, error) {
	state, err := s.billingCustomerState(ctx, scope)
	if err != nil {
		return bizbilling.InvoiceTotals{}, err
	}
	tax, err := s.config.TaxConfig(ctx, scope.AppID)
	if err != nil {
		return bizbilling.InvoiceTotals{}, err
	}
	overrides, _ := invoiceTaxOverrides(items)
	return bizbilling.ComputeInvoiceTotalsChecked(items, tax, state, overrides)
}

func (s *BizOperationService) buildFinancialInvoiceDraft(ctx context.Context, command bizbilling.Command, event *bizbilling.JournalEvent) error {
	return s.buildFinancialInvoiceDraftUsingPrice(ctx, command, event, nil)
}

func (s *BizOperationService) buildFinancialInvoiceDraftUsingPrice(ctx context.Context, command bizbilling.Command, event *bizbilling.JournalEvent, price *bizbilling.SubscriptionPriceSnapshot) error {
	input := command.InvoiceDraft
	mode := input.Request.GatewayMode
	if mode == "" {
		mode = bizbilling.GatewayLive
	}
	if !mode.Valid() {
		return bizbilling.ErrInvalidMoney
	}
	if (mode == bizbilling.GatewayTest) != (command.Scope.Mode == bizbilling.GatewayTest) {
		return bizbilling.ErrModeMismatch
	}
	items := append([]bizbilling.InvoiceLineItem(nil), input.Request.LineItems...)
	var totals bizbilling.InvoiceTotals
	var err error
	if price == nil {
		totals, err = s.invoiceTotals(ctx, command.Scope, items)
	} else {
		totals, err = validatedSnapshotTotals(price, command.Scope)
	}
	if err != nil {
		return err
	}
	terms, due, err := validatedInvoiceTerms(ctx, s.config, command.Scope.AppID, input.Request.PaymentTerms, event.CreatedAt)
	if err != nil {
		return err
	}
	// Sandbox documents must not consume the business's legal invoice sequence.
	number := "TEST-INV-" + event.OperationID
	if mode != bizbilling.GatewayTest {
		number, err = s.config.NextNumber(ctx, command.Scope.AppID, NumberKindInvoice)
		if err != nil {
			return err
		}
	}
	typeOf := input.InvoiceType
	if typeOf == "" {
		typeOf = bizbilling.InvoiceTypeOneTime
	}
	if typeOf != bizbilling.InvoiceTypeOneTime && typeOf != bizbilling.InvoiceTypeRecurring && typeOf != bizbilling.InvoiceTypeRetainer {
		return bizbilling.ErrInvalidMoney
	}
	invoice := &bizbilling.BizInvoice{ID: event.OperationID + "-invoice", AppID: command.Scope.AppID, UserID: command.Scope.UserID, Currency: command.Scope.Currency, LastSequence: event.Sequence,
		GatewayMode:   mode,
		InvoiceNumber: number, InvoiceType: typeOf, Status: bizbilling.InvoiceStatusDraft, SubscriptionID: input.SubscriptionID, EstimateID: input.EstimateID,
		LineItems: items, SubtotalPaisa: totals.SubtotalPaisa, DiscountPaisa: totals.DiscountPaisa, TaxAmountPaisa: totals.TaxPaisa, TaxBreakdown: totals.Breakdown,
		TotalPaisa: totals.TotalPaisa, AmountDuePaisa: totals.TotalPaisa, PaymentTerms: terms, DueDate: &due,
		Notes: input.Request.Notes, Metadata: input.Request.Metadata, CreatedAt: event.CreatedAt, UpdatedAt: event.CreatedAt}
	event.InvoiceAfter = invoice
	event.Resources = []bizbilling.ResourceRef{{Kind: "invoice", ID: invoice.ID}}
	return nil
}

func (s *BizOperationService) buildFinancialInvoiceChange(ctx context.Context, command bizbilling.Command, claim bizbilling.StreamClaim, event *bizbilling.JournalEvent) error {
	input := command.InvoiceChange
	inv, err := s.financialInvoice(ctx, command.Scope, input.InvoiceID)
	if err != nil {
		return err
	}
	if inv.CollectionOwner != "" {
		return bizbilling.ErrAutomaticCollectionInProgress
	}
	switch input.Action {
	case "amend":
		if inv.Status != bizbilling.InvoiceStatusDraft || inv.CouponID != "" {
			return bizbilling.ErrInvalidMoney
		}
		update := input.Update
		if update.LineItems != nil {
			items := append([]bizbilling.InvoiceLineItem(nil), update.LineItems...)
			totals, err := s.invoiceTotals(ctx, command.Scope, items)
			if err != nil {
				return err
			}
			settled, err := bizbilling.CheckedAdd(inv.AmountPaidPaisa, inv.TDSPaisa)
			if err != nil {
				return err
			}
			settled, err = bizbilling.CheckedAdd(settled, inv.CreditsApplied)
			if err != nil {
				return err
			}
			if totals.TotalPaisa < settled {
				return bizbilling.ErrInvalidMoney
			}
			inv.LineItems, inv.SubtotalPaisa, inv.DiscountPaisa = items, totals.SubtotalPaisa, totals.DiscountPaisa
			inv.TaxAmountPaisa, inv.TaxBreakdown, inv.TotalPaisa = totals.TaxPaisa, totals.Breakdown, totals.TotalPaisa
			inv.AmountDuePaisa = totals.TotalPaisa - settled
		}
		if update.PaymentTerms != nil {
			terms, due, err := validatedInvoiceTerms(ctx, s.config, command.Scope.AppID, *update.PaymentTerms, event.CreatedAt)
			if err != nil {
				return err
			}
			inv.PaymentTerms = terms
			inv.DueDate = &due
		}
		if update.Notes != nil {
			inv.Notes = *update.Notes
		}
		if update.Metadata != nil {
			inv.Metadata = update.Metadata
		}
	case "issue":
		if inv.Status != bizbilling.InvoiceStatusDraft {
			return bizbilling.ErrInvalidMoney
		}
		// Merchant collection runs separately from the customer money lock.
		if err = s.issueFinancialInvoice(ctx, event, inv); err != nil {
			return err
		}

	case "void", "write_off":
		if inv.Status == bizbilling.InvoiceStatusPaid || inv.Status == bizbilling.InvoiceStatusVoid || inv.Status == bizbilling.InvoiceStatusUncollectible {
			return bizbilling.ErrInvalidMoney
		}
		if input.Action == "void" && (inv.CreditsApplied != 0 || inv.AmountPaidPaisa != 0 || inv.TDSPaisa != 0) {
			return bizbilling.ErrInvalidMoney
		}
		inv.ForgivenPaisa, err = bizbilling.CheckedAdd(inv.ForgivenPaisa, inv.AmountDuePaisa)
		if err != nil {
			return err
		}
		inv.AmountDuePaisa = 0
		if input.Action == "void" {
			inv.Status, inv.VoidedAt = bizbilling.InvoiceStatusVoid, &event.CreatedAt
		} else {
			inv.Status, inv.WrittenOffAt = bizbilling.InvoiceStatusUncollectible, &event.CreatedAt
		}
	case "late_fee":
		if !bizbilling.PayableStatuses[inv.Status] || inv.DueDate == nil {
			return bizbilling.ErrInvalidMoney
		}
		config, err := s.config.LateFeeConfig(ctx, command.Scope.AppID)
		if err != nil {
			return err
		}
		days := int(event.CreatedAt.Sub(*inv.DueDate) / (24 * time.Hour))
		if !config.Enabled || days <= config.GraceDays || config.Value <= 0 {
			return bizbilling.ErrInvalidMoney
		}
		fee := config.Value
		if config.Type == "percentage" {
			fee, err = bizbilling.ProrateMinor(inv.AmountDuePaisa, config.Value, 10000)
			if err != nil {
				return err
			}
		} else if config.Type != "fixed" {
			return bizbilling.ErrInvalidMoney
		}
		if config.MaxLateFee > 0 {
			fee = min(fee, max(int64(0), config.MaxLateFee-inv.LateFeePaisa))
		}
		if fee <= 0 {
			return bizbilling.ErrInvalidMoney
		}
		inv.LateFeePaisa, err = bizbilling.CheckedAdd(inv.LateFeePaisa, fee)
		if err != nil {
			return err
		}
		inv.TotalPaisa, err = bizbilling.CheckedAdd(inv.TotalPaisa, fee)
		if err != nil {
			return err
		}
		inv.AmountDuePaisa, err = bizbilling.CheckedAdd(inv.AmountDuePaisa, fee)
		if err != nil {
			return err
		}
	case "overdue":
		if !bizbilling.PayableStatuses[inv.Status] || inv.AmountDuePaisa <= 0 || inv.DueDate == nil || !event.CreatedAt.After(*inv.DueDate) {
			return bizbilling.ErrInvalidMoney
		}
		inv.Status = bizbilling.InvoiceStatusOverdue
	default:
		return bizbilling.ErrInvalidMoney
	}
	if input.Action == "void" && inv.CouponRedemptionID != "" {
		if err = invoiceAccountingInvariant(inv); err != nil {
			return err
		}
		row, err := s.couponCapacity.Release(ctx, claim, inv.CouponRedemptionID)
		if err != nil {
			return err
		}
		row.State = "released"
		row.LastSequence = event.Sequence
		event.CouponRedemptionAfter = row
	}
	inv.LastSequence, inv.UpdatedAt = event.Sequence, event.CreatedAt
	event.InvoiceAfter = inv
	event.Resources = []bizbilling.ResourceRef{{Kind: "invoice", ID: inv.ID}}
	if event.CouponRedemptionAfter != nil {
		event.Resources = append(event.Resources, bizbilling.ResourceRef{Kind: "coupon_redemption", ID: event.CouponRedemptionAfter.ID})
	}
	return nil
}

func invoiceTaxOverrides(items []bizbilling.InvoiceLineItem) ([]bizbilling.LineTaxOverride, bool) {
	found := false
	overrides := make([]bizbilling.LineTaxOverride, len(items))
	for i, item := range items {
		overrides[i] = bizbilling.LineTaxOverride{Rate: item.TaxRateOverride, Exempt: item.TaxExempt}
		found = found || item.TaxRateOverride != nil || item.TaxExempt
	}
	if !found {
		return nil, false
	}
	return overrides, true
}
