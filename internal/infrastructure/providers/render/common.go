package render

import "github.com/the-monkeys/freerangenotify/internal/domain/notification"

// FormatContentString converts Content into a readable plain-text string.
func FormatContentString(c notification.Content) string {
	if c.Title != "" && c.Body != "" {
		return c.Title + "\n" + c.Body
	}
	if c.Body != "" {
		return c.Body
	}
	if c.Title != "" {
		return c.Title
	}
	return ""
}
