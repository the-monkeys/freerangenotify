package billingrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
)

// Compatibility reservations retain their existing IDs and Redis lifecycle, but
// the terminal identity and wallet delta must share an atomic durable boundary.
const legacySettlementScript = `
if (ctx._source.tenant_id != params.tenant || ctx._source.credit_reservation_mode == 'journal') { throw new IllegalArgumentException('legacy settlement tenant or mode mismatch'); }
def journal = ctx._source.credit_legacy_settlement_journal;
def existing = journal == null ? null : journal[params.id];
if (existing != null) {
 if (existing.reservation.notification_id != params.reservation.notification_id || existing.reservation.app_id != params.reservation.app_id || existing.reservation.credits_reserved != params.reservation.credits_reserved || existing.reservation.rate_card_version != params.reservation.rate_card_version) { throw new IllegalArgumentException('legacy settlement identity mismatch'); }
 ctx.op = 'noop'; return;
}
long held = ctx._source.credits_reserved == null ? 0 : ctx._source.credits_reserved;
long remaining = ctx._source.credits_remaining == null ? 0 : ctx._source.credits_remaining;
long amount = params.reservation.credits_reserved;
if (amount <= 0 || held < amount || remaining < held) { throw new IllegalArgumentException('legacy settlement requires a funded hold'); }
ctx._source.credits_reserved = held - amount;
if (params.status == 'committed') { ctx._source.credits_remaining = remaining - amount; }
ctx._source.updated_at = params.now;
def reservation = params.reservation;
reservation.status = params.status;
reservation.updated_at = params.now;
reservation.release_reason = params.reason;
def balance = ['id':ctx._source.id, 'tenant_id':params.tenant, 'credits_total':ctx._source.credits_total, 'credits_remaining':ctx._source.credits_remaining, 'credits_reserved':ctx._source.credits_reserved];
reservation.balance_after = balance;
if (journal == null) { ctx._source.credit_legacy_settlement_journal = [:]; journal = ctx._source.credit_legacy_settlement_journal; }
journal[params.id] = ['reservation':reservation, 'balance':balance, 'reason':params.reason];
`

func (r *SubscriptionCreditBalanceRepo) TransitionLegacyReservation(ctx context.Context, res *billing.CreditReservation, status billing.CreditReservationStatus, reason string) (*billing.CreditReservationReceipt, error) {
	if res == nil || res.ID == "" || res.TenantID == "" || res.SubscriptionID == "" || (status != billing.CreditReservationCommitted && status != billing.CreditReservationReleased) {
		return nil, fmt.Errorf("invalid legacy settlement identity")
	}
	repo, err := r.journalRepo(ctx)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(res)
	if err != nil {
		return nil, err
	}
	var reservation map[string]interface{}
	if err := json.Unmarshal(data, &reservation); err != nil {
		return nil, err
	}
	request := map[string]interface{}{"script": map[string]interface{}{"lang": "painless", "source": legacySettlementScript, "params": map[string]interface{}{"tenant": res.TenantID, "id": res.ID, "reservation": reservation, "status": status, "reason": reason, "now": time.Now().UTC().Format(time.RFC3339Nano)}}}
	var sub *license.Subscription
	if fast, ok := repo.(creditFastSourceUpdater); ok {
		sub, _, err = fast.ScriptUpdateCreditSource(ctx, res.SubscriptionID, request)
	} else {
		sub, _, err = repo.ScriptUpdateWithSource(ctx, res.SubscriptionID, request)
	}
	if err != nil {
		return nil, err
	}
	if sub == nil || sub.TenantID != res.TenantID {
		return nil, fmt.Errorf("invalid legacy settlement source")
	}
	var receipt billing.CreditReservationReceipt
	if err := json.Unmarshal(sub.CreditLegacySettlementJournal[res.ID], &receipt); err != nil {
		return nil, fmt.Errorf("decode legacy settlement: %w", err)
	}
	if receipt.Reservation.ID != res.ID || receipt.Reservation.SubscriptionID != res.SubscriptionID || receipt.Balance.ID != res.SubscriptionID {
		return nil, fmt.Errorf("invalid legacy settlement receipt")
	}
	return &receipt, nil
}
