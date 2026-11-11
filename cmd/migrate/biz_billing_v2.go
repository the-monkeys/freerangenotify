package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/the-monkeys/freerangenotify/internal/config"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"go.uber.org/zap"
)

func newBizBillingV2Command() *cobra.Command {
	var appID, currency, reportID, issueID, sourceHash, assignedCurrency, reviewer, reason string
	var apply, writersUpgraded bool
	var recoveryWriters []string
	var expectedWriterEpoch int64
	var writerDrained bool
	var timeout time.Duration
	cmd := &cobra.Command{Use: "biz-billing-v2", Short: "Review an application billing upgrade; defaults to a read-only inventory", Args: cobra.NoArgs}
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if appID == "" || currency == "" || timeout <= 0 {
			return fmt.Errorf("app-id, primary currency and a positive timeout are required")
		}
		if apply && reportID == "" {
			return fmt.Errorf("apply requires the reviewed report-id")
		}
		if apply && issueID == "" && len(recoveryWriters) == 0 && !writersUpgraded {
			return fmt.Errorf("confirm writers-upgraded only after older API and worker instances have been drained")
		}
		if issueID != "" && (!apply || sourceHash == "" || assignedCurrency == "" || reviewer == "" || reason == "") {
			return fmt.Errorf("a resolution requires apply, source-hash, assigned-currency, reviewer and reason")
		}
		if issueID != "" && len(recoveryWriters) > 0 {
			return fmt.Errorf("choose a currency resolution or writer recovery")
		}
		if len(recoveryWriters) > 0 && (!apply || expectedWriterEpoch < 0 || !writerDrained || reviewer == "" || reason == "") {
			return fmt.Errorf("writer recovery requires apply, report-id, expected-writer-epoch, writer-drained, reviewer and reason")
		}
		return nil
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		if !cfg.Features.BizBillingEnabled {
			return fmt.Errorf("business billing is disabled")
		}
		logger, _ := zap.NewProduction()
		defer logger.Sync()
		db, err := database.NewElasticsearchClient(cfg, logger)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
		defer cancel()
		gate := services.NewBizLifecycleGate(bizbillingrepo.NewLifecycleStateRepo(db.Client))
		operations := services.NewBizOperationService(db.Client, logger)
		operations.SetLifecycleGate(gate)
		service := services.NewBizMigrationService(db.Client, gate, operations, logger)
		service.SetNewCollectionEnabled(cfg.BizBilling.NewCollectionEnabled)
		if len(recoveryWriters) > 0 {
			id, err := service.RecoverWriter(ctx, appID, reportID, bizbilling.WriterPermitResolution{WriterIDs: recoveryWriters, ExpectedEpoch: expectedWriterEpoch, PrimaryCurrency: currency, ActorID: reviewer, Reason: reason, ConfirmedDrained: writerDrained})
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]string{"writer_review_id": id, "status": "resolved"})
		}
		if issueID != "" {
			if err = service.Resolve(ctx, appID, reportID, services.LegacyResolution{IssueID: issueID, ExpectedSourceHash: sourceHash, Currency: assignedCurrency, ActorID: reviewer, Reason: reason}); err != nil {
				return err
			}
		}
		var report *services.MigrationReport
		if apply && issueID == "" {
			report, err = applyReviewedBillingUpgrade(ctx, service, appID, currency, reportID, writersUpgraded)
		} else {
			report, err = service.Prepare(ctx, appID, services.MigrationOptions{DryRun: true, ExpectedReportID: reportID, PrimaryCurrency: currency})
		}
		if err != nil {
			return err
		}
		return json.NewEncoder(cmd.OutOrStdout()).Encode(report)
	}
	f := cmd.Flags()
	f.StringVar(&appID, "app-id", "", "Application to inspect")
	f.StringVar(&currency, "currency", "", "Primary billing currency; never assigns old credit automatically")
	f.StringVar(&reportID, "report-id", "", "Exact inventory report reviewed by the operator")
	f.BoolVar(&apply, "apply", false, "Persist reviewed changes and activate, or record the selected resolution")
	f.BoolVar(&writersUpgraded, "writers-upgraded", false, "Confirm all older money-writing instances have been drained")
	f.DurationVar(&timeout, "timeout", 15*time.Minute, "Maximum review or cutover duration")
	f.StringSliceVar(&recoveryWriters, "recover-writer", nil, "Abandoned writer IDs; all selected origins and storage requests must be drained")
	f.Int64Var(&expectedWriterEpoch, "expected-writer-epoch", -1, "Exact epoch from the writer permit inventory")
	f.BoolVar(&writerDrained, "writer-drained", false, "Confirm selected origins stopped and their outstanding storage requests drained")
	f.StringVar(&issueID, "resolve-issue", "", "Review issue to resolve instead of activating")
	f.StringVar(&sourceHash, "source-hash", "", "Expected source revision hash from the issue")
	f.StringVar(&assignedCurrency, "assigned-currency", "", "Currency supported by the reviewed original records")
	f.StringVar(&reviewer, "reviewer", "", "Accountable reviewer identifier")
	f.StringVar(&reason, "reason", "", "Reason and evidence for the resolution")
	return cmd
}
func init() { rootCmd.AddCommand(newBizBillingV2Command()) }

type billingUpgradeApplier interface {
	Prepare(context.Context, string, services.MigrationOptions) (*services.MigrationReport, error)
	Activate(context.Context, string, string) error
	Lifecycle(context.Context, string) (bizbilling.LifecycleState, bool, error)
	ReviewedReport(context.Context, string, string) (*services.MigrationReport, error)
}

func applyReviewedBillingUpgrade(ctx context.Context, service billingUpgradeApplier, appID, currency, reportID string, writers bool) (*services.MigrationReport, error) {
	state, _, err := service.Lifecycle(ctx, appID)
	if err != nil {
		return nil, err
	}
	var report *services.MigrationReport
	matchingSaved := state.MigrationReportID == reportID && (state.Status == "preparing" || (state.LifecycleVersion == 2 && state.Status == "active"))
	if matchingSaved {
		report, err = service.ReviewedReport(ctx, appID, reportID)
	} else {
		report, err = service.Prepare(ctx, appID, services.MigrationOptions{PrimaryCurrency: currency, ExpectedReportID: reportID, WritersUpgraded: writers})
	}
	if err != nil {
		return nil, err
	}
	if report.PrimaryCurrency != currency {
		return nil, bizbilling.ErrCurrencyMismatch
	}
	if err = service.Activate(ctx, appID, report.ID); err != nil {
		return nil, err
	}
	report.Checkpoint = "activated"
	return report, nil
}
