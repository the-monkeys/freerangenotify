package bizbilling

import "time"

// ─── Revenue Recognition (simplified) ────────────────────────────────────

// RevenueSchedule spreads an invoice's revenue across the service period so
// that deferred vs recognized revenue can be reported.
type RevenueSchedule struct {
	ID              string                 `json:"id"`
	AppID           string                 `json:"app_id"`
	InvoiceID       string                 `json:"invoice_id"`
	SubscriptionID  string                 `json:"subscription_id,omitempty"`
	TotalPaisa      int64                  `json:"total_paisa"`
	RecognizedPaisa int64                  `json:"recognized_paisa"`
	DeferredPaisa   int64                  `json:"deferred_paisa"`
	Entries         []RevenueScheduleEntry `json:"entries"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}

// RevenueScheduleEntry is one recognition period (a calendar month).
type RevenueScheduleEntry struct {
	Period      string `json:"period"` // "2026-07"
	AmountPaisa int64  `json:"amount_paisa"`
	Recognized  bool   `json:"recognized"`
}

// BuildRevenueSchedule spreads totalPaisa evenly across the calendar months
// between periodStart and periodEnd (inclusive of the start month). Any
// rounding remainder is added to the final entry. Pure function.
func BuildRevenueSchedule(totalPaisa int64, periodStart, periodEnd time.Time) []RevenueScheduleEntry {
	if totalPaisa <= 0 || !periodEnd.After(periodStart) {
		return []RevenueScheduleEntry{{Period: periodStart.Format("2006-01"), AmountPaisa: totalPaisa}}
	}

	months := monthsBetween(periodStart, periodEnd)
	perMonth := totalPaisa / int64(len(months))

	entries := make([]RevenueScheduleEntry, len(months))
	var allocated int64
	for i, m := range months {
		amount := perMonth
		if i == len(months)-1 {
			amount = totalPaisa - allocated // remainder absorbs rounding
		}
		entries[i] = RevenueScheduleEntry{Period: m, AmountPaisa: amount}
		allocated += amount
	}
	return entries
}

// RecognizeThrough marks entries whose period is at or before `now`'s month
// as recognized and returns the updated recognized/deferred split.
func RecognizeThrough(schedule *RevenueSchedule, now time.Time) (recognized, deferred int64) {
	current := now.Format("2006-01")
	for i := range schedule.Entries {
		if schedule.Entries[i].Period <= current {
			schedule.Entries[i].Recognized = true
			recognized += schedule.Entries[i].AmountPaisa
		} else {
			deferred += schedule.Entries[i].AmountPaisa
		}
	}
	schedule.RecognizedPaisa = recognized
	schedule.DeferredPaisa = deferred
	return recognized, deferred
}

// monthsBetween lists "YYYY-MM" strings from start's month through end's
// month (end exclusive when it falls exactly on the first instant of a month
// boundary is not distinguished — the end month is always included).
func monthsBetween(start, end time.Time) []string {
	var months []string
	cursor := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, time.UTC)
	last := time.Date(end.Year(), end.Month(), 1, 0, 0, 0, 0, time.UTC)
	for !cursor.After(last) {
		months = append(months, cursor.Format("2006-01"))
		cursor = cursor.AddDate(0, 1, 0)
	}
	if len(months) == 0 {
		months = []string{start.Format("2006-01")}
	}
	return months
}
