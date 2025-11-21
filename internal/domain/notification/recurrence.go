package notification

import (
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

// CalculateNextRun calculates the next run time based on recurrence rules
func (r *Recurrence) CalculateNextRun(lastRun time.Time) (time.Time, error) {
	if r.CronExpression == "" {
		return time.Time{}, fmt.Errorf("empty cron expression")
	}

	// Check if we've reached the max count
	if r.Count > 0 && r.CurrentCount >= r.Count {
		return time.Time{}, nil // No more runs
	}

	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	schedule, err := parser.Parse(r.CronExpression)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid cron expression: %w", err)
	}

	nextTime := schedule.Next(lastRun)

	// Check if we've passed the end date
	if r.EndDate != nil && nextTime.After(*r.EndDate) {
		return time.Time{}, nil // No more runs
	}

	return nextTime, nil
}

// ShouldRun returns true if the notification should run again
func (r *Recurrence) ShouldRun(now time.Time) bool {
	if r.Count > 0 && r.CurrentCount >= r.Count {
		return false
	}
	if r.EndDate != nil && now.After(*r.EndDate) {
		return false
	}
	return true
}
