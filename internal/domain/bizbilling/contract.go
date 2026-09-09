package bizbilling

import "time"

// ─── Contract Status Constants ───────────────────────────────────────────

type ContractStatus string

const (
	ContractStatusDraft      ContractStatus = "draft"
	ContractStatusSent       ContractStatus = "sent"
	ContractStatusActive     ContractStatus = "active"
	ContractStatusExpired    ContractStatus = "expired"
	ContractStatusTerminated ContractStatus = "terminated"
)

// ─── Domain Models ───────────────────────────────────────────────────────

// Contract represents a service agreement with a customer.
type Contract struct {
	ID              string                 `json:"id"`
	AppID           string                 `json:"app_id"`
	UserID          string                 `json:"user_id"`
	SubscriptionID  string                 `json:"subscription_id,omitempty"`
	ContractNumber  string                 `json:"contract_number"`
	Status          ContractStatus         `json:"status"`
	Terms           string                 `json:"terms,omitempty"`
	ValuePaisa      int64                  `json:"value_paisa"`
	StartDate       time.Time              `json:"start_date"`
	EndDate         time.Time              `json:"end_date"`
	AutoRenew       bool                   `json:"auto_renew"`
	AcceptedAt      *time.Time             `json:"accepted_at,omitempty"`
	TerminatedAt    *time.Time             `json:"terminated_at,omitempty"`
	Amendments      []ContractAmendment    `json:"amendments,omitempty"`
	NotificationID  string                 `json:"notification_id,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	ExpiryNotifiedAt *time.Time            `json:"expiry_notified_at,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}

// ContractAmendment records a mid-cycle modification to a contract.
type ContractAmendment struct {
	ID            string    `json:"id"`
	Description   string    `json:"description"`
	EffectiveDate time.Time `json:"effective_date"`
	CreatedAt     time.Time `json:"created_at"`
}

// ─── DTOs ────────────────────────────────────────────────────────────────

// CreateContractRequest is the payload for creating a contract.
type CreateContractRequest struct {
	UserID         string                 `json:"user_id" validate:"required"`
	SubscriptionID string                 `json:"subscription_id,omitempty"`
	Terms          string                 `json:"terms,omitempty"`
	ValuePaisa     int64                  `json:"value_paisa,omitempty"`
	StartDate      time.Time              `json:"start_date" validate:"required"`
	EndDate        time.Time              `json:"end_date" validate:"required"`
	AutoRenew      bool                   `json:"auto_renew,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// UpdateContractRequest is the payload for updating a draft contract.
type UpdateContractRequest struct {
	Terms      *string    `json:"terms,omitempty"`
	ValuePaisa *int64     `json:"value_paisa,omitempty"`
	StartDate  *time.Time `json:"start_date,omitempty"`
	EndDate    *time.Time `json:"end_date,omitempty"`
	AutoRenew  *bool      `json:"auto_renew,omitempty"`
}

// AmendContractRequest is the payload for adding an amendment.
type AmendContractRequest struct {
	Description   string    `json:"description" validate:"required"`
	EffectiveDate time.Time `json:"effective_date" validate:"required"`
}

// ContractFilter defines criteria for listing contracts.
type ContractFilter struct {
	AppID  string
	UserID string
	Status ContractStatus
	Limit  int
	Offset int
}
