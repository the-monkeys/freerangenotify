package billingrepo

import (
	"context"
	"fmt"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
	"go.uber.org/zap"
)

type creditScriptUpdater interface {
	ScriptUpdate(ctx context.Context, id string, script map[string]interface{}) error
}

type creditSourceUpdater interface {
	ScriptUpdateWithSource(context.Context, string, map[string]interface{}) (*license.Subscription, string, error)
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
	if balance == nil || balance.TenantID == "" {
		return fmt.Errorf("billingrepo: invalid credit balance")
	}
	_, err := r.applyCreditScript(ctx, balance.TenantID, creditCompatibilityUpsertScript, map[string]interface{}{
		"total": balance.CreditsTotal, "remaining": balance.CreditsRemaining, "reserved": balance.CreditsReserved,
		"expiry": balance.CreditsExpireAt.UTC().Format(time.RFC3339Nano), "now": time.Now().UTC().Format(time.RFC3339Nano),
	})
	return err
}

const creditCompatibilityUpsertScript = `
if (ctx._source.credit_reservation_mode == 'journal' || (ctx._source.credit_reservation_journal != null && !ctx._source.credit_reservation_journal.isEmpty())) {
 throw new IllegalArgumentException('journal credit replacement disabled; use atomic grant/bootstrap');
}
ctx._source.credits_total = params.total;
ctx._source.credits_remaining = params.remaining;
ctx._source.credits_reserved = params.reserved;
ctx._source.credits_expire_at = params.expiry;
ctx._source.updated_at = params.now;
`

func (r *SubscriptionCreditBalanceRepo) ReserveCredits(ctx context.Context, tenantID string, amount int64) (*billing.CreditBalance, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("credit amount must be positive")
	}
	return r.applyCreditScript(ctx, tenantID, creditReserveScript, map[string]interface{}{
		"amount": amount,
		"now":    time.Now().UTC().Format(time.RFC3339),
	})
}

func (r *SubscriptionCreditBalanceRepo) CommitReservedCredits(ctx context.Context, tenantID string, amount int64) (*billing.CreditBalance, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("credit amount must be positive")
	}
	return r.applyCreditScript(ctx, tenantID, creditCommitScript, map[string]interface{}{
		"amount": amount,
		"now":    time.Now().UTC().Format(time.RFC3339),
	})
}

func (r *SubscriptionCreditBalanceRepo) ReleaseReservedCredits(ctx context.Context, tenantID string, amount int64) (*billing.CreditBalance, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("credit amount must be positive")
	}
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
  ctx.op = 'noop'; return;
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
if (ctx._source.credit_reservation_mode == 'journal' && ctx._source.credits_reserved != null && ctx._source.credits_reserved != 0) { throw new IllegalArgumentException('cannot reset journal-backed or quarantined holds'); }
if (ctx._source.credit_reservation_journal != null) {
 for (def receipt : ctx._source.credit_reservation_journal.values()) {
  if (receipt.reservation.status == 'reserved') { throw new IllegalArgumentException('cannot reset active journal-backed holds'); }
 }
}
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

	updater, ok := r.subRepo.(creditSourceUpdater)
	if !ok {
		return nil, fmt.Errorf("billingrepo: subscription repository does not support atomic credit updates")
	}

	scriptSource := source
	if source == creditReserveScript || source == creditCommitScript || source == creditReleaseScript {
		scriptSource = `if (ctx._source.credit_reservation_mode == 'journal' || (ctx._source.credit_reservation_journal != null && !ctx._source.credit_reservation_journal.isEmpty())) { throw new IllegalArgumentException('legacy lifecycle writer blocked on journal-backed subscription'); }` + source
	}
	script := map[string]interface{}{
		"script": map[string]interface{}{
			"source": scriptSource,
			"lang":   "painless",
			"params": params,
		},
	}
	updated, result, err := updater.ScriptUpdateWithSource(ctx, sub.ID, script)
	if err != nil {
		return nil, fmt.Errorf("billingrepo: atomic credit update: %w", err)
	}
	if updated == nil || updated.TenantID != tenantID {
		return nil, fmt.Errorf("invalid atomic credit response")
	}
	balance := subscriptionToCreditBalance(updated)
	if source == creditReserveScript && result == "noop" {
		return nil, billing.NewCreditUnavailableError(balance, params["amount"].(int64), "")
	}
	if balance == nil {
		return nil, fmt.Errorf("billingrepo: credit balance missing after atomic update for tenant %s", tenantID)
	}
	if r.logger != nil {
		r.logger.Debug("Applied atomic credit update",
			zap.String("subscription_id", sub.ID),
			zap.String("tenant_id", tenantID),
			zap.Int64("credits_remaining", balance.CreditsRemaining),
			zap.Int64("credits_reserved", balance.CreditsReserved),
		)
	}
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
		JournalBacked:    sub.CreditReservationMode == billing.CreditReservationModeJournal || len(sub.CreditReservationJournal) > 0,
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
