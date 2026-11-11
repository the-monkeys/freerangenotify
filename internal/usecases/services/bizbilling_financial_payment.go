package services

import (
	"context"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
)

func financialScopeMatches(actual, expected bizbilling.Scope) error {
	if actual.AppID != expected.AppID || actual.UserID != expected.UserID {
		return apperrors.NotFound("billing resource", "scope")
	}
	if actual.Currency == "" {
		return bizbilling.ErrLegacyReviewRequired
	}
	if actual.Currency != expected.Currency {
		return bizbilling.ErrCurrencyMismatch
	}
	return nil
}

func (s *BizOperationService) financialInvoice(ctx context.Context, scope bizbilling.Scope, id string) (*bizbilling.BizInvoice, error) {
	i, err := s.stores.Invoices.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if err = financialScopeMatches(i.FinancialScope(), scope); err != nil {
		return nil, err
	}
	if err = invoiceAccountingInvariant(i); err != nil {
		return nil, err
	}
	return i, nil
}
func invoiceAccountingInvariant(i *bizbilling.BizInvoice) error {
	for _, amount := range []int64{i.TotalPaisa, i.AmountPaidPaisa, i.AmountDuePaisa, i.TDSPaisa, i.CreditsApplied, i.ExcessPaidPaisa, i.ForgivenPaisa} {
		if amount < 0 {
			return bizbilling.ErrReconciliationRequired
		}
	}
	total := int64(0)
	for _, amount := range []int64{i.AmountPaidPaisa, i.TDSPaisa, i.CreditsApplied, i.AmountDuePaisa, i.ForgivenPaisa, -i.ExcessPaidPaisa} {
		value, err := bizbilling.CheckedAdd(total, amount)
		if err != nil {
			return err
		}
		total = value
	}
	if total != i.TotalPaisa {
		return bizbilling.ErrReconciliationRequired
	}
	return nil
}

func (s *BizOperationService) buildResourceFinancialEvent(ctx context.Context, command bizbilling.Command, claim bizbilling.StreamClaim, event bizbilling.JournalEvent, balance bizbilling.CustomerBalance) (bizbilling.JournalEvent, error) {
	var err error
	switch command.Kind {
	case bizbilling.CommandInvoiceReminder:
		err = s.buildInvoiceReminder(ctx, command, &event)
	case bizbilling.CommandExpenseConvert:
		err = s.buildExpenseConversion(ctx, command, claim, &event)
	case bizbilling.CommandEstimateConvert:
		err = s.buildEstimateConversion(ctx, command, &event)
	case bizbilling.CommandOfflineRefund:
		err = s.buildOfflineRefund(ctx, command, &event, &balance)
	case bizbilling.CommandRefundRelease:
		err = s.buildRefundRelease(ctx, command, &event, &balance)
	case bizbilling.CommandCouponApply:
		err = s.buildCouponApplication(ctx, command, claim, &event)
	case bizbilling.CommandCreditSourceCreate:
		err = s.buildCreditSourceCreate(ctx, command, &event)
	case bizbilling.CommandCreditSourceChange:
		err = s.buildCreditSourceChange(ctx, command, &event)
	case bizbilling.CommandCreditReversal:
		err = s.buildCreditReversal(ctx, command, &event, &balance)

	case bizbilling.CommandInvoiceDraft:
		err = s.buildFinancialInvoiceDraft(ctx, command, &event)
	case bizbilling.CommandInvoiceChange:
		err = s.buildFinancialInvoiceChange(ctx, command, claim, &event)

	case bizbilling.CommandManualPayment:
		err = s.buildManualPayment(ctx, command, *command.ManualPayment, &event, &balance)
	case bizbilling.CommandBalanceApplication:
		err = s.buildBalanceApplication(ctx, command, *command.BalanceApplication, &event, &balance)
	case bizbilling.CommandRefundReservation:
		err = s.buildRefundReservation(ctx, command, *command.RefundReservation, &event, &balance)
	case bizbilling.CommandRefundSettlement:
		err = s.buildRefundSettlement(ctx, command, *command.RefundSettlement, &event, &balance)
	default:
		err = bizbilling.ErrInvalidMoney
	}
	if err != nil {
		return event, err
	}
	if event.InvoiceAfter != nil {
		if err = invoiceAccountingInvariant(event.InvoiceAfter); err != nil {
			return event, err
		}
	}
	if balance.AvailablePaisa < 0 || balance.ReservedPaisa < 0 || balance.NonRefundablePaisa < 0 || balance.NonRefundablePaisa > balance.AvailablePaisa {
		return event, bizbilling.ErrReconciliationRequired
	}
	return s.finalizeFinancialEvent(ctx, event, balance)
}

func (s *BizOperationService) buildManualPayment(ctx context.Context, command bizbilling.Command, input bizbilling.ManualPayment, event *bizbilling.JournalEvent, balance *bizbilling.CustomerBalance) error {
	inv, err := s.financialInvoice(ctx, command.Scope, input.InvoiceID)
	if err != nil {
		return err
	}
	if inv.CollectionOwner != "" && !input.VerifiedCapture {
		return bizbilling.ErrAutomaticCollectionInProgress
	}
	if !bizbilling.PayableStatuses[inv.Status] && !input.VerifiedCapture {
		return bizbilling.ErrInvalidMoney
	}
	if (input.Method == "gateway" || input.GatewayPaymentID != "") && !input.VerifiedCapture {
		return bizbilling.ErrInvalidMoney
	}
	amount := input.AmountPaisa
	if input.TDSPaisa > inv.AmountDuePaisa {
		return bizbilling.ErrInvalidMoney
	}
	if amount == 0 && !input.VerifiedCapture {
		amount = inv.AmountDuePaisa - input.TDSPaisa
	}
	if amount < 0 || input.TDSPaisa < 0 || (amount == 0 && input.TDSPaisa == 0) {
		return bizbilling.ErrInvalidMoney
	}
	settled, err := bizbilling.CheckedAdd(amount, input.TDSPaisa)
	if err != nil {
		return err
	}
	// A verified capture on a closed invoice is real cash, recorded entirely as
	// unapplied excess. It must not reopen or silently unvoid that invoice.
	payable := bizbilling.PayableStatuses[inv.Status]
	due := inv.AmountDuePaisa
	if !payable {
		due = 0
	}
	applied := min(due, settled)
	excess := settled - applied
	if excess > amount {
		return bizbilling.ErrInvalidMoney
	}
	paymentID := event.OperationID + "-payment"
	payment := &bizbilling.BizPayment{CreditProvenanceKnown: true, ID: paymentID, AppID: command.Scope.AppID, UserID: command.Scope.UserID, Currency: command.Scope.Currency, InvoiceIDs: []string{inv.ID}, AmountPaisa: amount, TDSPaisa: input.TDSPaisa, ExcessCreditPaisa: excess, Method: input.Method, GatewayPaymentID: input.GatewayPaymentID, Status: bizbilling.PaymentStatusSuccess, Notes: input.Notes, CreatedAt: event.CreatedAt, UpdatedAt: event.CreatedAt, LastSequence: event.Sequence}
	if input.VerifiedCapture {
		payment.Gateway = "razorpay"
	}
	if inv.AmountPaidPaisa, err = bizbilling.CheckedAdd(inv.AmountPaidPaisa, amount); err != nil {
		return err
	}
	if inv.TDSPaisa, err = bizbilling.CheckedAdd(inv.TDSPaisa, input.TDSPaisa); err != nil {
		return err
	}
	if inv.ExcessPaidPaisa, err = bizbilling.CheckedAdd(inv.ExcessPaidPaisa, excess); err != nil {
		return err
	}
	if payable {
		inv.AmountDuePaisa -= applied
		if inv.AmountDuePaisa == 0 {
			inv.Status = bizbilling.InvoiceStatusPaid
			inv.PaidAt = &event.CreatedAt
		} else {
			inv.Status = bizbilling.InvoiceStatusPartiallyPaid
		}
	}
	inv.LastSequence = event.Sequence
	inv.UpdatedAt = event.CreatedAt
	if excess > 0 {
		if balance.AvailablePaisa, err = bizbilling.CheckedAdd(balance.AvailablePaisa, excess); err != nil {
			return err
		}
		event.CreditLotsAfter = append(event.CreditLotsAfter, bizbilling.CreditLot{ID: paymentID + "-excess", Scope: command.Scope, SourcePaymentID: paymentID, Currency: command.Scope.Currency, OriginalPaisa: excess, AvailablePaisa: excess, LastSequence: event.Sequence})
	}
	event.InvoiceAfter = inv
	event.PaymentAfter = payment
	event.Resources = []bizbilling.ResourceRef{{Kind: "invoice", ID: inv.ID}, {Kind: "payment", ID: payment.ID}}
	event.Notifications = append(event.Notifications, bizbilling.NotificationIntent{ID: event.ID + "-payment-received-email", AppID: command.Scope.AppID, UserID: command.Scope.UserID, TemplateID: "biz_payment_received", EventID: event.ID, InvoiceID: inv.ID, Channel: "email", Payload: map[string]string{"invoice_number": inv.InvoiceNumber, "amount": FormatPaisa(amount, command.Scope.Currency)}})
	return nil
}

func (s *BizOperationService) buildBalanceApplication(ctx context.Context, command bizbilling.Command, input bizbilling.BalanceApplication, event *bizbilling.JournalEvent, balance *bizbilling.CustomerBalance) error {
	inv, err := s.financialInvoice(ctx, command.Scope, input.InvoiceID)
	if err != nil {
		return err
	}
	if inv.CollectionOwner != "" {
		return bizbilling.ErrAutomaticCollectionInProgress
	}
	if !bizbilling.PayableStatuses[inv.Status] && inv.Status != bizbilling.InvoiceStatusDraft {
		return bizbilling.ErrInvalidMoney
	}
	if balance.UnresolvedLegacy {
		return bizbilling.ErrLegacyReviewRequired
	}
	amount := min(inv.AmountDuePaisa, balance.AvailablePaisa)
	if input.MaxPaisa > 0 {
		amount = min(amount, input.MaxPaisa)
	}
	if amount <= 0 {
		return bizbilling.ErrInvalidMoney
	}
	event.CreditApplicationAfter = newCreditApplication(event, inv.ID, "balance", "", amount)
	if err = s.consumeFinancialCredit(ctx, event, balance, amount); err != nil {
		return err
	}
	balance.AvailablePaisa -= amount
	if err = applyFinancialCredit(inv, amount, event); err != nil {
		return err
	}
	event.Resources = []bizbilling.ResourceRef{{Kind: "invoice", ID: inv.ID}, {Kind: "credit_application", ID: event.CreditApplicationAfter.ID}}

	return nil
}

func validCreditLot(lot *bizbilling.CreditLot) error {
	if lot.ID == "" || lot.Currency != lot.Scope.Currency || lot.OriginalPaisa < 0 || lot.AvailablePaisa < 0 || lot.ReservedPaisa < 0 || lot.RefundedPaisa < 0 {
		return bizbilling.ErrReconciliationRequired
	}
	sum, err := bizbilling.CheckedAdd(lot.AvailablePaisa, lot.ReservedPaisa)
	if err != nil {
		return err
	}
	sum, err = bizbilling.CheckedAdd(sum, lot.RefundedPaisa)
	if err != nil {
		return err
	}
	if sum > lot.OriginalPaisa {
		return bizbilling.ErrReconciliationRequired
	}
	return nil
}

func validFinancialPayment(payment *bizbilling.BizPayment) error {
	if payment.AmountPaisa < 0 || payment.RefundedPaisa < 0 || payment.RefundReservedPaisa < 0 || payment.ExcessCreditPaisa < 0 || payment.ExcessCreditPaisa > payment.AmountPaisa || payment.RefundedExcessPaisa < 0 || payment.RefundReservedExcessPaisa < 0 || payment.RefundedExcessPaisa > payment.RefundedPaisa || payment.RefundReservedExcessPaisa > payment.RefundReservedPaisa {
		return bizbilling.ErrReconciliationRequired
	}
	returned, err := bizbilling.CheckedAdd(payment.RefundedPaisa, payment.RefundReservedPaisa)
	if err != nil {
		return err
	}
	if returned > payment.AmountPaisa {
		return bizbilling.ErrReconciliationRequired
	}
	excess, err := bizbilling.CheckedAdd(payment.RefundedExcessPaisa, payment.RefundReservedExcessPaisa)
	if err != nil {
		return err
	}
	if excess > payment.ExcessCreditPaisa {
		return bizbilling.ErrReconciliationRequired
	}
	principal := returned - excess
	if principal > payment.AmountPaisa-payment.ExcessCreditPaisa {
		return bizbilling.ErrReconciliationRequired
	}
	return nil
}

func (s *BizOperationService) consumeFinancialCredit(ctx context.Context, event *bizbilling.JournalEvent, balance *bizbilling.CustomerBalance, amount int64) error {
	if amount <= 0 || amount > balance.AvailablePaisa {
		return bizbilling.ErrInvalidMoney
	}
	fromAdjustment := min(amount, balance.NonRefundablePaisa)
	balance.NonRefundablePaisa -= fromAdjustment
	if event.CreditApplicationAfter != nil {
		event.CreditApplicationAfter.NonRefundablePaisa = fromAdjustment
	}
	remaining := amount - fromAdjustment
	after := ""
	for remaining > 0 {
		filters := []interface{}{
			map[string]interface{}{"term": map[string]interface{}{"scope.app_id": event.Scope.AppID}}, map[string]interface{}{"term": map[string]interface{}{"scope.user_id": event.Scope.UserID}}, map[string]interface{}{"term": map[string]interface{}{"scope.currency": event.Scope.Currency}}, map[string]interface{}{"range": map[string]interface{}{"available_paisa": map[string]interface{}{"gt": 0}}},
		}
		if after != "" {
			filters = append(filters, map[string]interface{}{"range": map[string]interface{}{"id": map[string]interface{}{"gt": after}}})
		}
		lots, _, err := s.stores.CreditLots.Find(ctx, map[string]interface{}{"query": map[string]interface{}{"bool": map[string]interface{}{"filter": filters}}, "size": 100, "sort": []interface{}{map[string]interface{}{"id": "asc"}}})
		if err != nil {
			return err
		}
		if len(lots) == 0 {
			return bizbilling.ErrReconciliationRequired
		}
		for _, candidate := range lots {
			if candidate.ID == "" || candidate.ID <= after {
				return bizbilling.ErrReconciliationRequired
			}
			after = candidate.ID
			// Search discovers IDs; realtime GET validates the spendable revision.
			lot, err := s.stores.CreditLots.Get(ctx, candidate.ID)
			if err != nil {
				return err
			}
			if err = financialScopeMatches(lot.Scope, event.Scope); err != nil {
				return err
			}
			if err = validCreditLot(lot); err != nil {
				return err
			}
			used := min(remaining, lot.AvailablePaisa)
			if used == 0 {
				continue
			}
			lot.AvailablePaisa -= used
			if event.CreditApplicationAfter != nil {
				event.CreditApplicationAfter.LotDebits = append(event.CreditApplicationAfter.LotDebits, bizbilling.CreditLotDebit{LotID: lot.ID, AmountPaisa: used})
			}
			lot.LastSequence = event.Sequence
			event.CreditLotsAfter = append(event.CreditLotsAfter, *lot)
			remaining -= used
			if remaining == 0 {
				break
			}
		}
	}
	return nil
}

func (s *BizOperationService) buildRefundReservation(ctx context.Context, command bizbilling.Command, input bizbilling.RefundReservationRequest, event *bizbilling.JournalEvent, balance *bizbilling.CustomerBalance) error {
	payment, err := s.stores.Payments.Get(ctx, input.PaymentID)
	if err != nil {
		return err
	}
	if err = financialScopeMatches(payment.FinancialScope(), command.Scope); err != nil {
		return err
	}
	if !payment.CreditProvenanceKnown {
		return bizbilling.ErrLegacyReviewRequired
	}

	if payment.Status != bizbilling.PaymentStatusSuccess {
		return bizbilling.ErrInvalidMoney
	}
	if err = validFinancialPayment(payment); err != nil {
		return err
	}
	spent := int64(0)
	var lot *bizbilling.CreditLot
	if payment.ExcessCreditPaisa > 0 {
		lot, err = s.stores.CreditLots.Get(ctx, payment.ID+"-excess")
		if err != nil {
			return err
		}
		if err = validCreditLot(lot); err != nil {
			return err
		}
		if lot.SourcePaymentID != payment.ID || lot.Scope != command.Scope || lot.OriginalPaisa != payment.ExcessCreditPaisa {
			return bizbilling.ErrReconciliationRequired
		}
		spent = lot.OriginalPaisa - lot.AvailablePaisa - lot.ReservedPaisa - lot.RefundedPaisa
	}
	remaining := payment.AmountPaisa - payment.RefundedPaisa - payment.RefundReservedPaisa
	if spent > remaining {
		return bizbilling.ErrReconciliationRequired
	}
	remaining -= spent
	amount := input.AmountPaisa
	if amount == 0 {
		amount = remaining
	}
	if amount <= 0 || amount > remaining {
		return bizbilling.ErrInvalidMoney
	}
	totalReserved, err := bizbilling.CheckedAdd(payment.RefundReservedPaisa, amount)
	if err != nil {
		return err
	}
	principalRemaining := payment.AmountPaisa - payment.ExcessCreditPaisa - (payment.RefundedPaisa - payment.RefundedExcessPaisa) - (payment.RefundReservedPaisa - payment.RefundReservedExcessPaisa)
	newExcess := max(int64(0), amount-principalRemaining)
	if newExcess < 0 || (newExcess > 0 && lot == nil) {
		return bizbilling.ErrReconciliationRequired
	}
	if newExcess > 0 {
		if newExcess > lot.AvailablePaisa || newExcess > balance.AvailablePaisa {
			return bizbilling.ErrInvalidMoney
		}
		lot.AvailablePaisa -= newExcess
		if lot.ReservedPaisa, err = bizbilling.CheckedAdd(lot.ReservedPaisa, newExcess); err != nil {
			return err
		}
		lot.LastSequence = event.Sequence
		event.CreditLotsAfter = append(event.CreditLotsAfter, *lot)
		balance.AvailablePaisa -= newExcess
		if balance.ReservedPaisa, err = bizbilling.CheckedAdd(balance.ReservedPaisa, newExcess); err != nil {
			return err
		}
	}
	payment.RefundReservedPaisa = totalReserved
	if payment.RefundReservedExcessPaisa, err = bizbilling.CheckedAdd(payment.RefundReservedExcessPaisa, newExcess); err != nil {
		return err
	}
	payment.LastSequence = event.Sequence
	payment.UpdatedAt = event.CreatedAt
	reservation := &bizbilling.RefundReservation{ID: event.OperationID + "-refund", Scope: command.Scope, PaymentID: payment.ID, AmountPaisa: amount, ExcessPaisa: newExcess, State: "reserved", LastSequence: event.Sequence}
	event.PaymentAfter = payment
	event.RefundReservationAfter = reservation
	event.Resources = []bizbilling.ResourceRef{{Kind: "payment", ID: payment.ID}, {Kind: "refund_reservation", ID: reservation.ID}}
	return nil
}

func (s *BizOperationService) buildRefundSettlement(ctx context.Context, command bizbilling.Command, input bizbilling.RefundSettlement, event *bizbilling.JournalEvent, balance *bizbilling.CustomerBalance) error {
	reservation, err := s.stores.RefundReservations.Get(ctx, input.ReservationID)
	if err != nil {
		return err
	}
	return s.settleFinancialRefund(ctx, command, input, event, balance, reservation)
}

func (s *BizOperationService) settleFinancialRefund(ctx context.Context, command bizbilling.Command, input bizbilling.RefundSettlement, event *bizbilling.JournalEvent, balance *bizbilling.CustomerBalance, reservation *bizbilling.RefundReservation) error {
	var err error

	if err = financialScopeMatches(reservation.Scope, command.Scope); err != nil {
		return err
	}
	if reservation.State != "reserved" {
		return bizbilling.ErrInvalidMoney
	}
	payment := event.PaymentAfter
	if payment == nil || payment.ID != reservation.PaymentID {
		payment, err = s.stores.Payments.Get(ctx, reservation.PaymentID)
	}
	if err != nil {
		return err
	}
	if err = financialScopeMatches(payment.FinancialScope(), command.Scope); err != nil {
		return err
	}
	if err = validFinancialPayment(payment); err != nil {
		return err
	}
	if reservation.AmountPaisa <= 0 || reservation.AmountPaisa > payment.RefundReservedPaisa || reservation.ExcessPaisa < 0 || reservation.ExcessPaisa > payment.RefundReservedExcessPaisa {
		return bizbilling.ErrReconciliationRequired
	}
	if payment.Gateway != "" && input.ConfirmationID == "offline-recorded" {
		return bizbilling.ErrInvalidMoney
	}
	if payment.RefundedPaisa, err = bizbilling.CheckedAdd(payment.RefundedPaisa, reservation.AmountPaisa); err != nil {
		return err
	}
	if payment.RefundedPaisa > payment.AmountPaisa {
		return bizbilling.ErrReconciliationRequired
	}
	payment.RefundReservedPaisa -= reservation.AmountPaisa
	payment.RefundReservedExcessPaisa -= reservation.ExcessPaisa
	if payment.RefundedExcessPaisa, err = bizbilling.CheckedAdd(payment.RefundedExcessPaisa, reservation.ExcessPaisa); err != nil {
		return err
	}
	if reservation.ExcessPaisa > 0 {
		var lot *bizbilling.CreditLot
		for i := range event.CreditLotsAfter {
			if event.CreditLotsAfter[i].ID == payment.ID+"-excess" {
				copy := event.CreditLotsAfter[i]
				lot = &copy
				break
			}
		}
		if lot == nil {
			lot, err = s.stores.CreditLots.Get(ctx, payment.ID+"-excess")
		}
		if err != nil {
			return err
		}
		if err = validCreditLot(lot); err != nil {
			return err
		}
		if lot.Scope != command.Scope || lot.ReservedPaisa < reservation.ExcessPaisa || balance.ReservedPaisa < reservation.ExcessPaisa {
			return bizbilling.ErrReconciliationRequired
		}
		lot.ReservedPaisa -= reservation.ExcessPaisa
		if lot.RefundedPaisa, err = bizbilling.CheckedAdd(lot.RefundedPaisa, reservation.ExcessPaisa); err != nil {
			return err
		}
		lot.LastSequence = event.Sequence
		putFinancialCreditLot(event, *lot)
		balance.ReservedPaisa -= reservation.ExcessPaisa
	}
	if payment.RefundedPaisa == payment.AmountPaisa {
		payment.Status = bizbilling.PaymentStatusRefunded
	}
	payment.LastSequence = event.Sequence
	payment.UpdatedAt = event.CreatedAt
	reservation.State = "settled"
	reservation.ConfirmationID = input.ConfirmationID
	reservation.LastSequence = event.Sequence
	if len(payment.InvoiceIDs) != 1 {
		return bizbilling.ErrReconciliationRequired
	}
	inv, err := s.financialInvoice(ctx, command.Scope, payment.InvoiceIDs[0])
	if err != nil {
		return err
	}
	if inv.CashRefundedPaisa, err = bizbilling.CheckedAdd(inv.CashRefundedPaisa, reservation.AmountPaisa); err != nil {
		return err
	}
	inv.LastSequence = event.Sequence
	inv.UpdatedAt = event.CreatedAt
	event.InvoiceAfter = inv
	event.PaymentAfter = payment
	event.RefundReservationAfter = reservation
	event.Resources = []bizbilling.ResourceRef{{Kind: "payment", ID: payment.ID}, {Kind: "refund_reservation", ID: reservation.ID}, {Kind: "invoice", ID: inv.ID}}
	return nil
}

func putFinancialCreditLot(event *bizbilling.JournalEvent, lot bizbilling.CreditLot) {
	for i := range event.CreditLotsAfter {
		if event.CreditLotsAfter[i].ID == lot.ID {
			event.CreditLotsAfter[i] = lot
			return
		}
	}
	event.CreditLotsAfter = append(event.CreditLotsAfter, lot)
}
func (s *BizOperationService) buildOfflineRefund(ctx context.Context, command bizbilling.Command, event *bizbilling.JournalEvent, balance *bizbilling.CustomerBalance) error {
	input := command.OfflineRefund
	payment, err := s.stores.Payments.Get(ctx, input.PaymentID)
	if err != nil {
		return err
	}
	if err = financialScopeMatches(payment.FinancialScope(), command.Scope); err != nil {
		return err
	}
	if payment.Gateway != "" || payment.GatewayPaymentID != "" || payment.Method == "gateway" {
		return apperrors.Conflict("Gateway refunds require verified provider confirmation")
	}
	if !payment.CreditProvenanceKnown {
		return bizbilling.ErrLegacyReviewRequired
	}
	if err = s.buildRefundReservation(ctx, command, *input, event, balance); err != nil {
		return err
	}
	return s.settleFinancialRefund(ctx, command, bizbilling.RefundSettlement{ReservationID: event.RefundReservationAfter.ID, ConfirmationID: "offline-recorded"}, event, balance, event.RefundReservationAfter)
}
