package bizbilling

import "time"

type BillingAddress struct {
	Line1   string `json:"line1,omitempty"`
	Line2   string `json:"line2,omitempty"`
	City    string `json:"city,omitempty"`
	State   string `json:"state,omitempty"`
	Pincode string `json:"pincode,omitempty"`
	Country string `json:"country,omitempty"`
}

type BillingParty struct {
	Name    string         `json:"name"`
	Email   string         `json:"email,omitempty"`
	Phone   string         `json:"phone,omitempty"`
	GSTIN   string         `json:"gstin,omitempty"`
	Address BillingAddress `json:"address"`
}

// InvoiceSnapshot is frozen in the issue event. Reading a historical invoice
// never substitutes today's customer, business identity, prices or tax policy.
type InvoiceSnapshot struct {
	Version        int               `json:"version"`
	InvoiceID      string            `json:"invoice_id"`
	InvoiceNumber  string            `json:"invoice_number"`
	Issuer         BillingParty      `json:"issuer"`
	Customer       BillingParty      `json:"customer"`
	LineItems      []InvoiceLineItem `json:"line_items"`
	Currency       string            `json:"currency"`
	SubtotalPaisa  int64             `json:"subtotal_paisa"`
	DiscountPaisa  int64             `json:"discount_paisa"`
	TaxAmountPaisa int64             `json:"tax_amount_paisa"`
	TaxBreakdown   TaxBreakdown      `json:"tax_breakdown"`
	TotalPaisa     int64             `json:"total_paisa"`
	IssuedAt       time.Time         `json:"issued_at"`
	DueDate        *time.Time        `json:"due_date,omitempty"`
	PaymentTerms   string            `json:"payment_terms,omitempty"`
	Notes          string            `json:"notes,omitempty"`
	FooterText     string            `json:"footer_text,omitempty"`
	TermsText      string            `json:"terms_text,omitempty"`
}

type DocumentPayment struct {
	Refunds          []DocumentRefund `json:"refunds"`
	ID               string           `json:"id"`
	Method           string           `json:"method"`
	Reference        string           `json:"reference,omitempty"`
	GatewayPaymentID string           `json:"gateway_payment_id,omitempty"`
	AmountPaisa      int64            `json:"amount_paisa"`
	TDSPaisa         int64            `json:"tds_paisa,omitempty"`
	RefundedPaisa    int64            `json:"refunded_paisa"`
	Status           PaymentStatus    `json:"status"`
	CreatedAt        time.Time        `json:"created_at"`
}

type DocumentRefund struct {
	ID          string `json:"id"`
	Reference   string `json:"reference,omitempty"`
	AmountPaisa int64  `json:"amount_paisa"`
	State       string `json:"state"`
}

type DocumentCredit struct {
	ID          string `json:"id"`
	SourceType  string `json:"source_type"`
	SourceID    string `json:"source_id,omitempty"`
	AmountPaisa int64  `json:"amount_paisa"`
	State       string `json:"state"`
}

// Current settlement is deliberately separate from the immutable invoice total.
type BillingDocument struct {
	InvoiceSnapshot
	Credits           []DocumentCredit  `json:"credits"`
	Status            InvoiceStatus     `json:"status"`
	AmountPaidPaisa   int64             `json:"amount_paid_paisa"`
	AmountDuePaisa    int64             `json:"amount_due_paisa"`
	CreditsApplied    int64             `json:"credits_applied"`
	TDSPaisa          int64             `json:"tds_paisa"`
	LateFeePaisa      int64             `json:"late_fee_paisa"`
	ForgivenPaisa     int64             `json:"forgiven_paisa"`
	CashRefundedPaisa int64             `json:"cash_refunded_paisa"`
	Payments          []DocumentPayment `json:"payments"`
}

type InvoicePreview struct {
	Currency       string            `json:"currency"`
	LineItems      []InvoiceLineItem `json:"line_items"`
	SubtotalPaisa  int64             `json:"subtotal_paisa"`
	DiscountPaisa  int64             `json:"discount_paisa"`
	TaxAmountPaisa int64             `json:"tax_amount_paisa"`
	TaxBreakdown   TaxBreakdown      `json:"tax_breakdown"`
	TotalPaisa     int64             `json:"total_paisa"`
	PaymentTerms   string            `json:"payment_terms"`
}
