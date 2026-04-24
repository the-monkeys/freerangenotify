package handlers

import (
	"encoding/json"
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/idempotency"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/dto"
	"github.com/the-monkeys/freerangenotify/internal/usecases"
	pkgerrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"github.com/the-monkeys/freerangenotify/pkg/validator"
	"go.uber.org/zap"
)

// QuickSendHandler handles the simplified POST /v1/quick-send endpoint.
type QuickSendHandler struct {
	service   *usecases.QuickSendService
	validator *validator.Validator
	logger    *zap.Logger
	idemp     *idempotency.Store
}

// NewQuickSendHandler creates a new QuickSendHandler.
func NewQuickSendHandler(service *usecases.QuickSendService, v *validator.Validator, logger *zap.Logger) *QuickSendHandler {
	return &QuickSendHandler{service: service, validator: v, logger: logger}
}

// SetIdempotencyStore injects the idempotency store for Idempotency-Key support.
func (h *QuickSendHandler) SetIdempotencyStore(store *idempotency.Store) {
	h.idemp = store
}

// Send handles POST /v1/quick-send
// @Summary Quick send a notification
// @Description Send a notification using a simplified one-step payload (auto-creates user if needed)
// @Tags Quick Send
// @Accept json
// @Produce json
// @Param body body dto.QuickSendRequest true "Quick send request"
// @Success 202 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /v1/quick-send [post]
func (h *QuickSendHandler) Send(c *fiber.Ctx) error {
	appID, ok := c.Locals("app_id").(string)
	if !ok || appID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "application not authenticated"})
	}

	// Idempotency: return cached response if key present and we've seen it before
	if h.idemp != nil {
		key := idempotency.GetIdempotencyKey(c)
		if key != "" {
			cached, err := h.idemp.Get(c.Context(), appID, key)
			if err == nil && cached != nil {
				return c.Status(cached.Status).Send(cached.Body)
			}
		}
	}

	var req dto.QuickSendRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	if err := h.validator.Validate(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if envID, ok := c.Locals("environment_id").(string); ok {
		req.EnvironmentID = envID
	}

	result, err := h.service.Send(c.Context(), appID, &req)
	if err != nil {
		// Known business errors — log as Warn, not Error
		if errors.Is(err, notification.ErrDNDEnabled) || errors.Is(err, notification.ErrQuietHours) ||
			errors.Is(err, notification.ErrRateLimitExceeded) || errors.Is(err, notification.ErrTemplateNotFound) ||
			notification.IsValidationError(err) {
			h.logger.Warn("Quick-send rejected", zap.String("app_id", appID), zap.Error(err))
		} else {
			h.logger.Error("Quick-send failed", zap.String("app_id", appID), zap.Error(err))
		}

		// Validation errors → 400
		if notification.IsValidationError(err) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
		}

		// DND / Quiet hours → 403 Forbidden
		if errors.Is(err, notification.ErrDNDEnabled) || errors.Is(err, notification.ErrQuietHours) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": err.Error()})
		}

		// Rate limit → 429
		if errors.Is(err, notification.ErrRateLimitExceeded) {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": err.Error()})
		}

		// Template / user not found → 404
		if errors.Is(err, notification.ErrTemplateNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
		}

		// Typed AppError (e.g. NotFound from user resolution)
		if appErr, ok := err.(*pkgerrors.AppError); ok {
			return c.Status(appErr.GetHTTPStatus()).JSON(fiber.Map{
				"success": false,
				"error": fiber.Map{
					"code":    appErr.Code,
					"message": appErr.Message,
				},
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Idempotency: cache success response for replay
	if h.idemp != nil {
		key := idempotency.GetIdempotencyKey(c)
		if key != "" {
			bodyBytes, _ := json.Marshal(result)
			_ = h.idemp.Set(c.Context(), appID, key, fiber.StatusAccepted, bodyBytes)
		}
	}

	return c.Status(fiber.StatusAccepted).JSON(result)
}
