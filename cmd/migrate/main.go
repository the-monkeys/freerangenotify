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
		indexManager := database.NewIndexManager(esClient, logger)
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

		indexManager := database.NewIndexManager(esClient, logger)

		// Delete indices in reverse order
		for i := len(allIndices) - 1; i >= 0; i-- {
			idx := allIndices[i]
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

		indexManager := database.NewIndexManager(esClient, logger)

		fmt.Println("Migration Status:")
		missing := 0
		for _, idx := range allIndices {
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
