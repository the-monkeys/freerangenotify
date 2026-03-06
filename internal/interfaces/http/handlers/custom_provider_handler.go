package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/usecases"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// CustomProviderHandler handles CRUD for custom delivery providers (Phase 3).
type CustomProviderHandler struct {
	service usecases.ApplicationService
	logger  *zap.Logger
}

// NewCustomProviderHandler creates a new CustomProviderHandler.
func NewCustomProviderHandler(service usecases.ApplicationService, logger *zap.Logger) *CustomProviderHandler {
	return &CustomProviderHandler{
		service: service,
		logger:  logger,
	}
}

// registerCustomProviderRequest is the request body for registering a custom provider.
type registerCustomProviderRequest struct {
	Name       string            `json:"name" validate:"required,min=1,max=50"`
	Channel    string            `json:"channel" validate:"required,min=1,max=50"`
	WebhookURL string            `json:"webhook_url" validate:"required,url"`
	Headers    map[string]string `json:"headers,omitempty"`
}

// Register registers a new custom provider for an application.
// POST /v1/apps/:id/providers
func (h *CustomProviderHandler) Register(c *fiber.Ctx) error {
	appID := c.Params("id")
	if appID == "" {
		return errors.BadRequest("app ID is required")
	}

	// Verify ownership via JWT
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return errors.Unauthorized("authentication required")
	}

	app, err := h.service.GetByID(c.Context(), appID)
	if err != nil {
		return err
	}
	if app.AdminUserID != userID {
		return errors.Forbidden("only the app owner can manage custom providers")
	}

	var req registerCustomProviderRequest
	if err := c.BodyParser(&req); err != nil {
		return errors.BadRequest("invalid request body")
	}

	// Check for duplicate channel name
	for _, cp := range app.Settings.CustomProviders {
		if cp.Channel == req.Channel && cp.Active {
			return errors.BadRequest("a custom provider for channel '" + req.Channel + "' already exists")
		}
	}

	// Generate signing key (32 bytes = 64 hex chars)
	signingKeyBytes := make([]byte, 32)
	if _, err := rand.Read(signingKeyBytes); err != nil {
		h.logger.Error("Failed to generate signing key", zap.Error(err))
		return errors.BadRequest("failed to generate signing key")
	}
	signingKey := hex.EncodeToString(signingKeyBytes)

	provider := application.CustomProviderConfig{
		ProviderID: uuid.New().String(),
		Name:       req.Name,
		Channel:    req.Channel,
		WebhookURL: req.WebhookURL,
		Headers:    req.Headers,
		SigningKey: signingKey,
		Active:     true,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
	}

	app.Settings.CustomProviders = append(app.Settings.CustomProviders, provider)

	if err := h.service.UpdateSettings(c.Context(), appID, app.Settings); err != nil {
		return err
	}

	h.logger.Info("Custom provider registered",
		zap.String("app_id", appID),
		zap.String("provider_id", provider.ProviderID),
		zap.String("channel", provider.Channel))

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    provider,
	})
}

// List returns all custom providers for an application.
// GET /v1/apps/:id/providers
func (h *CustomProviderHandler) List(c *fiber.Ctx) error {
	appID := c.Params("id")
	if appID == "" {
		return errors.BadRequest("app ID is required")
	}

	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return errors.Unauthorized("authentication required")
	}

	app, err := h.service.GetByID(c.Context(), appID)
	if err != nil {
		return err
	}
	if app.AdminUserID != userID {
		return errors.Forbidden("only the app owner can view custom providers")
	}

	// Redact signing keys for security — show only last 4 characters
	providers := make([]application.CustomProviderConfig, len(app.Settings.CustomProviders))
	copy(providers, app.Settings.CustomProviders)
	for i := range providers {
		if len(providers[i].SigningKey) > 4 {
			providers[i].SigningKey = "****" + providers[i].SigningKey[len(providers[i].SigningKey)-4:]
		}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    providers,
	})
}

// Remove removes a custom provider from an application.
// DELETE /v1/apps/:id/providers/:provider_id
func (h *CustomProviderHandler) Remove(c *fiber.Ctx) error {
	appID := c.Params("id")
	providerID := c.Params("provider_id")
	if appID == "" || providerID == "" {
		return errors.BadRequest("app ID and provider ID are required")
	}

	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return errors.Unauthorized("authentication required")
	}

	app, err := h.service.GetByID(c.Context(), appID)
	if err != nil {
		return err
	}
	if app.AdminUserID != userID {
		return errors.Forbidden("only the app owner can remove custom providers")
	}

	found := false
	filtered := make([]application.CustomProviderConfig, 0, len(app.Settings.CustomProviders))
	for _, cp := range app.Settings.CustomProviders {
		if cp.ProviderID == providerID {
			found = true
			continue
		}
		filtered = append(filtered, cp)
	}

	if !found {
		return errors.BadRequest("custom provider not found")
	}

	app.Settings.CustomProviders = filtered

	if err := h.service.UpdateSettings(c.Context(), appID, app.Settings); err != nil {
		return err
	}

	h.logger.Info("Custom provider removed",
		zap.String("app_id", appID),
		zap.String("provider_id", providerID))

	return c.JSON(fiber.Map{
		"success": true,
		"message": "custom provider removed",
	})
}
