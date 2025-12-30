package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/dto"
	"github.com/the-monkeys/freerangenotify/internal/usecases"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"github.com/the-monkeys/freerangenotify/pkg/validator"
	"go.uber.org/zap"
)

// UserHandler handles user-related HTTP requests
type UserHandler struct {
	service   usecases.UserService
	validator *validator.Validator
	logger    *zap.Logger
}

// NewUserHandler creates a new UserHandler
func NewUserHandler(service usecases.UserService, v *validator.Validator, logger *zap.Logger) *UserHandler {
	return &UserHandler{
		service:   service,
		validator: v,
		logger:    logger,
	}
}

// Create handles POST /v1/users
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
		Email:      req.Email,
		Phone:      req.Phone,
		Timezone:   req.Timezone,
		Language:   req.Language,
		WebhookURL: req.WebhookURL,
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
func (h *UserHandler) GetByID(c *fiber.Ctx) error {
	userID := c.Params("id")
	if userID == "" {
		return errors.BadRequest("user_id is required")
	}

	u, err := h.service.GetByID(c.Context(), userID)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    dto.ToUserResponse(u),
	})
}

// Update handles PUT /v1/users/:id
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

	// Get existing user
	u, err := h.service.GetByID(c.Context(), userID)
	if err != nil {
		return err
	}

	// Update fields
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
func (h *UserHandler) Delete(c *fiber.Ctx) error {
	userID := c.Params("id")
	if userID == "" {
		return errors.BadRequest("user_id is required")
	}

	if err := h.service.Delete(c.Context(), userID); err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "User deleted successfully",
	})
}

// List handles GET /v1/users
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
		AppID:    appID,
		Email:    c.Query("email"),
		Timezone: c.Query("timezone"),
		Language: c.Query("language"),
		Limit:    pageSize,
		Offset:   offset,
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
func (h *UserHandler) AddDevice(c *fiber.Ctx) error {
	userID := c.Params("id")
	if userID == "" {
		return errors.BadRequest("user_id is required")
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
func (h *UserHandler) RemoveDevice(c *fiber.Ctx) error {
	userID := c.Params("id")
	deviceID := c.Params("device_id")

	if userID == "" || deviceID == "" {
		return errors.BadRequest("user_id and device_id are required")
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
func (h *UserHandler) GetDevices(c *fiber.Ctx) error {
	userID := c.Params("id")
	if userID == "" {
		return errors.BadRequest("user_id is required")
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
func (h *UserHandler) UpdatePreferences(c *fiber.Ctx) error {
	userID := c.Params("id")
	if userID == "" {
		return errors.BadRequest("user_id is required")
	}

	var req dto.UpdatePreferencesRequest
	if err := c.BodyParser(&req); err != nil {
		return errors.BadRequest("Invalid request body")
	}

	preferences := user.Preferences{
		EmailEnabled: req.EmailEnabled,
		PushEnabled:  req.PushEnabled,
		SMSEnabled:   req.SMSEnabled,
		DND:          req.DND,
		Categories:   req.Categories,
		DailyLimit:   req.DailyLimit,
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
func (h *UserHandler) GetPreferences(c *fiber.Ctx) error {
	userID := c.Params("id")
	if userID == "" {
		return errors.BadRequest("user_id is required")
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
