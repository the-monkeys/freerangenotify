package services

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"time"
)

func (s *CreditService) ReserveQuotedDelivery(ctx context.Context, tenant, app, notif string, q *billing.ChannelQuote) (*billing.CreditReservation, error) {
	if tenant == "" || notif == "" || q.Validate() != nil || q.Connection.AppID != app || q.Channel != "sms" && q.Channel != "whatsapp" {
		return nil, fmt.Errorf("invalid quoted admission")
	}
	if !s.enforceCreditChecks {
		return nil, nil
	}
	sub, err := s.subRepo.GetActiveSubscription(ctx, tenant, "", time.Now().UTC())
	if err != nil {
		return nil, err
	}
	if sub != nil && billing.BillingModel(sub) == billing.BillingModelLegacy {
		return s.reserveLegacyQuota(ctx, tenant, app, notif, q.Channel, sub, q.Connection.CredentialSource)
	}
	if !s.reservationJournalEnabled {
		if q.PricingModel != "flat" {
			return nil, fmt.Errorf("detailed pricing requires durable reservation journal")
		}
		return s.reserveCredits(ctx, tenant, app, notif, q.Channel, q)
	}
	balance, err := s.getOrBootstrapBalance(ctx, tenant)
	if err != nil {
		return nil, err
	}
	if balance == nil {
		return nil, billing.ErrInsufficientCredits
	}
	id := quotedReservationID(balance.ID, notif, q.Attempt)
	repo, err := s.journal()
	if err != nil {
		return nil, err
	}
	receipt, err := repo.GetReservationReceipt(ctx, id)
	if err == nil && receipt != nil {
		if receipt.Reservation.Quote == nil || receipt.Reservation.Quote.ID != q.ID || receipt.Reservation.AppID != app {
			return nil, fmt.Errorf("quoted admission changed")
		}
		saved := receipt.Reservation
		return &saved, nil
	}
	if !errors.Is(err, billing.ErrCreditReservationNotFound) {
		if err == nil {
			err = fmt.Errorf("missing reservation receipt")
		}
		return nil, err
	}
	capKey, err := s.acquireQuotedDailyCap(ctx, tenant, q.Channel, id)
	if err != nil {
		return nil, err
	}
	return s.reserveJournal(ctx, balance, tenant, app, notif, q.Channel, q.Credits, q.RateCardVersion, capKey, q)
}
func quotedReservationID(subscription, notif string, attempt int) string {
	return billing.NewJournalReservationID(subscription, fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%s\x00%d", notif, attempt)))))
}

func (s *CreditService) acquireQuotedDailyCap(ctx context.Context, tenant, ch, id string) (string, error) {
	if s.redisClient == nil {
		return "", fmt.Errorf("quoted admission budget store unavailable")
	}
	sub, err := s.subRepo.GetActiveSubscription(ctx, tenant, "", time.Now().UTC())
	if err != nil {
		return "", err
	}
	if sub == nil || !billing.IsFreeTierPlan(sub.Plan) {
		return "", nil
	}
	limit := int64(3)
	if ch == "whatsapp" {
		limit = 2
	}
	now := time.Now().UTC()
	key := dailyCapKey(tenant, ch, now)
	expiry := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC).Unix()
	result, err := s.redisClient.Eval(ctx, `if redis.call('HEXISTS',KEYS[2],ARGV[1])==1 then return 1 end; local n=tonumber(redis.call('GET',KEYS[1]) or '0'); if n>=tonumber(ARGV[2]) then return 0 end; redis.call('INCR',KEYS[1]);redis.call('EXPIREAT',KEYS[1],ARGV[3]);redis.call('HSET',KEYS[2],ARGV[1],1);redis.call('EXPIREAT',KEYS[2],ARGV[3]);return 1`, []string{key, key + ":quoted"}, id, limit, expiry).Int()
	if err != nil {
		return "", err
	}
	if result != 1 {
		return "", ErrDailyCapExceeded
	}
	return key, nil
}
func (s *CreditService) undoQuotedDailyCap(ctx context.Context, key, id string) {
	if key == "" || s.redisClient == nil {
		return
	}
	_ = s.redisClient.Eval(ctx, `if redis.call('HDEL',KEYS[2],ARGV[1])==1 then local n=tonumber(redis.call('GET',KEYS[1]) or '0');if n>0 then redis.call('DECR',KEYS[1]) end end;return 1`, []string{key, key + ":quoted"}, id).Err()
}

func (s *CreditService) recordQuotedUsage(ctx context.Context, res *billing.CreditReservation) error {
	if res.Quote == nil || res.Status != billing.CreditReservationCommitted {
		return nil
	}
	if s.usageRepo == nil {
		return fmt.Errorf("quoted settlement usage repository unavailable")
	}
	q := res.Quote
	return s.usageRepo.Store(ctx, &billing.UsageEvent{ID: billing.CreditTransitionLedgerID(res.ID, res.Status), TenantID: res.TenantID, AppID: res.AppID, NotificationID: res.NotificationID, Channel: res.Channel, Provider: q.Connection.Provider, CredentialSource: q.Connection.CredentialSource, MessageType: q.Category, CreditsUsed: res.CreditsReserved, RateCardVersion: res.RateCardVersion, Status: "charged", Timestamp: res.UpdatedAt, Quote: q})
}
