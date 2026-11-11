package services

import (
	"context"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
)

func (s *BizOperationService) buildRefundRelease(ctx context.Context, command bizbilling.Command, event *bizbilling.JournalEvent, balance *bizbilling.CustomerBalance) error {
	input := command.RefundRelease
	if !input.DefinitiveFailure {
		return bizbilling.ErrInvalidMoney
	}
	reservation, err := s.stores.RefundReservations.Get(ctx, input.ReservationID)
	if err != nil {
		return err
	}
	if err = financialScopeMatches(reservation.Scope, command.Scope); err != nil {
		return err
	}
	if reservation.State != "reserved" {
		return bizbilling.ErrInvalidMoney
	}
	payment, err := s.stores.Payments.Get(ctx, reservation.PaymentID)
	if err != nil {
		return err
	}
	if err = financialScopeMatches(payment.FinancialScope(), command.Scope); err != nil {
		return err
	}
	if err = validFinancialPayment(payment); err != nil {
		return err
	}
	if reservation.AmountPaisa <= 0 || reservation.ExcessPaisa < 0 || reservation.ExcessPaisa > reservation.AmountPaisa || payment.RefundReservedPaisa < reservation.AmountPaisa || payment.RefundReservedExcessPaisa < reservation.ExcessPaisa {
		return bizbilling.ErrReconciliationRequired
	}
	if reservation.ExcessPaisa > 0 {
		lot, err := s.stores.CreditLots.Get(ctx, payment.ID+"-excess")
		if err != nil {
			return err
		}
		if err = financialScopeMatches(lot.Scope, command.Scope); err != nil {
			return err
		}
		if err = validCreditLot(lot); err != nil {
			return err
		}
		if lot.ReservedPaisa < reservation.ExcessPaisa || balance.ReservedPaisa < reservation.ExcessPaisa {
			return bizbilling.ErrReconciliationRequired
		}
		lot.ReservedPaisa -= reservation.ExcessPaisa
		lot.AvailablePaisa, err = bizbilling.CheckedAdd(lot.AvailablePaisa, reservation.ExcessPaisa)
		if err != nil {
			return err
		}
		lot.LastSequence = event.Sequence
		event.CreditLotsAfter = []bizbilling.CreditLot{*lot}
		balance.ReservedPaisa -= reservation.ExcessPaisa
		balance.AvailablePaisa, err = bizbilling.CheckedAdd(balance.AvailablePaisa, reservation.ExcessPaisa)
		if err != nil {
			return err
		}
	}
	payment.RefundReservedPaisa -= reservation.AmountPaisa
	payment.RefundReservedExcessPaisa -= reservation.ExcessPaisa
	payment.LastSequence = event.Sequence
	payment.UpdatedAt = event.CreatedAt
	reservation.State = "released"
	reservation.LastSequence = event.Sequence
	event.PaymentAfter = payment
	event.RefundReservationAfter = reservation
	event.Resources = []bizbilling.ResourceRef{{Kind: "payment", ID: payment.ID}, {Kind: "refund_reservation", ID: reservation.ID}}
	return nil
}
