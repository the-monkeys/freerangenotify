package services

import (
	"context"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
)

// Long source reads must not let a healthy owner lose its lease. Renewal never
// changes the fencing generation, and cannot revive an expired or replaced head.
func (s *BizOperationService) keepFinancialClaim(ctx context.Context, claim bizbilling.StreamClaim) func() {
	return keepBillingLease(ctx, 5*time.Second, func(renewCtx context.Context) error { return s.streams.Renew(renewCtx, claim) })
}

func keepBillingLease(ctx context.Context, cadence time.Duration, renew func(context.Context) error) func() {
	leaseCtx, cancel := context.WithCancel(ctx)
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		ticker := time.NewTicker(cadence)
		defer ticker.Stop()
		for {
			select {
			case <-leaseCtx.Done():
				return
			case <-ticker.C:
				renewCtx, stop := context.WithTimeout(leaseCtx, 15*time.Second)
				err := renew(renewCtx)
				stop()
				if err != nil {
					return
				}
			}
		}
	}()
	return func() { cancel(); <-finished }
}
