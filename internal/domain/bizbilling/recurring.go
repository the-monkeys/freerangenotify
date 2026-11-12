package bizbilling

import (
	"context"
	"time"
)

type CollectionMode string

const (
	CollectionInvoice             CollectionMode = "invoice"
	CollectionAutomatic           CollectionMode = "automatic"
	CommandSubscriptionCollection CommandKind    = "subscription_collection"
)

func (mode CollectionMode) Valid() bool {
	return mode == CollectionInvoice || mode == CollectionAutomatic
}
func (s BizSubscription) EffectiveCollectionMode() CollectionMode {
	if s.CollectionMode == "" {
		return CollectionInvoice
	}
	return s.CollectionMode
}
func (s BizSubscription) FinancialScope() Scope {
	scope := Scope{AppID: s.AppID, UserID: s.UserID, Currency: s.Currency}
	if s.GatewayMode == GatewayTest {
		scope.Mode = GatewayTest
	}
	return scope
}
func (s BizSubscription) FinancialSequence() int64 { return s.LastSequence }
func (s BizSubscription) FinancialID() string      { return s.ID }
func (s BizSubscription) EffectiveGatewayMode() GatewayMode {
	if s.GatewayMode == "" {
		return GatewayLive
	}
	return s.GatewayMode
}

// RecurringGateway is optional: existing payment adapters remain valid and
// unsupported adapters continue to serve manual invoices.
type RecurringGateway interface {
	BizGateway
	CheckRecurring(context.Context) error
	CreateRecurringPlan(context.Context, RecurringIntent) (GatewayRecurringPlan, error)
	FindRecurringPlan(context.Context, RecurringIntent) (*GatewayRecurringPlan, error)
	CreateSubscription(context.Context, RecurringIntent) (GatewaySubscription, error)
	FindSubscription(context.Context, RecurringIntent) (*GatewaySubscription, error)
	FetchSubscription(context.Context, string) (GatewaySubscription, error)
	ChangeSubscription(context.Context, string, SubscriptionAction) (GatewaySubscription, error)
}

type SubscriptionAction string

const (
	SubscriptionCancel SubscriptionAction = "cancel"
	SubscriptionPause  SubscriptionAction = "pause"
	SubscriptionResume SubscriptionAction = "resume"
)

type RecurringIntent struct {
	OperationID    string                    `json:"operation_id"`
	SubscriptionID string                    `json:"subscription_id"`
	Scope          Scope                     `json:"scope"`
	Gateway        GatewayRef                `json:"gateway"`
	Name           string                    `json:"name"`
	PlanID         string                    `json:"plan_id,omitempty"`
	Period         string                    `json:"period"`
	Interval       int                       `json:"interval"`
	TotalCount     int                       `json:"total_count"`
	AmountPaisa    int64                     `json:"amount_paisa"`
	Currency       string                    `json:"currency"`
	StartAt        time.Time                 `json:"start_at"`
	ExpireBy       time.Time                 `json:"expire_by"`
	Price          SubscriptionPriceSnapshot `json:"price"`
}

type SubscriptionPriceSnapshot struct {
	PlanID       string       `json:"plan_id"`
	PlanName     string       `json:"plan_name"`
	BillingCycle BillingCycle `json:"billing_cycle"`
	CycleDays    int          `json:"cycle_days,omitempty"`
	InvoicePreview
}

type GatewayRecurringPlan struct {
	ID          string `json:"id"`
	OperationID string `json:"operation_id"`
	AmountPaisa int64  `json:"amount_paisa"`
	Currency    string `json:"currency"`
	Period      string `json:"period"`
	Interval    int    `json:"interval"`
}

// Hosted bearer URLs and customer payment credentials are never persisted.
type GatewaySubscription struct {
	ID           string          `json:"id"`
	PlanID       string          `json:"plan_id"`
	Intent       RecurringIntent `json:"intent"`
	Status       string          `json:"status"`
	CurrentStart time.Time       `json:"current_start,omitempty"`
	CurrentEnd   time.Time       `json:"current_end,omitempty"`
}

type AuthorizationDescriptor struct {
	OperationID    string    `json:"operation_id"`
	SubscriptionID string    `json:"gateway_subscription_id,omitempty"`
	PublicKeyID    string    `json:"key_id,omitempty"`
	State          string    `json:"state"`
	AmountPaisa    int64     `json:"amount_paisa"`
	Currency       string    `json:"currency"`
	TotalCount     int       `json:"total_count"`
	StartAt        time.Time `json:"start_at"`
	ExpireBy       time.Time `json:"expire_by"`
}

// Internal command built from authorized requests or verified provider reads.
// Public handlers never accept arbitrary provider-state patches.
type SubscriptionCollection struct {
	AutomaticCycles    *int                       `json:"automatic_cycles,omitempty"`
	SubscriptionID     string                     `json:"subscription_id"`
	Action             string                     `json:"action"`
	Mode               CollectionMode             `json:"mode"`
	State              string                     `json:"state,omitempty"`
	EffectID           string                     `json:"effect_id,omitempty"`
	Gateway            *GatewayRef                `json:"gateway,omitempty"`
	RemoteID           string                     `json:"remote_id,omitempty"`
	Price              *SubscriptionPriceSnapshot `json:"price,omitempty"`
	ExpectedGeneration int64                      `json:"expected_generation"`
}

type ChangeCollectionRequest struct {
	CollectionMode  CollectionMode `json:"collection_mode"`
	AutomaticCycles *int           `json:"automatic_cycles,omitempty"`
}

type AuthorizeSubscriptionRequest struct {
	Mode GatewayMode `json:"mode"`
}

type SubscriptionAuthorizationProof struct {
	SubscriptionID string `json:"razorpay_subscription_id"`
	PaymentID      string `json:"razorpay_payment_id"`
	Signature      string `json:"razorpay_signature"`
}

type RecurringCapability struct {
	AppID      string     `json:"app_id"`
	Gateway    GatewayRef `json:"gateway"`
	VerifiedAt time.Time  `json:"verified_at"`
}
