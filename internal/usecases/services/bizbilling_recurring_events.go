package services

import (
	"context"
	"strings"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
)

func (s *BizRecurringService) ProcessEvent(ctx context.Context, receipt *bizbilling.GatewayEventReceipt) (bizbilling.OperationResult, error) {
	subscriptionID, err := s.callbackSubscription(ctx, receipt)
	if err != nil {
		return bizbilling.OperationResult{}, err
	}
	if subscriptionID == "" {
		return bizbilling.OperationResult{}, bizbilling.ErrGatewayPending
	}
	if err := s.effects.Refresh(ctx); err != nil {
		return bizbilling.OperationResult{}, err
	}
	rows, _, err := s.effects.Find(ctx, map[string]any{"size": 2, "query": map[string]any{"bool": map[string]any{"filter": []any{
		map[string]any{"term": map[string]any{"kind": "subscription_authorize"}},
		map[string]any{"term": map[string]any{"scope.app_id": receipt.AppID}},
		map[string]any{"term": map[string]any{"gateway.connection_id": receipt.Gateway.ConnectionID}},
		map[string]any{"term": map[string]any{"provider_id": subscriptionID}},
	}}}})
	if err != nil {
		return bizbilling.OperationResult{}, err
	}
	if len(rows) == 0 {
		return bizbilling.OperationResult{}, apperrors.NotFound("subscription authorization", receipt.ID)
	}
	if len(rows) != 1 {
		return bizbilling.OperationResult{}, bizbilling.ErrReconciliationRequired
	}
	effect := rows[0]
	if effect.RecurringIntent == nil || !sameMerchantIdentity(effect.Gateway, receipt.Gateway) {
		return bizbilling.OperationResult{}, bizbilling.ErrInvalidGatewayProof
	}
	// Establish the signed receipt revision's account link before attempting a
	// retired API key. The retained mandate still belongs to its original binding.
	var proved bizbilling.GatewaySubscription
	var proofErr error
	if proof, ok := s.factory.(interface {
		FetchSubscriptionForProof(context.Context, bizbilling.GatewayRef, bizbilling.GatewayRef, string) (bizbilling.GatewaySubscription, error)
	}); ok {
		proved, proofErr = proof.FetchSubscriptionForProof(ctx, effect.Gateway, receipt.Gateway, subscriptionID)
	} else if exact, ok := s.factory.(interface {
		ForExact(context.Context, bizbilling.GatewayRef) (bizbilling.BizGateway, error)
	}); ok {
		var base bizbilling.BizGateway
		base, proofErr = exact.ForExact(ctx, receipt.Gateway)
		if proofErr == nil {
			if recurring, ok := base.(bizbilling.RecurringGateway); ok {
				proved, proofErr = recurring.FetchSubscription(ctx, subscriptionID)
			} else {
				proofErr = bizbilling.ErrInvalidGatewayProof
			}
		}
	} else if receipt.Gateway == effect.Gateway {
		var gateway bizbilling.RecurringGateway
		gateway, proofErr = s.gateway(ctx, receipt.Gateway)
		if proofErr == nil {
			proved, proofErr = gateway.FetchSubscription(ctx, subscriptionID)
		}
	} else {
		proofErr = bizbilling.ErrGatewayPending
	}
	if proofErr == nil {
		if proved.ID != subscriptionID || !recurringSubscriptionMatches(proved, *effect.RecurringIntent) {
			return bizbilling.OperationResult{}, bizbilling.ErrInvalidGatewayProof
		}
		if err = s.merchants.connections.RecordAccountProof(ctx, &bizbilling.MerchantAccountProof{AppID: receipt.AppID, Gateway: receipt.Gateway, EventDigest: receipt.BodyDigest, SubscriptionID: proved.ID, VerifiedAt: time.Now().UTC()}); err != nil {
			return bizbilling.OperationResult{}, err
		}
	}
	gateway, err := s.gateway(ctx, effect.Gateway)
	if err != nil {
		return bizbilling.OperationResult{}, err
	}
	remote, err := gateway.FetchSubscription(ctx, subscriptionID)
	if err != nil {
		return bizbilling.OperationResult{}, err
	}
	if !recurringSubscriptionMatches(remote, *effect.RecurringIntent) || remote.ID != subscriptionID {
		return bizbilling.OperationResult{}, bizbilling.ErrInvalidGatewayProof
	}
	sub, err := s.lifecycle.GetSubscription(ctx, receipt.AppID, effect.SubscriptionID)
	if err != nil {
		return bizbilling.OperationResult{}, err
	}
	state, err := s.verifiedSubscriptionState(ctx, sub, effect, remote)
	if err != nil {
		return bizbilling.OperationResult{}, err
	}
	if state == "awaiting_account_verification" {
		return bizbilling.OperationResult{}, bizbilling.ErrGatewayPending
	}
	if receipt.Data.InvoiceID != "" || receipt.Data.Event == "subscription.charged" || strings.HasPrefix(receipt.Data.Event, "payment.") || strings.HasPrefix(receipt.Data.Event, "invoice.") {
		return s.reconcileProviderCallback(ctx, sub, effect, gateway, receipt)
	}
	updated, err := s.effects.Get(ctx, effect.ID)
	if err != nil {
		return bizbilling.OperationResult{}, err
	}
	if updated.Result == nil {
		return bizbilling.OperationResult{}, bizbilling.ErrGatewayPending
	}
	return *updated.Result, nil
}
