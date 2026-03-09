package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"github.com/the-monkeys/freerangenotify/internal/usecases"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// CustomProviderHandler handles CRUD for custom delivery providers (Phase 3).
type CustomProviderHandler struct {
	service        usecases.ApplicationService
	membershipRepo auth.MembershipRepository
	logger         *zap.Logger
}

// NewCustomProviderHandler creates a new CustomProviderHandler.
func NewCustomProviderHandler(service usecases.ApplicationService, membershipRepo auth.MembershipRepository, logger *zap.Logger) *CustomProviderHandler {
	return &CustomProviderHandler{
		service:        service,
		membershipRepo: membershipRepo,
		logger:         logger,
	}
}

// authorizeProviderAccess checks ownership or team membership and returns the
// app and the caller's resolved role.
func (h *CustomProviderHandler) authorizeProviderAccess(c *fiber.Ctx, appID, userID string) (*application.Application, auth.Role, error) {
	app, err := h.service.GetByID(c.Context(), appID)
	if err != nil {
		return nil, "", err
	}

	if app.AdminUserID == userID {
		return app, auth.RoleOwner, nil
	}

	if h.membershipRepo != nil {
		membership, mErr := h.membershipRepo.GetByAppAndUser(c.Context(), appID, userID)
		if mErr == nil && membership != nil {
			return app, membership.Role, nil
		}
	}

	return nil, "", errors.Forbidden("You do not have access to this application")
}

// registerCustomProviderRequest is the request body for registering a custom provider.
type registerCustomProviderRequest struct {
	Name       string            `json:"name" validate:"required,min=1,max=50"`
	Channel    string            `json:"channel" validate:"required,min=1,max=50"`
	WebhookURL string            `json:"webhook_url" validate:"required,url"`
	Headers    map[string]string `json:"headers,omitempty"`
}

// Register registers a new custom provider for an application.
// POST /v1/apps/:id/providers — requires admin or owner role
// @Summary Register a custom provider
// @Description Register a new custom delivery provider for an application (admin or owner)
// @Tags Custom Providers
// @Accept json
// @Produce json
// @Param id path string true "Application ID"
// @Param body body registerCustomProviderRequest true "Custom provider registration request"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Security BearerAuth
// @Router /v1/apps/{id}/providers [post]
func (h *CustomProviderHandler) Register(c *fiber.Ctx) error {
	appID := c.Params("id")
	if appID == "" {
		return errors.BadRequest("app ID is required")
	}

	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return errors.Unauthorized("authentication required")
	}

	app, role, err := h.authorizeProviderAccess(c, appID, userID)
	if err != nil {
		return err
	}
	if role != auth.RoleOwner && role != auth.RoleAdmin {
		return errors.Forbidden("admin or owner role required to manage custom providers")
	}

	var req registerCustomProviderRequest
	if err := c.BodyParser(&req); err != nil {
		return errors.BadRequest("invalid request body")
	}

	for _, cp := range app.Settings.CustomProviders {
		if cp.Name == req.Name && cp.Active {
			return errors.BadRequest("a custom provider named '" + req.Name + "' already exists")
		}
	}

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
		SigningKey:  signingKey,
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
// GET /v1/apps/:id/providers — any team member can view
// @Summary List custom providers
// @Description List all registered custom delivery providers for an application
// @Tags Custom Providers
// @Produce json
// @Param id path string true "Application ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Security BearerAuth
// @Router /v1/apps/{id}/providers [get]
func (h *CustomProviderHandler) List(c *fiber.Ctx) error {
	appID := c.Params("id")
	if appID == "" {
		return errors.BadRequest("app ID is required")
	}

	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return errors.Unauthorized("authentication required")
	}

	app, _, err := h.authorizeProviderAccess(c, appID, userID)
	if err != nil {
		return err
	}

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
// DELETE /v1/apps/:id/providers/:provider_id — requires admin or owner role
// @Summary Remove a custom provider
// @Description Remove a custom delivery provider from an application (admin or owner)
// @Tags Custom Providers
// @Produce json
// @Param id path string true "Application ID"
// @Param provider_id path string true "Provider ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Security BearerAuth
// @Router /v1/apps/{id}/providers/{provider_id} [delete]
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

	app, role, err := h.authorizeProviderAccess(c, appID, userID)
	if err != nil {
		return err
	}
	if role != auth.RoleOwner && role != auth.RoleAdmin {
		return errors.Forbidden("admin or owner role required to remove custom providers")
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
