package billing

import (
	"context"
	"time"
)

type CreditReservationStatus string

const (
	CreditReservationReserved  CreditReservationStatus = "reserved"
	CreditReservationCommitted CreditReservationStatus = "committed"
	CreditReservationReleased  CreditReservationStatus = "released"
)

type CreditLedgerEntryType string

const (
	CreditLedgerAllocation CreditLedgerEntryType = "allocation"
	CreditLedgerBurn       CreditLedgerEntryType = "burn"
	CreditLedgerRelease    CreditLedgerEntryType = "release"
	CreditLedgerExpire     CreditLedgerEntryType = "expire"
	CreditLedgerAdjust     CreditLedgerEntryType = "adjust"
)

// CreditBalance represents the shared credit wallet for a workspace/tenant.
type CreditBalance struct {
	ID               string    `json:"id"`
	TenantID         string    `json:"tenant_id"`
	CreditsTotal     int64     `json:"credits_total"`
	CreditsRemaining int64     `json:"credits_remaining"`
	CreditsReserved  int64     `json:"credits_reserved"`
	CreditsExpireAt  time.Time `json:"credits_expire_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	CreatedAt        time.Time `json:"created_at"`
}

// CreditLedgerEntry is an immutable ledger row for balance changes.
type CreditLedgerEntry struct {
	ID              string                 `json:"id"`
	TenantID        string                 `json:"tenant_id"`
	AppID           string                 `json:"app_id,omitempty"`
	ReservationID   string                 `json:"reservation_id,omitempty"`
	NotificationID  string                 `json:"notification_id,omitempty"`
	Channel         string                 `json:"channel,omitempty"`
	EntryType       CreditLedgerEntryType  `json:"entry_type"`
	CreditsDelta    int64                  `json:"credits_delta"`
	BalanceAfter    int64                  `json:"balance_after"`
	RateCardVersion string                 `json:"rate_card_version,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
}

// CreditReservation is a temporary hold on credits before final delivery outcome.
type CreditReservation struct {
	ID              string                  `json:"id"`
	TenantID        string                  `json:"tenant_id"`
	AppID           string                  `json:"app_id,omitempty"`
	NotificationID  string                  `json:"notification_id,omitempty"`
	Channel         string                  `json:"channel"`
	CreditsReserved int64                   `json:"credits_reserved"`
	RateCardVersion string                  `json:"rate_card_version,omitempty"`
	Status          CreditReservationStatus `json:"status"`
	ExpiresAt       time.Time               `json:"expires_at"`
	CreatedAt       time.Time               `json:"created_at"`
	UpdatedAt       time.Time               `json:"updated_at"`
}

// CreditReservationManager defines reservation lifecycle contracts.
type CreditReservationManager interface {
	Reserve(ctx context.Context, reservation *CreditReservation) error
	Commit(ctx context.Context, reservationID string, ledgerEntry *CreditLedgerEntry) error
	Release(ctx context.Context, reservationID string, reason string) error
}

type PlanBundle struct {
	ID              string                 `json:"id"`
	Name            string                 `json:"name"`
	AmountPaisa     int64                  `json:"amount_paisa"`
	Currency        string                 `json:"currency"`
	CreditsIncluded int64                  `json:"credits_included"`
	ValidityDays    int                    `json:"validity_days"`
	Active          bool                   `json:"active"`
	DisplayOrder    int                    `json:"display_order"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

type RateCard struct {
	Version           string                `json:"version"`
	Active            bool                  `json:"active"`
	CreditValueINR    float64               `json:"credit_value_inr"`
	ChannelCreditCost map[string]int64      `json:"channel_credit_cost"`
	OveragePerMessage map[string]int64      `json:"overage_per_message"`
	Plans             map[string]PlanBundle `json:"plans,omitempty"`
	CreatedAt         time.Time             `json:"created_at"`
	UpdatedAt         time.Time             `json:"updated_at"`
}

type CreditBalanceRepository interface {
	GetByTenantID(ctx context.Context, tenantID string) (*CreditBalance, error)
	Upsert(ctx context.Context, balance *CreditBalance) error
}

type CreditLedgerRepository interface {
	Append(ctx context.Context, entry *CreditLedgerEntry) error
	ListByTenantID(ctx context.Context, tenantID string, limit int) ([]CreditLedgerEntry, error)
}

type RateCardRepository interface {
	CreateVersion(ctx context.Context, card *RateCard) error
	GetByVersion(ctx context.Context, version string) (*RateCard, error)
	GetActive(ctx context.Context) (*RateCard, error)
	SetActiveVersion(ctx context.Context, version string) error
}

type RateCardManager interface {
	GetActiveRateCard() *RateCard
	GetChannelCreditCost(channel string) int64
	GetRateCardVersion() string
	GetCheckoutPlan(planID string) (PlanBundle, bool)
	ListCheckoutPlans() []PlanBundle
	RefreshActiveRateCard(ctx context.Context) error
	ActivateVersion(ctx context.Context, version string) error
	UpdateChannelCredits(ctx context.Context, channel string, credits int64) (*RateCard, error)
	UpdatePlanBundle(ctx context.Context, plan PlanBundle) (*RateCard, error)
}
