package services

import "context"

type bizOperationKeyContext struct{}
type bizOperationActorContext struct{}

// WithBizOperationKey carries an optional client retry key across compatible
// facades. An absent key deliberately represents a new action on each request.
func WithBizOperationKey(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, bizOperationKeyContext{}, key)
}
func WithBizOperationActor(ctx context.Context, actor string) context.Context {
	return context.WithValue(ctx, bizOperationActorContext{}, actor)
}
func bizOperationKey(ctx context.Context) string {
	key, _ := ctx.Value(bizOperationKeyContext{}).(string)
	return key
}
func bizOperationActor(ctx context.Context) string {
	actor, _ := ctx.Value(bizOperationActorContext{}).(string)
	return actor
}
