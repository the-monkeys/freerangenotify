package notification

import "errors"

// Domain errors for notification
var (
	ErrInvalidNotificationID = errors.New("invalid notification ID")
	ErrInvalidAppID          = errors.New("invalid app ID")
	ErrInvalidUserID         = errors.New("invalid user ID")
	ErrInvalidChannel        = errors.New("invalid channel")
	ErrInvalidPriority       = errors.New("invalid priority")
	ErrInvalidStatus         = errors.New("invalid status")
	ErrEmptyContent          = errors.New("notification content cannot be empty")
	ErrNotificationNotFound  = errors.New("notification not found")
	ErrCannotCancelSent      = errors.New("cannot cancel already sent notification")
	ErrCannotRetry           = errors.New("notification cannot be retried")
	ErrMaxRetriesExceeded    = errors.New("maximum retry attempts exceeded")
	ErrInvalidScheduleTime   = errors.New("invalid schedule time")
)

// IsValidationError checks if an error is a validation error
func IsValidationError(err error) bool {
	return errors.Is(err, ErrInvalidAppID) ||
		errors.Is(err, ErrInvalidUserID) ||
		errors.Is(err, ErrInvalidChannel) ||
		errors.Is(err, ErrInvalidPriority) ||
		errors.Is(err, ErrEmptyContent) ||
		errors.Is(err, ErrInvalidScheduleTime) ||
		errors.Is(err, ErrInvalidNotificationID) ||
		errors.Is(err, ErrInvalidStatus)
}
