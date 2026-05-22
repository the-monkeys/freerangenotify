package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/the-monkeys/freerangenotify/internal/config"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"go.uber.org/zap"
)

const (
	freePlanName    = "free"
	freePlanCredits = int64(500)
)

// backfillCreditsCmd is a one-shot migration that puts every existing
// subscription on the Free credit plan: plan="free", credits=500, status=active,
// period=now+1y, billing_model="credits". After this, customers can purchase
// higher-tier credit bundles through the normal upgrade flow.
//
// Idempotent: subscriptions already in the canonical Free shape are skipped.
var backfillCreditsCmd = &cobra.Command{
	Use:   "backfill-credits",
	Short: "One-time migration: put every subscription on the Free credit plan (500 credits)",
	Long: `Normalizes every subscription to the Free credit plan:

  - plan = "free"
  - credits_total = 500, credits_remaining = 500
  - status = "active"
  - current_period_start = now, current_period_end = now + 1 year
  - credits_expire_at = current_period_end
  - metadata.billing_model = "credits"

Stale "pending_checkout_*" metadata is cleared. After this migration runs,
existing customers see the same Free shape as new signups; higher tiers are
purchased on top of this baseline.

Idempotent: subscriptions already in the canonical Free shape are skipped.
Use --dry-run to preview without writing.`,
	Run: func(cmd *cobra.Command, args []string) {
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		cfg, err := config.Load()
		if err != nil {
			log.Fatalf("Failed to load configuration: %v", err)
		}

		logger, _ := zap.NewProduction()
		defer logger.Sync()

		logger.Info("Starting Free-plan backfill",
			zap.Bool("dry_run", dryRun),
			zap.Strings("es_urls", cfg.Database.URLs))

		esClient, err := database.NewElasticsearchClient(cfg, logger)
		if err != nil {
			log.Fatalf("Failed to connect to Elasticsearch: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		if _, err := esClient.Health(ctx); err != nil {
			log.Fatalf("Elasticsearch health check failed: %v", err)
		}

		subRepo := repository.NewSubscriptionRepository(esClient.Client, logger)

		subs, err := subRepo.List(ctx, license.SubscriptionFilter{Limit: 10000})
		if err != nil {
			log.Fatalf("Failed to list subscriptions: %v", err)
		}

		now := time.Now().UTC()
		expiry := now.AddDate(1, 0, 0)

		var updated, skippedNoop, failed int

		for _, sub := range subs {
			if isCanonicalFree(sub, now) {
				skippedNoop++
				fmt.Printf("· %s (tenant=%s) — already canonical Free, skipping\n",
					sub.ID, sub.TenantID)
				continue
			}

			oldPlan := sub.Plan
			oldStatus := string(sub.Status)
			oldCredits := sub.CreditsTotal

			sub.Plan = freePlanName
			sub.Status = license.SubscriptionStatusActive
			sub.CreditsTotal = freePlanCredits
			sub.CreditsRemaining = freePlanCredits
			sub.CreditsReserved = 0
			sub.CreditsExpireAt = &expiry
			sub.CurrentPeriodStart = now
			sub.CurrentPeriodEnd = expiry

			if sub.Metadata == nil {
				sub.Metadata = make(map[string]interface{})
			}
			sub.Metadata["billing_model"] = billing.BillingModelCredits
			sub.Metadata["migrated_to_free_at"] = now.Format(time.RFC3339)
			sub.Metadata["migrated_from_plan"] = oldPlan
			sub.Metadata["migrated_from_status"] = oldStatus
			sub.Metadata["migrated_from_credits"] = oldCredits
			for k := range sub.Metadata {
				if strings.HasPrefix(k, "pending_checkout_") {
					delete(sub.Metadata, k)
				}
			}

			if dryRun {
				fmt.Printf("→ %s (tenant=%s) — would migrate %s/%s/%d → free/active/500 (DRY RUN)\n",
					sub.ID, sub.TenantID, oldPlan, oldStatus, oldCredits)
				updated++
				continue
			}

			if err := subRepo.Update(ctx, sub); err != nil {
				failed++
				fmt.Printf("✗ %s (tenant=%s) — update failed: %v\n",
					sub.ID, sub.TenantID, err)
				continue
			}
			updated++
			fmt.Printf("✓ %s (tenant=%s) — migrated %s/%s/%d → free/active/500\n",
				sub.ID, sub.TenantID, oldPlan, oldStatus, oldCredits)
		}

		fmt.Printf("\nBackfill complete: %d migrated, %d already canonical, %d failed (total: %d)\n",
			updated, skippedNoop, failed, len(subs))

		if failed > 0 {
			os.Exit(1)
		}
	},
}

// isCanonicalFree returns true if the subscription is already in the exact
// shape this migration would produce — making the run a no-op.
func isCanonicalFree(sub *license.Subscription, now time.Time) bool {
	if sub == nil {
		return false
	}
	if billing.NormalizePlanName(sub.Plan) != freePlanName {
		return false
	}
	if sub.Status != license.SubscriptionStatusActive {
		return false
	}
	if sub.CreditsTotal != freePlanCredits || sub.CreditsRemaining > freePlanCredits {
		return false
	}
	if billing.BillingModel(sub) != billing.BillingModelCredits {
		return false
	}
	if sub.CurrentPeriodEnd.Before(now) {
		return false
	}
	return true
}

func init() {
	backfillCreditsCmd.Flags().Bool("dry-run", false, "Preview changes without writing to Elasticsearch")
	rootCmd.AddCommand(backfillCreditsCmd)
}
