package errors

import (
	"fmt"
	"net/http"
)

// ErrorCode represents application-specific error codes
type ErrorCode string

const (
	// Client errors (4xx)
	ErrCodeBadRequest          ErrorCode = "BAD_REQUEST"
	ErrCodeUnauthorized        ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden           ErrorCode = "FORBIDDEN"
	ErrCodeNotFound            ErrorCode = "NOT_FOUND"
	ErrCodeConflict            ErrorCode = "CONFLICT"
	ErrCodeValidation          ErrorCode = "VALIDATION_ERROR"
	ErrCodeInvalidAPIKey       ErrorCode = "INVALID_API_KEY"
	ErrCodeRateLimitExceeded   ErrorCode = "RATE_LIMIT_EXCEEDED"

	// Server errors (5xx)
	ErrCodeInternal            ErrorCode = "INTERNAL_ERROR"
	ErrCodeServiceUnavailable  ErrorCode = "SERVICE_UNAVAILABLE"
	ErrCodeDatabaseError       ErrorCode = "DATABASE_ERROR"
	ErrCodeExternalService     ErrorCode = "EXTERNAL_SERVICE_ERROR"

	// Domain-specific errors
	ErrCodeUserNotFound        ErrorCode = "USER_NOT_FOUND"
	ErrCodeUserAlreadyExists   ErrorCode = "USER_ALREADY_EXISTS"
	ErrCodeAppNotFound         ErrorCode = "APPLICATION_NOT_FOUND"
	ErrCodeAppAlreadyExists    ErrorCode = "APPLICATION_ALREADY_EXISTS"
	ErrCodeDeviceNotFound      ErrorCode = "DEVICE_NOT_FOUND"
	ErrCodeInvalidTemplate     ErrorCode = "INVALID_TEMPLATE"
	ErrCodeNotificationFailed  ErrorCode = "NOTIFICATION_FAILED"
)

// AppError represents a structured application error
type AppError struct {
	Code       ErrorCode              `json:"code"`
	Message    string                 `json:"message"`
	Details    map[string]interface{} `json:"details,omitempty"`
	StatusCode int                    `json:"-"`
	Err        error                  `json:"-"`
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap implements error unwrapping
func (e *AppError) Unwrap() error {
	return e.Err
}

// New creates a new AppError
func New(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		StatusCode: getStatusCode(code),
	}
}

// Wrap wraps an existing error with an AppError
func Wrap(code ErrorCode, message string, err error) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		StatusCode: getStatusCode(code),
		Err:        err,
	}
}

// WithDetails adds details to an AppError
func (e *AppError) WithDetails(details map[string]interface{}) *AppError {
	e.Details = details
	return e
}

// getStatusCode maps error codes to HTTP status codes
func getStatusCode(code ErrorCode) int {
	switch code {
	case ErrCodeBadRequest, ErrCodeValidation:
		return http.StatusBadRequest
	case ErrCodeUnauthorized, ErrCodeInvalidAPIKey:
		return http.StatusUnauthorized
	case ErrCodeForbidden:
		return http.StatusForbidden
	case ErrCodeNotFound, ErrCodeUserNotFound, ErrCodeAppNotFound, ErrCodeDeviceNotFound:
		return http.StatusNotFound
	case ErrCodeConflict, ErrCodeUserAlreadyExists, ErrCodeAppAlreadyExists:
		return http.StatusConflict
	case ErrCodeRateLimitExceeded:
		return http.StatusTooManyRequests
	case ErrCodeServiceUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

// Common error constructors
func NotFound(resource string, id string) *AppError {
	return New(ErrCodeNotFound, fmt.Sprintf("%s not found: %s", resource, id))
}

func BadRequest(message string) *AppError {
	return New(ErrCodeBadRequest, message)
}

func Unauthorized(message string) *AppError {
	return New(ErrCodeUnauthorized, message)
}

func Internal(message string, err error) *AppError {
	return Wrap(ErrCodeInternal, message, err)
}

func Validation(message string, details map[string]interface{}) *AppError {
	return New(ErrCodeValidation, message).WithDetails(details)
}

func Conflict(message string) *AppError {
	return New(ErrCodeConflict, message)
}

func DatabaseError(operation string, err error) *AppError {
	return Wrap(ErrCodeDatabaseError, fmt.Sprintf("Database error during %s", operation), err)
}
