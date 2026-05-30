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
	ErrRateLimitExceeded     = errors.New("user exceeded daily notification limit")
	ErrDNDEnabled            = errors.New("user has Do Not Disturb enabled")
	ErrQuietHours            = errors.New("user is in quiet hours, only critical notifications allowed")
	ErrTemplateRequired      = errors.New("template_id is required")
	ErrTemplateNotFound      = errors.New("template not found")
	ErrCannotSnooze          = errors.New("notification cannot be snoozed")
	ErrCannotArchive         = errors.New("notification cannot be archived")
	ErrInvalidSnoozeDuration = errors.New("invalid snooze duration")

	// Rich content validation (Phase 7 — Webhook channel expansion).
	ErrTooManyAttachments = errors.New("content.attachments: at most 10 allowed")
	ErrInvalidAttachment  = errors.New("content.attachments: each item requires type and a valid source/disposition")
	ErrTooManyActions     = errors.New("content.actions: at most 5 allowed")
	ErrInvalidAction      = errors.New("content.actions: type and label required; url required for link actions")
	ErrTooManyFields      = errors.New("content.fields: at most 25 allowed")
	ErrInvalidField       = errors.New("content.fields: each item requires key and value")
	ErrInvalidPoll        = errors.New("content.poll: question plus 2–10 non-empty choices required")

	// File-attachment source validation (file-attachments feature).
	ErrAttachmentMissingSource   = errors.New("content.attachments: each item requires exactly one of url, content_base64, or file_id")
	ErrAmbiguousAttachmentSource = errors.New("content.attachments: provide exactly one of url, content_base64, or file_id per item")
	ErrAttachmentTooLarge        = errors.New("content.attachments: inline payload exceeds the per-item size limit")
	ErrInvalidFileID             = errors.New("content.attachments: file_id must start with 'file_' and contain no URL/path characters — use the url field for remote URLs")
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
		errors.Is(err, ErrInvalidStatus) ||
		errors.Is(err, ErrTemplateRequired) ||
		errors.Is(err, ErrTemplateNotFound) ||
		errors.Is(err, ErrTooManyAttachments) ||
		errors.Is(err, ErrInvalidAttachment) ||
		errors.Is(err, ErrAttachmentMissingSource) ||
		errors.Is(err, ErrAmbiguousAttachmentSource) ||
		errors.Is(err, ErrAttachmentTooLarge) ||
		errors.Is(err, ErrInvalidFileID) ||
		errors.Is(err, ErrTooManyActions) ||
		errors.Is(err, ErrInvalidAction)
}
