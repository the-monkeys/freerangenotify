package services

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
	"go.uber.org/zap"
)

var (
	ErrInsufficientCredits = errors.New("insufficient credits")
	ErrDailyCapExceeded    = errors.New("daily cap exceeded")
)

type CreditUsageSnapshot struct {
	TenantID         string `json:"tenant_id"`
	CreditsTotal     int64  `json:"credits_total"`
	CreditsRemaining int64  `json:"credits_remaining"`
	CreditsReserved  int64  `json:"credits_reserved"`
}

type CreditService struct {
	balanceRepo billing.CreditBalanceRepository
	ledgerRepo  billing.CreditLedgerRepository
	subRepo     license.Repository
	rateCardSvc billing.RateCardManager
	redisClient *redis.Client
	logger      *zap.Logger

	reservationsMu sync.Mutex
	reservations   map[string]*billing.CreditReservation
}

func NewCreditService(
	balanceRepo billing.CreditBalanceRepository,
	ledgerRepo billing.CreditLedgerRepository,
	subRepo license.Repository,
	rateCardSvc billing.RateCardManager,
	redisClient *redis.Client,
	logger *zap.Logger,
) *CreditService {
	return &CreditService{
		balanceRepo:   balanceRepo,
		ledgerRepo:    ledgerRepo,
		subRepo:       subRepo,
		rateCardSvc:   rateCardSvc,
		redisClient:   redisClient,
		logger:        logger,
		reservations:  make(map[string]*billing.CreditReservation),
	}
}

func (s *CreditService) ReserveForNotification(
	ctx context.Context,
	tenantID string,
	appID string,
	notificationID string,
	channel string,
) (*billing.CreditReservation, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("credit: tenant id is required")
	}

	creditsNeeded := int64(1)
	if s.rateCardSvc != nil {
		creditsNeeded = s.rateCardSvc.GetChannelCreditCost(channel)
	}
	if creditsNeeded <= 0 {
		creditsNeeded = 1
	}

	normalizedChannel := normalizeCreditChannel(channel)
	if err := s.enforceDailyCaps(ctx, tenantID, normalizedChannel); err != nil {
		return nil, err
	}

	balance, err := s.getOrBootstrapBalance(ctx, tenantID)
	if err != nil {
		s.undoDailyCap(ctx, tenantID, normalizedChannel)
		return nil, err
	}
	if balance == nil {
		s.undoDailyCap(ctx, tenantID, normalizedChannel)
		return nil, ErrInsufficientCredits
	}

	available := balance.CreditsRemaining - balance.CreditsReserved
	if available < creditsNeeded {
		s.undoDailyCap(ctx, tenantID, normalizedChannel)
		return nil, ErrInsufficientCredits
	}

	balance.CreditsReserved += creditsNeeded
	if err := s.balanceRepo.Upsert(ctx, balance); err != nil {
		s.undoDailyCap(ctx, tenantID, normalizedChannel)
		return nil, err
	}

	reservation := &billing.CreditReservation{
		ID:              uuid.NewString(),
		TenantID:        tenantID,
		AppID:           appID,
		NotificationID:  notificationID,
		Channel:         normalizedChannel,
		CreditsReserved: creditsNeeded,
		RateCardVersion: s.currentRateCardVersion(),
		Status:          billing.CreditReservationReserved,
		ExpiresAt:       time.Now().UTC().Add(15 * time.Minute),
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}

	s.reservationsMu.Lock()
	s.reservations[reservation.ID] = reservation
	s.reservationsMu.Unlock()

	return reservation, nil
}

func (s *CreditService) CommitOnSuccess(ctx context.Context, reservationID string) (*billing.CreditReservation, error) {
	reservation, ok := s.loadReservation(reservationID)
	if !ok {
		return nil, nil
	}
	if reservation.Status != billing.CreditReservationReserved {
		return reservation, nil
	}

	balance, err := s.balanceRepo.GetByTenantID(ctx, reservation.TenantID)
	if err != nil || balance == nil {
		return nil, fmt.Errorf("credit: balance not found during commit: %w", err)
	}

	if balance.CreditsReserved < reservation.CreditsReserved {
		balance.CreditsReserved = 0
	} else {
		balance.CreditsReserved -= reservation.CreditsReserved
	}
	if balance.CreditsRemaining < reservation.CreditsReserved {
		return nil, ErrInsufficientCredits
	}
	balance.CreditsRemaining -= reservation.CreditsReserved

	if err := s.balanceRepo.Upsert(ctx, balance); err != nil {
		return nil, err
	}

	entry := &billing.CreditLedgerEntry{
		ID:              uuid.NewString(),
		TenantID:        reservation.TenantID,
		AppID:           reservation.AppID,
		ReservationID:   reservation.ID,
		NotificationID:  reservation.NotificationID,
		Channel:         reservation.Channel,
		EntryType:       billing.CreditLedgerBurn,
		CreditsDelta:    -reservation.CreditsReserved,
		BalanceAfter:    balance.CreditsRemaining,
		RateCardVersion: reservation.RateCardVersion,
		CreatedAt:       time.Now().UTC(),
	}
	if err := s.ledgerRepo.Append(ctx, entry); err != nil {
		return nil, err
	}

	reservation.Status = billing.CreditReservationCommitted
	reservation.UpdatedAt = time.Now().UTC()
	s.deleteReservation(reservation.ID)
	return reservation, nil
}

func (s *CreditService) ReleaseOnFailure(ctx context.Context, reservationID string, reason string) error {
	reservation, ok := s.loadReservation(reservationID)
	if !ok {
		return nil
	}
	if reservation.Status != billing.CreditReservationReserved {
		s.deleteReservation(reservationID)
		return nil
	}

	balance, err := s.balanceRepo.GetByTenantID(ctx, reservation.TenantID)
	if err != nil {
		return err
	}
	if balance != nil {
		if balance.CreditsReserved < reservation.CreditsReserved {
			balance.CreditsReserved = 0
		} else {
			balance.CreditsReserved -= reservation.CreditsReserved
		}
		if err := s.balanceRepo.Upsert(ctx, balance); err != nil {
			return err
		}
	}

	entry := &billing.CreditLedgerEntry{
		ID:              uuid.NewString(),
		TenantID:        reservation.TenantID,
		AppID:           reservation.AppID,
		ReservationID:   reservation.ID,
		NotificationID:  reservation.NotificationID,
		Channel:         reservation.Channel,
		EntryType:       billing.CreditLedgerRelease,
		CreditsDelta:    0,
		BalanceAfter:    0,
		RateCardVersion: reservation.RateCardVersion,
		Metadata:        map[string]interface{}{"reason": reason},
		CreatedAt:       time.Now().UTC(),
	}
	if balance != nil {
		entry.BalanceAfter = balance.CreditsRemaining
	}
	if err := s.ledgerRepo.Append(ctx, entry); err != nil {
		return err
	}

	reservation.Status = billing.CreditReservationReleased
	reservation.UpdatedAt = time.Now().UTC()
	s.deleteReservation(reservationID)
	s.undoDailyCap(ctx, reservation.TenantID, reservation.Channel)
	return nil
}

func (s *CreditService) GetUsageSnapshot(ctx context.Context, tenantID string) (*CreditUsageSnapshot, error) {
	balance, err := s.balanceRepo.GetByTenantID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if balance == nil {
		return &CreditUsageSnapshot{TenantID: tenantID}, nil
	}
	return &CreditUsageSnapshot{
		TenantID:         tenantID,
		CreditsTotal:     balance.CreditsTotal,
		CreditsRemaining: balance.CreditsRemaining,
		CreditsReserved:  balance.CreditsReserved,
	}, nil
}

func (s *CreditService) getOrBootstrapBalance(ctx context.Context, tenantID string) (*billing.CreditBalance, error) {
	balance, err := s.balanceRepo.GetByTenantID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if balance != nil {
		return balance, nil
	}

	now := time.Now().UTC()
	sub, err := s.subRepo.GetActiveSubscription(ctx, tenantID, "", now)
	if err != nil || sub == nil {
		return nil, err
	}

	plan := billing.DefaultRates()[sub.Plan]
	if plan.Name == "" {
		plan = billing.DefaultRates()["free"]
	}
	creditsTotal := plan.CreditsIncluded
	creditsRemaining := creditsTotal
	if sub.CreditsTotal > 0 {
		creditsTotal = sub.CreditsTotal
	}
	if sub.CreditsRemaining > 0 {
		creditsRemaining = sub.CreditsRemaining
	}

	expiry := now.AddDate(1, 0, 0)
	if sub.CreditsExpireAt != nil && !sub.CreditsExpireAt.IsZero() {
		expiry = *sub.CreditsExpireAt
	}
	balance = &billing.CreditBalance{
		ID:               tenantID,
		TenantID:         tenantID,
		CreditsTotal:     creditsTotal,
		CreditsRemaining: creditsRemaining,
		CreditsReserved:  0,
		CreditsExpireAt:  expiry,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.balanceRepo.Upsert(ctx, balance); err != nil {
		return nil, err
	}
	return balance, nil
}

func (s *CreditService) enforceDailyCaps(ctx context.Context, tenantID, channel string) error {
	if s.redisClient == nil {
		return nil
	}
	now := time.Now().UTC()
	sub, err := s.subRepo.GetActiveSubscription(ctx, tenantID, "", now)
	if err != nil || sub == nil {
		return nil
	}
	if sub.Plan != "free" {
		return nil
	}

	limit := int64(0)
	switch channel {
	case "whatsapp":
		limit = 2
	case "sms":
		limit = 3
	default:
		return nil
	}

	key := dailyCapKey(tenantID, channel, now)
	current, err := s.redisClient.Incr(ctx, key).Result()
	if err != nil {
		return err
	}
	if current == 1 {
		endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, time.UTC)
		_ = s.redisClient.ExpireAt(ctx, key, endOfDay).Err()
	}
	if current > limit {
		_ = s.redisClient.Decr(ctx, key).Err()
		return ErrDailyCapExceeded
	}
	return nil
}

func (s *CreditService) undoDailyCap(ctx context.Context, tenantID, channel string) {
	if s.redisClient == nil {
		return
	}
	if channel != "whatsapp" && channel != "sms" {
		return
	}
	_ = s.redisClient.Decr(ctx, dailyCapKey(tenantID, channel, time.Now().UTC())).Err()
}

func dailyCapKey(tenantID, channel string, t time.Time) string {
	return fmt.Sprintf("billing:dailycap:%s:%s:%s", tenantID, channel, t.Format("20060102"))
}

func (s *CreditService) currentRateCardVersion() string {
	if s.rateCardSvc == nil {
		return "default"
	}
	return s.rateCardSvc.GetRateCardVersion()
}

func (s *CreditService) loadReservation(reservationID string) (*billing.CreditReservation, bool) {
	s.reservationsMu.Lock()
	defer s.reservationsMu.Unlock()
	res, ok := s.reservations[reservationID]
	return res, ok
}

func (s *CreditService) deleteReservation(reservationID string) {
	s.reservationsMu.Lock()
	defer s.reservationsMu.Unlock()
	delete(s.reservations, reservationID)
}

func normalizeCreditChannel(channel string) string {
	switch channel {
	case "in_app", "inapp", "push":
		return "inapp"
	case "slack", "discord", "teams", "custom":
		return "webhook"
	default:
		return channel
	}
}
