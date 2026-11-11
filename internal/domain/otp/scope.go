package otp

import "context"

// Scope is attached by the authenticated HTTP boundary, never by request JSON.
type Scope struct{ AppID, EnvironmentID string }
type scopeKey struct{}

func WithScope(ctx context.Context, scope Scope) context.Context {
	if scope.EnvironmentID == "" {
		scope.EnvironmentID = "default"
	}
	return context.WithValue(ctx, scopeKey{}, scope)
}
func ScopeFromContext(ctx context.Context) (Scope, bool) {
	scope, ok := ctx.Value(scopeKey{}).(Scope)
	return scope, ok
}
