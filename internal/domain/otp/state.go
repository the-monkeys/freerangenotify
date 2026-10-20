package otp

import (
	"context"
	"errors"
)

var (
	ErrConflict         = errors.New("otp: state changed")
	ErrStateUnavailable = errors.New("otp: state unavailable")
)

// AtomicRepository is additive so older repository consumers remain compatible.
type AtomicRepository interface {
	Repository
	CompareAndSwap(context.Context, *Request, *Request) error
}
