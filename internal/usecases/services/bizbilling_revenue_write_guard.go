package services

import (
	"encoding/json"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
)

// Recognition is a derived accrual schedule, not another cash entry. Its source
// and total stay fixed while progress participates in app report revisions.
func guardRevenueSchedule(before, after json.RawMessage) error {
	if len(after) == 0 {
		return bizbilling.ErrReconciliationRequired
	}
	var previous, next bizbilling.RevenueSchedule
	if err := json.Unmarshal(after, &next); err != nil {
		return err
	}
	if next.ID == "" || next.InvoiceID == "" || next.TotalPaisa < 0 || next.RecognizedPaisa < 0 || next.DeferredPaisa < 0 {
		return bizbilling.ErrInvalidMoney
	}
	sum, err := bizbilling.CheckedAdd(next.RecognizedPaisa, next.DeferredPaisa)
	if err != nil {
		return err
	}
	if sum != next.TotalPaisa {
		return bizbilling.ErrReconciliationRequired
	}
	if len(before) > 0 {
		if err = json.Unmarshal(before, &previous); err != nil {
			return err
		}
		if previous.ID != next.ID || previous.AppID != next.AppID || previous.InvoiceID != next.InvoiceID || previous.SubscriptionID != next.SubscriptionID || previous.TotalPaisa != next.TotalPaisa || previous.RecognizedPaisa > next.RecognizedPaisa {
			return bizbilling.ErrReconciliationRequired
		}
	}
	return nil
}
