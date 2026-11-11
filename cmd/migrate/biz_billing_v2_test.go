package main

import (
	"context"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"testing"
)

type interruptedUpgrade struct{ prepared, activated int }

func (f *interruptedUpgrade) Prepare(context.Context, string, services.MigrationOptions) (*services.MigrationReport, error) {
	f.prepared++
	return nil, bizbilling.ErrActiveLegacyWriter
}
func (f *interruptedUpgrade) Activate(context.Context, string, string) error {
	f.activated++
	return nil
}
func (f *interruptedUpgrade) Lifecycle(context.Context, string) (bizbilling.LifecycleState, bool, error) {
	return bizbilling.LifecycleState{AppID: "app", LifecycleVersion: 1, Status: "preparing", MigrationReportID: "review"}, false, nil
}
func (f *interruptedUpgrade) ReviewedReport(context.Context, string, string) (*services.MigrationReport, error) {
	return &services.MigrationReport{ID: "review", AppID: "app", PrimaryCurrency: "INR", WritersUpgraded: true}, nil
}
func TestBizMigrationCLIResumesMatchingCutover(t *testing.T) {
	f := &interruptedUpgrade{}
	report, err := applyReviewedBillingUpgrade(context.Background(), f, "app", "INR", "review", true)
	require.NoError(t, err)
	require.Equal(t, "activated", report.Checkpoint)
	require.Zero(t, f.prepared)
	require.Equal(t, 1, f.activated)
}
