package services

import (
	"context"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
)

func (s *BizRecurringService) VerifyAuthorization(ctx context.Context, app, user, subscription string, proof bizbilling.SubscriptionAuthorizationProof) (bizbilling.AuthorizationDescriptor, error) {
	var descriptor bizbilling.AuthorizationDescriptor
	ctx, done, err := BeginBizMutation(ctx, s.lifecycle.gate, app)
	if err != nil {
		return descriptor, err
	}
	defer releaseBizMutation(s.logger, done)
	sub, err := s.lifecycle.GetSubscription(ctx, app, subscription)
	if err != nil {
		return descriptor, err
	}
	if sub.UserID != user {
		return descriptor, apperrors.NotFound("subscription", subscription)
	}
	if sub.GatewaySubscriptionID == "" || sub.GatewayBinding == nil || proof.SubscriptionID != sub.GatewaySubscriptionID {
		return descriptor, bizbilling.ErrInvalidGatewayProof
	}
	effect, err := s.effects.Get(ctx, sub.AuthorizationEffectID)
	if err != nil {
		return descriptor, err
	}
	if effect.Scope != sub.FinancialScope() || effect.Gateway != *sub.GatewayBinding || effect.RecurringSubscription == nil || effect.RecurringSubscription.ID != proof.SubscriptionID {
		return descriptor, bizbilling.ErrInvalidGatewayProof
	}
	base, err := s.factory.For(ctx, effect.Gateway)
	if exact, ok := s.factory.(interface {
		ForExact(context.Context, bizbilling.GatewayRef) (bizbilling.BizGateway, error)
	}); ok {
		base, err = exact.ForExact(ctx, effect.Gateway)
	}
	if err != nil {
		return descriptor, err
	}
	verifier, ok := base.(interface {
		VerifySubscriptionAuthorization(string, string, string) error
	})
	if !ok {
		return descriptor, bizbilling.ErrMerchantUnavailable
	}
	if err = verifier.VerifySubscriptionAuthorization(proof.PaymentID, sub.GatewaySubscriptionID, proof.Signature); err != nil {
		return descriptor, err
	}
	return s.resumeAuthorization(ctx, sub, effect)
}
