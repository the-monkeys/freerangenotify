package channelconnection

import "context"

type environmentContextKey struct{}

func WithEnvironment(ctx context.Context, id string) context.Context {
	if id == "" {
		id = "default"
	}
	return context.WithValue(ctx, environmentContextKey{}, id)
}
func EnvironmentID(ctx context.Context) string {
	id, _ := ctx.Value(environmentContextKey{}).(string)
	if id == "" {
		return "default"
	}
	return id
}
