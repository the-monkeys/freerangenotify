package workflow

import "errors"

// BeforeDispatch marks a failure known to precede every execution write/queue
// operation. Unknown storage/enqueue outcomes must not carry this marker.
type BeforeDispatch struct{ Err error }

func (e *BeforeDispatch) Error() string { return e.Err.Error() }
func (e *BeforeDispatch) Unwrap() error { return e.Err }
func IsBeforeDispatch(err error) bool   { var target *BeforeDispatch; return errors.As(err, &target) }
