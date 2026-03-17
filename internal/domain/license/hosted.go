package license

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/application"
)

// HostedChecker validates app/tenant subscriptions for hosted deployments.
type HostedChecker struct {
	repo Repository
	opts HostedOptions

	mu    sync.RWMutex
	cache map[string]decisionCacheEntry
}

func NewHostedChecker(repo Repository, opts HostedOptions) (Checker, error) {
	if opts.CacheTTL <= 0 {
		opts.CacheTTL = 5 * time.Minute
	}
	if opts.GraceWindow < 0 {
		opts.GraceWindow = 0
	}
	if opts.FailMode == "" {
		opts.FailMode = FailModeClosed
	}

	return &HostedChecker{
		repo:  repo,
		opts:  opts,
		cache: make(map[string]decisionCacheEntry),
	}, nil
}

func (h *HostedChecker) Enabled() bool { return true }

func (h *HostedChecker) Mode() Mode { return ModeHosted }

func (h *HostedChecker) Check(ctx context.Context, app *application.Application) (Decision, error) {
	now := time.Now().UTC()
	if app == nil {
		return Decision{}, fmt.Errorf("application context is required")
	}

	key := cacheKeyForApp(app.AppID, app.TenantID)
	if cached, ok := h.getCache(key); ok {
		if now.Sub(cached.fetchedAt) <= h.opts.CacheTTL {
			d := cached.decision
			d.Source = "cache"
			d.CheckedAt = now
			return d, nil
		}
	}

	sub, err := h.repo.GetActiveSubscription(ctx, app.TenantID, app.AppID, now)
	if err != nil {
		if cached, ok := h.getCache(key); ok && now.Sub(cached.fetchedAt) <= h.opts.CacheTTL+h.opts.GraceWindow {
			d := cached.decision
			d.Source = "cache_stale"
			d.State = StateGrace
			d.Reason = "subscription_backend_unavailable"
			d.CheckedAt = now
			return d, nil
		}

		if h.opts.FailMode == FailModeOpen {
			return Decision{Allowed: true, Mode: ModeHosted, State: StateGrace, Reason: "subscription_backend_unavailable", Source: "fail_open", CheckedAt: now}, nil
		}

		return Decision{Allowed: false, Mode: ModeHosted, State: StateInvalid, Reason: "subscription_check_unavailable", Source: "fail_closed", CheckedAt: now}, nil
	}

	// Fall back to the app owner's personal subscription when the app's tenant (org)
	// has no subscription of its own — e.g. the trial provisioned at registration is
	// keyed to the user ID, not the org ID.
	if sub == nil && app.AdminUserID != "" && app.AdminUserID != app.TenantID {
		sub, err = h.repo.GetActiveSubscription(ctx, app.AdminUserID, "", now)
		if err != nil {
			sub = nil
		}
	}

	if sub == nil {
		d := Decision{Allowed: false, Mode: ModeHosted, State: StateUnlicensed, Reason: "subscription_required", Source: "live", CheckedAt: now}
		h.setCache(key, d, now)
		return d, nil
	}

	d := Decision{Allowed: true, Mode: ModeHosted, State: StateActive, Reason: "subscription_active", Source: "live", CheckedAt: now, ValidUntil: &sub.CurrentPeriodEnd}
	h.setCache(key, d, now)
	return d, nil
}

func (h *HostedChecker) getCache(key string) (decisionCacheEntry, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	entry, ok := h.cache[key]
	return entry, ok
}

func (h *HostedChecker) setCache(key string, decision Decision, fetchedAt time.Time) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cache[key] = decisionCacheEntry{decision: decision, fetchedAt: fetchedAt}
}

// ClearCache forces subsequent checks to re-fetch from repository.
func (h *HostedChecker) ClearCache() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.cache = make(map[string]decisionCacheEntry)
}
