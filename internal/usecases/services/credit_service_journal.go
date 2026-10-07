package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"go.uber.org/zap"
)

func (s *CreditService) journal() (billing.CreditReservationJournal, error) {
	repo, ok := s.balanceRepo.(billing.CreditReservationJournal)
	if !ok {
		return nil, fmt.Errorf("credit: durable journal unavailable")
	}
	return repo, nil
}

// Call only after definitive provider failure, before release. Persisting this
// evidence does not change counters; an expired hold can be refunded even when
// the notification subsequently records a different successful attempt.
func (s *CreditService) RecordReservationDeliveryFailure(ctx context.Context, id string) error {
	if _, ok := billing.JournalSubscriptionID(id); !ok {
		return nil
	} // legacy caller compatibility
	recorder, ok := s.balanceRepo.(billing.CreditReservationFailureRecorder)
	if !ok {
		return fmt.Errorf("credit: failure evidence journal unavailable")
	}
	return recorder.RecordReservationDeliveryFailure(ctx, id)
}

func (s *CreditService) reserveJournal(ctx context.Context, balance *billing.CreditBalance, tenant, app, notification, channel string, cost int64, version, capKey string) (*billing.CreditReservation, error) {
	repo, err := s.journal()
	if err != nil {
		s.undoDailyCapKey(ctx, capKey)
		return nil, err
	}
	now := time.Now().UTC()
	res := &billing.CreditReservation{ID: billing.NewJournalReservationID(balance.ID, uuid.NewString()), SubscriptionID: balance.ID, TenantID: tenant, AppID: app, NotificationID: notification,
		Channel: channel, CreditsReserved: cost, RateCardVersion: version, DailyCapKey: capKey, Status: billing.CreditReservationReserved, CreatedAt: now, UpdatedAt: now, ExpiresAt: now.Add(15 * time.Minute)}
	outcome, err := repo.ReserveReservation(ctx, res)
	if err != nil {
		var unavailable *billing.CreditUnavailableError
		if errors.As(err, &unavailable) {
			s.undoDailyCapKey(ctx, capKey)
			if s.logger != nil {
				s.logger.Warn("insufficient credits", zap.String("tenant_id", tenant), zap.String("app_id", app), zap.String("notification_id", notification), zap.String("channel", channel), zap.String("reason", unavailable.Reason), zap.Int64("credits_required", unavailable.CreditsRequired), zap.Int64("credits_remaining", unavailable.CreditsRemaining), zap.Int64("credits_reserved", unavailable.CreditsReserved), zap.Int64("credits_available", unavailable.CreditsAvailable), zap.String("rate_card_version", version))
			}
			return nil, err
		}
		// The update may have succeeded before the response was lost. Resolve that
		// ambiguity from the durable ID before returning a failed admission.
		receipt, lookupErr := repo.GetReservationReceipt(ctx, res.ID)
		if lookupErr == nil && receipt != nil {
			outcome = &billing.CreditTransitionOutcome{Receipt: *receipt}
		} else {
			if errors.Is(lookupErr, billing.ErrCreditReservationNotFound) {
				s.undoDailyCapKey(ctx, capKey)
			}
			return nil, err
		}
	}
	if outcome == nil || outcome.Receipt.Reservation.Status != billing.CreditReservationReserved {
		return nil, fmt.Errorf("credit: invalid reserve receipt")
	}
	saved := outcome.Receipt.Reservation
	// Redis is a cache; admission is already durable. An index failure cannot
	// cancel a legitimate hold or cause an accounting-induced resend.
	if err := s.saveReservation(ctx, &saved); err != nil && s.logger != nil {
		s.logger.Warn("credit: reservation cache unavailable", zap.String("reservation_id", saved.ID), zap.Error(err))
	}
	return &saved, nil
}

func (s *CreditService) transitionJournal(ctx context.Context, id string, status billing.CreditReservationStatus, reason string) (*billing.CreditReservation, error) {
	repo, err := s.journal()
	if err != nil {
		return nil, err
	}
	outcome, err := repo.TransitionReservation(ctx, id, status, reason)
	if err != nil {
		// Reconcile a lost response after the terminal delta. Returning a durable
		// terminal outcome lets callers distinguish accounting repair from send.
		receipt, lookupErr := repo.GetReservationReceipt(ctx, id)
		if lookupErr != nil || receipt == nil || receipt.Reservation.Status == billing.CreditReservationReserved {
			return nil, err
		}
		outcome = &billing.CreditTransitionOutcome{Receipt: *receipt}
	}
	if outcome == nil {
		return nil, fmt.Errorf("credit: missing transition receipt")
	}
	receipt := outcome.Receipt
	res := receipt.Reservation
	// A competing terminal operation wins permanently. Repair that winning
	// operation's ledger, never append the requested but losing transition.
	if res.Status == billing.CreditReservationReleased {
		if err := s.refundReservationDailyCap(ctx, &res); err != nil {
			return &res, err
		}
	}
	if !receipt.LedgerRecorded {
		if err := s.appendTransitionLedger(ctx, &res, &receipt.Balance, receipt.Reason); err != nil {
			return &res, err
		}
		if err := repo.MarkReservationLedgerRecorded(ctx, id, res.Status); err != nil {
			return &res, err
		}
	}
	if err := s.saveReservation(ctx, &res); err != nil && s.logger != nil {
		s.logger.Warn("credit: terminal cache unavailable", zap.String("reservation_id", id), zap.Error(err))
	}
	return &res, nil
}

func (s *CreditService) refundReservationDailyCap(ctx context.Context, res *billing.CreditReservation) error {
	if res.DailyCapKey == "" {
		return nil
	}
	if s.redisClient == nil {
		return fmt.Errorf("credit: daily cap refund store unavailable")
	}
	// Use the durable reservation ID as a refund receipt. This can safely repair
	// a lost ES/Redis response without undoing any sibling's cap admission.
	return s.redisClient.Eval(ctx, `
 if redis.call('HEXISTS', KEYS[2], ARGV[1]) == 1 then return 0 end
 local n = tonumber(redis.call('GET', KEYS[1]) or '0')
 if n > 0 then redis.call('DECR', KEYS[1]) end
 redis.call('HSET', KEYS[2], ARGV[1], '1')
 redis.call('EXPIRE', KEYS[2], 172800)
 return 1`, []string{res.DailyCapKey, res.DailyCapKey + ":refunds"}, res.ID).Err()
}

func (s *CreditService) appendTransitionLedger(ctx context.Context, res *billing.CreditReservation, balance *billing.CreditBalance, reason string) error {
	if balance == nil {
		return fmt.Errorf("credit: missing transition balance")
	}
	kind, delta := billing.CreditLedgerRelease, int64(0)
	if res.Status == billing.CreditReservationCommitted {
		kind, delta = billing.CreditLedgerBurn, -res.CreditsReserved
	}
	if s.ledgerRepo == nil {
		return fmt.Errorf("credit: ledger repository unavailable")
	}
	return s.ledgerRepo.Append(ctx, &billing.CreditLedgerEntry{ID: billing.CreditTransitionLedgerID(res.ID, res.Status), TenantID: res.TenantID, AppID: res.AppID, ReservationID: res.ID, NotificationID: res.NotificationID,
		Channel: res.Channel, EntryType: kind, CreditsDelta: delta, BalanceAfter: balance.CreditsRemaining, RateCardVersion: res.RateCardVersion, CreatedAt: res.UpdatedAt,
		Metadata: map[string]interface{}{"reason": reason, "subscription_id": res.SubscriptionID}})
}

func (s *CreditService) reapJournal(ctx context.Context, now time.Time) (int, error) {
	repo, err := s.journal()
	if err != nil {
		return 0, err
	}
	cursor, released, entries := "", 0, 0
	var failures []error
	for {
		page, err := repo.ListReservationReceipts(ctx, cursor, 100)
		if err != nil {
			return released, errors.Join(append(failures, err)...)
		}
		if page == nil {
			return released, fmt.Errorf("credit: missing journal page")
		}
		entries += page.JournalEntries
		failures = append(failures, page.Errors...)
		for _, receipt := range page.Receipts {
			res := receipt.Reservation
			if res.Status == billing.CreditReservationReserved {
				if res.ExpiresAt.IsZero() || res.ExpiresAt.After(now) {
					continue
				}
				status, reason := billing.CreditReservationReleased, "reservation_expired"
				if receipt.DeliveryOutcome == "failed" {
					reason = "delivery_failure_reconciled"
				}
				if receipt.DeliveryOutcome != "failed" && s.reservationNotificationRepo != nil {
					notif, err := s.reservationNotificationRepo.GetByID(ctx, res.NotificationID)
					if err != nil || notif == nil {
						if err == nil {
							err = fmt.Errorf("notification delivery state missing")
						}
						failures = append(failures, fmt.Errorf("reconcile delivery %s: %w", res.ID, err))
						continue
					}
					if notif.AppID != res.AppID {
						failures = append(failures, fmt.Errorf("credit: notification/reservation application mismatch %s", res.ID))
						continue
					}
					if notif.SentAt != nil || notif.Status == notification.StatusSent || notif.Status == notification.StatusDelivered || notif.Status == notification.StatusRead {
						binding, _ := notif.Metadata[billing.CreditDeliveryReservationMetadataKey].(string)
						if binding != res.ID {
							failures = append(failures, fmt.Errorf("%w: %s", billing.ErrCreditReservationOutcomeUnknown, res.ID))
							continue
						}
						status, reason = billing.CreditReservationCommitted, "delivery_reconciled"
					} else {
						// A provider may still be in flight, or delivery persistence may
						// have failed. Failed notification status alone can describe an
						// ambiguous timeout; only a bound failure receipt authorizes refund.
						failures = append(failures, fmt.Errorf("%w: %s", billing.ErrCreditReservationOutcomeUnknown, res.ID))
						continue
					}
				}
				terminal, err := s.transitionJournal(ctx, res.ID, status, reason)
				if err != nil {
					failures = append(failures, fmt.Errorf("reap %s: %w", res.ID, err))
					continue
				}
				if terminal != nil && terminal.Status == billing.CreditReservationReleased {
					released++
				}
			} else if !receipt.LedgerRecorded {
				if _, err := s.transitionJournal(ctx, res.ID, res.Status, receipt.Reason); err != nil {
					failures = append(failures, fmt.Errorf("repair %s: %w", res.ID, err))
				}
			}
		}
		if page.NextCursor == "" {
			break
		}
		if page.NextCursor <= cursor {
			return released, fmt.Errorf("credit: journal cursor did not advance")
		}
		cursor = page.NextCursor
	}
	if s.logger != nil {
		s.logger.Info("credit: journal recovery scan", zap.Int("journal_entries", entries), zap.Int("expired_released", released))
	}
	return released, errors.Join(failures...)
}

func (s *CreditService) creditCostSnapshot(channel string) (int64, string) {
	if s.rateCardSvc == nil {
		return 1, "default"
	}
	card := s.rateCardSvc.GetActiveRateCard()
	version, cost := "default", int64(0)
	if card != nil {
		version, cost = card.Version, card.ChannelCreditCost[channel]
	}
	if cost <= 0 {
		// Match RateCardService fallback without fetching another card/version.
		switch channel {
		case "email":
			cost = 3
		case "sms":
			cost = 80
		case "whatsapp":
			cost = 108
		default:
			cost = 1
		}
	}
	return cost, version
}
