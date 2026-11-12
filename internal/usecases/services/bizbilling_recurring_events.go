package services

import (
	"context"
	"strings"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
)

func (s *BizRecurringService) ProcessEvent(ctx context.Context, receipt *bizbilling.GatewayEventReceipt) (bizbilling.OperationResult, error) {
	if !strings.HasPrefix(receipt.Data.Event, "subscription.") || receipt.Data.SubscriptionID == "" {
		return bizbilling.OperationResult{}, bizbilling.ErrGatewayPending
	}
	if err := s.effects.Refresh(ctx); err != nil {
		return bizbilling.OperationResult{}, err
	}
	rows, _, err := s.effects.Find(ctx, map[string]any{"size": 2, "query": map[string]any{"bool": map[string]any{"filter": []any{
		map[string]any{"term": map[string]any{"kind": "subscription_authorize"}},
		map[string]any{"term": map[string]any{"scope.app_id": receipt.AppID}},
		map[string]any{"term": map[string]any{"gateway.connection_id": receipt.Gateway.ConnectionID}},
		map[string]any{"term": map[string]any{"provider_id": receipt.Data.SubscriptionID}},
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
	gateway, err := s.gateway(ctx, effect.Gateway)
	if err != nil {
		return bizbilling.OperationResult{}, err
	}
	remote, err := gateway.FetchSubscription(ctx, receipt.Data.SubscriptionID)
	if err != nil {
		return bizbilling.OperationResult{}, err
	}
	if !recurringSubscriptionMatches(remote, *effect.RecurringIntent) || remote.ID != receipt.Data.SubscriptionID {
		return bizbilling.OperationResult{}, bizbilling.ErrInvalidGatewayProof
	}
	// Only an exact API revision can establish a new revision's account proof.
	// A verified successor may recover old work, without attesting retired keys.
	proofGateway := gateway
	if exact, ok := s.factory.(interface {
		ForExact(context.Context, bizbilling.GatewayRef) (bizbilling.BizGateway, error)
	}); ok {
		base, exactErr := exact.ForExact(ctx, receipt.Gateway)
		if exactErr == nil {
			proofGateway, _ = base.(bizbilling.RecurringGateway)
		} else {
			proofGateway = nil
		}
	}
	if proofGateway != nil && receipt.Gateway == effect.Gateway {
		exact, exactErr := proofGateway.FetchSubscription(ctx, remote.ID)
		if exactErr == nil && recurringSubscriptionMatches(exact, *effect.RecurringIntent) {
			if err = s.merchants.connections.RecordAccountProof(ctx, &bizbilling.MerchantAccountProof{AppID: receipt.AppID, Gateway: receipt.Gateway, EventDigest: receipt.BodyDigest, SubscriptionID: exact.ID, VerifiedAt: time.Now().UTC()}); err != nil {
				return bizbilling.OperationResult{}, err
			}
		}
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
	updated, err := s.effects.Get(ctx, effect.ID)
	if err != nil {
		return bizbilling.OperationResult{}, err
	}
	if updated.Result == nil {
		return bizbilling.OperationResult{}, bizbilling.ErrGatewayPending
	}
	return *updated.Result, nil
}
