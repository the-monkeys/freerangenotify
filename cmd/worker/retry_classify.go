package main

import (
	"errors"

	filedomain "github.com/the-monkeys/freerangenotify/internal/domain/file"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
)

// isNonRetryableError reports whether an error represents a permanent failure
// that another delivery attempt cannot fix. Retrying these only wastes queue
// slots and delays the inevitable dead-letter — e.g. a file_id that points at
// a missing ES doc or a URL pasted into the file_id slot will fail the same
// way on attempt 2 and attempt 3.
func isNonRetryableError(err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, notification.ErrInvalidFileID),
		errors.Is(err, notification.ErrInvalidAttachment),
		errors.Is(err, notification.ErrAttachmentMissingSource),
		errors.Is(err, notification.ErrAmbiguousAttachmentSource),
		errors.Is(err, notification.ErrTooManyAttachments),
		errors.Is(err, notification.ErrAttachmentTooLarge),
		errors.Is(err, notification.ErrTooManyActions),
		errors.Is(err, notification.ErrInvalidAction),
		errors.Is(err, filedomain.ErrFileNotFound),
		errors.Is(err, services.ErrFileSourceUnavailable),
		errors.Is(err, services.ErrAttachmentURLOversize):
		return true
	}
	return false
}
