package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// BizExpenseService records business expenses and converts billable ones
// into invoice line items.
type BizExpenseService struct {
	expenses     *bizbillingrepo.Store[bizbilling.Expense]
	lifecycleSvc *BizLifecycleService
	logger       *zap.Logger
}

// NewBizExpenseService creates a new BizExpenseService.
func NewBizExpenseService(stores *bizbillingrepo.Stores, lifecycleSvc *BizLifecycleService, logger *zap.Logger) *BizExpenseService {
	return &BizExpenseService{expenses: stores.Expenses, lifecycleSvc: lifecycleSvc, logger: logger}
}

// Create records an expense.
func (s *BizExpenseService) Create(ctx context.Context, appID string, req *bizbilling.CreateExpenseRequest) (*bizbilling.Expense, error) {
	if req.Billable && req.BilledToUser == "" {
		return nil, errors.BadRequest("billed_to_user is required for billable expenses")
	}

	now := time.Now().UTC()
	date := now
	if req.Date != nil {
		date = req.Date.UTC()
	}
	expense := &bizbilling.Expense{
		ID:              uuid.New().String(),
		AppID:           appID,
		Category:        req.Category,
		Description:     req.Description,
		AmountPaisa:     req.AmountPaisa,
		Vendor:          req.Vendor,
		Date:            date,
		Billable:        req.Billable,
		BilledToUser:    req.BilledToUser,
		ReceiptFileID:   req.ReceiptFileID,
		Recurring:       req.Recurring,
		RecurrenceCycle: req.RecurrenceCycle,
		Metadata:        req.Metadata,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.expenses.Create(ctx, expense.ID, expense); err != nil {
		return nil, err
	}
	return expense, nil
}

// Get returns an expense scoped to the app.
func (s *BizExpenseService) Get(ctx context.Context, appID, id string) (*bizbilling.Expense, error) {
	expense, err := s.expenses.Get(ctx, id)
	if err != nil || expense.AppID != appID {
		return nil, errors.NotFound("expense", id)
	}
	return expense, nil
}

// List lists expenses for an app.
func (s *BizExpenseService) List(ctx context.Context, filter bizbilling.ExpenseFilter) ([]*bizbilling.Expense, int64, error) {
	q := bizbillingrepo.NewQuery(filter.AppID).
		Term("category", filter.Category).
		Term("billed_to_user", filter.UserID)
	if filter.Billable != nil {
		q.Term("billable", *filter.Billable)
	}
	var from, to interface{}
	if filter.From != nil {
		from = filter.From
	}
	if filter.To != nil {
		to = filter.To
	}
	q.Range("date", from, to)
	return s.expenses.Find(ctx, q.Body("date", filter.Limit, filter.Offset))
}

// Update updates an expense that has not been invoiced yet.
func (s *BizExpenseService) Update(ctx context.Context, appID, id string, req *bizbilling.UpdateExpenseRequest) (*bizbilling.Expense, error) {
	return s.expenses.Update(ctx, id, func(e *bizbilling.Expense) error {
		if e.AppID != appID {
			return errors.NotFound("expense", id)
		}
		if e.InvoiceID != "" {
			return errors.BadRequest("expense is already attached to an invoice")
		}
		if req.Category != nil {
			e.Category = *req.Category
		}
		if req.Description != nil {
			e.Description = *req.Description
		}
		if req.AmountPaisa != nil {
			e.AmountPaisa = *req.AmountPaisa
		}
		if req.Vendor != nil {
			e.Vendor = *req.Vendor
		}
		if req.Date != nil {
			e.Date = req.Date.UTC()
		}
		if req.Billable != nil {
			e.Billable = *req.Billable
		}
		if req.BilledToUser != nil {
			e.BilledToUser = *req.BilledToUser
		}
		if req.ReceiptFileID != nil {
			e.ReceiptFileID = *req.ReceiptFileID
		}
		e.UpdatedAt = time.Now().UTC()
		return nil
	})
}

// Delete removes an un-invoiced expense.
func (s *BizExpenseService) Delete(ctx context.Context, appID, id string) error {
	expense, err := s.Get(ctx, appID, id)
	if err != nil {
		return err
	}
	if expense.InvoiceID != "" {
		return errors.BadRequest("expense is already attached to an invoice")
	}
	return s.expenses.Delete(ctx, id)
}

// ConvertToInvoice bundles a user's un-invoiced billable expenses into a new
// draft invoice (one line item per expense) and marks them as invoiced.
func (s *BizExpenseService) ConvertToInvoice(ctx context.Context, appID, userID string, expenseIDs []string) (*bizbilling.BizInvoice, error) {
	if len(expenseIDs) == 0 {
		return nil, errors.BadRequest("at least one expense_id is required")
	}

	expenses := make([]*bizbilling.Expense, 0, len(expenseIDs))
	lineItems := make([]bizbilling.InvoiceLineItem, 0, len(expenseIDs))
	for _, id := range expenseIDs {
		e, err := s.Get(ctx, appID, id)
		if err != nil {
			return nil, err
		}
		if !e.Billable {
			return nil, errors.BadRequest(fmt.Sprintf("expense %s is not billable", id))
		}
		if e.InvoiceID != "" {
			return nil, errors.BadRequest(fmt.Sprintf("expense %s is already invoiced", id))
		}
		if e.BilledToUser != userID {
			return nil, errors.BadRequest(fmt.Sprintf("expense %s is billed to a different user", id))
		}

		desc := e.Description
		if desc == "" {
			desc = e.Category
		}
		lineItems = append(lineItems, bizbilling.InvoiceLineItem{
			Description: fmt.Sprintf("Expense: %s (%s)", desc, e.Date.Format("02 Jan 2006")),
			Quantity:    1,
			UnitPrice:   e.AmountPaisa,
			AmountPaisa: e.AmountPaisa,
		})
		expenses = append(expenses, e)
	}

	invoice, err := s.lifecycleSvc.CreateOneTimeInvoice(ctx, appID, &bizbilling.CreateInvoiceRequest{
		UserID:    userID,
		LineItems: lineItems,
	})
	if err != nil {
		return nil, err
	}

	for _, e := range expenses {
		if _, uerr := s.expenses.Update(ctx, e.ID, func(ee *bizbilling.Expense) error {
			ee.InvoiceID = invoice.ID
			ee.UpdatedAt = time.Now().UTC()
			return nil
		}); uerr != nil {
			s.logger.Error("bizbilling: failed to mark expense invoiced",
				zap.String("expense_id", e.ID), zap.String("invoice_id", invoice.ID), zap.Error(uerr))
		}
	}
	return invoice, nil
}

// CategorySummary aggregates expense totals per category within a period.
func (s *BizExpenseService) CategorySummary(ctx context.Context, appID string, from, to *time.Time) (map[string]int64, error) {
	q := bizbillingrepo.NewQuery(appID)
	var gte, lte interface{}
	if from != nil {
		gte = from
	}
	if to != nil {
		lte = to
	}
	q.Range("date", gte, lte)

	aggs, err := s.expenses.Aggregate(ctx, map[string]interface{}{
		"query": q.Bool(),
		"size":  0,
		"aggs": map[string]interface{}{
			"by_category": map[string]interface{}{
				"terms": map[string]interface{}{"field": "category", "size": 50},
				"aggs": map[string]interface{}{
					"total": map[string]interface{}{
						"sum": map[string]interface{}{"field": "amount_paisa"},
					},
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	summary := map[string]int64{}
	byCat, _ := aggs["by_category"].(map[string]interface{})
	buckets, _ := byCat["buckets"].([]interface{})
	for _, b := range buckets {
		bucket, ok := b.(map[string]interface{})
		if !ok {
			continue
		}
		key, _ := bucket["key"].(string)
		total, _ := bucket["total"].(map[string]interface{})
		if v, ok := total["value"].(float64); ok {
			summary[key] = int64(v)
		}
	}
	return summary, nil
}
