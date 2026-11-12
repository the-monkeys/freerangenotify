package services

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

type gatewayRefundEventProcessor interface {
	ProcessEvent(context.Context, *bizbilling.GatewayEventReceipt) (bizbilling.OperationResult, error)
}
type BizGatewayEventService struct {
	inbox     *bizbillingrepo.GatewayEventRepo
	merchants *BizGatewayConnectionService
	checkout  *BizCheckoutService
	refunds   gatewayRefundEventProcessor
	logger    *zap.Logger
	mu        sync.Mutex
	cursor    string
}

func NewBizGatewayEventService(es *elasticsearch.Client, merchants *BizGatewayConnectionService, checkout *BizCheckoutService, logger *zap.Logger) *BizGatewayEventService {
	return &BizGatewayEventService{inbox: bizbillingrepo.NewGatewayEventRepo(es, logger), merchants: merchants, checkout: checkout, logger: logger}
}
func (s *BizGatewayEventService) SetRefundProcessor(processor gatewayRefundEventProcessor) {
	s.refunds = processor
}

func (s *BizGatewayEventService) Accept(ctx context.Context, connection, eventID string, raw []byte, signature string) (bizbilling.GatewayEventReceipt, error) {
	var empty bizbilling.GatewayEventReceipt
	if len(raw) == 0 || len(raw) > 1<<20 || len(eventID) > 200 || connection == "" {
		return empty, bizbilling.ErrInvalidGatewayProof
	}
	actual, err := hex.DecodeString(signature)
	if err != nil || len(actual) != sha256.Size {
		return empty, bizbilling.ErrGatewaySignature
	}
	credentials, err := s.merchants.CallbackCredentials(ctx, connection)
	if err != nil {
		return empty, bizbilling.ErrMerchantUnavailable
	}
	var signer *bizbilling.MerchantCredentials
	for i := len(credentials) - 1; i >= 0; i-- {
		mac := hmac.New(sha256.New, []byte(credentials[i].WebhookSecret))
		mac.Write(raw)
		if hmac.Equal(actual, mac.Sum(nil)) {
			signer = &credentials[i]
			break
		}
	}
	if signer == nil {
		return empty, bizbilling.ErrGatewaySignature
	}
	var payload struct {
		AccountID string `json:"account_id"`
		Event     string `json:"event"`
		Payload   struct {
			Payment struct {
				Entity struct {
					ID       string `json:"id"`
					OrderID  string `json:"order_id"`
					Amount   int64  `json:"amount"`
					Currency string `json:"currency"`
				} `json:"entity"`
			} `json:"payment"`
			Refund struct {
				Entity struct {
					ID        string `json:"id"`
					PaymentID string `json:"payment_id"`
					Amount    int64  `json:"amount"`
					Currency  string `json:"currency"`
				} `json:"entity"`
			} `json:"refund"`
		} `json:"payload"`
	}
	if json.Unmarshal(raw, &payload) != nil || payload.AccountID != signer.Reference.AccountID || payload.Event == "" || len(payload.Event) > 120 {
		return empty, bizbilling.ErrInvalidGatewayProof
	}
	data := bizbilling.GatewayEventData{Event: payload.Event, AccountID: payload.AccountID, PaymentID: payload.Payload.Payment.Entity.ID, OrderID: payload.Payload.Payment.Entity.OrderID, AmountPaisa: payload.Payload.Payment.Entity.Amount, Currency: payload.Payload.Payment.Entity.Currency}
	if strings.HasPrefix(payload.Event, "refund.") {
		refund := payload.Payload.Refund.Entity
		data.RefundID = refund.ID
		data.PaymentID = refund.PaymentID
		data.AmountPaisa = refund.Amount
		data.Currency = refund.Currency
	}
	if len(data.PaymentID) > 200 || len(data.OrderID) > 200 || len(data.RefundID) > 200 || len(data.Currency) > 3 || data.AmountPaisa < 0 {
		return empty, bizbilling.ErrInvalidGatewayProof
	}
	digest := sha256.Sum256(raw)
	digestHex := hex.EncodeToString(digest[:])
	id := gatewayEffectID("event", connection, digestHex)
	if eventID == "" {
		eventID = digestHex
	}
	aliasID := gatewayEffectID("event_alias", connection, eventID)
	collisionID := gatewayEffectID("event_collision", aliasID, digestHex)
	now := time.Now().UTC()
	receipt := &bizbilling.GatewayEventReceipt{SchemaVersion: 1, ID: id, Kind: "receipt", AppID: signer.AppID, Gateway: signer.Reference, BodyDigest: digestHex, Data: data, State: "pending", CreatedAt: now, UpdatedAt: now}
	stored, err := s.inbox.Accept(ctx, receipt, aliasID, collisionID)
	if err != nil {
		return empty, err
	}
	if stored.HeaderConflict {
		s.logger.Warn("Billing gateway event header collision", zap.String("app_id", stored.AppID), zap.String("gateway_event_id", stored.ID))
	}
	return *stored, nil
}

func sameMerchantIdentity(a, b bizbilling.GatewayRef) bool {
	return a.ConnectionID == b.ConnectionID && a.AccountID == b.AccountID && a.Mode == b.Mode
}

func (s *BizGatewayEventService) Process(ctx context.Context, id string) (bizbilling.OperationResult, error) {
	receipt, err := s.inbox.Get(ctx, id)
	if err != nil {
		return bizbilling.OperationResult{}, err
	}
	if receipt.State == "completed" && receipt.Result != nil && receipt.VerifiedRevision == receipt.Gateway.CredentialRevision {
		return *receipt.Result, nil
	}
	if strings.HasPrefix(receipt.Data.Event, "refund.") {
		if s.refunds == nil {
			return s.deferEvent(ctx, receipt, "retry", "refund_processor_unavailable", bizbilling.ErrGatewayPending)
		}
		result, err := s.refunds.ProcessEvent(ctx, receipt)
		if err != nil {
			if apperrors.IsNotFound(err) {
				return s.deferEvent(ctx, receipt, "unmatched", "refund_unmatched", err)
			}
			return s.deferEvent(ctx, receipt, "retry", "refund_pending", err)
		}
		if result.State != bizbilling.OperationCompleted {
			return s.deferEvent(ctx, receipt, "retry", "refund_pending", bizbilling.ErrGatewayPending)
		}
		return s.completeEvent(ctx, receipt, result)
	}
	switch receipt.Data.Event {
	case "payment.captured", "payment.authorized", "payment.failed", "order.paid":
	default:
		_, err = s.inbox.Update(ctx, id, func(current *bizbilling.GatewayEventReceipt) error {
			current.State = "ignored"
			current.FailureCode = "unsupported_event"
			current.UpdatedAt = time.Now().UTC()
			return nil
		})
		return bizbilling.OperationResult{}, err
	}
	if receipt.Data.PaymentID == "" || receipt.Data.OrderID == "" {
		return s.deferEvent(ctx, receipt, "unmatched", "payment_link_missing", bizbilling.ErrReconciliationRequired)
	}
	effect, err := s.checkout.findOrder(ctx, receipt.Gateway.ConnectionID, receipt.Data.OrderID)
	if err != nil {
		// Accepted order creation may have succeeded just before its response was lost.
		_, _ = s.checkout.ReconcileOrders(ctx, 50)
		effect, err = s.checkout.findOrder(ctx, receipt.Gateway.ConnectionID, receipt.Data.OrderID)
		if err != nil {
			return s.deferEvent(ctx, receipt, "unmatched", "order_unmatched", bizbilling.ErrReconciliationRequired)
		}
	}
	if effect.Scope.AppID != receipt.AppID || !sameMerchantIdentity(effect.Gateway, receipt.Gateway) || effect.OrderIntent == nil {
		return s.deferEvent(ctx, receipt, "unmatched", "merchant_mismatch", bizbilling.ErrInvalidGatewayProof)
	}
	// The callback's credential revision proves its own account linkage. A newer
	// verified API revision may be used only for recovery when exact keys retired.
	gateway, err := s.checkout.factory.For(ctx, receipt.Gateway)
	if exact, ok := s.checkout.factory.(interface {
		ForExact(context.Context, bizbilling.GatewayRef) (bizbilling.BizGateway, error)
	}); ok {
		gateway, err = exact.ForExact(ctx, receipt.Gateway)
	}
	var payment bizbilling.CapturedPayment
	exactProof := false
	if err == nil {
		payment, err = gateway.FetchPayment(ctx, receipt.Data.PaymentID)
		exactProof = err == nil && payment.Gateway == receipt.Gateway
	}
	if err != nil {
		gateway, err = s.checkout.factory.For(ctx, effect.Gateway)
		if err == nil {
			payment, err = gateway.FetchPayment(ctx, receipt.Data.PaymentID)
		}
	}
	if err != nil {
		return s.deferEvent(ctx, receipt, "retry", "provider_lookup_pending", bizbilling.ErrGatewayPending)
	}
	if payment.ID != receipt.Data.PaymentID || payment.OrderID != receipt.Data.OrderID || !sameMerchantIdentity(payment.Gateway, effect.Gateway) || payment.AmountPaisa != effect.OrderIntent.AmountPaisa || payment.Currency != effect.Scope.Currency || receipt.Data.AmountPaisa != payment.AmountPaisa || receipt.Data.Currency != payment.Currency {
		return s.deferEvent(ctx, receipt, "quarantined", "payment_facts_mismatch", bizbilling.ErrInvalidGatewayProof)
	}
	if exactProof {
		proof := &bizbilling.MerchantAccountProof{AppID: receipt.AppID, Gateway: receipt.Gateway, EventDigest: receipt.BodyDigest, PaymentID: payment.ID, VerifiedAt: time.Now().UTC()}
		if err = s.merchants.connections.RecordAccountProof(ctx, proof); err != nil {
			return s.deferEvent(ctx, receipt, "retry", "account_proof_pending", bizbilling.ErrGatewayPending)
		}
	}
	if !payment.Captured || payment.Status != "captured" {
		if payment.Status == "failed" && receipt.Data.Event != "payment.captured" && receipt.Data.Event != "order.paid" {
			_, err = s.inbox.Update(ctx, id, func(current *bizbilling.GatewayEventReceipt) error {
				if current.Result == nil {
					current.State = "ignored"
					current.FailureCode = "payment_failed"
				}
				return nil
			})
			return bizbilling.OperationResult{}, err
		}
		return s.deferEvent(ctx, receipt, "pending_capture", "capture_not_confirmed", bizbilling.ErrGatewayPending)
	}
	payment.Gateway = effect.Gateway // Preserve the original order/account revision.
	result, err := s.checkout.settleCapture(ctx, effect, payment)
	if err != nil {
		return s.deferEvent(ctx, receipt, "retry", "settlement_pending", err)
	}
	return s.completeEvent(ctx, receipt, result)
}
func (s *BizGatewayEventService) completeEvent(ctx context.Context, receipt *bizbilling.GatewayEventReceipt, result bizbilling.OperationResult) (bizbilling.OperationResult, error) {
	_, err := s.inbox.Update(ctx, receipt.ID, func(current *bizbilling.GatewayEventReceipt) error {
		current.Result = &result
		current.VerifiedRevision = max(current.VerifiedRevision, receipt.Gateway.CredentialRevision)
		current.FailureCode = ""
		current.UpdatedAt = time.Now().UTC()
		if current.VerifiedRevision == current.Gateway.CredentialRevision {
			current.State = "completed"
		} else {
			current.State = "pending"
		}
		return nil
	})
	if err != nil {
		return result, &bizbilling.OperationFailure{Result: result, Cause: bizbilling.ErrGatewayPending}
	}
	return result, nil
}
func (s *BizGatewayEventService) deferEvent(ctx context.Context, receipt *bizbilling.GatewayEventReceipt, state, code string, cause error) (bizbilling.OperationResult, error) {
	_, err := s.inbox.Update(ctx, receipt.ID, func(current *bizbilling.GatewayEventReceipt) error {
		if current.State == "completed" && current.VerifiedRevision == current.Gateway.CredentialRevision {
			return nil
		}
		current.State = state
		current.FailureCode = code
		current.UpdatedAt = time.Now().UTC()
		return nil
	})
	if err != nil {
		return bizbilling.OperationResult{}, bizbilling.ErrGatewayPending
	}
	return bizbilling.OperationResult{}, cause
}
func (s *BizGatewayEventService) Reconcile(ctx context.Context, limit int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit < 1 || limit > 200 {
		limit = 100
	}
	_, orderErr := s.checkout.ReconcileOrders(ctx, limit)
	rows, err := s.inbox.Pending(ctx, s.cursor, limit)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 && s.cursor != "" {
		s.cursor = ""
		rows, err = s.inbox.Pending(ctx, "", limit)
		if err != nil {
			return 0, err
		}
	}
	count := 0
	var pending error
	if orderErr != nil {
		pending = bizbilling.ErrGatewayPending
	}
	for _, receipt := range rows {
		if ctx.Err() != nil {
			return count, ctx.Err()
		}
		s.cursor = receipt.ID
		if _, err := s.Process(ctx, receipt.ID); err != nil {
			pending = bizbilling.ErrGatewayPending
		} else {
			count++
		}
	}
	return count, pending
}
func (s *BizGatewayEventService) Start(ctx context.Context) {
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
					s.logger.Warn("Billing gateway reconciliation pending", zap.String("failure_code", "gateway_recovery_pending"))
				}
			}
		}
	}()
}

func (s *BizGatewayEventService) GetForApp(ctx context.Context, app, id string) (*bizbilling.GatewayEventReceipt, error) {
	receipt, err := s.inbox.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if app == "" || receipt.AppID != app {
		return nil, apperrors.NotFound("gateway event", id)
	}
	return receipt, nil
}
func (s *BizGatewayEventService) ListForApp(ctx context.Context, app, state, cursor string, limit int) ([]*bizbilling.GatewayEventReceipt, error) {
	if app == "" || limit < 1 || limit > 200 || len(cursor) > 200 {
		return nil, apperrors.BadRequest("Provide a valid billing event page (limit 1-200)")
	}
	if state != "" {
		switch state {
		case "pending", "pending_capture", "unmatched", "quarantined", "retry", "completed", "ignored":
		default:
			return nil, apperrors.BadRequest("Choose a valid billing event state")
		}
	}
	rows, err := s.inbox.ListForApp(ctx, app, state, cursor, limit)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.AppID != app || row.Kind != "receipt" || row.SchemaVersion != 1 {
			return nil, bizbilling.ErrReconciliationRequired
		}
	}
	return rows, nil
}
