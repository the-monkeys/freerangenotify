package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
)

func (s *BizExpenseService) tryFinancialConversion(ctx context.Context, appID, userID string, ids []string) (*bizbilling.BizInvoice, bool, error) {
	if s.lifecycleSvc == nil || s.lifecycleSvc.gate == nil {
		return nil, false, nil
	}
	state, err := s.lifecycleSvc.gate.State(ctx, appID)
	if err != nil {
		return nil, true, err
	}
	if state.LifecycleVersion == 1 {
		return nil, false, nil
	}
	if len(ids) == 0 || len(ids) > 1000 {
		return nil, true, bizbilling.ErrInvalidMoney
	}
	ordered := append([]string(nil), ids...)
	sort.Strings(ordered)
	for i, id := range ordered {
		if id == "" || (i > 0 && ordered[i-1] == id) {
			return nil, true, bizbilling.ErrInvalidMoney
		}
	}
	first, err := s.Get(ctx, appID, ordered[0])
	if err != nil {
		return nil, true, err
	}
	if first.Currency == "" {
		return nil, true, bizbilling.ErrLegacyReviewRequired
	}
	scope, _, err := financialScopeFor(ctx, s.lifecycleSvc.gate, appID, userID, first.Currency)
	if err != nil {
		return nil, true, err
	}
	if bizOperationKey(ctx) == "" {
		data, _ := json.Marshal(ordered)
		digest := sha256.Sum256(data)
		ctx = WithBizOperationKey(ctx, "expense-convert:"+hex.EncodeToString(digest[:]))
	}
	event, err := s.lifecycleSvc.applyFinancialCommand(ctx, bizbilling.Command{Kind: bizbilling.CommandExpenseConvert, Scope: scope, ExpenseConvert: &bizbilling.ExpenseConvert{ExpenseIDs: ordered}})
	if err != nil {
		return nil, true, err
	}
	return event.InvoiceAfter, true, nil
}

func (s *BizOperationService) buildExpenseConversion(ctx context.Context, command bizbilling.Command, claim bizbilling.StreamClaim, event *bizbilling.JournalEvent) error {
	ids := command.ExpenseConvert.ExpenseIDs
	// Validate every source before acquiring any locks. Definitions remain protected
	// by conditional claim writes if a concurrent editor changes them afterwards.
	for _, id := range ids {
		e, err := s.stores.Expenses.Get(ctx, id)
		if err != nil {
			return err
		}
		if err = financialScopeMatches(e.FinancialScope(), command.Scope); err != nil {
			return err
		}
		if !e.Billable || e.InvoiceID != "" || e.LastSequence != 0 || e.AmountPaisa <= 0 {
			return bizbilling.ErrInvalidMoney
		}
	}
	items := make([]bizbilling.InvoiceLineItem, 0, len(ids))
	for _, id := range ids {
		if err := s.streams.Renew(ctx, claim); err != nil {
			return err
		}
		e, err := s.expenseClaims.Claim(ctx, id, claim)
		if err != nil {
			return err
		}
		desc := e.Description
		if desc == "" {
			desc = e.Category
		}
		items = append(items, bizbilling.InvoiceLineItem{Description: fmt.Sprintf("Expense: %s (%s)", desc, e.Date.Format("02 Jan 2006")), Quantity: 1, UnitPrice: e.AmountPaisa, AmountPaisa: e.AmountPaisa})
		event.ExpensesAfter = append(event.ExpensesAfter, *e)
	}
	draft := command
	draft.InvoiceDraft = &bizbilling.InvoiceDraft{Request: bizbilling.CreateInvoiceRequest{UserID: command.Scope.UserID, Currency: command.Scope.Currency, LineItems: items}}
	if err := s.buildFinancialInvoiceDraft(ctx, draft, event); err != nil {
		return err
	}
	for i := range event.ExpensesAfter {
		e := &event.ExpensesAfter[i]
		e.InvoiceID = event.InvoiceAfter.ID
		e.LastSequence = event.Sequence
		e.PendingOperationID, e.PendingGeneration = "", 0
		e.UpdatedAt = event.CreatedAt
		event.Resources = append(event.Resources, bizbilling.ResourceRef{Kind: "expense", ID: e.ID})
	}
	return nil
}

func (s *BizOperationService) releaseUnfrozenSources(ctx context.Context, command bizbilling.Command, claim bizbilling.StreamClaim) error {
	if command.Kind != bizbilling.CommandExpenseConvert || command.ExpenseConvert == nil {
		return nil
	}
	for _, id := range command.ExpenseConvert.ExpenseIDs {
		if err := s.streams.Renew(ctx, claim); err != nil {
			return err
		}
		if err := s.expenseClaims.Release(ctx, id, claim); err != nil {
			return err
		}
	}
	return nil
}

func guardExpenseDefinition(before, after json.RawMessage) error {
	var previous, next bizbilling.Expense
	if len(before) > 0 {
		if err := json.Unmarshal(before, &previous); err != nil {
			return err
		}
	}
	if previous.LastSequence != 0 || previous.InvoiceID != "" || previous.PendingOperationID != "" || previous.PendingGeneration != 0 {
		return bizbilling.ErrReconciliationRequired
	}
	if len(after) == 0 {
		return nil
	}
	if err := json.Unmarshal(after, &next); err != nil {
		return err
	}
	if next.LastSequence != 0 || next.InvoiceID != "" || next.PendingOperationID != "" || next.PendingGeneration != 0 {
		return bizbilling.ErrReconciliationRequired
	}
	if next.AmountPaisa <= 0 || next.Currency == "" || (next.Billable && next.BilledToUser == "") {
		return bizbilling.ErrInvalidMoney
	}
	return nil
}

func (s *BizExpenseService) financialExpenseSummary(ctx context.Context, appID string, from, to *time.Time) (map[string]int64, error) {
	revision, err := beginFinancialReport(ctx, s.lifecycleSvc.gate, appID)
	if err != nil {
		return nil, err
	}
	state, err := s.lifecycleSvc.gate.State(ctx, appID)
	if err != nil {
		return nil, err
	}
	result := map[string]int64{}
	err = walkReportDefinitions(ctx, s.expenses, appID, func(e *bizbilling.Expense) string { return e.ID }, func(e *bizbilling.Expense) error {
		if (from != nil && e.Date.Before(*from)) || (to != nil && e.Date.After(*to)) {
			return nil
		}
		if err := guardFinancialRead(ctx, s.lifecycleSvc.gate, s.lifecycleSvc.operations, e.FinancialScope(), e.LastSequence); err != nil {
			return err
		}
		matches, err := reportCurrencyMatches(e.Currency, state.PrimaryCurrency)
		if err != nil || !matches {
			return err
		}
		value := result[e.Category]
		if err = addReportAmount(&value, e.AmountPaisa); err != nil {
			return err
		}
		result[e.Category] = value
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err = revision.verify(ctx, s.lifecycleSvc.gate); err != nil {
		return nil, err
	}
	return result, nil
}
