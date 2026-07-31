package bizbilling

import "time"

// ─── Credit Note & Retainer Status Constants ─────────────────────────────

type CreditNoteStatus string

const (
	CreditNoteStatusOpen     CreditNoteStatus = "open"     // Balance available
	CreditNoteStatusApplied  CreditNoteStatus = "applied"  // Fully consumed against invoices
	CreditNoteStatusRefunded CreditNoteStatus = "refunded" // Refunded to the customer
	CreditNoteStatusVoid     CreditNoteStatus = "void"
)

type RetainerStatus string

const (
	RetainerStatusOpen    RetainerStatus = "open"    // Awaiting payment
	RetainerStatusPaid    RetainerStatus = "paid"    // Paid; balance can be drawn down
	RetainerStatusApplied RetainerStatus = "applied" // Fully consumed
	RetainerStatusVoid    RetainerStatus = "void"
)

// ─── Domain Models ───────────────────────────────────────────────────────

// CreditNote represents a credit issued to a customer for billing
// corrections or partial refunds. BalancePaisa tracks the unconsumed amount.
type CreditNote struct {
	ID               string           `json:"id"`
	AppID            string           `json:"app_id"`
	UserID           string           `json:"user_id"`
	InvoiceID        string           `json:"invoice_id,omitempty"` // Originating invoice, if any
	CreditNoteNumber string           `json:"credit_note_number"`
	AmountPaisa      int64            `json:"amount_paisa"`
	BalancePaisa     int64            `json:"balance_paisa"`
	Reason           string           `json:"reason,omitempty"`
	Status           CreditNoteStatus `json:"status"`
	AppliedInvoices  []string         `json:"applied_invoices,omitempty"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
}

// Retainer represents an advance-payment (deposit) invoice whose paid
// balance is drawn down against future invoices.
type Retainer struct {
	ID              string         `json:"id"`
	AppID           string         `json:"app_id"`
	UserID          string         `json:"user_id"`
	AmountPaisa     int64          `json:"amount_paisa"`
	BalancePaisa    int64          `json:"balance_paisa"`
	Status          RetainerStatus `json:"status"`
	Notes           string         `json:"notes,omitempty"`
	AppliedInvoices []string       `json:"applied_invoices,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

// ─── DTOs ────────────────────────────────────────────────────────────────

// CreateCreditNoteRequest is the payload for creating a credit note.
type CreateCreditNoteRequest struct {
	UserID      string `json:"user_id" validate:"required"`
	InvoiceID   string `json:"invoice_id,omitempty"`
	AmountPaisa int64  `json:"amount_paisa" validate:"required,gt=0"`
	Reason      string `json:"reason,omitempty"`
}

// CreateRetainerRequest is the payload for creating a retainer invoice.
type CreateRetainerRequest struct {
	UserID      string `json:"user_id" validate:"required"`
	AmountPaisa int64  `json:"amount_paisa" validate:"required,gt=0"`
	Notes       string `json:"notes,omitempty"`
}

// CreditNoteFilter defines criteria for listing credit notes.
type CreditNoteFilter struct {
	AppID  string
	UserID string
	Status CreditNoteStatus
	Limit  int
	Offset int
}

// RetainerFilter defines criteria for listing retainers.
type RetainerFilter struct {
	AppID  string
	UserID string
	Status RetainerStatus
	Limit  int
	Offset int
}
