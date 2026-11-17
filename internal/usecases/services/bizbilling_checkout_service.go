package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

type BizCheckoutService struct {
	lifecycle       *BizLifecycleService
	merchants       *BizGatewayConnectionService
	factory         bizbilling.GatewayFactory
	effects         *bizbillingrepo.Store[bizbilling.GatewayEffect]
	keys            *bizbillingrepo.Store[bizbilling.CheckoutKey]
	enabled         bool
	logger          *zap.Logger
	reconcileMu     sync.Mutex
	reconcileCursor string
}

func NewBizCheckoutService(es *elasticsearch.Client, lifecycle *BizLifecycleService, merchants *BizGatewayConnectionService, factory bizbilling.GatewayFactory, logger *zap.Logger) *BizCheckoutService {
	return &BizCheckoutService{lifecycle: lifecycle, merchants: merchants, factory: factory, effects: bizbillingrepo.NewStore[bizbilling.GatewayEffect](es, bizbillingrepo.GatewayEffectIndex, logger, repository.RefreshFalse), keys: bizbillingrepo.NewStore[bizbilling.CheckoutKey](es, bizbillingrepo.GatewayEffectIndex, logger, repository.RefreshFalse), logger: logger}
}
func (s *BizCheckoutService) SetNewCollectionEnabled(enabled bool) { s.enabled = enabled }
func gatewayEffectID(kind string, parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(append([]string{kind}, parts...), "\x00")))
	return "bizg_" + kind + "_" + hex.EncodeToString(sum[:])
}
func checkoutEffectID(app, invoice string) string { return gatewayEffectID("order", app, invoice) }

func (s *BizCheckoutService) Start(ctx context.Context, app, user, invoice string, mode bizbilling.GatewayMode, key string) (bizbilling.CheckoutDescriptor, error) {
	var descriptor bizbilling.CheckoutDescriptor
	if !mode.Valid() || strings.TrimSpace(key) == "" || len(key) > 200 {
		return descriptor, apperrors.BadRequest("Choose a payment mode and provide an idempotency key")
	}
	ctx, done, err := BeginBizMutation(ctx, s.lifecycle.gate, app)
	if err != nil {
		return descriptor, err
	}
	defer releaseBizMutation(s.logger, done)
	inv, err := s.lifecycle.GetInvoice(ctx, app, invoice)
	if err != nil {
		return descriptor, err
	}
	if inv.UserID != user {
		return descriptor, apperrors.NotFound("invoice", invoice)
	}
	invoiceMode := inv.GatewayMode
	if invoiceMode == "" {
		invoiceMode = bizbilling.GatewayLive
	}
	if mode != invoiceMode {
		return descriptor, bizbilling.ErrInvalidGatewayProof
	}
	state, err := s.lifecycle.gate.State(ctx, app)
	if err != nil {
		return descriptor, err
	}
	if state.LifecycleVersion != 2 || state.Status != "active" {
		return descriptor, bizbilling.ErrReconciliationRequired
	}

	if !bizbilling.PayableStatuses[inv.Status] || inv.AmountDuePaisa <= 0 {
		return descriptor, apperrors.BadRequest("This invoice has no payable balance")
	}
	if inv.CollectionOwner != "" {
		return descriptor, bizbilling.ErrAutomaticCollectionInProgress
	}
	aliasID := gatewayEffectID("checkout_key", app, key)
	alias, aliasErr := s.keys.Get(ctx, aliasID)
	var effect *bizbilling.GatewayEffect
	if aliasErr == nil {
		if alias.AppID != app || alias.UserID != user || alias.InvoiceID != invoice || alias.Mode != mode {
			return descriptor, bizbilling.ErrIdempotencyConflict
		}
		id := alias.EffectID
		if id == "" {
			id = checkoutEffectID(app, invoice)
		}
		effect, err = s.effects.Get(ctx, id)
		if err != nil {
			return descriptor, bizbilling.ErrGatewayPending
		}
	} else {
		if !apperrors.IsNotFound(aliasErr) {
			return descriptor, aliasErr
		}
		effect, err = s.currentAttempt(ctx, inv, mode)
		if err != nil {
			return descriptor, err
		}
		alias = &bizbilling.CheckoutKey{AppID: app, UserID: user, InvoiceID: invoice, Mode: mode, EffectID: effect.ID}
		created, createErr := s.keys.CreateIfAbsent(ctx, aliasID, alias)
		if createErr != nil || !created {
			stored, readErr := s.keys.Get(ctx, aliasID)
			if readErr != nil {
				return pendingCheckout(effect), bizbilling.ErrGatewayPending
			}
			if stored.AppID != app || stored.UserID != user || stored.InvoiceID != invoice || stored.Mode != mode {
				return descriptor, bizbilling.ErrIdempotencyConflict
			}
			id := stored.EffectID
			if id == "" {
				id = checkoutEffectID(app, invoice)
			}
			effect, err = s.effects.Get(ctx, id)
			if err != nil {
				return descriptor, bizbilling.ErrGatewayPending
			}
		}
	}
	if effect.Kind != "order" || effect.Scope != inv.FinancialScope() || effect.OrderIntent == nil || effect.OrderIntent.InvoiceID != invoice || effect.Gateway.Mode != mode {
		return descriptor, bizbilling.ErrIdempotencyConflict
	}
	if effect.State == "settled" || effect.State == "superseded" || effect.NextEffectID != "" || effect.OrderIntent.AmountPaisa != inv.AmountDuePaisa {
		return pendingCheckout(effect), bizbilling.ErrConflict
	}
	order, err := s.ensureOrder(ctx, effect)
	if err != nil {
		return pendingCheckout(effect), err
	}
	credentials, err := s.merchants.ResolveAPI(ctx, effect.Gateway)
	if err != nil {
		return pendingCheckout(effect), err
	}
	if credentials.AppID != app {
		return descriptor, bizbilling.ErrReconciliationRequired
	}
	return bizbilling.CheckoutDescriptor{OperationID: effect.ID, OrderID: order.ID, PublicKeyID: credentials.KeyID, AmountPaisa: order.Intent.AmountPaisa, Currency: order.Intent.Currency, InvoiceNumber: inv.InvoiceNumber, MerchantDisplayName: s.lifecycle.companyName(ctx, app), State: "ready"}, nil
}
func pendingCheckout(effect *bizbilling.GatewayEffect) bizbilling.CheckoutDescriptor {
	return bizbilling.CheckoutDescriptor{OperationID: effect.ID, State: "pending", AmountPaisa: effect.OrderIntent.AmountPaisa, Currency: effect.Scope.Currency}
}
func (s *BizCheckoutService) ensureOrder(ctx context.Context, effect *bizbilling.GatewayEffect) (*bizbilling.GatewayOrder, error) {
	current, _, err := s.effects.GetVersioned(ctx, effect.ID)
	if err != nil {
		return nil, bizbilling.ErrGatewayPending
	}
	if current.State == "superseded" || current.NextEffectID != "" {
		return nil, bizbilling.ErrConflict
	}
	effect = current
	if effect.Order != nil {
		if effect.Order.ID == "" || effect.Order.Intent != *effect.OrderIntent {
			return nil, bizbilling.ErrReconciliationRequired
		}
		return effect.Order, nil
	}
	gateway, err := s.factory.For(ctx, effect.Gateway)
	if err != nil {
		return nil, bizbilling.ErrGatewayPending
	}
	// Only a confirmed CAS from attempted=false may issue the first POST.
	// A lost marker acknowledgment intentionally falls back to read-only lookup.
	create := false
	if !effect.Attempted {
		current, version, err := s.effects.GetVersioned(ctx, effect.ID)
		if err != nil {
			return nil, bizbilling.ErrGatewayPending
		}
		if !current.Attempted {
			if current.State == "superseded" || current.NextEffectID != "" {
				return nil, bizbilling.ErrConflict
			}
			current.Attempted = true
			current.UpdatedAt = time.Now().UTC()
			err = s.effects.ReplaceVersioned(ctx, effect.ID, current, version)
			create = err == nil
		}
	}
	var order *bizbilling.GatewayOrder
	if create {
		created, createErr := gateway.CreateOrder(ctx, *effect.OrderIntent)
		if createErr == nil {
			order = &created
		}
	}
	if order == nil {
		order, err = gateway.FindOrder(ctx, *effect.OrderIntent)
		if err != nil || order == nil {
			return nil, bizbilling.ErrGatewayPending
		}
	}
	if order.ID == "" || order.Intent != *effect.OrderIntent {
		return nil, bizbilling.ErrInvalidGatewayProof
	}
	_, err = s.effects.Update(ctx, effect.ID, func(current *bizbilling.GatewayEffect) error {
		if current.OrderIntent == nil || *current.OrderIntent != order.Intent {
			return bizbilling.ErrReconciliationRequired
		}
		if current.Order != nil && *current.Order != *order {
			return bizbilling.ErrReconciliationRequired
		}
		current.Order = order
		current.ProviderID = order.ID
		if current.State != "settled" && current.State != "superseded" {
			current.State = "ready"
		}
		current.UpdatedAt = time.Now().UTC()
		return nil
	})
	if err != nil {
		confirmed, readErr := s.effects.Get(ctx, effect.ID)
		if readErr != nil || confirmed.Order == nil || *confirmed.Order != *order {
			return nil, bizbilling.ErrGatewayPending
		}
	}
	return order, nil
}

func (s *BizCheckoutService) Verify(ctx context.Context, app, user, invoice string, proof bizbilling.CheckoutProof) (bizbilling.OperationResult, error) {
	inv, err := s.lifecycle.GetInvoice(ctx, app, invoice)
	if err != nil {
		return bizbilling.OperationResult{}, err
	}
	if inv.UserID != user {
		return bizbilling.OperationResult{}, apperrors.NotFound("invoice", invoice)
	}

	effect, err := s.findInvoiceOrder(ctx, app, user, proof.OrderID)
	if err != nil || effect.Order == nil || effect.Order.ID != proof.OrderID || effect.Scope != inv.FinancialScope() || effect.OrderIntent == nil || effect.OrderIntent.InvoiceID != invoice {
		return bizbilling.OperationResult{}, bizbilling.ErrInvalidGatewayProof
	}
	gateway, err := s.factory.For(ctx, effect.Gateway)
	if err != nil {
		return bizbilling.OperationResult{}, bizbilling.ErrGatewayPending
	}
	if err = gateway.VerifyCheckout(effect.Order.ID, proof.PaymentID, proof.Signature); err != nil {
		refs, listErr := s.merchants.VerifiedCheckoutRevisions(ctx, effect.Gateway)
		if listErr != nil {
			return bizbilling.OperationResult{}, bizbilling.ErrMerchantUnavailable
		}
		valid := false
		var unavailable error
		if exact, ok := s.factory.(interface {
			ForExact(context.Context, bizbilling.GatewayRef) (bizbilling.BizGateway, error)
		}); ok {
			for _, ref := range refs {
				candidate, resolveErr := exact.ForExact(ctx, ref)
				if resolveErr != nil {
					unavailable = resolveErr
					continue
				}
				if candidate.VerifyCheckout(effect.Order.ID, proof.PaymentID, proof.Signature) == nil {
					valid = true
					break
				}
			}
		}
		if !valid {
			if unavailable != nil {
				return bizbilling.OperationResult{}, bizbilling.ErrMerchantUnavailable
			}
			return bizbilling.OperationResult{}, bizbilling.ErrInvalidGatewayProof
		}
	}
	payment, err := gateway.FetchPayment(ctx, proof.PaymentID)
	if err != nil {
		return bizbilling.OperationResult{}, bizbilling.ErrGatewayPending
	}
	if payment.ID != proof.PaymentID {
		return bizbilling.OperationResult{}, bizbilling.ErrInvalidGatewayProof
	}
	return s.settleCapture(ctx, effect, payment)
}

func (s *BizCheckoutService) IsUpgraded(ctx context.Context, app string) (bool, error) {
	state, err := s.lifecycle.gate.State(ctx, app)
	if err != nil {
		return false, err
	}
	if state.LifecycleVersion == 1 && state.Status == "legacy" {
		return false, nil
	}
	if state.LifecycleVersion == 2 && state.Status == "active" {
		return true, nil
	}
	return false, bizbilling.ErrActiveLegacyWriter
}
func (s *BizCheckoutService) settleCapture(ctx context.Context, effect *bizbilling.GatewayEffect, payment bizbilling.CapturedPayment) (bizbilling.OperationResult, error) {
	if effect.Order == nil || effect.OrderIntent == nil || payment.ID == "" || !payment.Captured || payment.Status != "captured" || payment.OrderID != effect.Order.ID || payment.Gateway != effect.Gateway || payment.AmountPaisa != effect.OrderIntent.AmountPaisa || payment.Currency != effect.Scope.Currency {
		return bizbilling.OperationResult{}, bizbilling.ErrInvalidGatewayProof
	}
	// Provider payment identity, independent of browser keys and callback headers.
	key := gatewayEffectID("capture", effect.Scope.AppID, effect.Gateway.ConnectionID, effect.Gateway.AccountID, string(effect.Gateway.Mode), payment.ID)
	command := bizbilling.Command{Kind: bizbilling.CommandGatewayCapture, Scope: effect.Scope, IdempotencyKey: key, ManualPayment: &bizbilling.ManualPayment{InvoiceID: effect.OrderIntent.InvoiceID, AmountPaisa: payment.AmountPaisa, Method: "gateway", GatewayPaymentID: payment.ID, VerifiedCapture: true, GatewayBinding: &effect.Gateway}}
	result, err := s.lifecycle.operations.Apply(ctx, command)
	if err != nil {
		return result, err
	}
	_, err = s.effects.Update(ctx, effect.ID, func(current *bizbilling.GatewayEffect) error {
		if current.Order == nil || current.Order.ID != payment.OrderID || current.Scope != effect.Scope {
			return bizbilling.ErrReconciliationRequired
		}
		current.State = "settled"
		current.UpdatedAt = time.Now().UTC()
		return nil
	})
	if err != nil {
		confirmed, readErr := s.effects.Get(ctx, effect.ID)
		if readErr != nil || confirmed.State != "settled" {
			return result, &bizbilling.OperationFailure{Result: result, Cause: bizbilling.ErrGatewayPending}
		}
	}
	return result, nil
}

func (s *BizCheckoutService) GetEffect(ctx context.Context, app, id string) (*bizbilling.GatewayEffect, error) {
	effect, err := s.effects.Get(ctx, id)
	if err != nil || effect.Scope.AppID != app {
		return nil, apperrors.NotFound("gateway operation", id)
	}
	return effect, nil
}

// Used by reconciliation, never by untrusted gateway metadata alone.
func (s *BizCheckoutService) findOrder(ctx context.Context, connectionID, orderID string) (*bizbilling.GatewayEffect, error) {
	if err := s.effects.Refresh(ctx); err != nil {
		return nil, err
	}
	rows, _, err := s.effects.Find(ctx, map[string]interface{}{"size": 2, "query": map[string]interface{}{"bool": map[string]interface{}{"filter": []interface{}{map[string]interface{}{"term": map[string]interface{}{"kind": "order"}}, map[string]interface{}{"term": map[string]interface{}{"gateway.connection_id": connectionID}}, map[string]interface{}{"term": map[string]interface{}{"provider_id": orderID}}}}}})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, apperrors.NotFound("gateway order", orderID)
	}
	if len(rows) != 1 {
		return nil, bizbilling.ErrReconciliationRequired
	}
	return rows[0], nil
}

// Walk the retained invoice chain. A changed balance can supersede an obsolete
// order only after a positive read-only provider check; unknown money blocks it.
func (s *BizCheckoutService) currentAttempt(ctx context.Context, inv *bizbilling.BizInvoice, mode bizbilling.GatewayMode) (*bizbilling.GatewayEffect, error) {
	id := checkoutEffectID(inv.AppID, inv.ID)
	for depth := 0; depth < 1024; depth++ {
		effect, err := s.effects.Get(ctx, id)
		if apperrors.IsNotFound(err) {
			return s.createOrderIntent(ctx, inv, mode, id)
		}
		if err != nil {
			return nil, err
		}
		if effect.Kind != "order" || effect.Scope != inv.FinancialScope() || effect.OrderIntent == nil || effect.OrderIntent.InvoiceID != inv.ID || effect.Gateway.Mode != mode {
			return nil, bizbilling.ErrReconciliationRequired
		}
		if effect.NextEffectID != "" {
			id = effect.NextEffectID
			continue
		}
		if effect.State != "settled" && effect.State != "superseded" {
			if effect.OrderIntent.AmountPaisa == inv.AmountDuePaisa {
				return effect, nil
			}
			order, err := s.ensureOrder(ctx, effect)
			if err != nil {
				return nil, err
			}
			gateway, err := s.factory.For(ctx, effect.Gateway)
			if err != nil {
				return nil, bizbilling.ErrGatewayPending
			}
			reader, ok := gateway.(interface {
				OrderIsUnpaid(context.Context, bizbilling.GatewayOrder) (bool, error)
			})
			if !ok {
				return nil, bizbilling.ErrGatewayPending
			}
			unpaid, err := reader.OrderIsUnpaid(ctx, *order)
			if err != nil || !unpaid {
				return nil, bizbilling.ErrGatewayPending
			}
			effect, err = s.effects.Update(ctx, effect.ID, func(current *bizbilling.GatewayEffect) error {
				if current.State == "settled" || current.State == "superseded" {
					return nil
				}
				if current.Order == nil || *current.Order != *order || current.NextEffectID != "" {
					return bizbilling.ErrConflict
				}
				current.State, current.UpdatedAt = "superseded", time.Now().UTC()
				return nil
			})
			if err != nil {
				return nil, bizbilling.ErrGatewayPending
			}
		}
		nextID := gatewayEffectID("order_next", effect.ID, strconv.FormatInt(inv.LastSequence, 10))
		_, err = s.createOrderIntent(ctx, inv, mode, nextID)
		if err != nil {
			return nil, err
		}
		updated, err := s.effects.Update(ctx, effect.ID, func(current *bizbilling.GatewayEffect) error {
			if current.State != "settled" && current.State != "superseded" {
				return bizbilling.ErrConflict
			}
			if current.NextEffectID == "" {
				current.NextEffectID = nextID
				current.UpdatedAt = time.Now().UTC()
			}
			return nil
		})
		if err != nil {
			confirmed, readErr := s.effects.Get(ctx, effect.ID)
			if readErr != nil || confirmed.NextEffectID == "" {
				return nil, bizbilling.ErrGatewayPending
			}
			updated = confirmed
		}
		id = updated.NextEffectID
	}
	return nil, bizbilling.ErrReconciliationRequired
}
func (s *BizCheckoutService) createOrderIntent(ctx context.Context, inv *bizbilling.BizInvoice, mode bizbilling.GatewayMode, id string) (*bizbilling.GatewayEffect, error) {
	// Existing children recover even when creation of new intents is disabled.
	existing, err := s.effects.Get(ctx, id)
	if err == nil {
		return existing, nil
	}
	if !apperrors.IsNotFound(err) {
		return nil, err
	}
	if !s.enabled {
		return nil, apperrors.Conflict("New online collection is disabled for this installation")
	}
	snapshot, err := s.merchants.Snapshot(ctx, inv.AppID, mode)
	if err != nil {
		return nil, err
	}
	if !snapshot.Configured || !snapshot.APIAuthenticated {
		return nil, bizbilling.ErrMerchantNotReady
	}
	ref := bizbilling.GatewayRef{ConnectionID: snapshot.ConnectionID, AccountID: snapshot.AccountID, Mode: mode, CredentialRevision: snapshot.Revision}
	sum := sha256.Sum256([]byte(id))
	now := time.Now().UTC()
	intent := bizbilling.OrderIntent{OperationID: id, InvoiceID: inv.ID, Scope: inv.FinancialScope(), Gateway: ref, AmountPaisa: inv.AmountDuePaisa, Currency: inv.Currency, Receipt: "frn_" + hex.EncodeToString(sum[:16])}
	effect := &bizbilling.GatewayEffect{ID: id, Kind: "order", Scope: intent.Scope, Gateway: ref, OrderIntent: &intent, State: "pending", CreatedAt: now, UpdatedAt: now}
	_, createErr := s.effects.CreateIfAbsent(ctx, id, effect)
	confirmed, readErr := s.effects.Get(ctx, id)
	if readErr != nil {
		return nil, bizbilling.ErrGatewayPending
	}
	if createErr != nil && confirmed.ID != id {
		return nil, bizbilling.ErrGatewayPending
	}
	return confirmed, nil
}
func (s *BizCheckoutService) findInvoiceOrder(ctx context.Context, app, user, orderID string) (*bizbilling.GatewayEffect, error) {
	if orderID == "" {
		return nil, bizbilling.ErrInvalidGatewayProof
	}
	if err := s.effects.Refresh(ctx); err != nil {
		return nil, err
	}
	rows, _, err := s.effects.Find(ctx, map[string]interface{}{"size": 2, "query": map[string]interface{}{"bool": map[string]interface{}{"filter": []interface{}{map[string]interface{}{"term": map[string]interface{}{"kind": "order"}}, map[string]interface{}{"term": map[string]interface{}{"scope.app_id": app}}, map[string]interface{}{"term": map[string]interface{}{"scope.user_id": user}}, map[string]interface{}{"term": map[string]interface{}{"provider_id": orderID}}}}}})
	if err != nil {
		return nil, err
	}
	if len(rows) != 1 {
		return nil, bizbilling.ErrInvalidGatewayProof
	}
	return rows[0], nil
}

// Recovery uses only accepted intents and their pinned credentials. It neither
// selects today's merchant slot nor manufactures a replacement order.
func (s *BizCheckoutService) ReconcileOrders(ctx context.Context, limit int) (int, error) {
	s.reconcileMu.Lock()
	defer s.reconcileMu.Unlock()
	if limit < 1 || limit > 200 {
		limit = 100
	}
	if err := s.effects.Refresh(ctx); err != nil {
		return 0, err
	}
	find := func(after string) ([]*bizbilling.GatewayEffect, error) {
		filters := []any{map[string]any{"term": map[string]any{"kind": "order"}}, map[string]any{"term": map[string]any{"state": "pending"}}}
		if after != "" {
			filters = append(filters, map[string]any{"range": map[string]any{"id": map[string]any{"gt": after}}})
		}
		rows, _, err := s.effects.Find(ctx, map[string]any{"size": limit, "sort": []any{map[string]any{"id": "asc"}}, "query": map[string]any{"bool": map[string]any{"filter": filters}}})
		return rows, err
	}
	rows, err := find(s.reconcileCursor)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 && s.reconcileCursor != "" {
		s.reconcileCursor = ""
		rows, err = find("")
		if err != nil {
			return 0, err
		}
	}
	count := 0
	var pending error
	for _, effect := range rows {
		if ctx.Err() != nil {
			return count, ctx.Err()
		}
		s.reconcileCursor = effect.ID
		if effect.OrderIntent == nil || effect.Scope.Validate() != nil || effect.OrderIntent.Scope != effect.Scope || effect.OrderIntent.Gateway != effect.Gateway {
			pending = bizbilling.ErrReconciliationRequired
			continue
		}
		run, done, err := BeginBizMutation(ctx, s.lifecycle.gate, effect.Scope.AppID)
		if err != nil {
			pending = bizbilling.ErrGatewayPending
			continue
		}
		_, err = s.ensureOrder(run, effect)
		releaseBizMutation(s.logger, done)
		if err != nil {
			pending = bizbilling.ErrGatewayPending
		} else {
			count++
		}
	}
	return count, pending
}
