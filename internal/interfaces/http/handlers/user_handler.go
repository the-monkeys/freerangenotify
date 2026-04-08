package handlers

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/resourcelink"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/dto"
	"github.com/the-monkeys/freerangenotify/internal/usecases"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"github.com/the-monkeys/freerangenotify/pkg/utils"
	"github.com/the-monkeys/freerangenotify/pkg/validator"
	"go.uber.org/zap"
)

// UserHandler handles user-related HTTP requests
type UserHandler struct {
	service   usecases.UserService
	validator *validator.Validator
	logger    *zap.Logger
	linkRepo  resourcelink.Repository
	userRepo  user.Repository
}

// NewUserHandler creates a new UserHandler
func NewUserHandler(service usecases.UserService, v *validator.Validator, logger *zap.Logger) *UserHandler {
	return &UserHandler{
		service:   service,
		validator: v,
		logger:    logger,
	}
}

func (h *UserHandler) SetLinkRepo(repo resourcelink.Repository) { h.linkRepo = repo }
func (h *UserHandler) SetUserRepo(repo user.Repository)         { h.userRepo = repo }

// getAppID extracts the authenticated app_id from Fiber context.
func (h *UserHandler) getAppID(c *fiber.Ctx) (string, error) {
	appID, ok := c.Locals("app_id").(string)
	if !ok || appID == "" {
		return "", errors.Unauthorized("Application not authenticated")
	}
	return appID, nil
}

// verifyUserOwnership fetches the user and ensures it belongs to the caller's app.
// Supports both internal UUID and external_id resolution.
func (h *UserHandler) verifyUserOwnership(c *fiber.Ctx, userID string) (*user.User, error) {
	appID, err := h.getAppID(c)
	if err != nil {
		return nil, err
	}

	// Try direct lookup first (UUID). If not a valid UUID, resolve via external_id.
	if _, parseErr := uuid.Parse(userID); parseErr != nil {
		h.logger.Debug("verifyUserOwnership: resolving external_id", zap.String("external_id", userID), zap.String("app_id", appID))
		u, lookupErr := h.service.GetByExternalID(c.Context(), appID, userID)
		if lookupErr != nil {
			h.logger.Debug("verifyUserOwnership: external_id resolution failed", zap.String("external_id", userID), zap.Error(lookupErr))
			return nil, errors.NotFound("user", userID)
		}
		h.logger.Debug("verifyUserOwnership: external_id resolved", zap.String("external_id", userID), zap.String("user_id", u.UserID))
		return u, nil
	}

	u, err := h.service.GetByID(c.Context(), userID)
	if err != nil {
		return nil, err
	}
	if u.AppID != appID {
		// Fallback: check if this user is linked (imported) to the requesting app
		if h.linkRepo != nil {
			if linked, _ := h.linkRepo.Exists(c.Context(), appID, resourcelink.TypeUser, userID); linked {
				return u, nil
			}
		}
		return nil, errors.NotFound("user", userID)
	}
	return u, nil
}

// Create handles POST /v1/users
// @Summary Create a new user
// @Description Create a new subscriber/user within an application
// @Tags Users
// @Accept json
// @Produce json
// @Param body body dto.CreateUserRequest true "User creation request"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /v1/users [post]
func (h *UserHandler) Create(c *fiber.Ctx) error {
	var req dto.CreateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return errors.BadRequest("Invalid request body")
	}

	if err := h.validator.Validate(req); err != nil {
		return errors.Validation("Validation failed", validator.FormatValidationErrors(err))
	}

	// Get app_id from context (set by auth middleware)
	appID, ok := c.Locals("app_id").(string)
	if !ok || appID == "" {
		return errors.Unauthorized("Application not authenticated")
	}

	u := &user.User{
		UserID:     req.UserID,
		AppID:      appID,
		ExternalID: req.ExternalID,
		FullName:   req.FullName,
		Email:      req.Email,
		Phone:      req.Phone,
		Timezone:   req.Timezone,
		Language:   req.Language,
		WebhookURL: req.WebhookURL,
	}

	if envID, ok := c.Locals("environment_id").(string); ok {
		u.EnvironmentID = envID
	}

	if req.Preferences != nil {
		u.Preferences = *req.Preferences
	}

	if err := h.service.Create(c.Context(), u); err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    dto.ToUserResponse(u),
	})
}

// GetByID handles GET /v1/users/:id
// @Summary Get a user by ID
// @Description Retrieve a single user by their ID
// @Tags Users
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /v1/users/{id} [get]
func (h *UserHandler) GetByID(c *fiber.Ctx) error {
	userID := c.Params("id")
	if userID == "" {
		return errors.BadRequest("user_id is required")
	}

	u, err := h.verifyUserOwnership(c, userID)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    dto.ToUserResponse(u),
	})
}

// GetByExternalID handles GET /v1/users/by-external-id/:external_id
// @Summary Get a user by external ID
// @Description Retrieve a single user by their external_id within the authenticated app
// @Tags Users
// @Produce json
// @Param external_id path string true "External ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /v1/users/by-external-id/{external_id} [get]
func (h *UserHandler) GetByExternalID(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return err
	}

	externalID := c.Params("external_id")
	if externalID == "" {
		return errors.BadRequest("external_id is required")
	}

	u, err := h.service.GetByExternalID(c.Context(), appID, externalID)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    dto.ToUserResponse(u),
	})
}

// Update handles PUT /v1/users/:id
// @Summary Update a user
// @Description Update an existing user's information
// @Tags Users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param body body dto.UpdateUserRequest true "User update request"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /v1/users/{id} [put]
func (h *UserHandler) Update(c *fiber.Ctx) error {
	userID := c.Params("id")
	if userID == "" {
		return errors.BadRequest("user_id is required")
	}

	var req dto.UpdateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return errors.BadRequest("Invalid request body")
	}

	if err := h.validator.Validate(req); err != nil {
		return errors.Validation("Validation failed", validator.FormatValidationErrors(err))
	}

	// Get existing user and verify ownership
	u, err := h.verifyUserOwnership(c, userID)
	if err != nil {
		return err
	}

	// Update fields
	if req.ExternalID != "" {
		u.ExternalID = req.ExternalID
	}
	if req.FullName != "" {
		u.FullName = req.FullName
	}
	if req.Email != "" {
		u.Email = req.Email
	}
	if req.Phone != "" {
		u.Phone = req.Phone
	}
	if req.Timezone != "" {
		u.Timezone = req.Timezone
	}
	if req.Language != "" {
		u.Language = req.Language
	}
	if req.WebhookURL != "" {
		u.WebhookURL = req.WebhookURL
	}
	if req.Preferences != nil {
		u.Preferences = *req.Preferences
	}

	if err := h.service.Update(c.Context(), u); err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    dto.ToUserResponse(u),
	})
}

// Delete handles DELETE /v1/users/:id
// @Summary Delete a user
// @Description Permanently remove a user
// @Tags Users
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /v1/users/{id} [delete]
func (h *UserHandler) Delete(c *fiber.Ctx) error {
	userID := c.Params("id")
	if userID == "" {
		return errors.BadRequest("user_id is required")
	}

	// Verify ownership before deleting; resolves external_id → internal UUID if needed.
	u, err := h.verifyUserOwnership(c, userID)
	if err == nil {
		// verifyUserOwnership succeeded — but if the user is not actually owned by
		// this app (returned via link fallback), treat it as a linked-user delete.
		appID, _ := h.getAppID(c)
		h.logger.Debug("Delete: ownership check",
			zap.String("param", userID),
			zap.String("resolved_user_id", u.UserID),
			zap.String("u.AppID", u.AppID),
			zap.String("ctx_appID", appID),
			zap.Bool("match", u.AppID == appID))
		if u.AppID != appID {
			err = errors.NotFound("user", userID)
		}
	}
	if err != nil {
		// If ownership check fails, the user may be linked (imported) from another app.
		if h.linkRepo != nil {
			appID, _ := h.getAppID(c)
			exists, _ := h.linkRepo.Exists(c.Context(), appID, resourcelink.TypeUser, userID)
			if exists {
				if unlinkErr := h.linkRepo.DeleteByTargetAndResource(c.Context(), appID, resourcelink.TypeUser, userID); unlinkErr != nil {
					h.logger.Error("Failed to unlink imported user",
						zap.String("user_id", userID), zap.String("app_id", appID), zap.Error(unlinkErr))
					return errors.Internal("failed to unlink user", unlinkErr)
				}
				h.logger.Info("Unlinked imported user from target app",
					zap.String("user_id", userID), zap.String("app_id", appID))
				return c.JSON(fiber.Map{
					"success": true,
					"message": "Linked user removed from this application",
				})
			}
			// User exists but no link — stale reference from coarse-grained listing.
			// Treat as already disassociated from this app.
			return c.JSON(fiber.Map{
				"success": true,
				"message": "User is not associated with this application",
			})
		}
		return err
	}

	// Before deleting, check if other apps have imported this resource.
	// If so, transfer ownership to the first consumer instead of destroying it.
	if h.linkRepo != nil && h.userRepo != nil {
		consumers, _ := h.linkRepo.ListBySourceAndResource(c.Context(), u.AppID, resourcelink.TypeUser, u.UserID)
		if len(consumers) > 0 {
			newOwner := consumers[0].TargetAppID
			u.AppID = newOwner
			u.UpdatedAt = time.Now()
			if err := h.userRepo.Update(c.Context(), u); err != nil {
				h.logger.Error("Failed to transfer user ownership",
					zap.String("user_id", u.UserID), zap.String("new_owner", newOwner), zap.Error(err))
				return errors.Internal("failed to transfer resource ownership", err)
			}
			// First consumer now owns it — remove their link record.
			_ = h.linkRepo.Delete(c.Context(), consumers[0].LinkID)
			// Re-point remaining consumer links to the new owner.
			for _, link := range consumers[1:] {
				link.SourceAppID = newOwner
				_ = h.linkRepo.UpdateLink(c.Context(), link)
			}
			h.logger.Info("Transferred user ownership to consumer app",
				zap.String("user_id", u.UserID), zap.String("from", c.Locals("app_id").(string)), zap.String("to", newOwner))
			return c.JSON(fiber.Map{
				"success": true,
				"message": "User removed from this application (transferred to consumer app)",
			})
		}
	}

	if err := h.service.Delete(c.Context(), u.UserID); err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "User deleted successfully",
	})
}

// List handles GET /v1/users
// @Summary List users
// @Description List users for the authenticated application with pagination and filtering
// @Tags Users
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param page_size query int false "Page size" default(20)
// @Param email query string false "Filter by email"
// @Param timezone query string false "Filter by timezone"
// @Param language query string false "Filter by language"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /v1/users [get]
func (h *UserHandler) List(c *fiber.Ctx) error {
	// Get app_id from context
	appID, ok := c.Locals("app_id").(string)
	if !ok || appID == "" {
		return errors.Unauthorized("Application not authenticated")
	}

	// Parse query parameters
	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize

	filter := user.UserFilter{
		AppID:      appID,
		ExternalID: c.Query("external_id"),
		Email:      c.Query("email"),
		Timezone:   c.Query("timezone"),
		Language:   c.Query("language"),
		Limit:      pageSize,
		Offset:     offset,
	}

	if envID, ok := c.Locals("environment_id").(string); ok {
		filter.EnvironmentID = envID
	}

	// Include linked users from other apps (cross-app resource linking)
	if h.linkRepo != nil {
		linkedIDs, _ := h.linkRepo.GetAllLinkedResourceIDs(c.Context(), appID, resourcelink.TypeUser)
		if len(linkedIDs) > 0 {
			filter.LinkedUserIDs = linkedIDs
		}
	}

	users, total, err := h.service.List(c.Context(), filter)
	if err != nil {
		return err
	}

	userResponses := make([]dto.UserResponse, len(users))
	for i, u := range users {
		userResponses[i] = dto.ToUserResponse(u)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": dto.ListUsersResponse{
			Users:      userResponses,
			TotalCount: total,
			Page:       page,
			PageSize:   pageSize,
		},
	})
}

// AddDevice handles POST /v1/users/:id/devices
// @Summary Add a device to a user
// @Description Register a push notification device token for a user
// @Tags Users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param body body dto.AddDeviceRequest true "Device registration request"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /v1/users/{id}/devices [post]
func (h *UserHandler) AddDevice(c *fiber.Ctx) error {
	userID := c.Params("id")
	if userID == "" {
		return errors.BadRequest("user_id is required")
	}

	if _, err := h.verifyUserOwnership(c, userID); err != nil {
		return err
	}

	var req dto.AddDeviceRequest
	if err := c.BodyParser(&req); err != nil {
		return errors.BadRequest("Invalid request body")
	}

	if err := h.validator.Validate(req); err != nil {
		return errors.Validation("Validation failed", validator.FormatValidationErrors(err))
	}

	device := user.Device{
		Platform: req.Platform,
		Token:    req.Token,
	}

	if err := h.service.AddDevice(c.Context(), userID, device); err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "Device added successfully",
	})
}

// RemoveDevice handles DELETE /v1/users/:id/devices/:device_id
// @Summary Remove a device from a user
// @Description Unregister a push notification device token
// @Tags Users
// @Produce json
// @Param id path string true "User ID"
// @Param device_id path string true "Device ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /v1/users/{id}/devices/{device_id} [delete]
func (h *UserHandler) RemoveDevice(c *fiber.Ctx) error {
	userID := c.Params("id")
	deviceID := c.Params("device_id")

	if userID == "" || deviceID == "" {
		return errors.BadRequest("user_id and device_id are required")
	}

	if _, err := h.verifyUserOwnership(c, userID); err != nil {
		return err
	}

	if err := h.service.RemoveDevice(c.Context(), userID, deviceID); err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Device removed successfully",
	})
}

// GetDevices handles GET /v1/users/:id/devices
// @Summary Get user devices
// @Description Retrieve all registered devices for a user
// @Tags Users
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /v1/users/{id}/devices [get]
func (h *UserHandler) GetDevices(c *fiber.Ctx) error {
	userID := c.Params("id")
	if userID == "" {
		return errors.BadRequest("user_id is required")
	}

	if _, err := h.verifyUserOwnership(c, userID); err != nil {
		return err
	}

	devices, err := h.service.GetDevices(c.Context(), userID)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    devices,
	})
}

// UpdatePreferences handles PUT /v1/users/:id/preferences
// @Summary Update user notification preferences
// @Description Update channel preferences, quiet hours, and DND settings for a user
// @Tags Users
// @Accept json
// @Produce json
// @Param id path string true "User ID"
// @Param body body dto.UpdatePreferencesRequest true "Preferences update request"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /v1/users/{id}/preferences [put]
func (h *UserHandler) UpdatePreferences(c *fiber.Ctx) error {
	userID := c.Params("id")
	if userID == "" {
		return errors.BadRequest("user_id is required")
	}

	if _, err := h.verifyUserOwnership(c, userID); err != nil {
		return err
	}

	var req dto.UpdatePreferencesRequest
	if err := c.BodyParser(&req); err != nil {
		return errors.BadRequest("Invalid request body")
	}

	preferences := user.Preferences{
		EmailEnabled:    req.EmailEnabled,
		PushEnabled:     req.PushEnabled,
		SMSEnabled:      req.SMSEnabled,
		SlackEnabled:    req.SlackEnabled,
		DiscordEnabled:  req.DiscordEnabled,
		WhatsAppEnabled: req.WhatsAppEnabled,
		DND:             req.DND,
		Categories:      req.Categories,
		DailyLimit:      req.DailyLimit,
	}

	if req.QuietHours != nil {
		preferences.QuietHours = *req.QuietHours
	}

	if err := h.service.UpdatePreferences(c.Context(), userID, preferences); err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "Preferences updated successfully",
	})
}

// GetPreferences handles GET /v1/users/:id/preferences
// @Summary Get user notification preferences
// @Description Retrieve notification preferences for a user
// @Tags Users
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /v1/users/{id}/preferences [get]
func (h *UserHandler) GetPreferences(c *fiber.Ctx) error {
	userID := c.Params("id")
	if userID == "" {
		return errors.BadRequest("user_id is required")
	}

	if _, err := h.verifyUserOwnership(c, userID); err != nil {
		return err
	}

	preferences, err := h.service.GetPreferences(c.Context(), userID)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    preferences,
	})
}

// BulkCreate handles POST /v1/users/bulk
// @Summary Bulk create users
// @Description Create multiple users in a single request
// @Tags Users
// @Accept json
// @Produce json
// @Param body body dto.BulkCreateUserRequest true "Bulk user creation request"
// @Success 201 {object} dto.BulkCreateUserResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /v1/users/bulk [post]
func (h *UserHandler) BulkCreate(c *fiber.Ctx) error {
	appID, ok := c.Locals("app_id").(string)
	if !ok || appID == "" {
		return errors.Unauthorized("Application not authenticated")
	}

	var req dto.BulkCreateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return errors.BadRequest("Invalid request body")
	}

	if err := h.validator.Validate(req); err != nil {
		return errors.Validation("Validation failed", validator.FormatValidationErrors(err))
	}

	var users []*user.User
	var updatedUsers []*user.User
	var bulkErrors []dto.BulkUserError
	skippedUsers := 0

	for i, ur := range req.Users {
		u := &user.User{
			UserID:     ur.UserID,
			AppID:      appID,
			ExternalID: ur.ExternalID,
			FullName:   ur.FullName,
			Email:      ur.Email,
			Phone:      ur.Phone,
			Timezone:   ur.Timezone,
			Language:   ur.Language,
			WebhookURL: ur.WebhookURL,
		}
		if ur.Preferences != nil {
			u.Preferences = *ur.Preferences
		}

		// Validate: require at least email or phone
		if u.Email == "" && u.Phone == "" {
			bulkErrors = append(bulkErrors, dto.BulkUserError{
				Index:   i,
				Email:   ur.Email,
				Message: "email or phone required",
			})
			continue
		}

		// Skip mode: if email exists, skip without error
		if req.SkipExisting && u.Email != "" {
			existing, _ := h.service.GetByEmail(c.Context(), appID, u.Email)
			if existing != nil {
				skippedUsers++
				continue
			}
		}

		// Upsert mode: if user with this email exists, update instead of failing
		if req.Upsert && !req.SkipExisting && u.Email != "" {
			existing, _ := h.service.GetByEmail(c.Context(), appID, u.Email)
			if existing != nil {
				if u.ExternalID != "" {
					existing.ExternalID = u.ExternalID
				}
				if u.FullName != "" {
					existing.FullName = u.FullName
				}
				if u.Phone != "" {
					existing.Phone = u.Phone
				}
				if u.Timezone != "" {
					existing.Timezone = u.Timezone
				}
				if u.Language != "" {
					existing.Language = u.Language
				}
				if u.WebhookURL != "" {
					existing.WebhookURL = u.WebhookURL
				}
				if err := h.service.Update(c.Context(), existing); err != nil {
					bulkErrors = append(bulkErrors, dto.BulkUserError{
						Index:   i,
						Email:   ur.Email,
						Message: "upsert update failed: " + err.Error(),
					})
				} else {
					updatedUsers = append(updatedUsers, existing)
				}
				continue
			}
		}

		users = append(users, u)
	}

	if len(users) > 0 {
		if err := h.service.BulkCreate(c.Context(), users); err != nil {
			h.logger.Error("Bulk user creation failed", zap.Error(err))
			return err
		}
	}

	return c.Status(fiber.StatusCreated).JSON(dto.BulkCreateUserResponse{
		Created: len(users),
		Updated: len(updatedUsers),
		Skipped: skippedUsers,
		Total:   len(req.Users),
		Errors:  bulkErrors,
	})
}

// ── Phase 5: Subscriber Hash ────────────────────────

// GetSubscriberHash handles GET /v1/users/:id/subscriber-hash
// @Summary Get subscriber hash
// @Description Generate an HMAC subscriber hash for SSE authentication
// @Tags Users
// @Produce json
// @Param id path string true "User ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /v1/users/{id}/subscriber-hash [get]
func (h *UserHandler) GetSubscriberHash(c *fiber.Ctx) error {
	userID := c.Params("id")
	if userID == "" {
		return errors.BadRequest("user_id is required")
	}

	appID, err := h.getAppID(c)
	if err != nil {
		return err
	}

	// Resolve external_id → internal UUID (same pattern as SSE token creation)
	if _, parseErr := uuid.Parse(userID); parseErr != nil {
		u, lookupErr := h.service.GetByExternalID(c.Context(), appID, userID)
		if lookupErr != nil {
			return errors.NotFound("user", userID)
		}
		userID = u.UserID
	} else {
		// Verify user belongs to this app
		if _, err := h.verifyUserOwnership(c, userID); err != nil {
			return err
		}
	}

	app, ok := c.Locals("app").(*application.Application)
	if !ok || app == nil {
		return errors.Unauthorized("Application not authenticated")
	}

	hash := utils.GenerateSubscriberHash(userID, app.APIKey)

	return c.JSON(fiber.Map{
		"user_id":         userID,
		"subscriber_hash": hash,
	})
}
