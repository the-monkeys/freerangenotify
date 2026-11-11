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
	AppID            string                    `json:"app_id"`
	PrimaryCurrency  string                    `json:"primary_currency,omitempty"`
	LifecycleVersion int                       `json:"lifecycle_version"`
	Status           string                    `json:"status"`
	Epoch            int64                     `json:"epoch"`
	MutationPermits  map[string]MutationPermit `json:"mutation_permits"`
	MerchantSlots    map[GatewayMode]string    `json:"merchant_slots,omitempty"`
}

func DefaultLifecycleState(app string) LifecycleState {
	return LifecycleState{AppID: app, LifecycleVersion: 1, Status: "legacy", MutationPermits: map[string]MutationPermit{}}
}

type LifecycleStateStore interface {
	Load(context.Context, string) (LifecycleState, error)
	Mutate(context.Context, string, func(*LifecycleState) error) error
}
