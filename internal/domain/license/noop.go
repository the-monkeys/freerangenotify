package license

import (
	"context"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/application"
)

// NoopChecker always allows actions and is used when licensing is disabled.
type NoopChecker struct{}

func NewNoopChecker() Checker {
	return &NoopChecker{}
}

func (n *NoopChecker) Enabled() bool {
	return false
}

func (n *NoopChecker) Mode() Mode {
	return ModeHosted
}

func (n *NoopChecker) Check(_ context.Context, _ *application.Application) (Decision, error) {
	return Decision{
		Allowed:   true,
		Mode:      ModeHosted,
		State:     StateActive,
		Reason:    "licensing_disabled",
		Source:    "noop",
		CheckedAt: time.Now().UTC(),
	}, nil
}
