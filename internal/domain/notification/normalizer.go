package notification

import (
	"strings"
	"unicode"
)

// NormalizeEmail lowercases and trims an email address.
// Returns empty string if the input is empty after trimming.
func NormalizeEmail(email string) string {
	trimmed := strings.TrimSpace(email)
	if trimmed == "" {
		return ""
	}
	return strings.ToLower(trimmed)
}

// NormalizePhone produces a lightweight E.164-ish representation suitable for
// deduplication/skipping. It strips spaces, dashes, parentheses, and dots.
// If the number starts with "00", it is converted to "+".
// If it has no "+" prefix, we prefix "+" to avoid ambiguity (best effort).
// Returns empty string when the input has no digits.
func NormalizePhone(phone string) string {
	if phone == "" {
		return ""
	}

	var b strings.Builder
	for _, r := range phone {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		} else if r == '+' {
			// keep plus to preserve country code marker
			if b.Len() == 0 {
				b.WriteRune(r)
			}
		}
	}
	clean := b.String()
	if clean == "" {
		return ""
	}

	// Convert leading 00 to +
	if strings.HasPrefix(clean, "00") {
		clean = "+" + strings.TrimPrefix(clean, "00")
	}

	// If no + present, prefix + (best effort; we won't infer country)
	if !strings.HasPrefix(clean, "+") {
		clean = "+" + clean
	}

	return clean
}
