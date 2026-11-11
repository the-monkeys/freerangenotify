package bizbilling

import (
	"errors"
	"time"
)

var ErrInvalidPeriod = errors.New("invalid billing period or anniversary")

// NextAnchoredPeriod retains the original anniversary across short months.
// Legacy stored period boundaries are never passed through a migration rewrite.
func NextAnchoredPeriod(start time.Time, cycle BillingCycle, cycleDays, anchorDay int) (time.Time, error) {
	start = start.UTC()
	if start.IsZero() || start.Year() < 1 || start.Year() > 9999 || anchorDay < 0 || anchorDay > 31 {
		return time.Time{}, ErrInvalidPeriod
	}
	if anchorDay == 0 {
		anchorDay = start.Day()
	}
	months := 0
	days := 0
	switch cycle {
	case BillingCycleWeekly:
		days = 7
	case BillingCycleMonthly:
		months = 1
	case BillingCycleQuarterly:
		months = 3
	case BillingCycleHalfYearly:
		months = 6
	case BillingCycleYearly:
		months = 12
	case BillingCycleCustom:
		if cycleDays <= 0 || cycleDays > (10000-start.Year())*366 {
			return time.Time{}, ErrInvalidPeriod
		}
		days = cycleDays
	default:
		return time.Time{}, ErrInvalidPeriod
	}
	var end time.Time
	if days > 0 {
		end = start.AddDate(0, 0, days)
	} else {
		month := int(start.Month()) + months
		year := start.Year() + (month-1)/12
		month = (month-1)%12 + 1
		if year > 9999 {
			return time.Time{}, ErrInvalidPeriod
		}
		last := time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day()
		day := anchorDay
		if day > last {
			day = last
		}
		end = time.Date(year, time.Month(month), day, start.Hour(), start.Minute(), start.Second(), start.Nanosecond(), time.UTC)
	}
	if !end.After(start) || end.Year() > 9999 {
		return time.Time{}, ErrInvalidPeriod
	}
	return end, nil
}
