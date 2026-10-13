package services

import (
	"context"
	"fmt"

	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
)

type legacyReservationSettler interface {
	TransitionLegacyReservation(context.Context, *billing.CreditReservation, billing.CreditReservationStatus, string) (*billing.CreditReservationReceipt, error)
}

func (s *CreditService) settleLegacyReservation(ctx context.Context, res *billing.CreditReservation, status billing.CreditReservationStatus, reason string) (*billing.CreditReservation, bool, error) {
	repo, ok := s.balanceRepo.(legacyReservationSettler)
	if !ok {
		return nil, false, nil
	}
	receipt, err := repo.TransitionLegacyReservation(ctx, res, status, reason)
	if err != nil {
		return nil, true, err
	}
	if receipt == nil {
		return nil, true, fmt.Errorf("missing durable legacy settlement")
	}
	terminal := receipt.Reservation
	// Keep the Redis receipt pending until its ledger is durable. A crash between
	// these writes replays the ES identity, never the wallet delta or sibling hold.
	if err := s.appendTransitionLedger(ctx, &terminal, &receipt.Balance, receipt.Reason); err != nil {
		return &terminal, true, err
	}
	if terminal.Status == billing.CreditReservationReleased {
		if err := s.refundReservationDailyCap(ctx, &terminal); err != nil {
			return &terminal, true, err
		}
	}
	return &terminal, true, s.saveReservation(ctx, &terminal)
}
