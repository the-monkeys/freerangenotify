package bizbillingrepo

import (
	"context"
	"encoding/json"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
)

// FinancialWriteGuard holds an app writer permit until persistence finishes.
// before/after retain exact JSON; nil after means deletion.
type FinancialWriteGuard func(context.Context, string, json.RawMessage, json.RawMessage) (func(), error)

func (s *Store[T]) SetFinancialWriteGuard(guard FinancialWriteGuard) {
	if s != nil {
		s.financialGuard = guard
	}
}
func (s *Store[T]) guardFinancialWrite(ctx context.Context, before, after json.RawMessage) (func(), error) {
	if s.financialGuard == nil {
		return func() {}, nil
	}
	return s.financialGuard(ctx, s.index, before, after)
}

func (s *Stores) SetFinancialWriteGuard(guard FinancialWriteGuard) {
	if s == nil {
		return
	}
	s.RevenueSchedules.SetFinancialWriteGuard(guard)
	s.Expenses.SetFinancialWriteGuard(guard)
	s.Estimates.SetFinancialWriteGuard(guard)
	s.Coupons.SetFinancialWriteGuard(guard)
	s.CouponRedemptions.SetFinancialWriteGuard(guard)
	s.Subscriptions.SetFinancialWriteGuard(guard)
	s.Invoices.SetFinancialWriteGuard(guard)
	s.Payments.SetFinancialWriteGuard(guard)
	s.CreditNotes.SetFinancialWriteGuard(guard)
	s.Retainers.SetFinancialWriteGuard(guard)
	s.CreditLots.SetFinancialWriteGuard(guard)
	s.RefundReservations.SetFinancialWriteGuard(guard)
	s.CreditApplications.SetFinancialWriteGuard(guard)
	s.UserBalances.SetFinancialWriteGuard(guard)
}
func (s *LifecycleRepo) SetFinancialWriteGuard(guard FinancialWriteGuard) {
	s.subscriptions.SetFinancialWriteGuard(guard)
	s.invoices.SetFinancialWriteGuard(guard)
	s.payments.SetFinancialWriteGuard(guard)
}

type financialProjectionContextKey struct{}

// WithFinancialProjection carries the exact lease being verified before every
// projection write. Merely possessing a scope or a sequence is insufficient.
func WithFinancialProjection(ctx context.Context, claim bizbilling.StreamClaim) context.Context {
	return context.WithValue(ctx, financialProjectionContextKey{}, claim)
}
func FinancialProjectionClaim(ctx context.Context) (bizbilling.StreamClaim, bool) {
	claim, ok := ctx.Value(financialProjectionContextKey{}).(bizbilling.StreamClaim)
	return claim, ok
}
