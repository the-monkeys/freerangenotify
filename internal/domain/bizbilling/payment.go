package bizbilling

import (
	"context"
	"time"
)

type PaymentStatus string

const (
	PaymentStatusPending  PaymentStatus = "pending"
	PaymentStatusSuccess  PaymentStatus = "success"
	PaymentStatusFailed   PaymentStatus = "failed"
	PaymentStatusRefunded PaymentStatus = "refunded"
)

// BizPayment represents a payment record for an invoice.
type BizPayment struct {
	ID               string                 `json:"id"`
	AppID            string                 `json:"app_id"`
	UserID           string                 `json:"user_id"`
	InvoiceIDs       []string               `json:"invoice_ids"`
	AmountPaisa      int64                  `json:"amount_paisa"`
	Currency         string                 `json:"currency"`
	Method           string                 `json:"method"` // "card", "bank_transfer", "cash"
	Gateway          string                 `json:"gateway,omitempty"`
	GatewayPaymentID string                 `json:"gateway_payment_id,omitempty"`
	Status           PaymentStatus          `json:"status"`
	RefundedPaisa    int64                  `json:"refunded_paisa,omitempty"`
	TDSPaisa         int64                  `json:"tds_paisa,omitempty"`
	Notes            string                 `json:"notes,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
}

// RefundPaymentRequest initiates a full or partial refund.
// AmountPaisa = 0 means full refund of the remaining refundable amount.
type RefundPaymentRequest struct {
	AmountPaisa int64  `json:"amount_paisa,omitempty" validate:"min=0"`
	Reason      string `json:"reason,omitempty"`
}

// SubscriptionFilter defines criteria for listing subscriptions.
type SubscriptionFilter struct {
	AppID     string
	UserID    string
	PlanID    string
	Status    SubscriptionStatus
	Limit     int
	Offset    int
}

// InvoiceFilter defines criteria for listing invoices.
type InvoiceFilter struct {
	AppID          string
	UserID         string
	SubscriptionID string
	Status         InvoiceStatus
	Limit          int
	Offset         int
}

// PaymentFilter defines criteria for listing payments.
type PaymentFilter struct {
	AppID     string
	UserID    string
	InvoiceID string
	Status    PaymentStatus
	Limit     int
	Offset    int
}

// BizLifecycleRepository handles CRUD for Subscriptions, Invoices, and Payments
// to reduce code bloat instead of having 3 separate repositories.
type BizLifecycleRepository interface {
	// Subscriptions
	CreateSubscription(ctx context.Context, sub *BizSubscription) error
	GetSubscription(ctx context.Context, id string) (*BizSubscription, error)
	UpdateSubscription(ctx context.Context, id string, updateFn func(*BizSubscription) error) error
	ListSubscriptions(ctx context.Context, filter SubscriptionFilter) ([]*BizSubscription, int, error)
	
	// Invoices
	CreateInvoice(ctx context.Context, inv *BizInvoice) error
	GetInvoice(ctx context.Context, id string) (*BizInvoice, error)
	UpdateInvoice(ctx context.Context, id string, updateFn func(*BizInvoice) error) error
	ListInvoices(ctx context.Context, filter InvoiceFilter) ([]*BizInvoice, int, error)

	// Payments
	RecordPayment(ctx context.Context, payment *BizPayment) error
	GetPayment(ctx context.Context, id string) (*BizPayment, error)
	ListPayments(ctx context.Context, filter PaymentFilter) ([]*BizPayment, int, error)
}
