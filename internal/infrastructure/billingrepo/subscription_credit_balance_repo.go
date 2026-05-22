package billingrepo

import (
	"context"
	"fmt"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
	"go.uber.org/zap"
)

// SubscriptionCreditBalanceRepo implements billing.CreditBalanceRepository by
// reading and updating the active subscription document for a tenant.
type SubscriptionCreditBalanceRepo struct {
	subRepo license.Repository
	logger  *zap.Logger
}

func NewSubscriptionCreditBalanceRepo(subRepo license.Repository, logger *zap.Logger) *SubscriptionCreditBalanceRepo {
	return &SubscriptionCreditBalanceRepo{subRepo: subRepo, logger: logger}
}

func (r *SubscriptionCreditBalanceRepo) GetByTenantID(ctx context.Context, tenantID string) (*billing.CreditBalance, error) {
	if tenantID == "" {
		return nil, nil
	}
	now := time.Now().UTC()
	sub, err := r.subRepo.GetActiveSubscription(ctx, tenantID, "", now)
	if err != nil {
		return nil, fmt.Errorf("billingrepo: get active subscription for credit balance: %w", err)
	}
	if sub == nil {
		return nil, nil
	}
	return subscriptionToCreditBalance(sub), nil
}

func (r *SubscriptionCreditBalanceRepo) Upsert(ctx context.Context, balance *billing.CreditBalance) error {
	if balance == nil {
		return fmt.Errorf("billingrepo: nil credit balance")
	}
	if balance.TenantID == "" {
		return fmt.Errorf("billingrepo: tenant_id is required")
	}
	now := time.Now().UTC()
	sub, err := r.subRepo.GetActiveSubscription(ctx, balance.TenantID, "", now)
	if err != nil {
		return fmt.Errorf("billingrepo: load subscription for credit upsert: %w", err)
	}
	if sub == nil {
		return fmt.Errorf("billingrepo: no active subscription for tenant %s", balance.TenantID)
	}

	sub.CreditsTotal = balance.CreditsTotal
	sub.CreditsRemaining = balance.CreditsRemaining
	sub.CreditsReserved = balance.CreditsReserved
	if !balance.CreditsExpireAt.IsZero() {
		exp := balance.CreditsExpireAt.UTC()
		sub.CreditsExpireAt = &exp
	}

	if err := r.subRepo.Update(ctx, sub); err != nil {
		return fmt.Errorf("billingrepo: update subscription credits: %w", err)
	}

	r.logger.Debug("Updated subscription credit fields",
		zap.String("subscription_id", sub.ID),
		zap.String("tenant_id", balance.TenantID),
		zap.Int64("credits_remaining", balance.CreditsRemaining),
		zap.Int64("credits_reserved", balance.CreditsReserved),
	)

	return nil
}

func subscriptionToCreditBalance(sub *license.Subscription) *billing.CreditBalance {
	if sub == nil {
		return nil
	}
	var exp time.Time
	if sub.CreditsExpireAt != nil {
		exp = *sub.CreditsExpireAt
	}
	return &billing.CreditBalance{
		ID:               sub.ID,
		TenantID:         sub.TenantID,
		CreditsTotal:     sub.CreditsTotal,
		CreditsRemaining: sub.CreditsRemaining,
		CreditsReserved:  sub.CreditsReserved,
		CreditsExpireAt:  exp,
		CreatedAt:        sub.CreatedAt,
		UpdatedAt:        sub.UpdatedAt,
	}
}
