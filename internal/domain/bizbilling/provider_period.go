package bizbilling

import (
	"context"
	"time"
)

// Optional provider capability. Existing one-time payment adapters are unchanged.
type RecurringInvoiceGateway interface {
	FetchRecurringInvoice(context.Context, string) (GatewayRecurringInvoice, error)
	ListRecurringInvoices(context.Context, string, int, int) ([]GatewayRecurringInvoice, error)
}

type GatewayRecurringInvoice struct {
	ID             string     `json:"id"`
	SubscriptionID string     `json:"subscription_id"`
	OrderID        string     `json:"order_id"`
	PaymentID      string     `json:"payment_id,omitempty"`
	Gateway        GatewayRef `json:"gateway"`
	Status         string     `json:"status"`
	AmountPaisa    int64      `json:"amount_paisa"`
	PaidPaisa      int64      `json:"paid_paisa"`
	DuePaisa       int64      `json:"due_paisa"`
	Currency       string     `json:"currency"`
	Start          time.Time  `json:"start"`
	End            time.Time  `json:"end"`
}

// Frozen only after server-to-server invoice/subscription ownership checks.
type VerifiedProviderPeriod struct {
	EffectID string                  `json:"effect_id"`
	Invoice  GatewayRecurringInvoice `json:"invoice"`
}
