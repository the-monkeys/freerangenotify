package bizbilling

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

type CommandKind string

const (
	CommandOpeningBalance    CommandKind = "opening_balance"
	CommandBalanceAdjustment CommandKind = "balance_adjustment"
)

// Command is a closed set of financial intents. Persist typed inputs; never
// store a generic patch or executable workflow received from an API client.
type Command struct {
	Kind              CommandKind        `json:"kind"`
	Scope             Scope              `json:"scope"`
	IdempotencyKey    string             `json:"idempotency_key,omitempty"`
	ActorID           string             `json:"actor_id,omitempty"`
	OpeningBalance    *OpeningBalance    `json:"opening_balance,omitempty"`
	BalanceAdjustment *BalanceAdjustment `json:"balance_adjustment,omitempty"`
}

var currencyCode = regexp.MustCompile(`^[A-Z]{3}$`)

func (s Scope) Validate() error {
	if strings.TrimSpace(s.AppID) == "" || strings.TrimSpace(s.UserID) == "" || len(s.AppID) > 200 || len(s.UserID) > 200 || !currencyCode.MatchString(s.Currency) {
		return fmt.Errorf("invalid billing scope")
	}
	return nil
}
func (c Command) Validate() error {
	if err := c.Scope.Validate(); err != nil {
		return err
	}
	if len(c.IdempotencyKey) > 200 || len(c.ActorID) > 200 {
		return ErrInvalidMoney
	}
	switch c.Kind {
	case CommandOpeningBalance:
		if c.OpeningBalance == nil || c.BalanceAdjustment != nil || c.OpeningBalance.AvailablePaisa < 0 || (c.OpeningBalance.UnresolvedLegacy && c.OpeningBalance.AvailablePaisa != 0) {
			return ErrInvalidMoney
		}
	case CommandBalanceAdjustment:
		if c.BalanceAdjustment == nil || c.OpeningBalance != nil || strings.TrimSpace(c.BalanceAdjustment.Reason) == "" || len(c.BalanceAdjustment.Reason) > 1000 {
			return ErrInvalidMoney
		}
	default:
		return ErrInvalidMoney
	}
	return nil
}

type OperationState string

const (
	OperationAccepted  OperationState = "accepted"
	OperationPrepared  OperationState = "prepared"
	OperationCompleted OperationState = "completed"
	OperationRejected  OperationState = "rejected"
)

type ResourceRef struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}
type Operation struct {
	SchemaVersion int            `json:"schema_version"`
	ID            string         `json:"id"`
	AppID         string         `json:"app_id"`
	Scope         Scope          `json:"scope"`
	IntentHash    string         `json:"intent_hash"`
	Command       Command        `json:"command"`
	State         OperationState `json:"state"`
	Event         *JournalEvent  `json:"event,omitempty"`
	Resources     []ResourceRef  `json:"resources,omitempty"`
	ErrorCode     string         `json:"error_code,omitempty"`
	HeadReleased  bool           `json:"head_released"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}
type OperationResult struct {
	OperationID string         `json:"operation_id"`
	State       OperationState `json:"state"`
	Resources   []ResourceRef  `json:"resources,omitempty"`
	ErrorCode   string         `json:"error_code,omitempty"`
}

func (o Operation) Result() OperationResult {
	return OperationResult{OperationID: o.ID, State: o.State, Resources: o.Resources, ErrorCode: o.ErrorCode}
}

type Stream struct {
	ID                string    `json:"id"`
	Scope             Scope     `json:"scope"`
	CommittedSequence int64     `json:"committed_sequence"`
	ActiveOperationID string    `json:"active_operation_id"`
	Owner             string    `json:"owner"`
	Generation        int64     `json:"generation"`
	LeaseUntil        time.Time `json:"lease_until"`
}
type StreamClaim struct {
	Scope              Scope
	Sequence           int64
	OperationID, Owner string
	Generation         int64
}

// JournalEvent is frozen before append. Absolute typed projections and their
// sequence checkpoints make recovery independent of later configuration edits.
type JournalEvent struct {
	SchemaVersion int                  `json:"schema_version"`
	ID            string               `json:"id"`
	AppID         string               `json:"app_id"`
	OperationID   string               `json:"operation_id"`
	Scope         Scope                `json:"scope"`
	Sequence      int64                `json:"sequence"`
	Kind          CommandKind          `json:"kind"`
	BalanceAfter  *CustomerBalance     `json:"balance_after,omitempty"`
	Notifications []NotificationIntent `json:"notifications,omitempty"`
	CreatedAt     time.Time            `json:"created_at"`
}
