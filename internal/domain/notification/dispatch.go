package notification

import "context"

type dispatchIdentityKey struct{}

func WithDispatchIdentity(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, dispatchIdentityKey{}, id)
}
func DispatchIdentityFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(dispatchIdentityKey{}).(string)
	return id, ok && id != ""
}

// DispatchError distinguishes uncertain queue acknowledgement from admission.
type DispatchError struct {
	NotificationID string
	MayBeEnqueued  bool
	Cause          error
}

func (e *DispatchError) Error() string { return "notification dispatch unavailable" }
func (e *DispatchError) Unwrap() error { return e.Cause }
