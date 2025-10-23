package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/config"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Starting %s worker", cfg.App.Name)
	log.Printf("Environment: %s", cfg.App.Environment)
	log.Printf("Workers: %d", cfg.Queue.Workers)
	log.Printf("Concurrency: %d", cfg.Queue.Concurrency)

	// TODO: Initialize queue connections
	// TODO: Initialize providers
	// TODO: Start worker pools

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start worker simulation (replace with actual worker logic)
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				log.Println("Worker heartbeat - processing notifications...")
			}
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down worker...")
	cancel()

	// Give workers time to finish current jobs
	time.Sleep(5 * time.Second)
	log.Println("Worker shutdown complete")
}
