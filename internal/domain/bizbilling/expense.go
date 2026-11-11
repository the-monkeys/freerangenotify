package bizbilling

import "time"

// ─── Expense Tracking (lightweight) ──────────────────────────────────────

// Expense represents a recorded business expense, optionally billable to a user.
type Expense struct {
	Currency           string                 `json:"currency,omitempty"`
	LastSequence       int64                  `json:"billing_sequence,omitempty"`
	PendingOperationID string                 `json:"pending_operation_id,omitempty"`
	PendingGeneration  int64                  `json:"pending_generation,omitempty"`
	ID                 string                 `json:"id"`
	AppID              string                 `json:"app_id"`
	Category           string                 `json:"category"`
	Description        string                 `json:"description,omitempty"`
	AmountPaisa        int64                  `json:"amount_paisa"`
	Vendor             string                 `json:"vendor,omitempty"`
	Date               time.Time              `json:"date"`
	Billable           bool                   `json:"billable"`
	BilledToUser       string                 `json:"billed_to_user,omitempty"`
	InvoiceID          string                 `json:"invoice_id,omitempty"` // Set once attached to an invoice
	ReceiptFileID      string                 `json:"receipt_file_id,omitempty"`
	Recurring          bool                   `json:"recurring"`
	RecurrenceCycle    string                 `json:"recurrence_cycle,omitempty"`
	Metadata           map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt          time.Time              `json:"created_at"`
	UpdatedAt          time.Time              `json:"updated_at"`
}

// ─── DTOs ────────────────────────────────────────────────────────────────

// CreateExpenseRequest is the payload for recording an expense.
type CreateExpenseRequest struct {
	Currency        string                 `json:"currency,omitempty"`
	Category        string                 `json:"category" validate:"required"`
	Description     string                 `json:"description,omitempty"`
	AmountPaisa     int64                  `json:"amount_paisa" validate:"required,gt=0"`
	Vendor          string                 `json:"vendor,omitempty"`
	Date            *time.Time             `json:"date,omitempty"` // defaults to now
	Billable        bool                   `json:"billable,omitempty"`
	BilledToUser    string                 `json:"billed_to_user,omitempty"`
	ReceiptFileID   string                 `json:"receipt_file_id,omitempty"`
	Recurring       bool                   `json:"recurring,omitempty"`
	RecurrenceCycle string                 `json:"recurrence_cycle,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// UpdateExpenseRequest is the payload for updating an expense.
type UpdateExpenseRequest struct {
	Category      *string    `json:"category,omitempty"`
	Description   *string    `json:"description,omitempty"`
	AmountPaisa   *int64     `json:"amount_paisa,omitempty"`
	Vendor        *string    `json:"vendor,omitempty"`
	Date          *time.Time `json:"date,omitempty"`
	Billable      *bool      `json:"billable,omitempty"`
	BilledToUser  *string    `json:"billed_to_user,omitempty"`
	ReceiptFileID *string    `json:"receipt_file_id,omitempty"`
}

// ExpenseFilter defines criteria for listing expenses.
type ExpenseFilter struct {
	AppID    string
	Category string
	Billable *bool
	UserID   string // billed_to_user
	From     *time.Time
	To       *time.Time
	Limit    int
	Offset   int
}

func (e Expense) FinancialScope() Scope {
	return Scope{AppID: e.AppID, UserID: e.BilledToUser, Currency: e.Currency}
}
func (e Expense) FinancialSequence() int64 { return e.LastSequence }
func (e Expense) FinancialID() string      { return e.ID }
