package bizbilling

import (
	"time"
)

// ─── Invoice Status Constants ────────────────────────────────────────────

type InvoiceStatus string
type InvoiceType string

const (
	InvoiceStatusDraft         InvoiceStatus = "draft"
	InvoiceStatusOpen          InvoiceStatus = "open" // Issued and waiting for payment
	InvoiceStatusPartiallyPaid InvoiceStatus = "partially_paid"
	InvoiceStatusPaid          InvoiceStatus = "paid"
	InvoiceStatusVoid          InvoiceStatus = "void"
	InvoiceStatusUncollectible InvoiceStatus = "uncollectible"
	InvoiceStatusOverdue       InvoiceStatus = "overdue"

	InvoiceTypeOneTime   InvoiceType = "one_time"
	InvoiceTypeRecurring InvoiceType = "recurring"
	InvoiceTypeRetainer  InvoiceType = "retainer"
)

// PayableStatuses are statuses in which an invoice can accept payments.
var PayableStatuses = map[InvoiceStatus]bool{
	InvoiceStatusOpen:          true,
	InvoiceStatusPartiallyPaid: true,
	InvoiceStatusOverdue:       true,
}

// ─── Domain Models ───────────────────────────────────────────────────────

// BizInvoice represents an invoice generated for a customer.
type BizInvoice struct {
	ID                  string                 `json:"id"`
	AppID               string                 `json:"app_id"`
	UserID              string                 `json:"user_id"`
	SubscriptionID      string                 `json:"subscription_id,omitempty"`
	EstimateID          string                 `json:"estimate_id,omitempty"`
	ContractID          string                 `json:"contract_id,omitempty"`
	InvoiceNumber       string                 `json:"invoice_number"`
	InvoiceType         InvoiceType            `json:"invoice_type"`
	Status              InvoiceStatus          `json:"status"`
	LineItems           []InvoiceLineItem      `json:"line_items"`
	SubtotalPaisa       int64                  `json:"subtotal_paisa"`
	DiscountPaisa       int64                  `json:"discount_paisa"`
	CouponID            string                 `json:"coupon_id,omitempty"`
	TaxAmountPaisa      int64                  `json:"tax_amount_paisa"`
	TaxBreakdown        TaxBreakdown           `json:"tax_breakdown"`
	LateFeePaisa        int64                  `json:"late_fee_paisa"`
	TotalPaisa          int64                  `json:"total_paisa"`
	AmountPaidPaisa     int64                  `json:"amount_paid_paisa"`
	AmountDuePaisa      int64                  `json:"amount_due_paisa"`
	CreditsApplied      int64                  `json:"credits_applied"`
	Currency            string                 `json:"currency"`
	ExchangeRate        float64                `json:"exchange_rate,omitempty"`
	PaymentLink         string                 `json:"payment_link,omitempty"`
	GatewayOrderID      string                 `json:"gateway_order_id,omitempty"`
	PaymentTerms        string                 `json:"payment_terms,omitempty"`
	DueDate             *time.Time             `json:"due_date,omitempty"`
	IssuedAt            *time.Time             `json:"issued_at,omitempty"`
	PaidAt              *time.Time             `json:"paid_at,omitempty"`
	VoidedAt            *time.Time             `json:"voided_at,omitempty"`
	WrittenOffAt        *time.Time             `json:"written_off_at,omitempty"`
	EInvoiceIRN         string                 `json:"e_invoice_irn,omitempty"`
	EInvoiceQR          string                 `json:"e_invoice_qr,omitempty"`
	Attachments         []string               `json:"attachments,omitempty"`
	NotificationID      string                 `json:"notification_id,omitempty"`
	Notes               string                 `json:"notes,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt           time.Time              `json:"created_at"`
	UpdatedAt           time.Time              `json:"updated_at"`
}

// InvoiceLineItem represents a single item on an invoice.
type InvoiceLineItem struct {
	Description   string `json:"description"`
	HSNCode       string `json:"hsn_code,omitempty"`
	Quantity      int    `json:"quantity"`
	UnitPrice     int64  `json:"unit_price"` // in paisa
	DiscountPaisa int64  `json:"discount_paisa,omitempty"`
	TaxRate       int    `json:"tax_rate"` // e.g., 18 for 18%
	AmountPaisa   int64  `json:"amount_paisa"` // (Quantity * UnitPrice) - Discount
}

// TaxBreakdown contains specific tax components for GST compliance.
type TaxBreakdown struct {
	CGSTPaisa int64 `json:"cgst_paisa"`
	SGSTPaisa int64 `json:"sgst_paisa"`
	IGSTPaisa int64 `json:"igst_paisa"`
}

// ─── DTOs for Creation/Updates ───────────────────────────────────────────

// CreateInvoiceRequest represents the payload for creating a one-time invoice.
type CreateInvoiceRequest struct {
	UserID        string                 `json:"user_id" validate:"required"`
	LineItems     []InvoiceLineItem      `json:"line_items" validate:"required,min=1"`
	Currency      string                 `json:"currency" validate:"required"`
	PaymentTerms  string                 `json:"payment_terms,omitempty"`
	Notes         string                 `json:"notes,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// UpdateInvoiceRequest represents the payload for updating a draft invoice.
type UpdateInvoiceRequest struct {
	LineItems     []InvoiceLineItem      `json:"line_items,omitempty"`
	PaymentTerms  *string                `json:"payment_terms,omitempty"`
	Notes         *string                `json:"notes,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// RecordPaymentRequest records a manual or gateway payment against an invoice.
// AmountPaisa = 0 means "pay the full amount due" (partial payments supported).
type RecordPaymentRequest struct {
	AmountPaisa  int64  `json:"amount_paisa,omitempty" validate:"min=0"`
	Method       string `json:"method" validate:"required"` // cash | bank_transfer | cheque | card | upi | gateway
	GatewayTxnID string `json:"gateway_txn_id,omitempty"`
	TDSPaisa     int64  `json:"tds_paisa,omitempty" validate:"min=0"`
	Notes        string `json:"notes,omitempty"`
}

// ApplyCouponRequest applies a coupon code to a draft invoice.
type ApplyCouponRequest struct {
	Code string `json:"code" validate:"required"`
}

// AdjustBalanceRequest adjusts a user's credit balance (positive or negative).
type AdjustBalanceRequest struct {
	AmountPaisa int64  `json:"amount_paisa" validate:"required"`
	Reason      string `json:"reason,omitempty"`
}

// ─── Customer Statement ──────────────────────────────────────────────────

// StatementEntry is one transaction row in a customer account statement.
type StatementEntry struct {
	Date        time.Time `json:"date"`
	Type        string    `json:"type"` // invoice | payment | credit_note | retainer
	Reference   string    `json:"reference"`
	Description string    `json:"description,omitempty"`
	DebitPaisa  int64     `json:"debit_paisa,omitempty"`  // What the customer owes
	CreditPaisa int64     `json:"credit_paisa,omitempty"` // What the customer paid / was credited
}

// Statement is a full customer account statement.
type Statement struct {
	UserID       string           `json:"user_id"`
	Entries      []StatementEntry `json:"entries"`
	TotalBilled  int64            `json:"total_billed_paisa"`
	TotalPaid    int64            `json:"total_paid_paisa"`
	BalanceOwed  int64            `json:"balance_owed_paisa"`
	CreditPaisa  int64            `json:"credit_balance_paisa"` // User's stored credit
	GeneratedAt  time.Time        `json:"generated_at"`
}


