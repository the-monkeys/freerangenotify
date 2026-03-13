package license

import (
	"context"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/application"
)

// Mode identifies licensing mode for a deployment.
type Mode string

const (
	ModeHosted     Mode = "hosted"
	ModeSelfHosted Mode = "self_hosted"
)

// State describes current license/subscription state.
type State string

const (
	StateUnlicensed State = "unlicensed"
	StateActive     State = "active"
	StateExpired    State = "expired"
	StateInvalid    State = "invalid"
	StateGrace      State = "grace"
)

// Decision is the checker output used by API middleware and worker.
type Decision struct {
	Allowed    bool       `json:"allowed"`
	Mode       Mode       `json:"mode"`
	State      State      `json:"state"`
	Reason     string     `json:"reason,omitempty"`
	Source     string     `json:"source,omitempty"`
	CheckedAt  time.Time  `json:"checked_at"`
	ValidUntil *time.Time `json:"valid_until,omitempty"`
}

// Checker validates whether an app can execute licensed actions.
type Checker interface {
	Enabled() bool
	Mode() Mode
	Check(ctx context.Context, app *application.Application) (Decision, error)
}
