package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
	"go.uber.org/zap"
)

var (
	ErrInsufficientCredits       = billing.ErrInsufficientCredits
	ErrDailyCapExceeded          = errors.New("daily cap exceeded")
	ErrGrantTenantRequired       = errors.New("tenant id is required")
	ErrGrantInvalidAmount        = errors.New("credits amount must be greater than zero")
	ErrGrantReasonRequired       = errors.New("reason is required")
	ErrGrantNoActiveSubscription = errors.New("no active subscription")
	ErrGrantLegacyBilling        = errors.New("legacy billing model does not support credit grants")
)

type CreditUsageSnapshot struct {
	TenantID         string `json:"tenant_id"`
	CreditsTotal     int64  `json:"credits_total"`
	CreditsRemaining int64  `json:"credits_remaining"`
	CreditsReserved  int64  `json:"credits_reserved"`
	CreditsAvailable int64  `json:"credits_available"`
}

type CreditService struct {
	balanceRepo         billing.CreditBalanceRepository
	ledgerRepo          billing.CreditLedgerRepository
	subRepo             license.Repository
	usageRepo           billing.UsageRepository
	appRepo             application.Repository
	rateCardSvc         billing.RateCardManager
	redisClient         *redis.Client
	logger              *zap.Logger
	enforceCreditChecks bool

	reservationsMu             sync.Mutex
	reservations               map[string]*billing.CreditReservation
	reservationStore           reservationStore
	legacyPendingMu            sync.Mutex
	legacyPending              map[string]int64 // quota hold key -> in-flight count
	legacyHoldKeyByReservation map[string]string
}

func NewCreditService(
	balanceRepo billing.CreditBalanceRepository,
	ledgerRepo billing.CreditLedgerRepository,
	subRepo license.Repository,
	usageRepo billing.UsageRepository,
	appRepo application.Repository,
	rateCardSvc billing.RateCardManager,
	redisClient *redis.Client,
	logger *zap.Logger,
	enforceCreditChecks bool,
) *CreditService {
	return &CreditService{
		balanceRepo:         balanceRepo,
		ledgerRepo:          ledgerRepo,
		subRepo:             subRepo,
		usageRepo:           usageRepo,
		appRepo:             appRepo,
		rateCardSvc:         rateCardSvc,
		redisClient:         redisClient,
		logger:              logger,
		enforceCreditChecks: enforceCreditChecks,
		reservations:               make(map[string]*billing.CreditReservation),
		reservationStore:           newReservationStore(redisClient),
		legacyPending:              make(map[string]int64),
		legacyHoldKeyByReservation: make(map[string]string),
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
	if !s.enforceCreditChecks {
		return nil, nil
	}

	now := time.Now().UTC()
	sub, err := s.subRepo.GetActiveSubscription(ctx, tenantID, "", now)
	if err != nil {
		return nil, err
	}
	if sub != nil && billing.BillingModel(sub) == billing.BillingModelLegacy {
		return s.reserveLegacyQuota(ctx, tenantID, appID, notificationID, channel, sub)
	}
	return s.reserveCredits(ctx, tenantID, appID, notificationID, channel)
}

func (s *CreditService) reserveCredits(
	ctx context.Context,
	tenantID string,
	appID string,
	notificationID string,
	channel string,
) (*billing.CreditReservation, error) {
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

	snapshot := balance
	balance, err = s.balanceRepo.ReserveCredits(ctx, tenantID, creditsNeeded)
	if err != nil {
		s.undoDailyCap(ctx, tenantID, normalizedChannel)
		if errors.Is(err, ErrInsufficientCredits) && s.logger != nil {
			s.logger.Warn("insufficient credits",
				zap.String("tenant_id", tenantID),
				zap.String("app_id", appID),
				zap.String("notification_id", notificationID),
				zap.String("channel", normalizedChannel),
				zap.Int64("credits_needed", creditsNeeded),
				zap.Int64("credits_remaining", snapshot.CreditsRemaining),
				zap.Int64("credits_reserved", snapshot.CreditsReserved),
				zap.Int64("credits_available", snapshot.Available()),
			)
		}
		return nil, err
	}
	if balance == nil {
		s.undoDailyCap(ctx, tenantID, normalizedChannel)
		return nil, ErrInsufficientCredits
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

	if err := s.saveReservation(ctx, reservation); err != nil {
		if _, releaseErr := s.balanceRepo.ReleaseReservedCredits(ctx, tenantID, creditsNeeded); releaseErr != nil && s.logger != nil {
			s.logger.Error("credit: failed to compensate reservation after store error",
				zap.String("tenant_id", tenantID),
				zap.Error(releaseErr))
		}
		s.undoDailyCap(ctx, tenantID, normalizedChannel)
		return nil, err
	}

	return reservation, nil
}

func (s *CreditService) reserveLegacyQuota(
	ctx context.Context,
	tenantID string,
	appID string,
	notificationID string,
	channel string,
	sub *license.Subscription,
) (*billing.CreditReservation, error) {
	normalizedChannel := normalizeCreditChannel(channel)
	if err := s.enforceDailyCaps(ctx, tenantID, normalizedChannel); err != nil {
		return nil, err
	}

	var app *application.Application
	if s.appRepo != nil && appID != "" {
		app, _ = s.appRepo.GetByID(ctx, appID)
	}
	credSource := billing.InferCredentialSource(app, channel)
	if credSource == billing.CredSourceBYOC || credSource == billing.CredSourcePlatform {
		return s.newLegacyReservation(tenantID, appID, notificationID, normalizedChannel, ""), nil
	}

	plan, ok := billing.ResolveLegacyPlan(sub.Plan)
	if !ok {
		s.undoDailyCap(ctx, tenantID, normalizedChannel)
		return nil, ErrInsufficientCredits
	}

	legacyChannel := billing.LegacyBillingChannel(normalizedChannel)
	holdKey := ""

	if sub.Metadata != nil {
		if _, hasMeta := sub.Metadata["message_limit"]; hasMeta {
			unifiedLimit := billing.LegacyMessageLimit(sub, plan)
			holdKey = legacyHoldKey(tenantID, "unified")
			used, err := s.legacySystemMessageCount(ctx, tenantID, sub)
			if err != nil {
				s.undoDailyCap(ctx, tenantID, normalizedChannel)
				return nil, err
			}
			pending := s.legacyPendingCount(holdKey)
			if used+pending >= unifiedLimit {
				s.undoDailyCap(ctx, tenantID, normalizedChannel)
				return nil, ErrInsufficientCredits
			}
		}
	}

	if holdKey == "" {
		quota := plan.IncludedQuotas[legacyChannel]
		if quota <= 0 {
			// Unknown channel with no quota — allow (e.g. future channels).
			return s.newLegacyReservation(tenantID, appID, notificationID, normalizedChannel, ""), nil
		}
		holdKey = legacyHoldKey(tenantID, legacyChannel)
		used, err := s.legacyChannelSystemUsage(ctx, tenantID, sub, legacyChannel)
		if err != nil {
			s.undoDailyCap(ctx, tenantID, normalizedChannel)
			return nil, err
		}
		pending := s.legacyPendingCount(holdKey)
		if used+pending >= quota {
			s.undoDailyCap(ctx, tenantID, normalizedChannel)
			return nil, ErrInsufficientCredits
		}
	}

	s.incLegacyPending(holdKey)
	return s.newLegacyReservation(tenantID, appID, notificationID, normalizedChannel, holdKey), nil
}

func (s *CreditService) newLegacyReservation(tenantID, appID, notificationID, channel, holdKey string) *billing.CreditReservation {
	reservation := &billing.CreditReservation{
		ID:              uuid.NewString(),
		TenantID:        tenantID,
		AppID:           appID,
		NotificationID:  notificationID,
		Channel:         channel,
		CreditsReserved: 1,
		RateCardVersion: billing.RateCardVersionLegacy,
		Status:          billing.CreditReservationReserved,
		ExpiresAt:       time.Now().UTC().Add(15 * time.Minute),
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}
	s.saveReservation(context.Background(), reservation)
	if holdKey != "" {
		s.reservationsMu.Lock()
		s.legacyHoldKeyByReservation[reservation.ID] = holdKey
		s.reservationsMu.Unlock()
	}
	return reservation
}

func (s *CreditService) CommitOnSuccess(ctx context.Context, reservationID string) (*billing.CreditReservation, error) {
	reservation, ok := s.loadReservation(ctx, reservationID)
	if !ok {
		return nil, nil
	}
	if reservation.Status != billing.CreditReservationReserved {
		return reservation, nil
	}
	if reservation.RateCardVersion == billing.RateCardVersionLegacy {
		s.releaseLegacyHold(reservationID)
		reservation.Status = billing.CreditReservationCommitted
		reservation.UpdatedAt = time.Now().UTC()
		s.deleteReservation(ctx, reservationID)
		return reservation, nil
	}

	balance, err := s.balanceRepo.CommitReservedCredits(ctx, reservation.TenantID, reservation.CreditsReserved)
	if err != nil {
		return nil, err
	}
	if balance == nil {
		return nil, fmt.Errorf("credit: balance not found during commit")
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
	s.deleteReservation(ctx, reservationID)
	return reservation, nil
}

func (s *CreditService) ReleaseOnFailure(ctx context.Context, reservationID string, reason string) error {
	reservation, ok := s.loadReservation(ctx, reservationID)
	if !ok {
		return nil
	}
	if reservation.Status != billing.CreditReservationReserved {
		s.deleteReservation(ctx, reservationID)
		return nil
	}

	if reservation.RateCardVersion == billing.RateCardVersionLegacy {
		s.releaseLegacyHold(reservationID)
		reservation.Status = billing.CreditReservationReleased
		reservation.UpdatedAt = time.Now().UTC()
		s.deleteReservation(ctx, reservationID)
		s.undoDailyCap(ctx, reservation.TenantID, reservation.Channel)
		return nil
	}

	balance, err := s.balanceRepo.ReleaseReservedCredits(ctx, reservation.TenantID, reservation.CreditsReserved)
	if err != nil {
		return err
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
	s.deleteReservation(ctx, reservationID)
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
		CreditsAvailable: balance.Available(),
	}, nil
}

func (s *CreditService) ReapExpiredReservations(ctx context.Context) (int, error) {
	if s.reservationStore == nil {
		return 0, nil
	}
	expired := s.reservationStore.ListExpired(ctx, time.Now().UTC())
	released := 0
	for _, res := range expired {
		if res == nil {
			continue
		}
		if err := s.ReleaseOnFailure(ctx, res.ID, "reservation_expired"); err != nil {
			if s.logger != nil {
				s.logger.Error("credit: failed to reap expired reservation",
					zap.String("reservation_id", res.ID),
					zap.String("tenant_id", res.TenantID),
					zap.Error(err))
			}
			continue
		}
		released++
	}
	if released > 0 && s.logger != nil {
		s.logger.Warn("credit: reaped expired reservations", zap.Int("count", released))
	}
	return released, nil
}

func (s *CreditService) ResetReservedCredits(ctx context.Context, tenantID, reason string) (*CreditUsageSnapshot, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, ErrGrantTenantRequired
	}
	if strings.TrimSpace(reason) == "" {
		return nil, ErrGrantReasonRequired
	}

	before, err := s.balanceRepo.GetByTenantID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if before == nil {
		return nil, ErrGrantNoActiveSubscription
	}

	balance, err := s.balanceRepo.ClearReservedCredits(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if balance == nil {
		return nil, ErrGrantNoActiveSubscription
	}

	entry := &billing.CreditLedgerEntry{
		ID:           uuid.NewString(),
		TenantID:     tenantID,
		EntryType:    billing.CreditLedgerAdjust,
		CreditsDelta: 0,
		BalanceAfter: balance.CreditsRemaining,
		Metadata: map[string]interface{}{
			"reason":                  reason,
			"source":                  "ops_reset_reserved",
			"credits_reserved_before": before.CreditsReserved,
			"credits_reserved_after":  int64(0),
		},
		CreatedAt: time.Now().UTC(),
	}
	if s.rateCardSvc != nil {
		entry.RateCardVersion = s.rateCardSvc.GetRateCardVersion()
	}
	if err := s.ledgerRepo.Append(ctx, entry); err != nil {
		return nil, err
	}

	return &CreditUsageSnapshot{
		TenantID:         tenantID,
		CreditsTotal:     balance.CreditsTotal,
		CreditsRemaining: balance.CreditsRemaining,
		CreditsReserved:  balance.CreditsReserved,
		CreditsAvailable: balance.Available(),
	}, nil
}

// GrantCredits adds credits to a tenant wallet (ops / manual top-up).
func (s *CreditService) GrantCredits(ctx context.Context, tenantID string, amount int64, reason string, metadata map[string]interface{}) (*CreditUsageSnapshot, error) {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil, ErrGrantTenantRequired
	}
	if amount <= 0 {
		return nil, ErrGrantInvalidAmount
	}
	if strings.TrimSpace(reason) == "" {
		return nil, ErrGrantReasonRequired
	}

	now := time.Now().UTC()
	sub, err := s.subRepo.GetActiveSubscription(ctx, tenantID, "", now)
	if err != nil {
		return nil, err
	}
	if sub == nil {
		return nil, ErrGrantNoActiveSubscription
	}
	if billing.BillingModel(sub) == billing.BillingModelLegacy {
		return nil, ErrGrantLegacyBilling
	}

	balance, err := s.balanceRepo.GetByTenantID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if balance == nil {
		balance, err = s.getOrBootstrapBalance(ctx, tenantID)
		if err != nil {
			return nil, err
		}
	}
	if balance == nil {
		balance = &billing.CreditBalance{
			ID:               sub.ID,
			TenantID:         tenantID,
			CreditsTotal:     sub.CreditsTotal,
			CreditsRemaining: sub.CreditsRemaining,
			CreditsReserved:  sub.CreditsReserved,
		}
		if sub.CreditsExpireAt != nil {
			balance.CreditsExpireAt = sub.CreditsExpireAt.UTC()
		}
	}

	balance.CreditsTotal += amount
	balance.CreditsRemaining += amount

	if err := s.balanceRepo.Upsert(ctx, balance); err != nil {
		return nil, err
	}

	entryMeta := map[string]interface{}{
		"reason": reason,
		"source": "ops_cli",
	}
	for k, v := range metadata {
		entryMeta[k] = v
	}

	entry := &billing.CreditLedgerEntry{
		ID:           uuid.NewString(),
		TenantID:     tenantID,
		EntryType:    billing.CreditLedgerAdjust,
		CreditsDelta: amount,
		BalanceAfter: balance.CreditsRemaining,
		Metadata:     entryMeta,
		CreatedAt:    now,
	}
	if s.rateCardSvc != nil {
		entry.RateCardVersion = s.rateCardSvc.GetRateCardVersion()
	}
	if err := s.ledgerRepo.Append(ctx, entry); err != nil {
		return nil, err
	}

	return &CreditUsageSnapshot{
		TenantID:         tenantID,
		CreditsTotal:     balance.CreditsTotal,
		CreditsRemaining: balance.CreditsRemaining,
		CreditsReserved:  balance.CreditsReserved,
		CreditsAvailable: balance.Available(),
	}, nil
}

func (s *CreditService) getOrBootstrapBalance(ctx context.Context, tenantID string) (*billing.CreditBalance, error) {
	balance, err := s.balanceRepo.GetByTenantID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if balance == nil {
		return nil, nil
	}

	now := time.Now().UTC()
	sub, err := s.subRepo.GetActiveSubscription(ctx, tenantID, "", now)
	if err != nil {
		return nil, err
	}
	if sub == nil {
		return balance, nil
	}
	if billing.BillingModel(sub) == billing.BillingModelLegacy {
		return nil, nil
	}

	if sub.CreditsTotal > 0 || sub.CreditsRemaining > 0 {
		return balance, nil
	}

	plan, ok := billing.ResolvePlan(sub.Plan)
	if !ok || plan.CreditsIncluded <= 0 {
		return nil, nil
	}
	creditsTotal := plan.CreditsIncluded
	expiry := now.AddDate(1, 0, 0)
	if sub.CreditsExpireAt != nil && !sub.CreditsExpireAt.IsZero() {
		expiry = *sub.CreditsExpireAt
	}
	balance.CreditsTotal = creditsTotal
	balance.CreditsRemaining = creditsTotal
	balance.CreditsReserved = 0
	balance.CreditsExpireAt = expiry
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
	if !billing.IsFreeTierPlan(sub.Plan) {
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

func legacyHoldKey(tenantID, scope string) string {
	return tenantID + ":" + scope
}

func (s *CreditService) incLegacyPending(key string) {
	if key == "" {
		return
	}
	s.legacyPendingMu.Lock()
	s.legacyPending[key]++
	s.legacyPendingMu.Unlock()
}

func (s *CreditService) legacyPendingCount(key string) int64 {
	if key == "" {
		return 0
	}
	s.legacyPendingMu.Lock()
	defer s.legacyPendingMu.Unlock()
	return s.legacyPending[key]
}

func (s *CreditService) releaseLegacyHold(reservationID string) {
	s.reservationsMu.Lock()
	holdKey := s.legacyHoldKeyByReservation[reservationID]
	delete(s.legacyHoldKeyByReservation, reservationID)
	s.reservationsMu.Unlock()

	if holdKey == "" {
		return
	}
	s.legacyPendingMu.Lock()
	if s.legacyPending[holdKey] > 0 {
		s.legacyPending[holdKey]--
	}
	if s.legacyPending[holdKey] <= 0 {
		delete(s.legacyPending, holdKey)
	}
	s.legacyPendingMu.Unlock()
}

func (s *CreditService) legacySystemMessageCount(ctx context.Context, tenantID string, sub *license.Subscription) (int64, error) {
	if s.usageRepo == nil || s.appRepo == nil {
		return legacyMessagesSentFromMeta(sub), nil
	}
	apps, err := s.appRepo.List(ctx, application.ApplicationFilter{AdminUserID: tenantID})
	if err != nil {
		return legacyMessagesSentFromMeta(sub), nil
	}
	appIDs := make([]string, 0, len(apps))
	for _, a := range apps {
		appIDs = append(appIDs, a.AppID)
	}
	summaries, err := s.usageRepo.GetSummary(ctx, appIDs, sub.CurrentPeriodStart, sub.CurrentPeriodEnd)
	if err != nil {
		return legacyMessagesSentFromMeta(sub), nil
	}
	var total int64
	for _, u := range summaries {
		if u.CredentialSource == billing.CredSourceSystem {
			total += u.MessageCount
		}
	}
	if total == 0 {
		return legacyMessagesSentFromMeta(sub), nil
	}
	return total, nil
}

func (s *CreditService) legacyChannelSystemUsage(ctx context.Context, tenantID string, sub *license.Subscription, legacyChannel string) (int64, error) {
	if s.usageRepo == nil || s.appRepo == nil {
		return 0, nil
	}
	apps, err := s.appRepo.List(ctx, application.ApplicationFilter{AdminUserID: tenantID})
	if err != nil {
		return 0, err
	}
	appIDs := make([]string, 0, len(apps))
	for _, a := range apps {
		appIDs = append(appIDs, a.AppID)
	}
	summaries, err := s.usageRepo.GetSummary(ctx, appIDs, sub.CurrentPeriodStart, sub.CurrentPeriodEnd)
	if err != nil {
		return 0, err
	}
	var total int64
	for _, u := range summaries {
		if u.CredentialSource != billing.CredSourceSystem {
			continue
		}
		ch := billing.LegacyBillingChannel(u.Channel)
		if ch == legacyChannel {
			total += u.MessageCount
		}
	}
	return total, nil
}

func legacyMessagesSentFromMeta(sub *license.Subscription) int64 {
	if sub == nil || sub.Metadata == nil {
		return 0
	}
	v, ok := sub.Metadata["messages_sent"]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case int:
		return int64(n)
	case int64:
		return n
	case float64:
		return int64(n)
	default:
		return 0
	}
}

func (s *CreditService) currentRateCardVersion() string {
	if s.rateCardSvc == nil {
		return "default"
	}
	return s.rateCardSvc.GetRateCardVersion()
}

func (s *CreditService) loadReservation(ctx context.Context, reservationID string) (*billing.CreditReservation, bool) {
	s.reservationsMu.Lock()
	res, ok := s.reservations[reservationID]
	s.reservationsMu.Unlock()
	if ok {
		return res, true
	}
	if s.reservationStore == nil {
		return nil, false
	}
	return s.reservationStore.Get(ctx, reservationID)
}

func (s *CreditService) saveReservation(ctx context.Context, reservation *billing.CreditReservation) error {
	if reservation == nil {
		return nil
	}
	s.reservationsMu.Lock()
	s.reservations[reservation.ID] = reservation
	s.reservationsMu.Unlock()
	if s.reservationStore == nil {
		return nil
	}
	return s.reservationStore.Save(ctx, reservation)
}

func (s *CreditService) deleteReservation(ctx context.Context, reservationID string) {
	s.reservationsMu.Lock()
	delete(s.reservations, reservationID)
	delete(s.legacyHoldKeyByReservation, reservationID)
	s.reservationsMu.Unlock()
	if s.reservationStore != nil {
		s.reservationStore.Delete(ctx, reservationID)
	}
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
