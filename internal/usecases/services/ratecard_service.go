package services

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"go.uber.org/zap"
)

const defaultRateCardPubSubChannel = "billing:ratecard:updated"

type RateCardServiceConfig struct {
	RefreshInterval time.Duration
	PubSubChannel   string
}

type RateCardService struct {
	repo            billing.RateCardRepository
	redisClient     *redis.Client
	logger          *zap.Logger
	refreshInterval time.Duration
	pubSubChannel   string

	mu         sync.RWMutex
	activeCard *billing.RateCard

	cancel context.CancelFunc
}

func NewRateCardService(
	repo billing.RateCardRepository,
	redisClient *redis.Client,
	logger *zap.Logger,
	cfg RateCardServiceConfig,
) *RateCardService {
	refreshInterval := cfg.RefreshInterval
	if refreshInterval <= 0 {
		refreshInterval = 45 * time.Second
	}
	channel := strings.TrimSpace(cfg.PubSubChannel)
	if channel == "" {
		channel = defaultRateCardPubSubChannel
	}

	return &RateCardService{
		repo:            repo,
		redisClient:     redisClient,
		logger:          logger,
		refreshInterval: refreshInterval,
		pubSubChannel:   channel,
	}
}

func (s *RateCardService) Start(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	serviceCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	if err := s.RefreshActiveRateCard(serviceCtx); err != nil {
		s.logger.Warn("ratecard: initial refresh failed", zap.Error(err))
	}

	go s.refreshLoop(serviceCtx)
	go s.subscribeInvalidations(serviceCtx)
}

func (s *RateCardService) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *RateCardService) GetActiveRateCard() *billing.RateCard {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.activeCard == nil {
		return nil
	}
	cp := *s.activeCard
	cp.ChannelCreditCost = cloneMap(s.activeCard.ChannelCreditCost)
	cp.OveragePerMessage = cloneMap(s.activeCard.OveragePerMessage)
	return &cp
}

func (s *RateCardService) GetRateCardVersion() string {
	card := s.GetActiveRateCard()
	if card == nil {
		return "default"
	}
	return card.Version
}

func (s *RateCardService) GetChannelCreditCost(channel string) int64 {
	normalized := normalizeBillingChannel(channel)

	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.activeCard != nil {
		if credits, ok := s.activeCard.ChannelCreditCost[normalized]; ok && credits > 0 {
			return credits
		}
	}

	// Safe fallback for channels not explicitly mapped in a dynamic card.
	switch normalized {
	case "email":
		return 3
	case "sms":
		return 80
	case "whatsapp":
		return 108
	case "inapp", "webhook", "sse":
		return 1
	default:
		return 1
	}
}

func (s *RateCardService) RefreshActiveRateCard(ctx context.Context) error {
	card, err := s.repo.GetActive(ctx)
	if err != nil {
		return fmt.Errorf("ratecard: get active card: %w", err)
	}
	if card == nil {
		card = s.bootstrapDefaultRateCard(ctx)
	}
	if card == nil {
		return nil
	}

	s.mu.Lock()
	s.activeCard = card
	s.mu.Unlock()
	return nil
}

func (s *RateCardService) ActivateVersion(ctx context.Context, version string) error {
	if strings.TrimSpace(version) == "" {
		return fmt.Errorf("ratecard: version is required")
	}
	if err := s.repo.SetActiveVersion(ctx, version); err != nil {
		return err
	}
	if err := s.publishInvalidation(ctx, version); err != nil {
		s.logger.Warn("ratecard: failed to publish invalidation after activate", zap.Error(err))
	}
	return s.RefreshActiveRateCard(ctx)
}

func (s *RateCardService) UpdateChannelCredits(ctx context.Context, channel string, credits int64) (*billing.RateCard, error) {
	if credits <= 0 {
		return nil, fmt.Errorf("ratecard: credits must be > 0")
	}
	normalized := normalizeBillingChannel(channel)

	current := s.GetActiveRateCard()
	if current == nil {
		if err := s.RefreshActiveRateCard(ctx); err != nil {
			return nil, err
		}
		current = s.GetActiveRateCard()
	}
	if current == nil {
		return nil, fmt.Errorf("ratecard: no active card available")
	}

	next := &billing.RateCard{
		Version:           fmt.Sprintf("v%d", time.Now().UTC().UnixNano()),
		Active:            false,
		CreditValueINR:    current.CreditValueINR,
		ChannelCreditCost: cloneMap(current.ChannelCreditCost),
		OveragePerMessage: cloneMap(current.OveragePerMessage),
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
	}
	next.ChannelCreditCost[normalized] = credits

	if err := s.repo.CreateVersion(ctx, next); err != nil {
		return nil, err
	}
	if err := s.ActivateVersion(ctx, next.Version); err != nil {
		return nil, err
	}
	return s.GetActiveRateCard(), nil
}

func (s *RateCardService) refreshLoop(ctx context.Context) {
	ticker := time.NewTicker(s.refreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.RefreshActiveRateCard(ctx); err != nil {
				s.logger.Warn("ratecard: periodic refresh failed", zap.Error(err))
			}
		}
	}
}

func (s *RateCardService) subscribeInvalidations(ctx context.Context) {
	if s.redisClient == nil {
		return
	}
	pubsub := s.redisClient.Subscribe(ctx, s.pubSubChannel)
	defer pubsub.Close()

	ch := pubsub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-ch:
			if !ok {
				return
			}
			if err := s.RefreshActiveRateCard(ctx); err != nil {
				s.logger.Warn("ratecard: refresh after invalidation failed", zap.Error(err))
			}
		}
	}
}

func (s *RateCardService) publishInvalidation(ctx context.Context, version string) error {
	if s.redisClient == nil {
		return nil
	}
	return s.redisClient.Publish(ctx, s.pubSubChannel, version).Err()
}

func (s *RateCardService) bootstrapDefaultRateCard(ctx context.Context) *billing.RateCard {
	defaultPlans := billing.DefaultRates()
	proPlan := defaultPlans["pro"]
	if len(proPlan.ChannelCreditCost) == 0 {
		return nil
	}

	card := &billing.RateCard{
		Version:           "v1",
		Active:            true,
		CreditValueINR:    proPlan.CreditValueINR,
		ChannelCreditCost: cloneMap(proPlan.ChannelCreditCost),
		OveragePerMessage: cloneMap(proPlan.OveragePerMessage),
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
	}
	if err := s.repo.CreateVersion(ctx, card); err != nil {
		s.logger.Warn("ratecard: failed to create bootstrap card", zap.Error(err))
		return nil
	}
	if err := s.repo.SetActiveVersion(ctx, card.Version); err != nil {
		s.logger.Warn("ratecard: failed to activate bootstrap card", zap.Error(err))
		return nil
	}
	return card
}

func normalizeBillingChannel(channel string) string {
	ch := strings.ToLower(strings.TrimSpace(channel))
	switch ch {
	case "in_app", "inapp", "push":
		return "inapp"
	case "slack", "discord", "teams", "custom":
		return "webhook"
	default:
		return ch
	}
}

func cloneMap(src map[string]int64) map[string]int64 {
	dst := make(map[string]int64, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
