package billing

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"time"
)

var ErrInsufficientCredits = errors.New("insufficient credits")

const (
	CreditReservationModeJournal               = "journal"
	CreditDeliveryReservationMetadataKey       = "delivery_reservation_id"
	CreditUnavailableReasonInsufficientCredits = "insufficient_credits"
	CreditUnavailableReasonTemporarilyReserved = "credits_temporarily_reserved"
	CreditUnavailableReasonDailyCapExceeded    = "daily_cap_exceeded"
)

// CreditUnavailableError describes the snapshot returned by the rejecting
// atomic operation. Never populate it from a preceding balance read.
type CreditUnavailableError struct {
	Reason           string
	CreditsRequired  int64
	CreditsRemaining int64
	CreditsReserved  int64
	CreditsAvailable int64
	RateCardVersion  string
}

func (e *CreditUnavailableError) Error() string { return "insufficient credits" }
func (e *CreditUnavailableError) Unwrap() error { return ErrInsufficientCredits }
func NewCreditUnavailableError(b *CreditBalance, cost int64, version string) *CreditUnavailableError {
	reason := CreditUnavailableReasonInsufficientCredits
	if b.CreditsRemaining >= cost && b.Available() < cost {
		reason = CreditUnavailableReasonTemporarilyReserved
	}
	return &CreditUnavailableError{Reason: reason, CreditsRequired: cost, CreditsRemaining: b.CreditsRemaining, CreditsReserved: b.CreditsReserved, CreditsAvailable: b.Available(), RateCardVersion: version}
}

var (
	ErrCreditReservationNotFound       = errors.New("credit reservation not found")
	ErrCreditJournalInvariant          = errors.New("credit journal invariant violation")
	ErrCreditJournalAdmissionsDisabled = errors.New("credit journal admissions disabled; recovery remains enabled")
	ErrCreditReservationOutcomeUnknown = errors.New("credit reservation delivery outcome unknown; held for reconciliation")
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
	JournalBacked    bool      `json:"-"`
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
	SubscriptionID  string                  `json:"subscription_id,omitempty"`
	DailyCapKey     string                  `json:"daily_cap_key,omitempty"`
	BalanceAfter    *CreditBalance          `json:"balance_after,omitempty"` // legacy retry compatibility; journal stores its own snapshot
	ReleaseReason   string                  `json:"release_reason,omitempty"`
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

// Journal receipts are retained in subscription _source. Balance captures the
// exact operation result, so a delayed ledger repair never observes later work.
type CreditReservationReceipt struct {
	Reservation     CreditReservation `json:"reservation"`
	Balance         CreditBalance     `json:"balance"`
	LedgerRecorded  bool              `json:"ledger_recorded"`
	Reason          string            `json:"reason,omitempty"`
	DeliveryOutcome string            `json:"delivery_outcome,omitempty"`
}
type CreditTransitionOutcome struct {
	Receipt CreditReservationReceipt
	Applied bool
}
type CreditReceiptPage struct {
	Receipts       []CreditReservationReceipt
	NextCursor     string
	JournalEntries int     // document-growth observation, including retained terminals
	Errors         []error // quarantined source errors; independent pages still recover
}

// Optional interfaces preserve legacy repositories and lifecycle callers.
type CreditReservationJournal interface {
	ReserveReservation(context.Context, *CreditReservation) (*CreditTransitionOutcome, error)
	TransitionReservation(context.Context, string, CreditReservationStatus, string) (*CreditTransitionOutcome, error)
	GetReservationReceipt(context.Context, string) (*CreditReservationReceipt, error)
	MarkReservationLedgerRecorded(context.Context, string, CreditReservationStatus) error
	ListReservationReceipts(context.Context, string, int) (*CreditReceiptPage, error)
}
type AtomicCreditAllocator interface {
	GrantCreditBalance(context.Context, string, int64) (*CreditBalance, error)
	BootstrapCreditBalance(context.Context, string, int64, time.Time) (*CreditBalance, error)
}

// Optional: definitive provider failure evidence, retained across retries.
type CreditReservationFailureRecorder interface {
	RecordReservationDeliveryFailure(context.Context, string) error
}

// The opaque ID routes recovery to the originating subscription after renewal
// and without Redis. IDs supplied to workers must be persisted unchanged.
func NewJournalReservationID(subscriptionID, uniqueID string) string {
	return "cr1." + base64.RawURLEncoding.EncodeToString([]byte(subscriptionID)) + "." + uniqueID
}
func JournalSubscriptionID(id string) (string, bool) {
	p := strings.Split(id, ".")
	if len(p) != 3 || p[0] != "cr1" || p[2] == "" {
		return "", false
	}
	b, err := base64.RawURLEncoding.DecodeString(p[1])
	return string(b), err == nil && len(b) > 0
}
func CreditTransitionLedgerID(id string, status CreditReservationStatus) string {
	return "credit:" + string(status) + ":" + id
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
	ReserveCredits(ctx context.Context, tenantID string, amount int64) (*CreditBalance, error)
	CommitReservedCredits(ctx context.Context, tenantID string, amount int64) (*CreditBalance, error)
	ReleaseReservedCredits(ctx context.Context, tenantID string, amount int64) (*CreditBalance, error)
	ClearReservedCredits(ctx context.Context, tenantID string) (*CreditBalance, error)
}

func (b *CreditBalance) Available() int64 {
	if b == nil {
		return 0
	}
	available := b.CreditsRemaining - b.CreditsReserved
	if available < 0 {
		return 0
	}
	return available
}

func (b *CreditBalance) Reserve(amount int64) error {
	if b == nil {
		return ErrInsufficientCredits
	}
	if b.Available() < amount {
		return ErrInsufficientCredits
	}
	b.CreditsReserved += amount
	return nil
}

func (b *CreditBalance) Commit(amount int64) error {
	if b == nil {
		return ErrInsufficientCredits
	}
	if b.CreditsReserved < amount {
		b.CreditsReserved = 0
	} else {
		b.CreditsReserved -= amount
	}
	if b.CreditsRemaining < amount {
		return ErrInsufficientCredits
	}
	b.CreditsRemaining -= amount
	return nil
}

func (b *CreditBalance) Release(amount int64) {
	if b == nil {
		return
	}
	if b.CreditsReserved < amount {
		b.CreditsReserved = 0
		return
	}
	b.CreditsReserved -= amount
}

func (b *CreditBalance) ClearReserved() {
	if b == nil {
		return
	}
	b.CreditsReserved = 0
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
