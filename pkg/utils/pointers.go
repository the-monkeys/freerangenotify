package utils

// BoolPtr returns a pointer to the given bool value
func BoolPtr(b bool) *bool {
	return &b
}

// BoolValue returns the value of the bool pointer or false if nil
func BoolValue(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

// StringPtr returns a pointer to the given string value
func StringPtr(s string) *string {
	return &s
}

// StringValue returns the value of the string pointer or empty string if nil
func StringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// IntPtr returns a pointer to the given int value
func IntPtr(i int) *int {
	return &i
}

// IntValue returns the value of the int pointer or 0 if nil
func IntValue(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}
