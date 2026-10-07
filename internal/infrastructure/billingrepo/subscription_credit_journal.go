package billingrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
)

type journalSubscriptionRepository interface {
	creditSourceUpdater
	EnsureCreditJournalMapping(context.Context) error
	EnableCreditJournalSubscription(context.Context, string) error
	ListCreditJournalSubscriptions(context.Context, string, int) ([]*license.Subscription, string, error)
}

func (r *SubscriptionCreditBalanceRepo) journalRepo(ctx context.Context) (journalSubscriptionRepository, error) {
	repo, ok := r.subRepo.(journalSubscriptionRepository)
	if !ok {
		return nil, fmt.Errorf("subscription repository has no durable credit journal")
	}
	if err := repo.EnsureCreditJournalMapping(ctx); err != nil {
		return nil, err
	}
	return repo, nil
}

// Receipt and counters live in one document and one atomic operation. Retain
// all receipts until a replay-safe pruning policy has been established.
const creditJournalScript = `
if (ctx._source.credit_reservation_mode != 'journal') { throw new IllegalArgumentException('credit journal mode required'); }
if (ctx._source.tenant_id != params.tenant) { throw new IllegalArgumentException('credit tenant mismatch'); }
def journal = ctx._source.credit_reservation_journal;
def receipt = journal == null ? null : journal[params.id];
if (params.action == 'reserve') {
 if (receipt != null) {
  if (receipt.reservation.credits_reserved != params.reservation.credits_reserved ||
      receipt.reservation.notification_id != params.reservation.notification_id ||
      receipt.reservation.channel != params.reservation.channel ||
      receipt.reservation.app_id != params.reservation.app_id ||
      receipt.reservation.rate_card_version != params.reservation.rate_card_version ||
      receipt.reservation.subscription_id != params.subscription) {
   throw new IllegalArgumentException('reservation identity mismatch');
  }
  ctx.op = 'noop'; return;
 }
 long remaining = ctx._source.credits_remaining == null ? 0 : ctx._source.credits_remaining;
 long reserved = ctx._source.credits_reserved == null ? 0 : ctx._source.credits_reserved;
 if (reserved < 0 || remaining < reserved) { throw new IllegalArgumentException('invalid credit counters'); }
 long journalReserved = 0;
 if (journal != null) {
  for (def item : journal.values()) {
   if (item.reservation.status == 'reserved') {
    long held = item.reservation.credits_reserved;
    if (held <= 0 || journalReserved > 9223372036854775807L - held) { throw new IllegalArgumentException('invalid journal sum'); }
    journalReserved += held;
   }
  }
 }
 if (reserved != journalReserved) { throw new IllegalArgumentException('unmatched credit reservations require paused-worker import or quarantine'); }
 if (remaining - reserved < params.reservation.credits_reserved) { ctx.op = 'noop'; return; }
 if (journal == null) { ctx._source.credit_reservation_journal = [:]; journal = ctx._source.credit_reservation_journal; }
 ctx._source.credits_reserved = reserved + params.reservation.credits_reserved;
 receipt = ['reservation':params.reservation, 'ledger_recorded':false];
 journal[params.id] = receipt;
} else {
 if (receipt == null) { throw new IllegalArgumentException('credit reservation not found'); }
 if (params.action == 'ack') {
  if (receipt.reservation.status != params.status || receipt.reservation.status == 'reserved') { throw new IllegalArgumentException('invalid credit ledger acknowledgement'); }
  if (receipt.ledger_recorded == true) { ctx.op = 'noop'; return; }
  receipt.ledger_recorded = true;
  ctx._source.updated_at = params.now; return;
 }
 if (receipt.reservation.status != 'reserved') { ctx.op = 'noop'; return; }
 if (params.action == 'failed') {
  if (receipt.delivery_outcome == 'failed') { ctx.op = 'noop'; return; }
  receipt.delivery_outcome = 'failed'; ctx._source.updated_at = params.now; return;
 }
 long amount = receipt.reservation.credits_reserved;
 if (params.status == 'committed' && receipt.delivery_outcome == 'failed') { throw new IllegalArgumentException('cannot charge definitively failed delivery'); }
 if (ctx._source.credits_reserved == null || ctx._source.credits_reserved < amount ||
     ctx._source.credits_remaining == null || ctx._source.credits_remaining < amount) {
  throw new IllegalArgumentException('credit journal counter invariant');
 }
 ctx._source.credits_reserved -= amount;
 if (params.status == 'committed') { ctx._source.credits_remaining -= amount; }
 receipt.reservation.status = params.status;
 receipt.reservation.updated_at = params.now;
 receipt.reason = params.reason;
 receipt.ledger_recorded = false;
}
ctx._source.updated_at = params.now;
receipt.balance = ['id':params.subscription, 'tenant_id':params.tenant,
 'credits_total':ctx._source.credits_total == null ? 0 : ctx._source.credits_total,
 'credits_remaining':ctx._source.credits_remaining, 'credits_reserved':ctx._source.credits_reserved,
 'updated_at':params.now, 'created_at':ctx._source.created_at];
`

func (r *SubscriptionCreditBalanceRepo) ReserveReservation(ctx context.Context, res *billing.CreditReservation) (*billing.CreditTransitionOutcome, error) {
	if res == nil || res.CreditsReserved <= 0 || res.TenantID == "" || res.ExpiresAt.IsZero() || res.Status != billing.CreditReservationReserved {
		return nil, fmt.Errorf("invalid credit reservation")
	}
	subscriptionID, ok := billing.JournalSubscriptionID(res.ID)
	if !ok || subscriptionID != res.SubscriptionID {
		return nil, fmt.Errorf("invalid bound credit reservation id")
	}
	repo, err := r.journalRepo(ctx)
	if err != nil {
		return nil, err
	}
	if err := repo.EnableCreditJournalSubscription(ctx, subscriptionID); err != nil {
		return nil, err
	}
	return r.applyJournal(ctx, res.ID, res.TenantID, "reserve", res.Status, "", res)
}

func (r *SubscriptionCreditBalanceRepo) RecordReservationDeliveryFailure(ctx context.Context, id string) error {
	receipt, err := r.GetReservationReceipt(ctx, id)
	if err != nil {
		return err
	}
	if receipt.Reservation.Status != billing.CreditReservationReserved {
		return nil
	}
	_, err = r.applyJournal(ctx, id, receipt.Reservation.TenantID, "failed", receipt.Reservation.Status, "", nil)
	return err
}

func (r *SubscriptionCreditBalanceRepo) TransitionReservation(ctx context.Context, id string, status billing.CreditReservationStatus, reason string) (*billing.CreditTransitionOutcome, error) {
	if status != billing.CreditReservationCommitted && status != billing.CreditReservationReleased {
		return nil, fmt.Errorf("invalid terminal credit status")
	}
	receipt, err := r.GetReservationReceipt(ctx, id)
	if err != nil {
		return nil, err
	}
	return r.applyJournal(ctx, id, receipt.Reservation.TenantID, "transition", status, reason, nil)
}

func (r *SubscriptionCreditBalanceRepo) applyJournal(ctx context.Context, id, tenant, action string, status billing.CreditReservationStatus, reason string, reservation *billing.CreditReservation) (*billing.CreditTransitionOutcome, error) {
	subID, ok := billing.JournalSubscriptionID(id)
	if !ok {
		return nil, fmt.Errorf("invalid credit journal id")
	}
	repo, err := r.journalRepo(ctx)
	if err != nil {
		return nil, err
	}
	params := map[string]interface{}{"id": id, "subscription": subID, "tenant": tenant, "action": action, "status": status, "reason": reason, "now": time.Now().UTC().Format(time.RFC3339Nano), "reservation": reservation}
	sub, result, err := repo.ScriptUpdateWithSource(ctx, subID, map[string]interface{}{"script": map[string]interface{}{"source": creditJournalScript, "lang": "painless", "params": params}})
	if err != nil {
		return nil, err
	}
	if sub == nil || sub.ID != subID || sub.TenantID != tenant {
		return nil, fmt.Errorf("malformed credit journal response")
	}
	receipt, err := decodeReceipt(sub, id)
	if err != nil {
		if action == "reserve" && result == "noop" && sub.CreditReservationJournal[id] == nil {
			return nil, billing.NewCreditUnavailableError(subscriptionToCreditBalance(sub), reservation.CreditsReserved, reservation.RateCardVersion)
		}
		return nil, err
	}
	return &billing.CreditTransitionOutcome{Receipt: *receipt, Applied: result == "updated"}, nil
}

func decodeReceipt(sub *license.Subscription, id string) (*billing.CreditReservationReceipt, error) {
	raw, ok := sub.CreditReservationJournal[id]
	if !ok {
		return nil, billing.ErrCreditReservationNotFound
	}
	var receipt billing.CreditReservationReceipt
	if err := json.Unmarshal(raw, &receipt); err != nil {
		return nil, fmt.Errorf("decode credit receipt: %w", err)
	}
	res := receipt.Reservation
	if receipt.DeliveryOutcome != "" && receipt.DeliveryOutcome != "failed" {
		return nil, fmt.Errorf("%w: invalid delivery outcome %s", billing.ErrCreditJournalInvariant, id)
	}
	if res.ID != id || res.SubscriptionID != sub.ID || res.TenantID != sub.TenantID || res.CreditsReserved <= 0 || res.ExpiresAt.IsZero() || receipt.Balance.ID != sub.ID || receipt.Balance.TenantID != sub.TenantID ||
		(res.Status != billing.CreditReservationReserved && res.Status != billing.CreditReservationCommitted && res.Status != billing.CreditReservationReleased) {
		return nil, fmt.Errorf("%w: malformed receipt %s", billing.ErrCreditJournalInvariant, id)
	}
	return &receipt, nil
}

func (r *SubscriptionCreditBalanceRepo) GetReservationReceipt(ctx context.Context, id string) (*billing.CreditReservationReceipt, error) {
	subID, ok := billing.JournalSubscriptionID(id)
	if !ok {
		return nil, fmt.Errorf("invalid credit journal id")
	}
	sub, err := r.subRepo.GetByID(ctx, subID)
	if err != nil {
		return nil, err
	}
	if sub == nil {
		return nil, billing.ErrCreditReservationNotFound
	}
	return decodeReceipt(sub, id)
}
func (r *SubscriptionCreditBalanceRepo) MarkReservationLedgerRecorded(ctx context.Context, id string, status billing.CreditReservationStatus) error {
	receipt, err := r.GetReservationReceipt(ctx, id)
	if err != nil {
		return err
	}
	_, err = r.applyJournal(ctx, id, receipt.Reservation.TenantID, "ack", status, "", nil)
	return err
}
func (r *SubscriptionCreditBalanceRepo) ListReservationReceipts(ctx context.Context, cursor string, limit int) (*billing.CreditReceiptPage, error) {
	repo, err := r.journalRepo(ctx)
	if err != nil {
		return nil, err
	}
	subs, next, err := repo.ListCreditJournalSubscriptions(ctx, cursor, limit)
	if err != nil {
		return nil, err
	}
	page := &billing.CreditReceiptPage{NextCursor: next}
	for _, sub := range subs {
		ids := make([]string, 0, len(sub.CreditReservationJournal))
		for id := range sub.CreditReservationJournal {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		var valid []billing.CreditReservationReceipt
		damaged := false
		for _, id := range ids {
			page.JournalEntries++
			receipt, err := decodeReceipt(sub, id)
			if err != nil {
				page.Errors = append(page.Errors, fmt.Errorf("quarantine subscription %s: %w", sub.ID, err))
				damaged = true
				continue
			}
			valid = append(valid, *receipt)
		}
		if !damaged {
			page.Receipts = append(page.Receipts, valid...)
		}
	}
	return page, nil
}

const creditGrantScript = `
if (ctx._source.credits_total == null) { ctx._source.credits_total = 0; }
if (ctx._source.credits_remaining == null) { ctx._source.credits_remaining = 0; }
if (ctx._source.credits_total < 0 || ctx._source.credits_remaining < 0 ||
    ctx._source.credits_total > 9223372036854775807L - params.amount ||
    ctx._source.credits_remaining > 9223372036854775807L - params.amount) { throw new IllegalArgumentException('credit grant overflow'); }
if (ctx._source.credits_reserved == null) { ctx._source.credits_reserved = 0; }
ctx._source.credits_total += params.amount;
ctx._source.credits_remaining += params.amount;
ctx._source.updated_at = params.now;
`
const creditBootstrapScript = `
if ((ctx._source.credits_total != null && ctx._source.credits_total != 0) ||
    (ctx._source.credits_remaining != null && ctx._source.credits_remaining != 0) ||
    (ctx._source.credits_reserved != null && ctx._source.credits_reserved != 0) ||
    (ctx._source.credit_reservation_journal != null && !ctx._source.credit_reservation_journal.isEmpty())) { ctx.op = 'noop'; return; }
ctx._source.credits_total = params.amount;
ctx._source.credits_remaining = params.amount;
ctx._source.credits_reserved = 0;
ctx._source.credits_expire_at = params.expiry;
ctx._source.updated_at = params.now;
`

func (r *SubscriptionCreditBalanceRepo) GrantCreditBalance(ctx context.Context, tenant string, amount int64) (*billing.CreditBalance, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("credit grant must be positive")
	}
	return r.applyCreditScript(ctx, tenant, creditGrantScript, map[string]interface{}{"amount": amount, "now": time.Now().UTC().Format(time.RFC3339Nano)})
}
func (r *SubscriptionCreditBalanceRepo) BootstrapCreditBalance(ctx context.Context, tenant string, amount int64, expiry time.Time) (*billing.CreditBalance, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("credit bootstrap must be positive")
	}
	return r.applyCreditScript(ctx, tenant, creditBootstrapScript, map[string]interface{}{"amount": amount, "expiry": expiry.UTC().Format(time.RFC3339Nano), "now": time.Now().UTC().Format(time.RFC3339Nano)})
}
