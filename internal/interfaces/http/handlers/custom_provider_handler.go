package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/resourcelink"
	"github.com/the-monkeys/freerangenotify/internal/domain/tenant"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/providers"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/dto"
	"github.com/the-monkeys/freerangenotify/internal/usecases"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// CustomProviderHandler handles CRUD for custom delivery providers (Phase 3).
type CustomProviderHandler struct {
	tenantService  tenant.Service
	service        usecases.ApplicationService
	membershipRepo auth.MembershipRepository
	logger         *zap.Logger
	linkRepo       resourcelink.Repository
}

func (h *CustomProviderHandler) SetLinkRepo(repo resourcelink.Repository) { h.linkRepo = repo }

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

	role, accessErr := channelAccess(c.Context(), app, userID, h.membershipRepo, h.tenantService, false)
	return app, role, accessErr
}

// registerCustomProviderRequest is the request body for registering a custom provider.
type registerCustomProviderRequest struct {
	Name             string            `json:"name" validate:"required,min=1,max=50"`
	Channel          string            `json:"channel" validate:"required,min=1,max=50"`
	Kind             string            `json:"kind,omitempty" validate:"omitempty,oneof=generic discord slack teams"`
	WebhookURL       string            `json:"webhook_url" validate:"required,url"`
	Headers          map[string]string `json:"headers,omitempty"`
	SignatureVersion string            `json:"signature_version,omitempty" validate:"omitempty,oneof=v1 v2"`
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
// @Router /v1/apps/{id}/providers [post]
// @Security BearerAuth
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

	// Phase 7: infer kind from URL host when omitted, then enforce host match
	// so users cannot paste a Slack URL into a provider tagged discord.
	kind := req.Kind
	if kind == "" {
		kind = inferProviderKind(req.WebhookURL)
	}
	if err := validateProviderURLSecurity(req.WebhookURL); err != nil {
		return errors.BadRequest(err.Error())
	}
	if err := validateProviderURLForKind(kind, req.WebhookURL); err != nil {
		return errors.BadRequest(err.Error())
	}
	signatureVersion := req.SignatureVersion
	if signatureVersion == "" {
		signatureVersion = "v1" // preserve existing receiver contract for new rows
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
		ProviderID:       uuid.New().String(),
		Name:             req.Name,
		Channel:          req.Channel,
		Kind:             kind,
		WebhookURL:       req.WebhookURL,
		Headers:          req.Headers,
		SigningKey:       signingKey,
		SignatureVersion: signatureVersion,
		Active:           true,
		CreatedAt:        time.Now().UTC().Format(time.RFC3339),
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
// @Router /v1/apps/{id}/providers [get]
// @Security BearerAuth
func (h *CustomProviderHandler) List(c *fiber.Ctx) error {
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

	providers := make([]application.CustomProviderConfig, len(app.Settings.CustomProviders))
	copy(providers, app.Settings.CustomProviders)
	origins := map[string]string{}

	// Merge linked providers from source apps.
	if h.linkRepo != nil {
		linkedAppIDs, linkErr := h.linkRepo.GetLinkedAppIDs(c.Context(), appID, resourcelink.TypeProvider)
		if linkErr != nil {
			return errors.Internal("Imported provider links unavailable", linkErr)
		}
		for _, srcAppID := range linkedAppIDs {
			linkedIDs, linkErr := h.linkRepo.GetLinkedResourceIDs(c.Context(), appID, srcAppID, resourcelink.TypeProvider)
			if linkErr != nil {
				return errors.Internal("Imported provider resources unavailable", linkErr)
			}
			allowed := map[string]bool{}
			for _, id := range linkedIDs {
				allowed[id] = true
			}
			srcApp, sErr := h.service.GetByID(c.Context(), srcAppID)
			if sErr != nil || srcApp == nil {
				return errors.BadRequest("Imported provider source unavailable")
			}
			for _, provider := range srcApp.Settings.CustomProviders {
				if allowed[provider.ProviderID] {
					providers = append(providers, provider)
					origins[provider.ProviderID] = srcAppID
				}
			}
		}
	}

	safe := dto.ToSafeApplicationSettings(application.Settings{CustomProviders: providers})
	providers = safe.CustomProviders
	type resourceView struct {
		application.CustomProviderConfig
		OriginAppID    string          `json:"origin_app_id"`
		Imported       bool            `json:"imported"`
		AllowedActions map[string]bool `json:"allowed_actions"`
	}
	views := make([]resourceView, 0, len(providers))
	manage := role == auth.RoleOwner || role == auth.RoleAdmin
	for _, provider := range providers {
		origin := origins[provider.ProviderID]
		imported := origin != ""
		if origin == "" {
			origin = appID
		}
		views = append(views, resourceView{provider, origin, imported, map[string]bool{"remove": manage, "test": manage && !imported, "rotate": manage && !imported}})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    views,
	})
}

// Test sends a synthetic notification to a registered provider endpoint.
// POST /v1/apps/:id/providers/:provider_id/test — requires admin or owner role
// @Summary Test
// @Description Requires an account JWT and permission for the selected application or tenant.
// @Tags CustomProvider
// @Produce json
// @Param id path string true "id"
// @Param provider_id path string true "provider_id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /v1/apps/{id}/providers/{provider_id}/test [post]
// @Security BearerAuth
func (h *CustomProviderHandler) Test(c *fiber.Ctx) error {
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
		return errors.Forbidden("admin or owner role required to test custom providers")
	}

	idx := findCustomProviderIndex(app.Settings.CustomProviders, providerID)
	if idx < 0 {
		return errors.BadRequest("custom provider not found")
	}
	cp := app.Settings.CustomProviders[idx]
	if !cp.Active {
		return errors.BadRequest("custom provider is inactive")
	}

	now := time.Now().UTC()
	notif := &notification.Notification{
		NotificationID: uuid.New().String(),
		AppID:          appID,
		UserID:         userID,
		Channel:        notification.Channel(cp.Channel),
		Priority:       notification.PriorityNormal,
		Status:         notification.StatusPending,
		Content: notification.Content{
			Title: "FreeRangeNotify Provider Test",
			Body:  "This is a test delivery from provider settings.",
		},
		Metadata: map[string]interface{}{
			"source":      "provider_test",
			"provider_id": cp.ProviderID,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	customProvider := providers.NewCustomProvider(
		cp.Name, cp.Channel, cp.Kind, cp.WebhookURL, cp.SigningKey, cp.SignatureVersion, cp.Headers, h.logger,
	)
	result, sendErr := customProvider.Send(c.Context(), notif, nil)
	if sendErr != nil {
		return errors.Internal("failed to send provider test notification", sendErr)
	}
	if result == nil || !result.Success {
		if result != nil && result.Error != nil {
			return errors.Internal("provider test delivery failed", result.Error)
		}
		return errors.Internal("provider test delivery failed", fmt.Errorf("provider returned unsuccessful result"))
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"provider_id":         cp.ProviderID,
			"provider_name":       cp.Name,
			"provider_message_id": result.ProviderMessageID,
			"delivery_time_ms":    result.DeliveryTime.Milliseconds(),
			"metadata":            result.Metadata,
		},
	})
}

// RotateSigningKey rotates a provider signing key and returns the new key once.
// POST /v1/apps/:id/providers/:provider_id/rotate — requires admin or owner role
// @Summary Rotate Signing Key
// @Description Requires an account JWT and permission for the selected application or tenant.
// @Tags CustomProvider
// @Produce json
// @Param id path string true "id"
// @Param provider_id path string true "provider_id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /v1/apps/{id}/providers/{provider_id}/rotate [post]
// @Security BearerAuth
func (h *CustomProviderHandler) RotateSigningKey(c *fiber.Ctx) error {
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
		return errors.Forbidden("admin or owner role required to rotate provider signing keys")
	}

	idx := findCustomProviderIndex(app.Settings.CustomProviders, providerID)
	if idx < 0 {
		return errors.BadRequest("custom provider not found")
	}

	signingKeyBytes := make([]byte, 32)
	if _, err := rand.Read(signingKeyBytes); err != nil {
		h.logger.Error("Failed to rotate signing key", zap.Error(err))
		return errors.BadRequest("failed to rotate signing key")
	}
	signingKey := hex.EncodeToString(signingKeyBytes)

	app.Settings.CustomProviders[idx].SigningKey = signingKey
	if err := h.service.UpdateSettings(c.Context(), appID, app.Settings); err != nil {
		return err
	}

	h.logger.Info("Custom provider signing key rotated",
		zap.String("app_id", appID),
		zap.String("provider_id", providerID))

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"provider_id": providerID,
			"signing_key": signingKey,
		},
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
// @Router /v1/apps/{id}/providers/{provider_id} [delete]
// @Security BearerAuth
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
		// Provider not in this app's settings — check if it's a linked (imported) provider.
		if h.linkRepo != nil {
			exists, _ := h.linkRepo.Exists(c.Context(), appID, resourcelink.TypeProvider, providerID)
			if exists {
				if unlinkErr := h.linkRepo.DeleteByTargetAndResource(c.Context(), appID, resourcelink.TypeProvider, providerID); unlinkErr != nil {
					h.logger.Error("Failed to unlink imported provider",
						zap.String("provider_id", providerID), zap.String("app_id", appID), zap.Error(unlinkErr))
					return errors.Internal("failed to unlink provider", unlinkErr)
				}
				h.logger.Info("Unlinked imported provider from target app",
					zap.String("provider_id", providerID), zap.String("app_id", appID))
				return c.JSON(fiber.Map{
					"success": true,
					"message": "linked provider removed from this application",
				})
			}
		}
		return errors.BadRequest("custom provider not found")
	}

	// Before removing, check if other apps have imported this provider.
	// If so, transfer the config to the first consumer instead of destroying it.
	if h.linkRepo != nil {
		consumers, _ := h.linkRepo.ListBySourceAndResource(c.Context(), appID, resourcelink.TypeProvider, providerID)
		if len(consumers) > 0 {
			newOwner := consumers[0].TargetAppID
			// Copy provider config to the first consumer's app settings.
			consumerApp, cErr := h.service.GetByID(c.Context(), newOwner)
			if cErr == nil && consumerApp != nil {
				// Find the removed provider config.
				var transferredConfig application.CustomProviderConfig
				for _, cp := range app.Settings.CustomProviders {
					if cp.ProviderID == providerID {
						transferredConfig = cp
						break
					}
				}
				consumerApp.Settings.CustomProviders = append(consumerApp.Settings.CustomProviders, transferredConfig)
				if upErr := h.service.UpdateSettings(c.Context(), newOwner, consumerApp.Settings); upErr != nil {
					h.logger.Error("Failed to transfer provider to consumer",
						zap.String("provider_id", providerID), zap.String("new_owner", newOwner), zap.Error(upErr))
					return errors.Internal("failed to transfer provider ownership", upErr)
				}
			}
			// Remove from source app settings.
			app.Settings.CustomProviders = filtered
			if err := h.service.UpdateSettings(c.Context(), appID, app.Settings); err != nil {
				return err
			}
			// First consumer now owns it — remove their link record.
			_ = h.linkRepo.Delete(c.Context(), consumers[0].LinkID)
			// Re-point remaining consumer links to the new owner.
			for _, link := range consumers[1:] {
				link.SourceAppID = newOwner
				_ = h.linkRepo.UpdateLink(c.Context(), link)
			}
			h.logger.Info("Transferred provider to consumer app",
				zap.String("provider_id", providerID), zap.String("from", appID), zap.String("to", newOwner))
			return c.JSON(fiber.Map{
				"success": true,
				"message": "custom provider removed (transferred to consumer app)",
			})
		}
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

func findCustomProviderIndex(providers []application.CustomProviderConfig, providerID string) int {
	for i := range providers {
		if providers[i].ProviderID == providerID {
			return i
		}
	}
	return -1
}

func (h *CustomProviderHandler) SetTenantService(service tenant.Service) { h.tenantService = service }
