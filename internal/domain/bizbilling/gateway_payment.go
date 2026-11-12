package bizbilling

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalidGatewayProof = errors.New("payment could not be verified")
	ErrGatewayPending      = errors.New("payment provider outcome is pending reconciliation")
)

type GatewayFactory interface {
	For(context.Context, GatewayRef) (BizGateway, error)
}
type BizGateway interface {
	Refund(context.Context, RefundIntent) (GatewayRefund, error)
	FindRefund(context.Context, RefundIntent) (*GatewayRefund, error)
	CreateOrder(context.Context, OrderIntent) (GatewayOrder, error)
	FindOrder(context.Context, OrderIntent) (*GatewayOrder, error)
	FetchPayment(context.Context, string) (CapturedPayment, error)
	VerifyCheckout(orderID, paymentID, signature string) error
}

type OrderIntent struct {
	OperationID string     `json:"operation_id"`
	InvoiceID   string     `json:"invoice_id"`
	Scope       Scope      `json:"scope"`
	Gateway     GatewayRef `json:"gateway"`
	AmountPaisa int64      `json:"amount_paisa"`
	Currency    string     `json:"currency"`
	Receipt     string     `json:"receipt"`
}
type GatewayOrder struct {
	ID     string      `json:"id"`
	Intent OrderIntent `json:"intent"`
}
type CapturedPayment struct {
	RefundedPaisa int64      `json:"refunded_paisa,omitempty"`
	ID            string     `json:"id"`
	OrderID       string     `json:"order_id"`
	Gateway       GatewayRef `json:"gateway"`
	AmountPaisa   int64      `json:"amount_paisa"`
	Currency      string     `json:"currency"`
	Captured      bool       `json:"captured"`
	Status        string     `json:"status"`
}
type CheckoutProof struct {
	OrderID   string `json:"razorpay_order_id"`
	PaymentID string `json:"razorpay_payment_id"`
	Signature string `json:"razorpay_signature"`
}
type CheckoutDescriptor struct {
	OperationID         string `json:"operation_id"`
	OrderID             string `json:"gateway_order_id,omitempty"`
	PublicKeyID         string `json:"key_id,omitempty"`
	AmountPaisa         int64  `json:"amount_due_paisa"`
	Currency            string `json:"currency"`
	InvoiceNumber       string `json:"invoice_number"`
	MerchantDisplayName string `json:"merchant_display_name"`
	State               string `json:"state"`
}

// External intents and attempt markers survive timeouts. They do not themselves
// represent a payment; only a verified financial journal operation can do that.
type GatewayEffect struct {
	RefundPaymentID  string                `json:"refund_payment_id,omitempty"`
	GatewayPaymentID string                `json:"gateway_payment_id,omitempty"`
	RefundRequest    *RefundPaymentRequest `json:"refund_request,omitempty"`
	RequestHash      string                `json:"request_hash,omitempty"`
	RefundIntent     *RefundIntent         `json:"refund_intent,omitempty"`
	Refund           *GatewayRefund        `json:"refund,omitempty"`
	Result           *OperationResult      `json:"result,omitempty"`
	FailureCode      string                `json:"failure_code,omitempty"`
	NextEffectID     string                `json:"next_effect_id,omitempty"`
	ID               string                `json:"id"`
	Kind             string                `json:"kind"`
	Scope            Scope                 `json:"scope"`
	Gateway          GatewayRef            `json:"gateway"`
	OrderIntent      *OrderIntent          `json:"order_intent,omitempty"`
	Order            *GatewayOrder         `json:"order,omitempty"`
	ProviderID       string                `json:"provider_id,omitempty"`
	Attempted        bool                  `json:"attempted"`
	State            string                `json:"state"`
	CreatedAt        time.Time             `json:"created_at"`
	UpdatedAt        time.Time             `json:"updated_at"`
}

type RefundIntent struct {
	OperationID   string     `json:"operation_id"`
	ReservationID string     `json:"reservation_id"`
	PaymentID     string     `json:"payment_id"`
	Scope         Scope      `json:"scope"`
	Gateway       GatewayRef `json:"gateway"`
	AmountPaisa   int64      `json:"amount_paisa"`
	Currency      string     `json:"currency"`
	Receipt       string     `json:"receipt"`
}
type GatewayRefund struct {
	ID     string       `json:"id"`
	Intent RefundIntent `json:"intent"`
	Status string       `json:"status"`
}
type CheckoutKey struct {
	EffectID  string      `json:"effect_id,omitempty"`
	AppID     string      `json:"app_id"`
	UserID    string      `json:"user_id"`
	InvoiceID string      `json:"invoice_id"`
	Mode      GatewayMode `json:"mode"`
}
