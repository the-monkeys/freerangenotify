package main

import (
	"context"
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
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/gofiber/swagger"
	"github.com/the-monkeys/freerangenotify/internal/config"
	"github.com/the-monkeys/freerangenotify/internal/container"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/middleware"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/routes"
	"go.uber.org/zap"

	_ "github.com/the-monkeys/freerangenotify/docs" // Swagger docs
)

// @title FreeRangeNotify API
// @version 0.1.2-alpha
// @description High-performance notification service with multi-channel delivery support
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.email support@freerangenotify.com

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:8080
// @BasePath /
// @schemes http https

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and your API key

func main() {
	// Initialize logger
	zapLogger, _ := zap.NewDevelopment()
	defer zapLogger.Sync()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}

	// Initialize dependency injection container
	c, err := container.NewContainer(cfg, zapLogger)
	if err != nil {
		log.Fatalf("Failed to create container: %v", err)
	}
	defer c.Close()

	// Initialize database system
	ctx := context.Background()
	if err := c.DatabaseManager.Initialize(ctx); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	zapLogger.Info("Database system initialized successfully")

	// Create Fiber app with configuration
	app := fiber.New(fiber.Config{
		AppName:      cfg.App.Name,
		ServerHeader: fmt.Sprintf("%s/%s", cfg.App.Name, cfg.App.Version),
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(cfg.Server.IdleTimeout) * time.Second,
		Prefork:      cfg.App.Environment == "production", // Enable prefork in production
		ErrorHandler: middleware.ErrorHandler(zapLogger),  // Custom error handler
	})

	// Add global middleware
	app.Use(recover.New())   // Panic recovery
	app.Use(requestid.New()) // Request ID
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${latency} ${method} ${path}\n",
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins:  "*",
		AllowHeaders:  "Origin, Content-Type, Accept, Authorization, X-API-Key",
		AllowMethods:  "GET,POST,PUT,DELETE,OPTIONS,PATCH",
		ExposeHeaders: "Content-Length",
		MaxAge:        86400,
	}))

	// Health check endpoint
	app.Get("/health", func(ctx *fiber.Ctx) error {
		// Check database health
		dbStatus := "ok"
		if err := c.DatabaseManager.Health(ctx.Context()); err != nil {
			dbStatus = "unhealthy"
			zapLogger.Error("Database health check failed", zap.Error(err))
		}

		return ctx.JSON(fiber.Map{
			"status":      "ok",
			"service":     cfg.App.Name,
			"version":     cfg.App.Version,
			"environment": cfg.App.Environment,
			"database":    dbStatus,
			"timestamp":   time.Now().UTC().Format(time.RFC3339),
		})
	})

	// Database stats endpoint
	app.Get("/database/stats", func(ctx *fiber.Ctx) error {
		stats, err := c.DatabaseManager.Stats(ctx.Context())
		if err != nil {
			return ctx.Status(500).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		return ctx.JSON(stats)
	})

	// Version endpoint
	app.Get("/version", func(ctx *fiber.Ctx) error {
		return ctx.JSON(fiber.Map{
			"name":        cfg.App.Name,
			"version":     cfg.App.Version,
			"environment": cfg.App.Environment,
			"build_time":  time.Now().UTC().Format(time.RFC3339),
		})
	})

	// Basic API routes
	api := app.Group("/api/v1")
	api.Get("/status", func(ctx *fiber.Ctx) error {
		return ctx.JSON(fiber.Map{
			"message": "FreeRangeNotify API is running",
			"status":  "operational",
		})
	})

	// Setup v1 routes
	routes.SetupRoutes(app, c)

	// Swagger documentation endpoint
	app.Get("/swagger/*", swagger.New(swagger.Config{
		URL:         "/openapi/swagger.yaml",
		DeepLinking: true,
	}))

	// Serve OpenAPI spec
	app.Static("/openapi", "./docs/openapi")

	// Prometheus metrics endpoint
	app.Get("/metrics", func(ctx *fiber.Ctx) error {
		return ctx.JSON(fiber.Map{
			"status": "metrics endpoint - TODO: implement prometheus metrics",
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

	// Cleanup database connections
	if err := c.DatabaseManager.Close(); err != nil {
		zapLogger.Error("Error closing database connections", zap.Error(err))
	}

	// Gracefully shutdown the server with timeout
	if err := app.ShutdownWithTimeout(30 * time.Second); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
