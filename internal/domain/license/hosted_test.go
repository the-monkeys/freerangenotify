package license

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
)

type fakeSubscriptionRepo struct {
	activeSub  *Subscription
	err        error
	checkCalls int
}

func (f *fakeSubscriptionRepo) Create(_ context.Context, _ *Subscription) error { return nil }
func (f *fakeSubscriptionRepo) GetByID(_ context.Context, _ string) (*Subscription, error) {
	return nil, nil
}
func (f *fakeSubscriptionRepo) Update(_ context.Context, _ *Subscription) error { return nil }
func (f *fakeSubscriptionRepo) Delete(_ context.Context, _ string) error        { return nil }
func (f *fakeSubscriptionRepo) List(_ context.Context, _ SubscriptionFilter) ([]*Subscription, error) {
	return nil, nil
}

func (f *fakeSubscriptionRepo) GetActiveSubscription(_ context.Context, _ string, _ string, _ time.Time) (*Subscription, error) {
	f.checkCalls++
	if f.err != nil {
		return nil, f.err
	}
	return f.activeSub, nil
}

func TestHostedChecker_AllowsActiveSubscription(t *testing.T) {
	now := time.Now().UTC()
	repo := &fakeSubscriptionRepo{
		activeSub: &Subscription{
			ID:               "sub-1",
			Status:           SubscriptionStatusActive,
			CurrentPeriodEnd: now.Add(24 * time.Hour),
		},
	}

	checker, err := NewHostedChecker(repo, HostedOptions{CacheTTL: 10 * time.Minute, GraceWindow: 2 * time.Minute, FailMode: FailModeClosed})
	require.NoError(t, err)

	decision, err := checker.Check(context.Background(), &application.Application{AppID: "app-1", TenantID: "tenant-1"})
	require.NoError(t, err)
	assert.True(t, decision.Allowed)
	assert.Equal(t, ModeHosted, decision.Mode)
	assert.Equal(t, StateActive, decision.State)
	assert.Equal(t, "subscription_active", decision.Reason)
	assert.Equal(t, "live", decision.Source)
	assert.Equal(t, 1, repo.checkCalls)
}

func TestHostedChecker_BlocksWithoutSubscription(t *testing.T) {
	repo := &fakeSubscriptionRepo{}
	checker, err := NewHostedChecker(repo, HostedOptions{FailMode: FailModeClosed})
	require.NoError(t, err)

	decision, err := checker.Check(context.Background(), &application.Application{AppID: "app-1", TenantID: "tenant-1"})
	require.NoError(t, err)
	assert.False(t, decision.Allowed)
	assert.Equal(t, StateUnlicensed, decision.State)
	assert.Equal(t, "subscription_required", decision.Reason)
}

func TestHostedChecker_UsesCacheWithinTTL(t *testing.T) {
	now := time.Now().UTC()
	repo := &fakeSubscriptionRepo{
		activeSub: &Subscription{ID: "sub-1", Status: SubscriptionStatusActive, CurrentPeriodEnd: now.Add(48 * time.Hour)},
	}

	checker, err := NewHostedChecker(repo, HostedOptions{CacheTTL: 30 * time.Minute, GraceWindow: 2 * time.Minute, FailMode: FailModeClosed})
	require.NoError(t, err)

	app := &application.Application{AppID: "app-1", TenantID: "tenant-1"}
	first, err := checker.Check(context.Background(), app)
	require.NoError(t, err)
	require.True(t, first.Allowed)

	repo.err = errors.New("backend unavailable")
	second, err := checker.Check(context.Background(), app)
	require.NoError(t, err)
	assert.True(t, second.Allowed)
	assert.Equal(t, "cache", second.Source)
	assert.Equal(t, 1, repo.checkCalls)
}

func TestHostedChecker_FailOpenOnBackendError(t *testing.T) {
	repo := &fakeSubscriptionRepo{err: errors.New("backend unavailable")}
	checker, err := NewHostedChecker(repo, HostedOptions{CacheTTL: time.Millisecond, GraceWindow: 0, FailMode: FailModeOpen})
	require.NoError(t, err)

	decision, err := checker.Check(context.Background(), &application.Application{AppID: "app-1", TenantID: "tenant-1"})
	require.NoError(t, err)
	assert.True(t, decision.Allowed)
	assert.Equal(t, StateGrace, decision.State)
	assert.Equal(t, "fail_open", decision.Source)
}
