package services

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// BizConnectorService manages third-party billing connectors (Layer 2):
// businesses keep billing in Razorpay/Stripe/etc. and FreeRangeNotify
// consumes their webhooks to drive notifications and dunning.
type BizConnectorService struct {
	connectors *bizbillingrepo.Store[bizbilling.Connector]
	notifier   *BizNotifier
	logger     *zap.Logger
}

// NewBizConnectorService creates a new BizConnectorService.
func NewBizConnectorService(stores *bizbillingrepo.Stores, notifier *BizNotifier, logger *zap.Logger) *BizConnectorService {
	return &BizConnectorService{connectors: stores.Connectors, notifier: notifier, logger: logger}
}

// Create registers a connector for an app.
func (s *BizConnectorService) Create(ctx context.Context, appID string, req *bizbilling.CreateConnectorRequest) (*bizbilling.Connector, error) {
	if !bizbilling.ValidConnectorProviders[req.Provider] {
		return nil, errors.BadRequest("provider must be razorpay, stripe, or custom")
	}
	if req.WebhookSecret == "" {
		return nil, errors.BadRequest("webhook_secret is required")
	}

	now := time.Now().UTC()
	connector := &bizbilling.Connector{
		ID:            uuid.New().String(),
		AppID:         appID,
		Provider:      req.Provider,
		WebhookSecret: req.WebhookSecret,
		EventMappings: req.EventMappings,
		Active:        true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.connectors.Create(ctx, connector.ID, connector); err != nil {
		return nil, err
	}
	return connector, nil
}

// Get returns a connector scoped to the app.
func (s *BizConnectorService) Get(ctx context.Context, appID, id string) (*bizbilling.Connector, error) {
	connector, err := s.connectors.Get(ctx, id)
	if err != nil || connector.AppID != appID {
		return nil, errors.NotFound("connector", id)
	}
	return connector, nil
}

// List lists an app's connectors with secrets redacted.
func (s *BizConnectorService) List(ctx context.Context, appID string) ([]*bizbilling.Connector, error) {
	connectors, _, err := s.connectors.Find(ctx, bizbillingrepo.NewQuery(appID).Body("created_at", 50, 0))
	if err != nil {
		return nil, err
	}
	for _, cn := range connectors {
		cn.WebhookSecret = ""
	}
	return connectors, nil
}

// Update updates a connector.
func (s *BizConnectorService) Update(ctx context.Context, appID, id string, req *bizbilling.UpdateConnectorRequest) (*bizbilling.Connector, error) {
	return s.connectors.Update(ctx, id, func(cn *bizbilling.Connector) error {
		if cn.AppID != appID {
			return errors.NotFound("connector", id)
		}
		if req.WebhookSecret != nil {
			cn.WebhookSecret = *req.WebhookSecret
		}
		if req.EventMappings != nil {
			cn.EventMappings = req.EventMappings
		}
		if req.Active != nil {
			cn.Active = *req.Active
		}
		cn.UpdatedAt = time.Now().UTC()
		return nil
	})
}

// Delete removes a connector.
func (s *BizConnectorService) Delete(ctx context.Context, appID, id string) error {
	if _, err := s.Get(ctx, appID, id); err != nil {
		return err
	}
	return s.connectors.Delete(ctx, id)
}

// HandleWebhook verifies and processes an inbound third-party webhook. The
// connector is looked up by ID (no app auth — the URL + HMAC signature are
// the credentials). Returns the normalized event for logging.
func (s *BizConnectorService) HandleWebhook(ctx context.Context, connectorID string, body []byte, signature string) (*bizbilling.ConnectorEvent, error) {
	connector, err := s.connectors.Get(ctx, connectorID)
	if err != nil || !connector.Active {
		return nil, errors.NotFound("connector", connectorID)
	}

	// Webhook secret is mandatory — unsigned connectors are rejected so a
	// leaked connector ID alone cannot inject billing events.
	if connector.WebhookSecret == "" {
		return nil, errors.Unauthorized("connector webhook secret is not configured")
	}
	if !verifyHMACSHA256(body, signature, connector.WebhookSecret) {
		return nil, errors.Unauthorized("invalid webhook signature")
	}

	event, err := parseConnectorEvent(connector.Provider, body)
	if err != nil {
		return nil, errors.BadRequest(fmt.Sprintf("unparseable webhook payload: %v", err))
	}

	s.dispatch(ctx, connector, event)
	return event, nil
}

// dispatch sends the notification matching the normalized event type.
// UserID resolution: connector events carry the business's own user
// reference — apps must use their FreeRangeNotify user ID as the external
// reference (customer notes / metadata) for notifications to route.
func (s *BizConnectorService) dispatch(ctx context.Context, connector *bizbilling.Connector, event *bizbilling.ConnectorEvent) {
	templateByEvent := map[string]string{
		"invoice.paid":           "biz_payment_received",
		"invoice.payment_failed": "biz_payment_failed",
		"invoice.created":        "biz_invoice_issued",
		"subscription.canceled":  "biz_subscription_canceled",
	}
	tmpl, ok := templateByEvent[event.EventType]
	if !ok || event.ExternalUserID == "" {
		s.logger.Info("bizbilling: connector event ignored",
			zap.String("connector_id", connector.ID),
			zap.String("event_type", event.EventType),
			zap.Bool("has_user", event.ExternalUserID != ""))
		return
	}

	s.notifier.Send(ctx, connector.AppID, event.ExternalUserID, tmpl, "", map[string]interface{}{
		"amount":      FormatPaisa(event.AmountPaisa, event.Currency),
		"invoice_ref": event.InvoiceRef,
		"provider":    event.Provider,
	})
}

// parseConnectorEvent normalizes provider-specific webhook payloads.
func parseConnectorEvent(provider string, body []byte) (*bizbilling.ConnectorEvent, error) {
	switch provider {
	case bizbilling.ConnectorProviderRazorpay:
		return parseRazorpayEvent(body)
	case bizbilling.ConnectorProviderStripe:
		return parseStripeEvent(body)
	default:
		return parseCustomEvent(body)
	}
}

// parseRazorpayEvent maps Razorpay webhook events to normalized events.
func parseRazorpayEvent(body []byte) (*bizbilling.ConnectorEvent, error) {
	var payload struct {
		Event   string `json:"event"`
		Payload struct {
			Payment struct {
				Entity struct {
					ID       string                 `json:"id"`
					Amount   int64                  `json:"amount"`
					Currency string                 `json:"currency"`
					Email    string                 `json:"email"`
					Contact  string                 `json:"contact"`
					Notes    map[string]interface{} `json:"notes"`
					OrderID  string                 `json:"order_id"`
				} `json:"entity"`
			} `json:"payment"`
			Subscription struct {
				Entity struct {
					ID    string                 `json:"id"`
					Notes map[string]interface{} `json:"notes"`
				} `json:"entity"`
			} `json:"subscription"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	eventMap := map[string]string{
		"payment.captured":      "invoice.paid",
		"payment.failed":        "invoice.payment_failed",
		"invoice.paid":          "invoice.paid",
		"subscription.cancelled": "subscription.canceled",
	}
	normalized, ok := eventMap[payload.Event]
	if !ok {
		normalized = payload.Event
	}

	p := payload.Payload.Payment.Entity
	event := &bizbilling.ConnectorEvent{
		Provider:    bizbilling.ConnectorProviderRazorpay,
		EventType:   normalized,
		Email:       p.Email,
		Phone:       p.Contact,
		PaymentRef:  p.ID,
		InvoiceRef:  p.OrderID,
		AmountPaisa: p.Amount,
		Currency:    p.Currency,
	}
	event.ExternalUserID = noteString(p.Notes, "frn_user_id")
	if event.ExternalUserID == "" {
		event.ExternalUserID = noteString(payload.Payload.Subscription.Entity.Notes, "frn_user_id")
	}
	return event, nil
}

// parseStripeEvent maps Stripe webhook events to normalized events.
func parseStripeEvent(body []byte) (*bizbilling.ConnectorEvent, error) {
	var payload struct {
		Type string `json:"type"`
		Data struct {
			Object struct {
				ID            string                 `json:"id"`
				AmountPaid    int64                  `json:"amount_paid"`
				AmountDue     int64                  `json:"amount_due"`
				Currency      string                 `json:"currency"`
				CustomerEmail string                 `json:"customer_email"`
				Metadata      map[string]interface{} `json:"metadata"`
			} `json:"object"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	eventMap := map[string]string{
		"invoice.paid":                  "invoice.paid",
		"invoice.payment_failed":        "invoice.payment_failed",
		"invoice.created":               "invoice.created",
		"customer.subscription.deleted": "subscription.canceled",
	}
	normalized, ok := eventMap[payload.Type]
	if !ok {
		normalized = payload.Type
	}

	obj := payload.Data.Object
	amount := obj.AmountPaid
	if amount == 0 {
		amount = obj.AmountDue
	}
	return &bizbilling.ConnectorEvent{
		Provider:       bizbilling.ConnectorProviderStripe,
		EventType:      normalized,
		Email:          obj.CustomerEmail,
		InvoiceRef:     obj.ID,
		AmountPaisa:    amount, // Stripe reports smallest currency unit, same as paisa
		Currency:       obj.Currency,
		ExternalUserID: noteString(obj.Metadata, "frn_user_id"),
	}, nil
}

// parseCustomEvent expects the normalized shape directly.
func parseCustomEvent(body []byte) (*bizbilling.ConnectorEvent, error) {
	var event bizbilling.ConnectorEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return nil, err
	}
	if event.EventType == "" {
		return nil, fmt.Errorf("event_type is required")
	}
	event.Provider = bizbilling.ConnectorProviderCustom
	return &event, nil
}

// noteString extracts a string value from a metadata/notes map.
func noteString(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// verifyHMACSHA256 checks an hex-encoded HMAC-SHA256 signature.
func verifyHMACSHA256(body []byte, signature, secret string) bool {
	if signature == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}
