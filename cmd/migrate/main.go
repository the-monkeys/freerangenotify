package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/the-monkeys/freerangenotify/internal/config"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"go.uber.org/zap"
)

// allIndices lists every index the system uses.
// The IndexManager.CreateIndices() creates these via index_templates.go mappings.
var allIndices = []string{
	// Core
	"applications",
	"users",
	"notifications",
	"templates",
	"analytics",
	"auth_users",
	"password_reset_tokens",
	"refresh_tokens",
	// Phase 1
	"workflows",
	"workflow_executions",
	"digest_rules",
	// Phase 2
	"topics",
	"topic_subscriptions",
	"audit_logs",
	"app_memberships",
	// Phase 6
	"environments",
	// Billing indices (credit ledger, rate cards, runtime). Credit wallet fields live on `subscriptions`.
	// Renamed from frn_* — existing clusters: reindex or recreate then merge data before deleting old indices.
	"credit_ledger",
	"billing_rate_cards",
	"billing_runtime",
	// WhatsApp rich templates (Phase 0 of WHATSAPP_RICH_INTERACTIVE_PLAN.md)
	"whatsapp_rich_templates",
}

// bizBillingIndices are created only when features.biz_billing_enabled=true.
var bizBillingIndices = []string{
	"frn_biz_products",
	"frn_biz_plans",
	"frn_biz_plan_addons",
	"frn_biz_configs",
	"frn_biz_subscriptions",
	"frn_biz_invoices",
	"frn_biz_payments",
	"frn_biz_estimates",
	"frn_biz_coupons",
	"frn_biz_credit_notes",
	"frn_biz_retainers",
	"frn_biz_contracts",
	"frn_biz_usage_meters",
	"frn_biz_usage_events",
	"frn_biz_expenses",
	"frn_biz_revenue_schedules",
	"frn_biz_connectors",
	"frn_biz_portal_tokens",
}

// indicesFor returns the full index list for the given configuration.
func indicesFor(cfg *config.Config) []string {
	indices := allIndices
	if cfg.Features.BizBillingEnabled {
		indices = append(append([]string{}, allIndices...), bizBillingIndices...)
	}
	return indices
}

var rootCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Database migration tool for FreeRangeNotify",
	Long:  `A CLI tool for managing database migrations and index setup for FreeRangeNotify service.`,
}

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Run pending migrations",
	Long:  `Run all pending database migrations and create necessary Elasticsearch indices.`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			log.Fatalf("Failed to load configuration: %v", err)
		}

		logger, _ := zap.NewProduction()
		defer logger.Sync()

		logger.Info("Running migrations",
			zap.String("app", cfg.App.Name),
			zap.String("env", cfg.App.Environment),
			zap.Strings("es_urls", cfg.Database.URLs))

		// Connect to Elasticsearch
		esClient, err := database.NewElasticsearchClient(cfg, logger)
		if err != nil {
			log.Fatalf("Failed to connect to Elasticsearch: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		// Verify connectivity
		if _, err := esClient.Health(ctx); err != nil {
			log.Fatalf("Elasticsearch health check failed: %v", err)
		}

		// Create all indices via IndexManager
		indexManager := database.NewIndexManager(esClient, logger, cfg.Features.BizBillingEnabled)
		operations, err := indexManager.CreateIndices(ctx)
		if err != nil {
			log.Fatalf("Failed to create indices: %v", err)
		}

		created, existed, failed := 0, 0, 0
		for _, op := range operations {
			if !op.Success {
				failed++
				fmt.Printf("✗ %s — %s\n", op.IndexName, op.Message)
			} else if op.Message == "index already exists" {
				existed++
				fmt.Printf("· %s — already exists\n", op.IndexName)
			} else {
				created++
				fmt.Printf("✓ %s — created\n", op.IndexName)
			}
		}

		fmt.Printf("\nMigration complete: %d created, %d already existed, %d failed (total: %d)\n",
			created, existed, failed, len(operations))

		if failed > 0 {
			os.Exit(1)
		}
	},
}

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Rollback migrations",
	Long:  `Rollback database migrations and remove all indices. USE WITH EXTREME CAUTION — this deletes all data.`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			log.Fatalf("Failed to load configuration: %v", err)
		}

		logger, _ := zap.NewProduction()
		defer logger.Sync()

		logger.Info("Rolling back migrations",
			zap.String("app", cfg.App.Name))

		esClient, err := database.NewElasticsearchClient(cfg, logger)
		if err != nil {
			log.Fatalf("Failed to connect to Elasticsearch: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		indexManager := database.NewIndexManager(esClient, logger, cfg.Features.BizBillingEnabled)

		// Delete indices in reverse order
		indices := indicesFor(cfg)
		for i := len(indices) - 1; i >= 0; i-- {
			idx := indices[i]
			exists, err := indexManager.IndexExists(ctx, idx)
			if err != nil {
				fmt.Printf("✗ %s — error checking: %v\n", idx, err)
				continue
			}
			if !exists {
				fmt.Printf("· %s — does not exist\n", idx)
				continue
			}
			if err := indexManager.DeleteIndex(ctx, idx); err != nil {
				fmt.Printf("✗ %s — failed to delete: %v\n", idx, err)
			} else {
				fmt.Printf("✓ %s — deleted\n", idx)
			}
		}

		fmt.Println("\nRollback completed.")
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show migration status",
	Long:  `Display the current status of all Elasticsearch indices.`,
	Run: func(cmd *cobra.Command, args []string) {
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

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		indexManager := database.NewIndexManager(esClient, logger, cfg.Features.BizBillingEnabled)

		fmt.Println("Migration Status:")
		missing := 0
		for _, idx := range indicesFor(cfg) {
			exists, err := indexManager.IndexExists(ctx, idx)
			if err != nil {
				fmt.Printf("  ✗ %-25s error: %v\n", idx, err)
				missing++
			} else if exists {
				fmt.Printf("  ✓ %-25s exists\n", idx)
			} else {
				fmt.Printf("  ✗ %-25s MISSING\n", idx)
				missing++
			}
		}

		if missing > 0 {
			fmt.Printf("\n%d index(es) missing. Run 'migrate up' to create them.\n", missing)
			os.Exit(1)
		} else {
			fmt.Println("\nAll indices present.")
		}
	},
}

func init() {
	rootCmd.AddCommand(upCmd)
	rootCmd.AddCommand(downCmd)
	rootCmd.AddCommand(statusCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
