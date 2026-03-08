package freerangenotify

import "fmt"

// APIError represents an error response from the FreeRangeNotify API.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("freerangenotify: API error %d: %s", e.StatusCode, e.Body)
}

// IsNotFound returns true if the error is a 404 Not Found.
func (e *APIError) IsNotFound() bool { return e.StatusCode == 404 }

// IsUnauthorized returns true if the error is a 401 Unauthorized.
func (e *APIError) IsUnauthorized() bool { return e.StatusCode == 401 }

// IsRateLimited returns true if the error is a 429 Too Many Requests.
func (e *APIError) IsRateLimited() bool { return e.StatusCode == 429 }

// IsValidationError returns true if the error is a 400 Bad Request or 422 Unprocessable Entity.
func (e *APIError) IsValidationError() bool {
	return e.StatusCode == 400 || e.StatusCode == 422
}
