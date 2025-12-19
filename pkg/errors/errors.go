package errors

import (
	"fmt"
)

// ErrorCode represents application-specific error codes
type ErrorCode string

const (
	// Client errors (4xx)
	ErrCodeBadRequest        ErrorCode = "BAD_REQUEST"
	ErrCodeUnauthorized      ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden         ErrorCode = "FORBIDDEN"
	ErrCodeNotFound          ErrorCode = "NOT_FOUND"
	ErrCodeConflict          ErrorCode = "CONFLICT"
	ErrCodeValidation        ErrorCode = "VALIDATION_ERROR"
	ErrCodeInvalidAPIKey     ErrorCode = "INVALID_API_KEY"
	ErrCodeRateLimitExceeded ErrorCode = "RATE_LIMIT_EXCEEDED"

	// Server errors (5xx)
	ErrCodeInternal           ErrorCode = "INTERNAL_ERROR"
	ErrCodeServiceUnavailable ErrorCode = "SERVICE_UNAVAILABLE"
	ErrCodeDatabaseError      ErrorCode = "DATABASE_ERROR"
	ErrCodeExternalService    ErrorCode = "EXTERNAL_SERVICE_ERROR"

	// Domain-specific errors
	ErrCodeUserNotFound       ErrorCode = "USER_NOT_FOUND"
	ErrCodeUserAlreadyExists  ErrorCode = "USER_ALREADY_EXISTS"
	ErrCodeAppNotFound        ErrorCode = "APPLICATION_NOT_FOUND"
	ErrCodeAppAlreadyExists   ErrorCode = "APPLICATION_ALREADY_EXISTS"
	ErrCodeDeviceNotFound     ErrorCode = "DEVICE_NOT_FOUND"
	ErrCodeInvalidTemplate    ErrorCode = "INVALID_TEMPLATE"
	ErrCodeNotificationFailed ErrorCode = "NOTIFICATION_FAILED"
)

// New creates a new AppError with the given code and message
func New(code ErrorCode, message string) *AppError {
	category := CategoryInternal
	severity := SeverityMedium
	retryable := false

	// Map error code to category
	switch code {
	case ErrCodeValidation, ErrCodeBadRequest:
		category = CategoryValidation
		severity = SeverityLow
	case ErrCodeUnauthorized, ErrCodeInvalidAPIKey:
		category = CategoryAuth
		severity = SeverityMedium
	case ErrCodeForbidden:
		category = CategoryAuth
		severity = SeverityMedium
	case ErrCodeNotFound, ErrCodeUserNotFound, ErrCodeAppNotFound, ErrCodeDeviceNotFound:
		category = CategoryNotFound
		severity = SeverityLow
	case ErrCodeConflict, ErrCodeUserAlreadyExists, ErrCodeAppAlreadyExists:
		category = CategoryConflict
		severity = SeverityLow
	case ErrCodeRateLimitExceeded:
		category = CategoryRateLimit
		severity = SeverityMedium
		retryable = true
	case ErrCodeDatabaseError:
		category = CategoryDatabase
		severity = SeverityHigh
		retryable = true
	case ErrCodeServiceUnavailable:
		category = CategoryUnavailable
		severity = SeverityHigh
		retryable = true
	default:
		category = CategoryInternal
		severity = SeverityHigh
		retryable = true
	}

	return &AppError{
		Category:  category,
		Severity:  severity,
		Message:   message,
		Code:      string(code),
		Retryable: retryable,
	}
}

// Common error constructors for backward compatibility

// NotFound creates a not found error with resource and ID
func NotFound(resource string, id string) *AppError {
	return &AppError{
		Category:  CategoryNotFound,
		Severity:  SeverityLow,
		Message:   fmt.Sprintf("%s not found: %s", resource, id),
		Code:      string(ErrCodeNotFound),
		Retryable: false,
	}
}

// BadRequest creates a bad request error
func BadRequest(message string) *AppError {
	return &AppError{
		Category:  CategoryValidation,
		Severity:  SeverityLow,
		Message:   message,
		Code:      string(ErrCodeBadRequest),
		Retryable: false,
	}
}

// Unauthorized creates an unauthorized error
func Unauthorized(message string) *AppError {
	return &AppError{
		Category:  CategoryAuth,
		Severity:  SeverityMedium,
		Message:   message,
		Code:      string(ErrCodeUnauthorized),
		Retryable: false,
	}
}

// Internal creates an internal error wrapping another error
func Internal(message string, err error) *AppError {
	return &AppError{
		Category:   CategoryInternal,
		Severity:   SeverityHigh,
		Message:    message,
		Code:       string(ErrCodeInternal),
		Retryable:  true,
		Underlying: err,
	}
}

// Validation creates a validation error with details
func Validation(message string, details map[string]interface{}) *AppError {
	return &AppError{
		Category:  CategoryValidation,
		Severity:  SeverityLow,
		Message:   message,
		Code:      string(ErrCodeValidation),
		Retryable: false,
		Metadata:  details,
	}
}

// Conflict creates a conflict error
func Conflict(message string) *AppError {
	return &AppError{
		Category:  CategoryConflict,
		Severity:  SeverityLow,
		Message:   message,
		Code:      string(ErrCodeConflict),
		Retryable: false,
	}
}

// DatabaseError creates a database error
func DatabaseError(operation string, err error) *AppError {
	return &AppError{
		Category:   CategoryDatabase,
		Severity:   SeverityHigh,
		Message:    fmt.Sprintf("Database error during %s", operation),
		Code:       string(ErrCodeDatabaseError),
		Retryable:  true,
		Underlying: err,
	}
}
