package bizbilling

import (
	"context"
	"time"
)

type MutationPermit struct {
	AppID       string    `json:"app_id"`
	WriterID    string    `json:"writer_id"`
	OperationID string    `json:"operation_id,omitempty"`
	Epoch       int64     `json:"epoch"`
	StartedAt   time.Time `json:"started_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}
type LifecycleState struct {
	MerchantSwitch        *MerchantSwitchFence      `json:"merchant_switch,omitempty"`
	MerchantSlotRevisions map[GatewayMode]int64     `json:"merchant_slot_revisions,omitempty"`
	PermitReview          *WriterPermitReviewFence  `json:"permit_review,omitempty"`
	LastWriterReviewID    string                    `json:"last_writer_review_id,omitempty"`
	MigrationReportID     string                    `json:"migration_report_id,omitempty"`
	MigrationOwner        string                    `json:"migration_owner,omitempty"`
	MigrationFactsStarted bool                      `json:"migration_facts_started,omitempty"`
	MigrationLeaseUntil   time.Time                 `json:"migration_lease_until,omitempty"`
	WriteRevision         int64                     `json:"write_revision,omitempty"`
	AppID                 string                    `json:"app_id"`
	PrimaryCurrency       string                    `json:"primary_currency,omitempty"`
	LifecycleVersion      int                       `json:"lifecycle_version"`
	Status                string                    `json:"status"`
	Epoch                 int64                     `json:"epoch"`
	MutationPermits       map[string]MutationPermit `json:"mutation_permits"`
	MerchantSlots         map[GatewayMode]string    `json:"merchant_slots,omitempty"`
}

func DefaultLifecycleState(app string) LifecycleState {
	return LifecycleState{AppID: app, LifecycleVersion: 1, Status: "legacy", MutationPermits: map[string]MutationPermit{}}
}

type LifecycleStateStore interface {
	Load(context.Context, string) (LifecycleState, error)
	Mutate(context.Context, string, func(*LifecycleState) error) error
}
