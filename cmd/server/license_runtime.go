package main

import (
	"context"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/config"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
	gozap "go.uber.org/zap"
)

// startSelfHostedLicenseRuntime starts a background periodic revalidation loop.
// It runs independently of request traffic so remote verification still happens on idle systems.
func startSelfHostedLicenseRuntime(ctx context.Context, cfg *config.Config, checker license.Checker, logger *gozap.Logger) {
	if cfg == nil || checker == nil {
		return
	}
	if !cfg.Licensing.Enabled || checker.Mode() != license.ModeSelfHosted {
		return
	}

	intervalSeconds := cfg.Licensing.SelfHosted.VerifyIntervalSeconds
	if intervalSeconds <= 0 {
		intervalSeconds = 300
	}
	interval := time.Duration(intervalSeconds) * time.Second

	type cacheResetter interface {
		ClearCache()
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		logger.Info("Started self-hosted license runtime revalidation loop", gozap.Duration("interval", interval))
		for {
			select {
			case <-ctx.Done():
				logger.Info("Stopped self-hosted license runtime revalidation loop")
				return
			case <-ticker.C:
				if resetter, ok := checker.(cacheResetter); ok {
					resetter.ClearCache()
				}

				decision, err := checker.Check(ctx, nil)
				if err != nil {
					logger.Error("Periodic self-hosted license revalidation failed", gozap.Error(err))
					continue
				}
				if !decision.Allowed {
					logger.Warn("Periodic self-hosted license revalidation denied",
						gozap.String("state", string(decision.State)),
						gozap.String("reason", decision.Reason),
					)
				}
			}
		}
	}()
}
