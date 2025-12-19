package errors

import (
	"errors"
	"fmt"
)

// ErrorCategory represents the category of an error
type ErrorCategory string

const (
	// Client errors (4xx equivalent)
	CategoryValidation ErrorCategory = "validation" // Invalid input
	CategoryAuth       ErrorCategory = "auth"       // Authentication/authorization
	CategoryNotFound   ErrorCategory = "not_found"  // Resource not found
	CategoryConflict   ErrorCategory = "conflict"   // Resource conflict
	CategoryRateLimit  ErrorCategory = "rate_limit" // Rate limit exceeded

	// Server errors (5xx equivalent)
	CategoryInternal    ErrorCategory = "internal"    // Internal server error
	CategoryDatabase    ErrorCategory = "database"    // Database error
	CategoryNetwork     ErrorCategory = "network"     // Network/connectivity error
	CategoryProvider    ErrorCategory = "provider"    // External provider error
	CategoryTimeout     ErrorCategory = "timeout"     // Operation timeout
	CategoryUnavailable ErrorCategory = "unavailable" // Service unavailable

	// Business logic errors
	CategoryBusiness  ErrorCategory = "business"  // Business rule violation
	CategoryQuota     ErrorCategory = "quota"     // Quota exceeded
	CategoryFrequency ErrorCategory = "frequency" // Frequency limit
)

// ErrorSeverity represents how critical an error is
type ErrorSeverity string

const (
	SeverityLow      ErrorSeverity = "low"      // Minor issue, can continue
	SeverityMedium   ErrorSeverity = "medium"   // Significant but not critical
	SeverityHigh     ErrorSeverity = "high"     // Critical issue
	SeverityCritical ErrorSeverity = "critical" // System-level failure
)

// AppError represents a categorized application error
type AppError struct {
	Category   ErrorCategory          `json:"category"`
	Severity   ErrorSeverity          `json:"severity"`
	Message    string                 `json:"message"`
	Code       string                 `json:"code"`
	Retryable  bool                   `json:"retryable"`
	Underlying error                  `json:"-"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Underlying != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Underlying)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying error
func (e *AppError) Unwrap() error {
	return e.Underlying
}

// WithMetadata adds metadata to the error
func (e *AppError) WithMetadata(key string, value interface{}) *AppError {
	if e.Metadata == nil {
		e.Metadata = make(map[string]interface{})
	}
	e.Metadata[key] = value
	return e
}

// IsRetryable returns whether the error is retryable
func (e *AppError) IsRetryable() bool {
	return e.Retryable
}

// GetHTTPStatus returns the appropriate HTTP status code
func (e *AppError) GetHTTPStatus() int {
	switch e.Category {
	case CategoryValidation:
		return 400
	case CategoryAuth:
		return 401
	case CategoryNotFound:
		return 404
	case CategoryConflict:
		return 409
	case CategoryRateLimit:
		return 429
	case CategoryUnavailable:
		return 503
	case CategoryTimeout:
		return 504
	default:
		return 500
	}
}

// Common error constructors

func NewValidationError(message string, err error) *AppError {
	return &AppError{
		Category:   CategoryValidation,
		Severity:   SeverityLow,
		Message:    message,
		Code:       "VALIDATION_ERROR",
		Retryable:  false,
		Underlying: err,
	}
}

func NewAuthError(message string, err error) *AppError {
	return &AppError{
		Category:   CategoryAuth,
		Severity:   SeverityMedium,
		Message:    message,
		Code:       "AUTH_ERROR",
		Retryable:  false,
		Underlying: err,
	}
}

func NewNotFoundError(resource string, id string) *AppError {
	return &AppError{
		Category:  CategoryNotFound,
		Severity:  SeverityLow,
		Message:   fmt.Sprintf("%s not found", resource),
		Code:      "NOT_FOUND",
		Retryable: false,
		Metadata:  map[string]interface{}{"resource": resource, "id": id},
	}
}

func NewConflictError(message string, err error) *AppError {
	return &AppError{
		Category:   CategoryConflict,
		Severity:   SeverityMedium,
		Message:    message,
		Code:       "CONFLICT",
		Retryable:  false,
		Underlying: err,
	}
}

func NewRateLimitError(retryAfter int) *AppError {
	return &AppError{
		Category:  CategoryRateLimit,
		Severity:  SeverityMedium,
		Message:   "Rate limit exceeded",
		Code:      "RATE_LIMIT_EXCEEDED",
		Retryable: true,
		Metadata:  map[string]interface{}{"retry_after": retryAfter},
	}
}

func NewDatabaseError(message string, err error) *AppError {
	return &AppError{
		Category:   CategoryDatabase,
		Severity:   SeverityHigh,
		Message:    message,
		Code:       "DATABASE_ERROR",
		Retryable:  true,
		Underlying: err,
	}
}

func NewNetworkError(message string, err error) *AppError {
	return &AppError{
		Category:   CategoryNetwork,
		Severity:   SeverityHigh,
		Message:    message,
		Code:       "NETWORK_ERROR",
		Retryable:  true,
		Underlying: err,
	}
}

func NewProviderError(provider string, err error) *AppError {
	return &AppError{
		Category:   CategoryProvider,
		Severity:   SeverityHigh,
		Message:    fmt.Sprintf("Provider %s failed", provider),
		Code:       "PROVIDER_ERROR",
		Retryable:  true,
		Underlying: err,
		Metadata:   map[string]interface{}{"provider": provider},
	}
}

func NewTimeoutError(operation string, err error) *AppError {
	return &AppError{
		Category:   CategoryTimeout,
		Severity:   SeverityHigh,
		Message:    fmt.Sprintf("Operation %s timed out", operation),
		Code:       "TIMEOUT",
		Retryable:  true,
		Underlying: err,
		Metadata:   map[string]interface{}{"operation": operation},
	}
}

func NewUnavailableError(service string) *AppError {
	return &AppError{
		Category:  CategoryUnavailable,
		Severity:  SeverityCritical,
		Message:   fmt.Sprintf("Service %s is unavailable", service),
		Code:      "SERVICE_UNAVAILABLE",
		Retryable: true,
		Metadata:  map[string]interface{}{"service": service},
	}
}

func NewInternalError(message string, err error) *AppError {
	return &AppError{
		Category:   CategoryInternal,
		Severity:   SeverityCritical,
		Message:    message,
		Code:       "INTERNAL_ERROR",
		Retryable:  false,
		Underlying: err,
	}
}

func NewBusinessError(message string, code string) *AppError {
	return &AppError{
		Category:  CategoryBusiness,
		Severity:  SeverityMedium,
		Message:   message,
		Code:      code,
		Retryable: false,
	}
}

func NewQuotaError(resource string, limit int) *AppError {
	return &AppError{
		Category:  CategoryQuota,
		Severity:  SeverityMedium,
		Message:   fmt.Sprintf("Quota exceeded for %s", resource),
		Code:      "QUOTA_EXCEEDED",
		Retryable: false,
		Metadata:  map[string]interface{}{"resource": resource, "limit": limit},
	}
}

func NewFrequencyError(window string, limit int) *AppError {
	return &AppError{
		Category:  CategoryFrequency,
		Severity:  SeverityMedium,
		Message:   fmt.Sprintf("Frequency limit exceeded: %d per %s", limit, window),
		Code:      "FREQUENCY_EXCEEDED",
		Retryable: true,
		Metadata:  map[string]interface{}{"window": window, "limit": limit},
	}
}

// IsAppError checks if an error is an AppError
func IsAppError(err error) bool {
	var appErr *AppError
	return errors.As(err, &appErr)
}

// AsAppError converts an error to AppError if possible
func AsAppError(err error) (*AppError, bool) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr, true
	}
	return nil, false
}

// GetCategory returns the error category or a default
func GetCategory(err error) ErrorCategory {
	if appErr, ok := AsAppError(err); ok {
		return appErr.Category
	}
	return CategoryInternal
}

// IsRetryableError checks if an error should be retried
func IsRetryableError(err error) bool {
	if appErr, ok := AsAppError(err); ok {
		return appErr.Retryable
	}
	return false
}
