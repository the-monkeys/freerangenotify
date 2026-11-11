package bizbillingrepo

import (
	"context"
	"fmt"
	"io"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
)

// RefreshFinancialProjections makes discovery queries include every completed
// fact. Realtime GET alone cannot make multi-resource reports consistent.
func (s *Stores) RefreshFinancialProjections(ctx context.Context, event bizbilling.JournalEvent) error {
	indices := []string{}
	for _, item := range []struct {
		changed bool
		index   string
	}{
		{event.InvoiceAfter != nil, invoiceIndex}, {event.PaymentAfter != nil, paymentIndex},
		{len(event.ExpensesAfter) > 0, ExpenseIndex}, {event.EstimateAfter != nil, EstimateIndex}, {event.CreditNoteAfter != nil, CreditNoteIndex},
		{event.RetainerAfter != nil, RetainerIndex}, {event.CreditApplicationAfter != nil, CreditApplicationIndex},
		{len(event.CreditLotsAfter) > 0, CreditLotIndex}, {event.RefundReservationAfter != nil, RefundReservationIndex},
		{event.CouponRedemptionAfter != nil, CouponRedemptionIndex}, {event.CouponRedemptionAfter != nil, CouponIndex},
		{event.CouponRedemptionAfter != nil, CouponCustomerUsageIndex},
	} {
		if item.changed {
			indices = append(indices, item.index)
		}
	}
	if len(indices) == 0 {
		return nil
	}
	res, err := s.Invoices.es.Indices.Refresh(s.Invoices.es.Indices.Refresh.WithIndex(indices...), s.Invoices.es.Indices.Refresh.WithContext(ctx))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, res.Body)
	if res.IsError() {
		return fmt.Errorf("financial discovery refresh HTTP %d", res.StatusCode)
	}
	return nil
}
