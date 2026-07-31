package bizbilling

import "time"

// ─── Estimate Status Constants ───────────────────────────────────────────

type EstimateStatus string

const (
	EstimateStatusDraft     EstimateStatus = "draft"
	EstimateStatusSent      EstimateStatus = "sent"
	EstimateStatusAccepted  EstimateStatus = "accepted"
	EstimateStatusRejected  EstimateStatus = "rejected"
	EstimateStatusExpired   EstimateStatus = "expired"
	EstimateStatusConverted EstimateStatus = "converted"
)

// ─── Domain Models ───────────────────────────────────────────────────────

// Estimate represents a quote sent to a customer that can be accepted,
// rejected, and converted into an invoice.
type Estimate struct {
	ID                 string                 `json:"id"`
	AppID              string                 `json:"app_id"`
	UserID             string                 `json:"user_id"`
	EstimateNumber     string                 `json:"estimate_number"`
	Status             EstimateStatus         `json:"status"`
	LineItems          []InvoiceLineItem      `json:"line_items"`
	SubtotalPaisa      int64                  `json:"subtotal_paisa"`
	TaxPaisa           int64                  `json:"tax_paisa"`
	TotalPaisa         int64                  `json:"total_paisa"`
	Currency           string                 `json:"currency"`
	ValidUntil         *time.Time             `json:"valid_until,omitempty"`
	Notes              string                 `json:"notes,omitempty"`
	Terms              string                 `json:"terms,omitempty"`
	AcceptedAt         *time.Time             `json:"accepted_at,omitempty"`
	RejectedAt         *time.Time             `json:"rejected_at,omitempty"`
	ConvertedInvoiceID string                 `json:"converted_invoice_id,omitempty"`
	NotificationID     string                 `json:"notification_id,omitempty"`
	Metadata           map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt          time.Time              `json:"created_at"`
	UpdatedAt          time.Time              `json:"updated_at"`
}

// ─── DTOs ────────────────────────────────────────────────────────────────

// CreateEstimateRequest is the payload for creating an estimate.
type CreateEstimateRequest struct {
	UserID     string                 `json:"user_id" validate:"required"`
	LineItems  []InvoiceLineItem      `json:"line_items" validate:"required,min=1"`
	Currency   string                 `json:"currency,omitempty"`
	ValidUntil *time.Time             `json:"valid_until,omitempty"`
	Notes      string                 `json:"notes,omitempty"`
	Terms      string                 `json:"terms,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// UpdateEstimateRequest is the payload for updating a draft estimate.
type UpdateEstimateRequest struct {
	LineItems  []InvoiceLineItem      `json:"line_items,omitempty"`
	ValidUntil *time.Time             `json:"valid_until,omitempty"`
	Notes      *string                `json:"notes,omitempty"`
	Terms      *string                `json:"terms,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// EstimateFilter defines criteria for listing estimates.
type EstimateFilter struct {
	AppID  string
	UserID string
	Status EstimateStatus
	Limit  int
	Offset int
}
