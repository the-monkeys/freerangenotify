package services

import (
	"context"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
)

// Subscription debt status belongs to the same frozen fact as the invoice
// change. Paused, trialing and canceled subscriptions retain their lifecycle.
func (s *BizOperationService) syncInvoiceSubscriptionStatus(ctx context.Context, event *bizbilling.JournalEvent) error {
	invoice := event.InvoiceAfter
	if invoice == nil || invoice.SubscriptionID == "" || invoice.Status == bizbilling.InvoiceStatusDraft || invoice.Status != bizbilling.InvoiceStatusOverdue && invoice.AmountDuePaisa != 0 {
		return nil
	}
	sub := event.SubscriptionAfter
	if sub == nil {
		var err error
		sub, err = s.stores.Subscriptions.Get(ctx, invoice.SubscriptionID)
		if err != nil {
			return err
		}
	}
	if sub.ID != invoice.SubscriptionID {
		return bizbilling.ErrReconciliationRequired
	}
	if err := financialScopeMatches(sub.FinancialScope(), event.Scope); err != nil {
		return err
	}
	status := sub.Status
	if invoice.Status == bizbilling.InvoiceStatusOverdue && invoice.AmountDuePaisa > 0 && status == bizbilling.SubscriptionStatusActive {
		status = bizbilling.SubscriptionStatusPastDue
	} else if invoice.AmountDuePaisa == 0 && status == bizbilling.SubscriptionStatusPastDue {
		// Financial writes in this customer stream are fenced, and prior
		// invoices are refreshed before commit. Find rejects partial shards.
		rows, _, err := s.stores.Invoices.Find(ctx, map[string]any{"size": 1, "query": map[string]any{"bool": map[string]any{
			"filter": []any{
				map[string]any{"term": map[string]any{"app_id": event.Scope.AppID}},
				map[string]any{"term": map[string]any{"user_id": event.Scope.UserID}},
				map[string]any{"term": map[string]any{"subscription_id": sub.ID}},
				map[string]any{"range": map[string]any{"amount_due_paisa": map[string]any{"gt": 0}}},
				map[string]any{"terms": map[string]any{"status": []string{"open", "partially_paid", "overdue"}}},
				map[string]any{"bool": map[string]any{"minimum_should_match": 1, "should": []any{
					map[string]any{"term": map[string]any{"status": "overdue"}},
					map[string]any{"range": map[string]any{"due_date": map[string]any{"lte": event.CreatedAt}}},
				}}},
			},
			"must_not": []any{map[string]any{"term": map[string]any{"id": invoice.ID}}},
		}}})
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			status = bizbilling.SubscriptionStatusActive
		}
	}
	if status != sub.Status {
		sub.Status, sub.LastSequence, sub.UpdatedAt = status, event.Sequence, event.CreatedAt
		event.SubscriptionAfter = sub
		event.Resources = append(event.Resources, bizbilling.ResourceRef{Kind: "subscription", ID: sub.ID})
	}
	return nil
}
