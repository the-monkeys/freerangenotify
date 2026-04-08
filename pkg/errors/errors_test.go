package errors

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsNotFound_WithNotFoundError(t *testing.T) {
	err := NotFound("user", "123")
	assert.True(t, IsNotFound(err))
}

func TestIsNotFound_WithNewNotFoundError(t *testing.T) {
	err := NewNotFoundError("user", "123")
	assert.True(t, IsNotFound(err))
}

func TestIsNotFound_WithDatabaseError(t *testing.T) {
	err := DatabaseError("get user", fmt.Errorf("connection refused"))
	assert.False(t, IsNotFound(err))
}

func TestIsNotFound_WithInternalError(t *testing.T) {
	err := Internal("server error", nil)
	assert.False(t, IsNotFound(err))
}

func TestIsNotFound_WithNilError(t *testing.T) {
	assert.False(t, IsNotFound(nil))
}

func TestIsNotFound_WithPlainError(t *testing.T) {
	err := fmt.Errorf("something went wrong")
	assert.False(t, IsNotFound(err))
}

func TestIsNotFound_WithPlainNotFoundString(t *testing.T) {
	// The existing IsNotFound in categorized_errors.go has a string fallback
	err := fmt.Errorf("user not found")
	assert.True(t, IsNotFound(err))
}

func TestIsNotFound_WithWrappedNotFoundError(t *testing.T) {
	inner := NotFound("user", "456")
	wrapped := fmt.Errorf("lookup failed: %w", inner)
	assert.True(t, IsNotFound(wrapped))
}

func TestNotFound_HTTPStatus(t *testing.T) {
	err := NotFound("user", "123")
	assert.Equal(t, 404, err.GetHTTPStatus())
}

func TestNotFound_Fields(t *testing.T) {
	err := NotFound("user", "abc-123")
	assert.Equal(t, CategoryNotFound, err.Category)
	assert.Equal(t, SeverityLow, err.Severity)
	assert.Equal(t, string(ErrCodeNotFound), err.Code)
	assert.False(t, err.Retryable)
	assert.Contains(t, err.Message, "user not found")
}

func TestNotFound_WithMetadata(t *testing.T) {
	err := NotFound("user", "123").
		WithMetadata("hint", "try a different identifier")
	assert.Equal(t, "try a different identifier", err.Metadata["hint"])
	assert.True(t, IsNotFound(err))
}
