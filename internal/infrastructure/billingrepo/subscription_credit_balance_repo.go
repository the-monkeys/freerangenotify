package billingrepo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
	"go.uber.org/zap"
)

type creditScriptUpdater interface {
	ScriptUpdate(ctx context.Context, id string, script map[string]interface{}) error
}

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

func (r *SubscriptionCreditBalanceRepo) ReserveCredits(ctx context.Context, tenantID string, amount int64) (*billing.CreditBalance, error) {
	return r.applyCreditScript(ctx, tenantID, creditReserveScript, map[string]interface{}{
		"amount": amount,
		"now":    time.Now().UTC().Format(time.RFC3339),
	})
}

func (r *SubscriptionCreditBalanceRepo) CommitReservedCredits(ctx context.Context, tenantID string, amount int64) (*billing.CreditBalance, error) {
	return r.applyCreditScript(ctx, tenantID, creditCommitScript, map[string]interface{}{
		"amount": amount,
		"now":    time.Now().UTC().Format(time.RFC3339),
	})
}

func (r *SubscriptionCreditBalanceRepo) ReleaseReservedCredits(ctx context.Context, tenantID string, amount int64) (*billing.CreditBalance, error) {
	return r.applyCreditScript(ctx, tenantID, creditReleaseScript, map[string]interface{}{
		"amount": amount,
		"now":    time.Now().UTC().Format(time.RFC3339),
	})
}

func (r *SubscriptionCreditBalanceRepo) ClearReservedCredits(ctx context.Context, tenantID string) (*billing.CreditBalance, error) {
	return r.applyCreditScript(ctx, tenantID, creditClearReservedScript, map[string]interface{}{
		"now": time.Now().UTC().Format(time.RFC3339),
	})
}

const (
	creditReserveScript = `
if (ctx._source.credits_remaining == null) { ctx._source.credits_remaining = 0; }
if (ctx._source.credits_reserved == null) { ctx._source.credits_reserved = 0; }
if (ctx._source.credits_remaining - ctx._source.credits_reserved < params.amount) {
  throw new IllegalArgumentException('insufficient credits');
}
ctx._source.credits_reserved += params.amount;
ctx._source.updated_at = params.now;
`
	creditCommitScript = `
if (ctx._source.credits_remaining == null) { ctx._source.credits_remaining = 0; }
if (ctx._source.credits_reserved == null) { ctx._source.credits_reserved = 0; }
if (ctx._source.credits_reserved < params.amount) {
  ctx._source.credits_reserved = 0;
} else {
  ctx._source.credits_reserved -= params.amount;
}
if (ctx._source.credits_remaining < params.amount) {
  throw new IllegalArgumentException('insufficient credits');
}
ctx._source.credits_remaining -= params.amount;
ctx._source.updated_at = params.now;
`
	creditReleaseScript = `
if (ctx._source.credits_reserved == null) { ctx._source.credits_reserved = 0; }
if (ctx._source.credits_reserved < params.amount) {
  ctx._source.credits_reserved = 0;
} else {
  ctx._source.credits_reserved -= params.amount;
}
ctx._source.updated_at = params.now;
`
	creditClearReservedScript = `
ctx._source.credits_reserved = 0;
ctx._source.updated_at = params.now;
`
)

func (r *SubscriptionCreditBalanceRepo) applyCreditScript(ctx context.Context, tenantID, source string, params map[string]interface{}) (*billing.CreditBalance, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("billingrepo: tenant_id is required")
	}
	now := time.Now().UTC()
	sub, err := r.subRepo.GetActiveSubscription(ctx, tenantID, "", now)
	if err != nil {
		return nil, fmt.Errorf("billingrepo: get active subscription for credit script: %w", err)
	}
	if sub == nil {
		return nil, fmt.Errorf("billingrepo: no active subscription for tenant %s", tenantID)
	}

	updater, ok := r.subRepo.(creditScriptUpdater)
	if !ok {
		return nil, fmt.Errorf("billingrepo: subscription repository does not support atomic credit updates")
	}

	script := map[string]interface{}{
		"script": map[string]interface{}{
			"source": source,
			"lang":   "painless",
			"params": params,
		},
	}
	if err := updater.ScriptUpdate(ctx, sub.ID, script); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "insufficient credits") {
			return nil, billing.ErrInsufficientCredits
		}
		return nil, fmt.Errorf("billingrepo: atomic credit update: %w", err)
	}

	balance, err := r.GetByTenantID(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if balance == nil {
		return nil, fmt.Errorf("billingrepo: credit balance missing after atomic update for tenant %s", tenantID)
	}
	r.logger.Debug("Applied atomic credit update",
		zap.String("subscription_id", sub.ID),
		zap.String("tenant_id", tenantID),
		zap.Int64("credits_remaining", balance.CreditsRemaining),
		zap.Int64("credits_reserved", balance.CreditsReserved),
	)
	return balance, nil
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
