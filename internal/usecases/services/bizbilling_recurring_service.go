package services

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

type BizRecurringService struct {
	mu        sync.Mutex
	cursor    string
	es        *elasticsearch.Client
	lifecycle *BizLifecycleService
	merchants *BizGatewayConnectionService
	factory   bizbilling.GatewayFactory
	effects   *bizbillingrepo.Store[bizbilling.GatewayEffect]
	logger    *zap.Logger
	enabled   bool
}

func NewBizRecurringService(es *elasticsearch.Client, lifecycle *BizLifecycleService, merchants *BizGatewayConnectionService, factory bizbilling.GatewayFactory, logger *zap.Logger) *BizRecurringService {
	return &BizRecurringService{es: es, lifecycle: lifecycle, merchants: merchants, factory: factory, effects: bizbillingrepo.NewStore[bizbilling.GatewayEffect](es, bizbillingrepo.GatewayEffectIndex, logger, repository.RefreshFalse), logger: logger}
}
func (s *BizRecurringService) SetNewCollectionEnabled(enabled bool) { s.enabled = enabled }

func recurringKeyValid(key string) bool { return strings.TrimSpace(key) != "" && len(key) <= 200 }

func (s *BizRecurringService) applyCollection(ctx context.Context, sub *bizbilling.BizSubscription, input bizbilling.SubscriptionCollection, key string) (bizbilling.OperationResult, error) {
	if s.lifecycle.operations == nil {
		return bizbilling.OperationResult{}, bizbilling.ErrReconciliationRequired
	}
	input.SubscriptionID = sub.ID
	return s.lifecycle.operations.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandSubscriptionCollection, Scope: sub.FinancialScope(), IdempotencyKey: key, ActorID: bizOperationActor(ctx), SubscriptionCollection: &input})
}

func (s *BizRecurringService) gateway(ctx context.Context, ref bizbilling.GatewayRef) (bizbilling.RecurringGateway, error) {
	base, err := s.factory.For(ctx, ref)
	if err != nil {
		return nil, err
	}
	gateway, ok := base.(bizbilling.RecurringGateway)
	if !ok {
		return nil, apperrors.BadRequest("This payment provider does not support automatic subscriptions")
	}
	return gateway, nil
}

func (s *BizRecurringService) Authorize(ctx context.Context, app, user, subscription string, mode bizbilling.GatewayMode, key string) (bizbilling.AuthorizationDescriptor, error) {
	var descriptor bizbilling.AuthorizationDescriptor
	if !recurringKeyValid(key) || !mode.Valid() {
		return descriptor, apperrors.BadRequest("Choose a payment mode and provide an idempotency key")
	}
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
	if sub.EffectiveGatewayMode() != mode {
		return descriptor, bizbilling.ErrModeMismatch
	}
	effect, err := s.authorizationEffect(ctx, sub, mode, key)
	if err != nil {
		return descriptor, err
	}
	return s.resumeAuthorization(ctx, sub, effect)
}

func (s *BizRecurringService) resumeAuthorization(ctx context.Context, sub *bizbilling.BizSubscription, effect *bizbilling.GatewayEffect) (bizbilling.AuthorizationDescriptor, error) {
	intent := effect.RecurringIntent
	if intent == nil || effect.Scope != sub.FinancialScope() || intent.SubscriptionID != sub.ID {
		return bizbilling.AuthorizationDescriptor{}, bizbilling.ErrReconciliationRequired
	}
	descriptor := bizbilling.AuthorizationDescriptor{OperationID: effect.ID, State: "pending", AmountPaisa: intent.AmountPaisa, Currency: intent.Currency, TotalCount: intent.TotalCount, StartAt: intent.StartAt, ExpireBy: intent.ExpireBy}
	if effect.StopRequested {
		descriptor.State = "cancellation_pending"
		if effect.State == "stopped" {
			descriptor.State = "cancelled"
			return descriptor, nil
		}
		return descriptor, bizbilling.ErrGatewayPending
	}
	if sub.AuthorizationGeneration == effect.SubscriptionGeneration && sub.EffectiveCollectionMode() == bizbilling.CollectionAutomatic {
		_, err := s.applyCollection(ctx, sub, bizbilling.SubscriptionCollection{Action: "bind", Mode: bizbilling.CollectionAutomatic, EffectID: effect.ID, Gateway: &effect.Gateway, Price: &intent.Price, ExpectedGeneration: effect.SubscriptionGeneration}, effect.ID+":bind")
		if err != nil {
			return descriptor, err
		}
	}
	gateway, err := s.gateway(ctx, effect.Gateway)
	if err != nil {
		return descriptor, err
	}
	remote, err := s.ensureSubscription(ctx, effect, gateway)
	if err != nil {
		return descriptor, err
	}
	descriptor.SubscriptionID = remote.ID
	credentials, err := s.merchants.ResolveAPI(ctx, effect.Gateway)
	if err != nil {
		return descriptor, err
	}
	if credentials.AppID != sub.AppID {
		return descriptor, bizbilling.ErrInvalidGatewayProof
	}
	descriptor.PublicKeyID = credentials.KeyID
	state, err := s.verifiedSubscriptionState(ctx, sub, effect, *remote)
	if err != nil {
		return descriptor, err
	}
	descriptor.State = state
	return descriptor, nil
}

func (s *BizRecurringService) verifiedSubscriptionState(ctx context.Context, sub *bizbilling.BizSubscription, effect *bizbilling.GatewayEffect, remote bizbilling.GatewaySubscription) (string, error) {
	fresh, freshErr := s.effects.Get(ctx, effect.ID)
	if freshErr != nil {
		return "pending", freshErr
	}
	if fresh.StopRequested && !automaticStopped(remote.Status) {
		return "cancellation_pending", bizbilling.ErrGatewayPending
	}
	state := remote.Status
	switch state {
	case "created":
		state = "awaiting_authorization"
	case "authenticated":
		state = "authorized"
	case "active", "pending", "halted", "paused", "completed", "expired", "cancelled":
	default:
		return "pending", bizbilling.ErrInvalidGatewayProof
	}
	if state == "authorized" || state == "active" {
		proof, err := s.merchants.connections.AccountProof(ctx, effect.Gateway)
		if apperrors.IsNotFound(err) {
			refs, successorErr := s.merchants.VerifiedCheckoutRevisions(ctx, effect.Gateway)
			if successorErr != nil {
				return "pending", successorErr
			}
			if len(refs) == 0 {
				state = "awaiting_account_verification"
			}
		} else if err != nil {
			return "pending", err
		} else if proof.AppID != sub.AppID {
			return "pending", bizbilling.ErrInvalidGatewayProof
		}
	}
	current, err := s.lifecycle.GetSubscription(ctx, sub.AppID, sub.ID)
	if err != nil {
		return state, err
	}
	// Replaying an old authorization can show its outcome, never overwrite a
	// newer generation or turn invoice collection back on automatically.
	if current.AuthorizationEffectID != effect.ID || current.AuthorizationGeneration != effect.SubscriptionGeneration || current.EffectiveCollectionMode() != bizbilling.CollectionAutomatic {
		return state, nil
	}
	action := "confirm"
	mode := bizbilling.CollectionAutomatic
	if state == "cancelled" {
		action, mode = "cancel", bizbilling.CollectionInvoice
	}
	result, err := s.applyCollection(ctx, current, bizbilling.SubscriptionCollection{Action: action, Mode: mode, State: state, EffectID: effect.ID, Gateway: &effect.Gateway, RemoteID: remote.ID, ExpectedGeneration: effect.SubscriptionGeneration}, effect.ID+":state:"+state)
	if err != nil {
		return state, err
	}
	_, err = s.effects.Update(ctx, effect.ID, func(row *bizbilling.GatewayEffect) error {
		row.Result = &result
		row.State = "provider_created"
		if state == "authorized" || state == "active" {
			row.State = state
		}
		if automaticStopped(remote.Status) {
			row.State = "settled"
		}
		row.UpdatedAt = time.Now().UTC()
		return nil
	})
	return state, err
}

func (s *BizRecurringService) ChangeCollection(ctx context.Context, app, subscription string, mode bizbilling.CollectionMode, key string) (bizbilling.OperationResult, error) {
	return s.ConfigureCollection(ctx, app, subscription, bizbilling.ChangeCollectionRequest{CollectionMode: mode}, key)
}

func (s *BizRecurringService) ConfigureCollection(ctx context.Context, app, subscription string, request bizbilling.ChangeCollectionRequest, key string) (bizbilling.OperationResult, error) {
	return s.configureCollection(ctx, app, subscription, request, key, true)
}
func (s *BizRecurringService) configureCollection(ctx context.Context, app, subscription string, request bizbilling.ChangeCollectionRequest, key string, startInvoice bool) (bizbilling.OperationResult, error) {
	mode := request.CollectionMode
	if !recurringKeyValid(key) || !mode.Valid() {
		return bizbilling.OperationResult{}, apperrors.BadRequest("Choose invoice or automatic collection and provide an idempotency key")
	}
	ctx, done, err := BeginBizMutation(ctx, s.lifecycle.gate, app)
	if err != nil {
		return bizbilling.OperationResult{}, err
	}
	defer releaseBizMutation(s.logger, done)
	sub, err := s.lifecycle.GetSubscription(ctx, app, subscription)
	if err != nil {
		return bizbilling.OperationResult{}, err
	}
	if sub.NativeTransitionID != "" {
		return bizbilling.OperationResult{}, bizbilling.ErrAutomaticCollectionInProgress
	}
	result, err := s.configureCollectionChoice(ctx, sub, request, key)
	if err != nil {
		return result, err
	}
	if startInvoice && mode == bizbilling.CollectionInvoice {
		return s.completeInvoiceCollectionChoice(ctx, app, subscription, key, result)
	}
	return result, nil
}

func (s *BizRecurringService) authorizationEffect(ctx context.Context, sub *bizbilling.BizSubscription, mode bizbilling.GatewayMode, key string) (*bizbilling.GatewayEffect, error) {
	aliasID := gatewayEffectID("recurring_key", sub.AppID, key)
	hash := gatewayEffectID("authorize_request", sub.AppID, sub.UserID, sub.ID, string(mode))
	alias, err := s.effects.Get(ctx, aliasID)
	if err == nil {
		if alias.Kind != "recurring_key" || alias.RequestHash != hash || alias.Scope != sub.FinancialScope() || alias.NextEffectID == "" {
			return nil, bizbilling.ErrIdempotencyConflict
		}
		return s.effects.Get(ctx, alias.NextEffectID)
	}
	if !apperrors.IsNotFound(err) {
		return nil, err
	}
	if sub.EffectiveCollectionMode() != bizbilling.CollectionAutomatic {
		return nil, apperrors.BadRequest("Choose automatic collection before requesting customer authorization")
	}
	if !s.enabled {
		return nil, apperrors.Conflict("New online collection is disabled for this installation")
	}
	id := gatewayEffectID("subscription_authorize", sub.AppID, sub.ID, string(mode), strconv.FormatInt(sub.AuthorizationGeneration, 10))
	effect, err := s.effects.Get(ctx, id)
	if apperrors.IsNotFound(err) {
		intent, err := s.recurringIntent(ctx, sub, mode, id)
		if err != nil {
			return nil, err
		}
		now := time.Now().UTC()
		effect = &bizbilling.GatewayEffect{ID: id, Kind: "subscription_authorize", Scope: sub.FinancialScope(), Gateway: intent.Gateway, RecurringIntent: &intent, SubscriptionID: sub.ID, SubscriptionGeneration: sub.AuthorizationGeneration, State: "pending", CreatedAt: now, UpdatedAt: now}
		_, createErr := s.effects.CreateIfAbsent(ctx, id, effect)
		stored, readErr := s.effects.Get(ctx, id)
		if readErr != nil {
			return nil, bizbilling.ErrGatewayPending
		}
		if createErr != nil && stored.ID != id {
			return nil, bizbilling.ErrGatewayPending
		}
		effect = stored
	} else if err != nil {
		return nil, err
	}
	if effect.Kind != "subscription_authorize" || effect.Scope != sub.FinancialScope() || effect.SubscriptionID != sub.ID || effect.RecurringIntent == nil || effect.SubscriptionGeneration != sub.AuthorizationGeneration {
		return nil, bizbilling.ErrReconciliationRequired
	}
	alias = &bizbilling.GatewayEffect{ID: aliasID, Kind: "recurring_key", Scope: sub.FinancialScope(), RequestHash: hash, NextEffectID: effect.ID}
	_, _ = s.effects.CreateIfAbsent(ctx, aliasID, alias)
	confirmed, err := s.effects.Get(ctx, aliasID)
	if err != nil {
		return nil, bizbilling.ErrGatewayPending
	}
	if confirmed.RequestHash != hash || confirmed.NextEffectID != effect.ID {
		return nil, bizbilling.ErrIdempotencyConflict
	}
	return effect, nil
}
