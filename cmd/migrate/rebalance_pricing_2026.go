package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/spf13/cobra"
	"github.com/the-monkeys/freerangenotify/internal/config"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/billingrepo"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"go.uber.org/zap"
)

// rebalance2026Version is the fixed version identifier for the 2026 pricing
// rebalance. Using a fixed string (not a timestamp) makes the migration
// re-runnable: a second invocation upserts the same document.
const rebalance2026Version = "rebalance-2026"

// rebalancePricing2026Cmd seeds the new India 2026 rate card into Elasticsearch
// and activates it. Idempotent: if the active card is already this version,
// the command is a no-op.
//
// See documents/PRICING_REBALANCE_PLAN.md for the full rationale.
var rebalancePricing2026Cmd = &cobra.Command{
	Use:   "rebalance-pricing-2026",
	Short: "Seed the 2026 India pricing rebalance into the active rate card",
	Long: `Creates rate-card version "rebalance-2026" with:

  Channel credit costs:
    inapp/webhook/sse = 1   (~₹0.01/msg)
    email             = 5   (~₹0.05/msg, carrier ₹0.021)
    whatsapp          = 80  (~₹0.80/msg, carrier ₹0.56)
    sms               = 800 (~₹8.00/msg, carrier ₹6.93)

  Overage (paisa/msg, future invoice path):
    inapp=2, email=8, whatsapp=120, sms=1200

  Plan bundles (validity 365 days):
    free    ₹0       1,500 credits
    starter ₹499     35,000 credits
    pro     ₹1,499   120,000 credits
    growth  ₹4,999   450,000 credits
    scale   ₹14,999  1,600,000 credits

  The "lite" plan is dropped (not present in the new card's Plans map).

Existing tenants keep their wallet balance; new prices apply to new
reservations. Idempotent — safe to re-run.`,
	Run: func(cmd *cobra.Command, args []string) {
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		cfg, err := config.Load()
		if err != nil {
			log.Fatalf("Failed to load configuration: %v", err)
		}

		logger, _ := zap.NewProduction()
		defer logger.Sync()

		esClient, err := database.NewElasticsearchClient(cfg, logger)
		if err != nil {
			log.Fatalf("Failed to connect to Elasticsearch: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		if _, err := esClient.Health(ctx); err != nil {
			log.Fatalf("Elasticsearch health check failed: %v", err)
		}

		repo := billingrepo.NewESRateCardRepo(esClient.Client, logger)

		active, err := repo.GetActive(ctx)
		if err != nil {
			log.Fatalf("Failed to read active rate card: %v", err)
		}
		if active != nil && active.Version == rebalance2026Version {
			fmt.Printf("· active rate card is already %q — nothing to do\n", rebalance2026Version)
			return
		}

		card := buildRebalance2026Card()

		if dryRun {
			fmt.Printf("→ would create + activate rate card version %q (DRY RUN)\n", card.Version)
			fmt.Printf("  channels: %v\n", card.ChannelCreditCost)
			fmt.Printf("  overage:  %v\n", card.OveragePerMessage)
			fmt.Printf("  plans:    %d (free, starter, pro, growth, scale)\n", len(card.Plans))
			return
		}

		if err := repo.CreateVersion(ctx, card); err != nil {
			log.Fatalf("Failed to create rate card version: %v", err)
		}
		if err := repo.SetActiveVersion(ctx, card.Version); err != nil {
			log.Fatalf("Failed to activate rate card version: %v", err)
		}
		fmt.Printf("✓ created and activated rate card version %q\n", card.Version)

		publishRateCardInvalidation(ctx, cfg, card.Version, logger)
		fmt.Println("✓ broadcast invalidation on billing:ratecard:updated — running pods will refresh within seconds")
	},
}

// buildRebalance2026Card constructs the new rate card. Values come directly
// from documents/PRICING_REBALANCE_PLAN.md §4.
func buildRebalance2026Card() *billing.RateCard {
	now := time.Now().UTC()
	return &billing.RateCard{
		Version:        rebalance2026Version,
		Active:         false, // SetActiveVersion will flip the runtime pointer
		CreditValueINR: 0.01,
		ChannelCreditCost: map[string]int64{
			"inapp":    1,
			"webhook":  1,
			"sse":      1,
			"email":    5,
			"whatsapp": 80,
			"sms":      800,
		},
		OveragePerMessage: map[string]int64{
			"inapp":    2,    // ₹0.02
			"email":    8,    // ₹0.08
			"whatsapp": 120,  // ₹1.20
			"sms":      1200, // ₹12.00
		},
		Plans: map[string]billing.PlanBundle{
			"free": {
				ID: "free", Name: "Free", AmountPaisa: 0, Currency: "INR",
				CreditsIncluded: 1500, ValidityDays: 365, Active: true, DisplayOrder: 10,
			},
			"starter": {
				ID: "starter", Name: "Starter", AmountPaisa: 49900, Currency: "INR",
				CreditsIncluded: 35000, ValidityDays: 365, Active: true, DisplayOrder: 20,
			},
			"pro": {
				ID: "pro", Name: "Pro", AmountPaisa: 149900, Currency: "INR",
				CreditsIncluded: 120000, ValidityDays: 365, Active: true, DisplayOrder: 30,
			},
			"growth": {
				ID: "growth", Name: "Growth", AmountPaisa: 499900, Currency: "INR",
				CreditsIncluded: 450000, ValidityDays: 365, Active: true, DisplayOrder: 40,
			},
			"scale": {
				ID: "scale", Name: "Scale", AmountPaisa: 1499900, Currency: "INR",
				CreditsIncluded: 1600000, ValidityDays: 365, Active: true, DisplayOrder: 50,
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// publishRateCardInvalidation publishes a Redis pub/sub message on the
// well-known channel so all running pods refresh their cached card without
// waiting for the 45s periodic tick. Failure is non-fatal — pods will pick
// up the change on the next refresh anyway.
func publishRateCardInvalidation(ctx context.Context, cfg *config.Config, version string, logger *zap.Logger) {
	if cfg == nil {
		return
	}
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer client.Close()

	if err := client.Publish(ctx, "billing:ratecard:updated", version).Err(); err != nil {
		fmt.Fprintf(os.Stderr, "! pub/sub broadcast failed (non-fatal): %v\n", err)
		logger.Warn("rebalance-pricing-2026: pub/sub broadcast failed", zap.Error(err))
	}
}

func init() {
	rebalancePricing2026Cmd.Flags().Bool("dry-run", false, "Print what would change without writing to Elasticsearch")
	rootCmd.AddCommand(rebalancePricing2026Cmd)
}
