package utils

import (
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/user"
)

// IsQuietHours checks if the current time falls within a user's configured quiet hours.
// Returns false if quiet hours are not configured.
func IsQuietHours(u *user.User) bool {
	if u.Preferences.QuietHours.Start == "" || u.Preferences.QuietHours.End == "" {
		return false
	}

	now := time.Now()
	if u.Timezone != "" {
		if loc, err := time.LoadLocation(u.Timezone); err == nil {
			now = now.In(loc)
		}
	}

	currentTime := now.Format("15:04")
	start := u.Preferences.QuietHours.Start
	end := u.Preferences.QuietHours.End

	// Handle quiet hours spanning midnight (e.g., 22:00 - 07:00)
	if start < end {
		return currentTime >= start && currentTime < end
	}
	return currentTime >= start || currentTime < end
}
