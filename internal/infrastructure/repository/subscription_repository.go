package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
	"go.uber.org/zap"
)

// SubscriptionRepository implements license.Repository with Elasticsearch.
type SubscriptionRepository struct {
	base                     *BaseRepository
	journalMappingMu         sync.Mutex
	journalMappingReady      bool
	creditJournalModeEnabled bool
}

// Configure once during application wiring. Durable markers survive rollback.
func (r *SubscriptionRepository) SetCreditJournalModeEnabled(enabled bool) {
	r.creditJournalModeEnabled = enabled
}
func (r *SubscriptionRepository) CreditJournalModeEnabled() bool { return r.creditJournalModeEnabled }
func (r *SubscriptionRepository) EnableCreditJournalSubscription(ctx context.Context, id string) error {
	if err := r.EnsureCreditJournalMapping(ctx); err != nil {
		return err
	}
	return r.ScriptUpdate(ctx, id, map[string]interface{}{"script": map[string]interface{}{"lang": "painless", "source": `if (ctx._source.credit_reservation_mode == 'journal') { ctx.op = 'noop'; return; } ctx._source.credit_reservation_mode = 'journal';`}})
}

// NewSubscriptionRepository creates a new subscription repository.
func NewSubscriptionRepository(client *elasticsearch.Client, logger *zap.Logger) license.Repository {
	return &SubscriptionRepository{
		base: NewBaseRepository(client, "subscriptions", logger, RefreshWaitFor),
	}
}

func (r *SubscriptionRepository) Create(ctx context.Context, sub *license.Subscription) error {
	if r.creditJournalModeEnabled {
		if err := r.EnsureCreditJournalMapping(ctx); err != nil {
			return err
		}
		sub.CreditReservationMode = "journal"
	}
	now := time.Now().UTC()
	sub.CreatedAt = now
	sub.UpdatedAt = now
	return r.base.Create(ctx, sub.ID, sub)
}

func (r *SubscriptionRepository) GetByID(ctx context.Context, id string) (*license.Subscription, error) {
	doc, err := r.base.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	var sub license.Subscription
	if err := mapToStruct(doc, &sub); err != nil {
		return nil, fmt.Errorf("failed to map document to subscription: %w", err)
	}

	return &sub, nil
}

func (r *SubscriptionRepository) Update(ctx context.Context, sub *license.Subscription) error {
	return r.updateSubscription(ctx, sub, false)
}

// Explicit allocation intent is required for journal-backed subscriptions;
// generic metadata/lifecycle writers cannot be mistaken for a replenishment.
func (r *SubscriptionRepository) UpdateCreditAllocation(ctx context.Context, sub *license.Subscription) error {
	if err := r.EnsureCreditJournalMapping(ctx); err != nil {
		return err
	}
	return r.updateSubscription(ctx, sub, true)
}

// Checkout saves only checkout metadata. Concurrent sends must never have their
// counters overwritten by the wallet snapshot used to prepare an order.
func (r *SubscriptionRepository) UpdateCheckout(ctx context.Context, sub *license.Subscription) error {
	if sub == nil || sub.Metadata == nil {
		return fmt.Errorf("missing checkout subscription")
	}
	orderID, _ := sub.Metadata["pending_checkout_order_id"].(string)
	if orderID == "" {
		return fmt.Errorf("missing checkout order")
	}
	return r.ScriptUpdate(ctx, sub.ID, map[string]interface{}{"script": map[string]interface{}{
		"lang": "painless", "source": subscriptionCheckoutScript,
		"params": map[string]interface{}{"metadata": sub.Metadata, "updated_at": time.Now().UTC().Format(time.RFC3339Nano)},
	}})
}

const subscriptionCheckoutScript = `
if (ctx._source.metadata == null) { ctx._source.metadata = [:]; }
def m = ctx._source.metadata;
if (m.checkout_orders == null) { m.checkout_orders = [:]; }
// Preserve the pre-archive pending snapshot when an older client created it.
if (m.pending_checkout_order_id != null && !m.checkout_orders.containsKey(m.pending_checkout_order_id)) {
 def snapshot = [:];
 for (def e : m.entrySet()) { if (e.getKey().startsWith('pending_checkout_')) { snapshot[e.getKey()] = e.getValue(); } }
 m.checkout_orders[m.pending_checkout_order_id] = snapshot;
}
if (params.metadata.checkout_orders != null) {
 for (def e : params.metadata.checkout_orders.entrySet()) {
  if (!m.checkout_orders.containsKey(e.getKey())) { m.checkout_orders[e.getKey()] = e.getValue(); }
 }
}
for (def e : params.metadata.entrySet()) { if (e.getKey().startsWith('pending_checkout_')) { m[e.getKey()] = e.getValue(); } }
ctx._source.updated_at = params.updated_at;
`

func (r *SubscriptionRepository) updateSubscription(ctx context.Context, sub *license.Subscription, allocate bool) error {
	if sub == nil {
		return fmt.Errorf("nil subscription")
	}
	if r.creditJournalModeEnabled {
		if err := r.EnableCreditJournalSubscription(ctx, sub.ID); err != nil {
			return err
		}
	}
	sub.UpdatedAt = time.Now().UTC()
	data, err := json.Marshal(sub)
	if err != nil {
		return err
	}
	var doc map[string]interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		return err
	}
	// Counters and journal are owned by atomic billing operations. A forward
	// renewal may allocate once, but must first drain every old hold. Replaying
	// the same renewal cannot replenish already-consumed credits.
	doc["credits_total"] = sub.CreditsTotal
	doc["credits_remaining"] = sub.CreditsRemaining
	doc["credits_reserved"] = sub.CreditsReserved
	delete(doc, "credit_reservation_journal")
	allocationID := "cycle:" + sub.CurrentPeriodStart.UTC().Format(time.RFC3339Nano)
	if sub.Metadata != nil {
		if intent, _ := sub.Metadata["credit_allocation_id"].(string); intent != "" {
			allocationID = "intent:" + intent
		}
		method, _ := sub.Metadata["renewal_method"].(string)
		if paymentID, ok := sub.Metadata["last_payment_id"].(string); ok && paymentID != "" && strings.HasPrefix(method, "razorpay") {
			allocationID = "payment:" + paymentID
		}
	}
	if allocate {
		var delta int64
		if sub.Metadata != nil {
			switch value := sub.Metadata["credit_allocation_delta"].(type) {
			case int64:
				delta = value
			case float64:
				delta = int64(value)
			}
		}
		if delta < 0 {
			return fmt.Errorf("negative credit allocation")
		}
		paymentOrderID := ""
		if strings.HasPrefix(allocationID, "payment:") {
			paymentOrderID, _ = sub.Metadata["last_order_id"].(string)
		}
		source, _, err := r.ScriptUpdateWithSource(ctx, sub.ID, map[string]interface{}{"script": map[string]interface{}{
			"lang": "painless", "source": subscriptionProtectedUpdateScript, "params": map[string]interface{}{"doc": doc, "allocate": true, "allocation_id": allocationID, "grant_delta": delta, "allocation_at": sub.UpdatedAt.UTC().Format(time.RFC3339Nano), "payment_order_id": paymentOrderID},
		}})
		if err == nil {
			*sub = *source
		}
		return err
	}
	return r.ScriptUpdate(ctx, sub.ID, map[string]interface{}{"script": map[string]interface{}{
		"lang": "painless", "source": subscriptionProtectedUpdateScript, "params": map[string]interface{}{"doc": doc, "allocate": allocate, "allocation_id": allocationID},
	}})
}

const subscriptionProtectedUpdateScript = `
def d = params.doc;
boolean allocation = params.allocate;
// Explicit replay identities are durable in both modes and survive adoption.
if (allocation && ctx._source.credit_allocation_journal != null && ctx._source.credit_allocation_journal[params.allocation_id] == true) {
 ctx.op = 'noop'; return;
}
// Recharge adds to the current source, never a caller's stale snapshot. Holds
// and their delivery receipts remain valid across recharge and renewal.
if (allocation && params.grant_delta > 0) {
 def orderID = params.payment_order_id;
 def m = ctx._source.metadata != null ? ctx._source.metadata : [:];
 def orders = m.checkout_orders != null ? m.checkout_orders : [:];
 if (orderID != '') {
  if (ctx._source.credit_allocation_journal != null && ctx._source.credit_allocation_journal['order:' + orderID] == true) { throw new IllegalArgumentException('checkout order already allocated'); }
  def snapshot = orders[orderID];
  if (snapshot == null && m.pending_checkout_order_id == orderID) {
   snapshot = [:];
   for (def e : m.entrySet()) { if (e.getKey().startsWith('pending_checkout_')) { snapshot[e.getKey()] = e.getValue(); } }
   orders[orderID] = snapshot;
  }
  if (snapshot == null) { throw new IllegalArgumentException('payment order has no checkout snapshot'); }
  if (snapshot.pending_checkout_credits != null && (long)snapshot.pending_checkout_credits != (long)params.grant_delta) { throw new IllegalArgumentException('credit allocation differs from checkout'); }
 }
 def oldExpiry = ctx._source.credits_expire_at != null ? ctx._source.credits_expire_at : ctx._source.current_period_end;
 boolean expired = oldExpiry != null && ZonedDateTime.parse(oldExpiry).isBefore(ZonedDateTime.parse(params.allocation_at));
 long held = ctx._source.credits_reserved != null ? ctx._source.credits_reserved : 0;
 long remaining = ctx._source.credits_remaining != null ? ctx._source.credits_remaining : 0;
 long total = ctx._source.credits_total != null ? ctx._source.credits_total : 0;
 if (expired) { remaining = held; total = held; }
 if (remaining < held || total < 0 || held < 0) { throw new IllegalArgumentException('invalid wallet counters'); }
 long newRemaining = remaining + (long)params.grant_delta;
 long newTotal = total + (long)params.grant_delta;
 if (newRemaining < remaining || newTotal < total) { throw new IllegalArgumentException('credit allocation overflow'); }
 ctx._source.credits_remaining = newRemaining;
 ctx._source.credits_total = newTotal;
 ctx._source.credits_reserved = held;
 def expiry = d.credits_expire_at;
 if (oldExpiry != null && (expiry == null || ZonedDateTime.parse(oldExpiry).isAfter(ZonedDateTime.parse(expiry)))) { expiry = oldExpiry; }
 for (def e : d.entrySet()) {
  if (e.getKey() != 'metadata' && !e.getKey().startsWith('credits_') && !e.getKey().startsWith('credit_')) { ctx._source[e.getKey()] = e.getValue(); }
 }
 if (d.metadata != null) {
  for (def e : d.metadata.entrySet()) {
   if (e.getKey() != 'checkout_orders' && (orderID == '' || !e.getKey().startsWith('pending_checkout_'))) { m[e.getKey()] = e.getValue(); }
  }
 }
 if (orderID != '') {
  orders[orderID].paid_payment_id = d.metadata.last_payment_id;
  if (m.pending_checkout_order_id == orderID) {
   def keys = [];
   for (def key : m.keySet()) { if (key.startsWith('pending_checkout_')) { keys.add(key); } }
   for (def key : keys) { m.remove(key); }
  }
 }
 m.checkout_orders = orders;
 ctx._source.metadata = m;
 if (expiry != null) { ctx._source.credits_expire_at = expiry; ctx._source.current_period_end = expiry; }
 if (ctx._source.credit_allocation_journal == null) { ctx._source.credit_allocation_journal = [:]; }
 ctx._source.credit_allocation_journal[params.allocation_id] = true;
 if (orderID != '') { ctx._source.credit_allocation_journal['order:' + orderID] = true; }
 return;
}
// Preserve pre-cutover full Update only for unmarked subscriptions. The durable
// marker protects counters even before the first receipt and during rollback.
if (ctx._source.credit_reservation_mode != 'journal' && (ctx._source.credit_reservation_journal == null || ctx._source.credit_reservation_journal.isEmpty())) {
 for (def e : d.entrySet()) { if (!e.getKey().startsWith('credit_reservation_') && e.getKey() != 'credit_allocation_journal') { ctx._source[e.getKey()] = e.getValue(); } }
 if (allocation) {
  if (ctx._source.credit_allocation_journal == null) { ctx._source.credit_allocation_journal = [:]; }
  ctx._source.credit_allocation_journal[params.allocation_id] = true;
 }
 return;
}
boolean cycle = ctx._source.current_period_start != d.current_period_start;
if (cycle && ctx._source.current_period_start != null &&
    ZonedDateTime.parse(d.current_period_start).isBefore(ZonedDateTime.parse(ctx._source.current_period_start))) {
 throw new IllegalArgumentException('stale subscription cycle');
}
if (allocation) {
 if (ctx._source.credits_reserved != null && ctx._source.credits_reserved != 0) {
  throw new IllegalArgumentException('renewal requires draining credit reservations');
 }
 if (ctx._source.credit_reservation_journal != null) {
  for (def receipt : ctx._source.credit_reservation_journal.values()) {
   if (receipt.reservation.status == 'reserved') { throw new IllegalArgumentException('renewal requires draining credit journal'); }
  }
 }
 ctx._source.credits_total = d.credits_total;
 ctx._source.credits_remaining = d.credits_remaining;
 ctx._source.credits_reserved = 0;
 ctx._source.credits_expire_at = d.credits_expire_at;
 if (ctx._source.credit_allocation_journal == null) { ctx._source.credit_allocation_journal = [:]; }
 ctx._source.credit_allocation_journal[params.allocation_id] = true;
}
for (def e : d.entrySet()) {
 if (!e.getKey().startsWith('credits_') && !e.getKey().startsWith('credit_')) {
  ctx._source[e.getKey()] = e.getValue();
 }
}
`

// ScriptUpdateWithSource is deliberately separate from generic ScriptUpdate.
// The source belongs to this update (including a rejecting noop), never a GET
// performed after a concurrent writer. Conflicts stay storage errors.
func (r *SubscriptionRepository) ScriptUpdateWithSource(ctx context.Context, id string, script map[string]interface{}) (*license.Subscription, string, error) {
	return r.scriptUpdateWithSource(ctx, id, script, r.base.defaultRefresh)
}

// Credit updates return their own atomic source; per-send refresh barriers are unnecessary.
func (r *SubscriptionRepository) ScriptUpdateCreditSource(ctx context.Context, id string, script map[string]interface{}) (*license.Subscription, string, error) {
	return r.scriptUpdateWithSource(ctx, id, script, RefreshFalse)
}
func (r *SubscriptionRepository) scriptUpdateWithSource(ctx context.Context, id string, script map[string]interface{}, refresh RefreshPolicy) (*license.Subscription, string, error) {
	request := make(map[string]interface{}, len(script)+1)
	for k, v := range script {
		request[k] = v
	}
	request["_source"] = true
	data, err := json.Marshal(request)
	if err != nil {
		return nil, "", err
	}
	retry := 5
	req := esapi.UpdateRequest{Index: r.base.indexName, DocumentID: id, Body: strings.NewReader(string(data)), RetryOnConflict: &retry, Refresh: string(refresh)}
	res, err := req.Do(ctx, r.base.client)
	if err != nil {
		return nil, "", fmt.Errorf("subscription atomic update: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		var failure struct {
			Error subscriptionAtomicError `json:"error"`
		}
		if err := json.NewDecoder(res.Body).Decode(&failure); err != nil {
			return nil, "", fmt.Errorf("subscription atomic update status %d (malformed error): %w", res.StatusCode, err)
		}
		cause := &failure.Error
		for cause.CausedBy != nil {
			cause = cause.CausedBy
		}
		return nil, "", fmt.Errorf("subscription atomic update status %d: %s: %s", res.StatusCode, cause.Type, cause.Reason)
	}
	var raw struct {
		Result string `json:"result"`
		Get    struct {
			Source json.RawMessage `json:"_source"`
		} `json:"get"`
	}
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return nil, "", fmt.Errorf("decode atomic subscription response: %w", err)
	}
	var sub license.Subscription
	var counters struct {
		Remaining *int64 `json:"credits_remaining"`
		Reserved  *int64 `json:"credits_reserved"`
	}
	if err := json.Unmarshal(raw.Get.Source, &sub); err != nil {
		return nil, "", fmt.Errorf("malformed atomic subscription source: %w", err)
	}
	if err := json.Unmarshal(raw.Get.Source, &counters); err != nil {
		return nil, "", err
	}
	if (raw.Result != "updated" && raw.Result != "noop") || sub.ID != id || counters.Remaining == nil || counters.Reserved == nil {
		return nil, "", fmt.Errorf("malformed atomic subscription response for %s", id)
	}
	return &sub, raw.Result, nil
}

type subscriptionAtomicError struct {
	Type     string                   `json:"type"`
	Reason   string                   `json:"reason"`
	CausedBy *subscriptionAtomicError `json:"caused_by"`
}

// Install on existing indices before the first journal write; never allow
// dynamic reservation IDs to create indexed mapping fields.
func (r *SubscriptionRepository) EnsureCreditJournalMapping(ctx context.Context) error {
	r.journalMappingMu.Lock()
	defer r.journalMappingMu.Unlock()
	if r.journalMappingReady {
		return nil
	}
	res, err := r.base.client.Indices.PutMapping([]string{r.base.indexName}, strings.NewReader(`{"properties":{"credit_reservation_mode":{"type":"keyword"},"credit_reservation_journal":{"type":"object","enabled":false},"credit_allocation_journal":{"type":"object","enabled":false}}}`), r.base.client.Indices.PutMapping.WithContext(ctx))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("install source-only credit journal mapping: %s", res.String())
	}
	var ack struct {
		Acknowledged bool `json:"acknowledged"`
	}
	if err := json.NewDecoder(res.Body).Decode(&ack); err != nil {
		return err
	}
	if !ack.Acknowledged {
		return fmt.Errorf("credit journal mapping not acknowledged")
	}
	r.journalMappingReady = true
	return nil
}

// Walk all subscription documents, including expired/renewed subscriptions.
// Immutable unique id sorting prevents updates/deletions from moving offsets.
func (r *SubscriptionRepository) ListCreditJournalSubscriptions(ctx context.Context, cursor string, limit int) ([]*license.Subscription, string, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := map[string]interface{}{"query": map[string]interface{}{"match_all": map[string]interface{}{}}, "size": limit, "sort": []map[string]interface{}{{"id": map[string]interface{}{"order": "asc"}}}}
	if cursor != "" {
		q["search_after"] = []string{cursor}
	}
	result, err := r.base.Search(ctx, q)
	if err != nil {
		return nil, "", err
	}
	subs := make([]*license.Subscription, 0, len(result.Hits))
	for _, hit := range result.Hits {
		var sub license.Subscription
		if err := mapToStruct(hit, &sub); err != nil {
			return nil, "", err
		}
		if sub.ID == "" || (cursor != "" && sub.ID <= cursor) {
			return nil, "", fmt.Errorf("invalid journal pagination id")
		}
		current, err := r.GetByID(ctx, sub.ID)
		if err != nil {
			return nil, "", err
		}
		if current == nil || current.ID != sub.ID {
			return nil, "", fmt.Errorf("journal subscription unavailable")
		}
		subs = append(subs, current)
	}
	next := ""
	if len(subs) == limit {
		next = subs[len(subs)-1].ID
	}
	return subs, next, nil
}

func (r *SubscriptionRepository) ScriptUpdate(ctx context.Context, id string, script map[string]interface{}) error {
	return r.base.ScriptUpdate(ctx, id, script)
}

func (r *SubscriptionRepository) Delete(ctx context.Context, id string) error {
	return r.base.Delete(ctx, id)
}

func (r *SubscriptionRepository) List(ctx context.Context, filter license.SubscriptionFilter) ([]*license.Subscription, error) {
	query := map[string]interface{}{}
	must := make([]map[string]interface{}, 0)

	if filter.TenantID != "" {
		must = append(must, map[string]interface{}{
			"term": map[string]interface{}{"tenant_id": filter.TenantID},
		})
	}
	if filter.AppID != "" {
		must = append(must, map[string]interface{}{
			"term": map[string]interface{}{"app_id": filter.AppID},
		})
	}
	if len(filter.Statuses) > 0 {
		statuses := make([]string, 0, len(filter.Statuses))
		for _, status := range filter.Statuses {
			statuses = append(statuses, string(status))
		}
		must = append(must, map[string]interface{}{
			"terms": map[string]interface{}{"status": statuses},
		})
	}

	if len(must) > 0 {
		query["query"] = map[string]interface{}{
			"bool": map[string]interface{}{
				"must": must,
			},
		}
	} else {
		query["query"] = map[string]interface{}{
			"match_all": map[string]interface{}{},
		}
	}

	query["sort"] = []map[string]interface{}{
		{"updated_at": map[string]interface{}{"order": "desc"}},
	}

	if filter.Offset > 0 {
		query["from"] = filter.Offset
	}
	if filter.Limit > 0 {
		query["size"] = filter.Limit
	}

	result, err := r.base.Search(ctx, query)
	if err != nil {
		return nil, err
	}

	subs := make([]*license.Subscription, 0, len(result.Hits))
	for _, hit := range result.Hits {
		var sub license.Subscription
		if err := mapToStruct(hit, &sub); err != nil {
			r.base.logger.Warn("Failed to map document to subscription", zap.Error(err))
			continue
		}
		subs = append(subs, &sub)
	}

	return subs, nil
}

func (r *SubscriptionRepository) GetActiveSubscription(ctx context.Context, tenantID, appID string, now time.Time) (*license.Subscription, error) {
	if appID != "" {
		sub, err := r.getActiveByField(ctx, "app_id", appID, now)
		if err != nil {
			return nil, err
		}
		if err == nil && sub != nil {
			return sub, nil
		}
	}

	if tenantID != "" {
		sub, err := r.getActiveByField(ctx, "tenant_id", tenantID, now)
		if err != nil {
			return nil, err
		}
		if err == nil && sub != nil {
			return sub, nil
		}
	}

	return nil, nil
}

func (r *SubscriptionRepository) getActiveByField(ctx context.Context, field, value string, now time.Time) (*license.Subscription, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []map[string]interface{}{
					{"term": map[string]interface{}{field: value}},
					{"terms": map[string]interface{}{"status": []string{string(license.SubscriptionStatusActive), string(license.SubscriptionStatusTrial)}}},
					{"range": map[string]interface{}{"current_period_start": map[string]interface{}{"lte": now.Format(time.RFC3339Nano)}}},
					{"range": map[string]interface{}{"current_period_end": map[string]interface{}{"gte": now.Format(time.RFC3339Nano)}}},
				},
			},
		},
		"sort": []map[string]interface{}{
			{"current_period_end": map[string]interface{}{"order": "desc"}},
			{"updated_at": map[string]interface{}{"order": "desc"}},
		},
		"size": 1,
	}

	result, err := r.base.Search(ctx, query)
	if err != nil {
		return nil, err
	}
	if result.Total == 0 {
		return nil, nil
	}

	var sub license.Subscription
	if err := mapToStruct(result.Hits[0], &sub); err != nil {
		return nil, fmt.Errorf("failed to map document to subscription: %w", err)
	}

	// Search selects the subscription ID; counters and journal reads are realtime.
	current, err := r.GetByID(ctx, sub.ID)
	if err != nil {
		return nil, err
	}
	if current == nil || (current.Status != license.SubscriptionStatusActive && current.Status != license.SubscriptionStatusTrial) || current.CurrentPeriodStart.After(now) || current.CurrentPeriodEnd.Before(now) {
		return nil, nil
	}
	return current, nil
}
