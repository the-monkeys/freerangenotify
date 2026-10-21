package otp

import (
	"context"
	"errors"
	"time"
)

var (
	ErrConflict         = errors.New("otp: state changed")
	ErrStateUnavailable = errors.New("otp: state unavailable")
	ErrOperationPending = errors.New("otp: delivery operation pending")
)

// AtomicRepository is additive so older repository consumers remain compatible.
type AtomicRepository interface {
	Repository
	CompareAndSwap(context.Context, *Request, *Request) error
}

type DeliveryOperation struct {
	ID             string    `json:"id"`
	NotificationID string    `json:"notification_id"`
	State          string    `json:"state"`
	CodeHash       string    `json:"code_hash"`
	Salt           string    `json:"salt"`
	StartedAt      time.Time `json:"started_at"`
}
type DeliveryAuthorizer interface {
	AuthorizeDelivery(ctx context.Context, scope Scope, requestID, operationID, notificationID string) error
}
