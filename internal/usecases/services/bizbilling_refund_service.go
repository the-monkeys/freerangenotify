package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
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

type BizRefundService struct {
	lifecycle *BizLifecycleService
	merchants *BizGatewayConnectionService
	factory   bizbilling.GatewayFactory
	effects   *bizbillingrepo.Store[bizbilling.GatewayEffect]
	logger    *zap.Logger
	mu        sync.Mutex
	cursor    string
}

func NewBizRefundService(es *elasticsearch.Client, lifecycle *BizLifecycleService, merchants *BizGatewayConnectionService, factory bizbilling.GatewayFactory, logger *zap.Logger) *BizRefundService {
	return &BizRefundService{lifecycle: lifecycle, merchants: merchants, factory: factory, effects: bizbillingrepo.NewStore[bizbilling.GatewayEffect](es, bizbillingrepo.GatewayEffectIndex, logger, repository.RefreshFalse), logger: logger}
}

func (s *BizLifecycleService) SetRefundService(service *BizRefundService) { s.refunds = service }
func refundRequestHash(paymentID string, request bizbilling.RefundPaymentRequest) string {
	data, _ := json.Marshal(struct {
		PaymentID string                          `json:"payment_id"`
		Request   bizbilling.RefundPaymentRequest `json:"request"`
	}{paymentID, request})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func pendingRefund(effect *bizbilling.GatewayEffect) bizbilling.OperationResult {
	resources := []bizbilling.ResourceRef{{Kind: "gateway_effect", ID: effect.ID}, {Kind: "payment", ID: effect.RefundPaymentID}}
	if effect.RefundIntent != nil {
		resources = append(resources, bizbilling.ResourceRef{Kind: "refund_reservation", ID: effect.RefundIntent.ReservationID})
	}
	return bizbilling.OperationResult{OperationID: effect.ID, State: bizbilling.OperationAccepted, Resources: resources}
}
func refundPending(effect *bizbilling.GatewayEffect) (bizbilling.OperationResult, error) {
	result := pendingRefund(effect)
	return result, &bizbilling.OperationFailure{Result: result, Cause: bizbilling.ErrGatewayPending}
}
func refundRejected(effect *bizbilling.GatewayEffect) (bizbilling.OperationResult, error) {
	result := bizbilling.OperationResult{OperationID: effect.ID, State: bizbilling.OperationRejected, ErrorCode: effect.FailureCode}
	return result, &bizbilling.OperationFailure{Result: result, Cause: apperrors.BadRequest("The refund was rejected. Check its status before making a new request.")}
}

func (s *BizRefundService) Request(ctx context.Context, app, paymentID string, request bizbilling.RefundPaymentRequest, key string) (bizbilling.OperationResult, error) {
	if strings.TrimSpace(key) == "" || len(key) > 200 || request.AmountPaisa < 0 || len(request.Reason) > 1000 {
		return bizbilling.OperationResult{}, apperrors.BadRequest("Provide an idempotency key and a valid refund request")
	}
	if request.Reason == "" {
		request.Reason = "customer refund requested"
	}
	ctx, done, err := BeginBizMutation(ctx, s.lifecycle.gate, app)
	if err != nil {
		return bizbilling.OperationResult{}, err
	}
	defer releaseBizMutation(s.logger, done)
	id := gatewayEffectID("refund", app, key)
	hash := refundRequestHash(paymentID, request)
	effect, err := s.effects.Get(ctx, id)
	if apperrors.IsNotFound(err) {
		payment, readErr := s.lifecycle.GetPayment(ctx, app, paymentID)
		if readErr != nil {
			return bizbilling.OperationResult{}, readErr
		}
		state, readErr := s.lifecycle.gate.State(ctx, app)
		if readErr != nil {
			return bizbilling.OperationResult{}, readErr
		}
		if state.LifecycleVersion != 2 || state.Status != "active" {
			return bizbilling.OperationResult{}, bizbilling.ErrReconciliationRequired
		}
		if payment.GatewayBinding == nil || payment.GatewayPaymentID == "" || !payment.CreditProvenanceKnown {
			return bizbilling.OperationResult{}, bizbilling.ErrLegacyReviewRequired
		}
		if payment.FinancialScope().Validate() != nil {
			return bizbilling.OperationResult{}, bizbilling.ErrReconciliationRequired
		}
		now := time.Now().UTC()
		effect = &bizbilling.GatewayEffect{ID: id, Kind: "refund", Scope: payment.FinancialScope(), Gateway: *payment.GatewayBinding, RefundPaymentID: paymentID, GatewayPaymentID: payment.GatewayPaymentID, RefundRequest: &request, RequestHash: hash, State: "reserving", CreatedAt: now, UpdatedAt: now}
		_, _ = s.effects.CreateIfAbsent(ctx, id, effect)
		effect, err = s.effects.Get(ctx, id)
	}
	if err != nil {
		return bizbilling.OperationResult{}, bizbilling.ErrGatewayPending
	}
	if effect.Kind != "refund" || effect.Scope.AppID != app || effect.RefundPaymentID != paymentID || effect.RequestHash != hash || effect.RefundRequest == nil {
		return bizbilling.OperationResult{}, bizbilling.ErrIdempotencyConflict
	}
	return s.resumeRefund(ctx, effect)
}
func (s *BizRefundService) resumeRefund(ctx context.Context, effect *bizbilling.GatewayEffect) (bizbilling.OperationResult, error) {
	if effect.State == "completed" && effect.Result != nil {
		return *effect.Result, nil
	}
	if effect.State == "failed" || effect.State == "rejected" {
		return refundRejected(effect)
	}
	if effect.RefundIntent == nil {
		prepared, reserveErr := s.reserve(ctx, effect)
		effect = prepared
		if reserveErr != nil {
			return pendingOrRejectedRefund(effect, reserveErr)
		}
	}
	if effect.State == "completed" && effect.Result != nil {
		return *effect.Result, nil
	}
	if effect.State == "failed" || effect.State == "rejected" {
		return refundRejected(effect)
	}
	return s.execute(ctx, effect)
}
func pendingOrRejectedRefund(effect *bizbilling.GatewayEffect, err error) (bizbilling.OperationResult, error) {
	if effect == nil {
		return bizbilling.OperationResult{}, err
	}
	if effect.State == "rejected" || effect.State == "failed" {
		return refundRejected(effect)
	}
	return refundPending(effect)
}

func (s *BizRefundService) reserve(ctx context.Context, effect *bizbilling.GatewayEffect) (*bizbilling.GatewayEffect, error) {
	payment, err := s.lifecycle.GetPayment(ctx, effect.Scope.AppID, effect.RefundPaymentID)
	if err != nil {
		return effect, err
	}
	if payment.FinancialScope() != effect.Scope || payment.GatewayBinding == nil || *payment.GatewayBinding != effect.Gateway || payment.GatewayPaymentID != effect.GatewayPaymentID {
		return effect, bizbilling.ErrReconciliationRequired
	}
	// A completed full refund is an irreversible local fact. The provider may
	// now report "refunded" rather than "captured"; do not turn an impossible
	// new refund into permanently pending recovery work.
	if payment.AmountPaisa > 0 && payment.RefundedPaisa == payment.AmountPaisa {
		updated, err := s.effects.Update(ctx, effect.ID, func(current *bizbilling.GatewayEffect) error {
			if current.State == "reserving" && current.RefundIntent == nil && !current.Attempted {
				current.State = "rejected"
				current.FailureCode = "payment_already_refunded"
				current.UpdatedAt = time.Now().UTC()
			}
			return nil
		})
		if err != nil {
			return effect, bizbilling.ErrGatewayPending
		}
		return updated, nil
	}
	gateway, err := s.factory.For(ctx, effect.Gateway)
	if err != nil {
		return effect, err
	}
	remote, err := gateway.FetchPayment(ctx, effect.GatewayPaymentID)
	if err != nil {
		return effect, err
	}
	if remote.ID != payment.GatewayPaymentID || !sameMerchantIdentity(remote.Gateway, effect.Gateway) || remote.AmountPaisa != payment.AmountPaisa || remote.Currency != effect.Scope.Currency || !remote.Captured || remote.Status != "captured" {
		return effect, bizbilling.ErrInvalidGatewayProof
	}
	known, err := bizbilling.CheckedAdd(payment.RefundedPaisa, payment.RefundReservedPaisa)
	if err != nil || remote.RefundedPaisa > known {
		return effect, bizbilling.ErrReconciliationRequired
	}
	request := effect.RefundRequest
	result, err := s.lifecycle.operations.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandRefundReservation, Scope: effect.Scope, IdempotencyKey: effect.ID + ":reserve", RefundReservation: &bizbilling.RefundReservationRequest{PaymentID: payment.ID, AmountPaisa: request.AmountPaisa, Reason: request.Reason}})
	if err != nil {
		if result.State == bizbilling.OperationRejected {
			updated, writeErr := s.effects.Update(ctx, effect.ID, func(current *bizbilling.GatewayEffect) error {
				current.State = "rejected"
				current.FailureCode = "refund_budget_rejected"
				current.Result = &result
				current.UpdatedAt = time.Now().UTC()
				return nil
			})
			if writeErr == nil {
				effect = updated
			}
		}
		return effect, err
	}
	event, err := s.lifecycle.operations.completedFinancialEvent(ctx, effect.Scope.AppID, result.OperationID)
	if err != nil || event.RefundReservationAfter == nil {
		return effect, bizbilling.ErrGatewayPending
	}
	reservation := event.RefundReservationAfter
	if reservation.Scope != effect.Scope || reservation.PaymentID != payment.ID || reservation.AmountPaisa <= 0 {
		return effect, bizbilling.ErrReconciliationRequired
	}
	sum := sha256.Sum256([]byte(effect.ID))
	intent := bizbilling.RefundIntent{OperationID: effect.ID, ReservationID: reservation.ID, PaymentID: effect.GatewayPaymentID, Scope: effect.Scope, Gateway: effect.Gateway, AmountPaisa: reservation.AmountPaisa, Currency: effect.Scope.Currency, Receipt: "frn_" + hex.EncodeToString(sum[:16])}
	updated, err := s.effects.Update(ctx, effect.ID, func(current *bizbilling.GatewayEffect) error {
		if current.State == "completed" || current.State == "failed" || current.State == "rejected" {
			return nil
		}
		if current.RequestHash != effect.RequestHash {
			return bizbilling.ErrIdempotencyConflict
		}
		if current.RefundIntent != nil && *current.RefundIntent != intent {
			return bizbilling.ErrReconciliationRequired
		}
		current.RefundIntent = &intent
		current.State = "reserved"
		current.UpdatedAt = time.Now().UTC()
		return nil
	})
	if err != nil {
		confirmed, readErr := s.effects.Get(ctx, effect.ID)
		if readErr != nil || confirmed.RefundIntent == nil || *confirmed.RefundIntent != intent {
			return effect, bizbilling.ErrGatewayPending
		}
		updated = confirmed
	}
	return updated, nil
}

func (s *BizRefundService) execute(ctx context.Context, effect *bizbilling.GatewayEffect) (bizbilling.OperationResult, error) {
	if effect.RefundIntent == nil {
		return refundPending(effect)
	}
	if effect.Refund != nil && (effect.Refund.Status == "processed" || effect.Refund.Status == "failed") {
		result, err := s.Confirm(ctx, *effect.Refund)
		if err == nil && effect.Refund.Status == "failed" {
			return refundRejected(effect)
		}
		return result, err
	}
	gateway, err := s.factory.For(ctx, effect.Gateway)
	if err != nil {
		return refundPending(effect)
	}
	create := false
	if !effect.Attempted {
		current, version, readErr := s.effects.GetVersioned(ctx, effect.ID)
		if readErr != nil {
			return refundPending(effect)
		}
		if !current.Attempted {
			current.Attempted = true
			current.UpdatedAt = time.Now().UTC()
			create = s.effects.ReplaceVersioned(ctx, effect.ID, current, version) == nil
		}
	}
	var refund *bizbilling.GatewayRefund
	if create {
		created, createErr := gateway.Refund(ctx, *effect.RefundIntent)
		if createErr == nil {
			refund = &created
		} else if errors.Is(createErr, bizbilling.ErrMerchantAuthentication) {
			_, releaseErr := s.release(ctx, effect, "provider_authentication_rejected")
			if releaseErr != nil {
				return refundPending(effect)
			}
			effect.FailureCode = "provider_authentication_rejected"
			return refundRejected(effect)
		}
	}
	if refund == nil {
		refund, err = gateway.FindRefund(ctx, *effect.RefundIntent)
		if err != nil || refund == nil {
			return refundPending(effect)
		}
	}
	result, err := s.Confirm(ctx, *refund)
	if err == nil && refund.Status == "failed" {
		effect.FailureCode = "provider_refund_failed"
		return refundRejected(effect)
	}
	return result, err
}

// Confirm accepts a server-fetched fact, never raw callback/client JSON.
func (s *BizRefundService) Confirm(ctx context.Context, refund bizbilling.GatewayRefund) (bizbilling.OperationResult, error) {
	effect, err := s.effects.Get(ctx, refund.Intent.OperationID)
	if err != nil {
		return bizbilling.OperationResult{}, bizbilling.ErrGatewayPending
	}
	if effect.Kind != "refund" || effect.RefundIntent == nil || *effect.RefundIntent != refund.Intent || refund.ID == "" {
		return bizbilling.OperationResult{}, bizbilling.ErrInvalidGatewayProof
	}
	if effect.State == "completed" && effect.Result != nil {
		return *effect.Result, nil
	}
	if effect.State == "failed" && effect.Result != nil {
		if refund.Status != "failed" {
			return bizbilling.OperationResult{}, bizbilling.ErrReconciliationRequired
		}
		return *effect.Result, nil
	}
	if effect.Refund != nil && effect.Refund.ID != refund.ID {
		return bizbilling.OperationResult{}, bizbilling.ErrReconciliationRequired
	}
	if refund.Status != "pending" && refund.Status != "processed" && refund.Status != "failed" {
		return bizbilling.OperationResult{}, bizbilling.ErrInvalidGatewayProof
	}
	updated, err := s.effects.Update(ctx, effect.ID, func(current *bizbilling.GatewayEffect) error {
		if current.RefundIntent == nil || *current.RefundIntent != refund.Intent {
			return bizbilling.ErrReconciliationRequired
		}
		if current.Refund != nil && current.Refund.ID != refund.ID {
			return bizbilling.ErrReconciliationRequired
		}
		if current.State == "completed" || current.State == "failed" {
			return nil
		}
		if current.Refund != nil && current.Refund.Status == "processed" && refund.Status != "processed" {
			return nil
		}
		current.Refund = &refund
		current.ProviderID = refund.ID
		current.State = "provider_pending"
		if refund.Status == "processed" {
			current.State = "confirming"
		}
		current.UpdatedAt = time.Now().UTC()
		return nil
	})
	if err != nil {
		confirmed, readErr := s.effects.Get(ctx, effect.ID)
		if readErr != nil || confirmed.Refund == nil || *confirmed.Refund != refund {
			return refundPending(effect)
		}
		updated = confirmed
	}
	effect = updated
	if effect.State == "completed" && effect.Result != nil {
		return *effect.Result, nil
	}
	if effect.State == "failed" {
		return bizbilling.OperationResult{}, bizbilling.ErrReconciliationRequired
	}
	if effect.Refund != nil {
		refund = *effect.Refund
	}
	if refund.Status == "pending" {
		return refundPending(effect)
	}
	if refund.Status == "failed" {
		return s.release(ctx, effect, "provider_refund_failed")
	}
	result, err := s.lifecycle.operations.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandRefundSettlement, Scope: effect.Scope, IdempotencyKey: effect.ID + ":settle", RefundSettlement: &bizbilling.RefundSettlement{ReservationID: refund.Intent.ReservationID, ConfirmationID: refund.ID}})
	if err != nil {
		return refundPending(effect)
	}
	_, err = s.effects.Update(ctx, effect.ID, func(current *bizbilling.GatewayEffect) error {
		if current.Refund == nil || current.Refund.ID != refund.ID {
			return bizbilling.ErrReconciliationRequired
		}
		current.State = "completed"
		current.Result = &result
		current.FailureCode = ""
		current.UpdatedAt = time.Now().UTC()
		return nil
	})
	if err != nil {
		confirmed, readErr := s.effects.Get(ctx, effect.ID)
		if readErr != nil || confirmed.State != "completed" || confirmed.Result == nil || confirmed.Result.OperationID != result.OperationID {
			return result, &bizbilling.OperationFailure{Result: result, Cause: bizbilling.ErrGatewayPending}
		}
	}
	return result, nil
}
func (s *BizRefundService) release(ctx context.Context, effect *bizbilling.GatewayEffect, code string) (bizbilling.OperationResult, error) {
	if effect.RefundIntent == nil {
		return refundPending(effect)
	}
	result, err := s.lifecycle.operations.Apply(ctx, bizbilling.Command{Kind: bizbilling.CommandRefundRelease, Scope: effect.Scope, IdempotencyKey: effect.ID + ":release", RefundRelease: &bizbilling.RefundRelease{ReservationID: effect.RefundIntent.ReservationID, Reason: "provider definitively rejected refund", DefinitiveFailure: true}})
	if err != nil {
		return refundPending(effect)
	}
	_, err = s.effects.Update(ctx, effect.ID, func(current *bizbilling.GatewayEffect) error {
		current.State = "failed"
		current.FailureCode = code
		current.Result = &result
		current.UpdatedAt = time.Now().UTC()
		return nil
	})
	if err != nil {
		confirmed, readErr := s.effects.Get(ctx, effect.ID)
		if readErr != nil || confirmed.State != "failed" || confirmed.Result == nil || confirmed.Result.OperationID != result.OperationID {
			return refundPending(effect)
		}
	}
	return result, nil
}

func (s *BizRefundService) ProcessEvent(ctx context.Context, receipt *bizbilling.GatewayEventReceipt) (bizbilling.OperationResult, error) {
	if receipt.Data.RefundID == "" || receipt.Data.PaymentID == "" {
		return bizbilling.OperationResult{}, apperrors.NotFound("refund", receipt.ID)
	}
	if err := s.effects.Refresh(ctx); err != nil {
		return bizbilling.OperationResult{}, err
	}
	rows, _, err := s.effects.Find(ctx, map[string]any{"size": 1000, "query": map[string]any{"bool": map[string]any{"filter": []any{map[string]any{"term": map[string]any{"kind": "refund"}}, map[string]any{"term": map[string]any{"scope.app_id": receipt.AppID}}, map[string]any{"term": map[string]any{"gateway.connection_id": receipt.Gateway.ConnectionID}}, map[string]any{"term": map[string]any{"gateway_payment_id": receipt.Data.PaymentID}}}}}})
	if err != nil {
		return bizbilling.OperationResult{}, err
	}
	for _, effect := range rows {
		if effect.RefundIntent == nil || !sameMerchantIdentity(effect.Gateway, receipt.Gateway) || effect.Scope.AppID != receipt.AppID {
			continue
		}
		if effect.RefundIntent.AmountPaisa != receipt.Data.AmountPaisa || effect.Scope.Currency != receipt.Data.Currency {
			continue
		}
		// A newly signed revision must prove it can read the original payment
		// before it becomes eligible to recover this account's retired API key.
		if exact, ok := s.factory.(interface {
			ForExact(context.Context, bizbilling.GatewayRef) (bizbilling.BizGateway, error)
		}); ok {
			gateway, lookupErr := exact.ForExact(ctx, receipt.Gateway)
			if lookupErr == nil {
				remote, lookupErr := gateway.FetchPayment(ctx, receipt.Data.PaymentID)
				payment, readErr := s.lifecycle.GetPayment(ctx, effect.Scope.AppID, effect.RefundPaymentID)
				if lookupErr == nil && readErr == nil && remote.ID == payment.GatewayPaymentID && remote.AmountPaisa == payment.AmountPaisa && remote.Currency == payment.Currency && remote.Gateway == receipt.Gateway {
					if proofErr := s.merchants.connections.RecordAccountProof(ctx, &bizbilling.MerchantAccountProof{AppID: receipt.AppID, Gateway: receipt.Gateway, EventDigest: receipt.BodyDigest, PaymentID: remote.ID, VerifiedAt: time.Now().UTC()}); proofErr != nil {
						return bizbilling.OperationResult{}, bizbilling.ErrGatewayPending
					}
				}
			}
		}
		gateway, lookupErr := s.factory.For(ctx, effect.Gateway)
		if lookupErr != nil {
			continue
		}
		refund, lookupErr := gateway.FindRefund(ctx, *effect.RefundIntent)
		if lookupErr != nil {
			return bizbilling.OperationResult{}, bizbilling.ErrGatewayPending
		}
		if refund == nil || refund.ID != receipt.Data.RefundID {
			continue
		}
		return s.Confirm(ctx, *refund)
	}
	return bizbilling.OperationResult{}, apperrors.NotFound("refund", receipt.Data.RefundID)
}
func (s *BizRefundService) Reconcile(ctx context.Context, limit int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit < 1 || limit > 200 {
		limit = 100
	}
	if err := s.effects.Refresh(ctx); err != nil {
		return 0, err
	}
	find := func(after string) ([]*bizbilling.GatewayEffect, error) {
		filters := []any{map[string]any{"term": map[string]any{"kind": "refund"}}, map[string]any{"terms": map[string]any{"state": []string{"reserving", "reserved", "provider_pending", "confirming"}}}}
		if after != "" {
			filters = append(filters, map[string]any{"range": map[string]any{"id": map[string]any{"gt": after}}})
		}
		rows, _, err := s.effects.Find(ctx, map[string]any{"size": limit, "sort": []any{map[string]any{"id": "asc"}}, "query": map[string]any{"bool": map[string]any{"filter": filters}}})
		return rows, err
	}
	rows, err := find(s.cursor)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 && s.cursor != "" {
		s.cursor = ""
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
		s.cursor = effect.ID
		if effect.RefundRequest == nil {
			pending = bizbilling.ErrReconciliationRequired
			continue
		}
		run, done, err := BeginBizMutation(ctx, s.lifecycle.gate, effect.Scope.AppID)
		if err != nil {
			pending = bizbilling.ErrGatewayPending
			continue
		}
		_, err = s.resumeRefund(run, effect)
		releaseBizMutation(s.logger, done)
		if err != nil {
			pending = bizbilling.ErrGatewayPending
		} else {
			count++
		}
	}
	return count, pending
}
func (s *BizRefundService) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				run, cancel := context.WithTimeout(ctx, 25*time.Second)
				_, err := s.Reconcile(run, 100)
				cancel()
				if err != nil && ctx.Err() == nil {
					s.logger.Warn("Billing refund reconciliation pending", zap.String("failure_code", "refund_recovery_pending"))
				}
			}
		}
	}()
}
