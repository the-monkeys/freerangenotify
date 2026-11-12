package bizbillingrepo

import (
	"context"
	"encoding/json"
)

type FinancialReadOverlay func(context.Context, string, string, json.RawMessage) (json.RawMessage, error)

func (s *Store[T]) SetFinancialReadOverlay(reader FinancialReadOverlay) {
	if s != nil {
		s.financialReader = reader
	}
}
func (s *Stores) SetFinancialReadOverlay(reader FinancialReadOverlay) {
	if s == nil {
		return
	}
	s.Invoices.SetFinancialReadOverlay(reader)
	s.Subscriptions.SetFinancialReadOverlay(reader)
	s.Payments.SetFinancialReadOverlay(reader)
	s.CreditNotes.SetFinancialReadOverlay(reader)
	s.Retainers.SetFinancialReadOverlay(reader)
	s.Estimates.SetFinancialReadOverlay(reader)
	s.Expenses.SetFinancialReadOverlay(reader)
}
func (r *LifecycleRepo) SetFinancialReadOverlay(reader FinancialReadOverlay) {
	r.invoices.SetFinancialReadOverlay(reader)
	r.subscriptions.SetFinancialReadOverlay(reader)
	r.payments.SetFinancialReadOverlay(reader)
}
