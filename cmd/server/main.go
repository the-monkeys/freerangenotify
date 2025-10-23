package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/the-monkeys/freerangenotify/internal/config"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}

	// Create Fiber app with configuration
	app := fiber.New(fiber.Config{
		AppName:      cfg.App.Name,
		ServerHeader: fmt.Sprintf("%s/%s", cfg.App.Name, cfg.App.Version),
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(cfg.Server.IdleTimeout) * time.Second,
		Prefork:      cfg.App.Environment == "production", // Enable prefork in production
	})

	// Add middleware
	app.Use(recover.New()) // Panic recovery
	app.Use(logger.New())  // Request logging
	app.Use(cors.New(cors.Config{
		AllowOrigins: "http://localhost:3000,http://localhost:8080",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization, X-API-Key",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
	}))

	// Health check endpoint
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":      "ok",
			"service":     cfg.App.Name,
			"version":     cfg.App.Version,
			"environment": cfg.App.Environment,
			"timestamp":   time.Now().UTC().Format(time.RFC3339),
		})
	})

	// Version endpoint
	app.Get("/version", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"name":        cfg.App.Name,
			"version":     cfg.App.Version,
			"environment": cfg.App.Environment,
			"build_time":  time.Now().UTC().Format(time.RFC3339),
		})
	})

	// Basic API routes
	api := app.Group("/api/v1")
	api.Get("/status", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "FreeRangeNotify API is running",
			"status":  "operational",
		})
	})

	// Create address
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)

	// Start server in a goroutine
	go func() {
		log.Printf("Starting %s server on %s", cfg.App.Name, addr)
		if err := app.Listen(addr); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Gracefully shutdown the server with timeout
	if err := app.ShutdownWithTimeout(30 * time.Second); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
