package notification

import (
	"context"
	"errors"
)

type processingClaimKey struct{}

var ErrProcessingClaimLost = errors.New("notification processing claim lost")

func WithProcessingClaim(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, processingClaimKey{}, token)
}
func ProcessingClaim(ctx context.Context) string {
	token, _ := ctx.Value(processingClaimKey{}).(string)
	return token
}
