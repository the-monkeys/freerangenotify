package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/domain/workflow"
	"github.com/the-monkeys/freerangenotify/internal/usecases"
	pkgerrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// InboundWebhookRequest is the request body for POST /v1/webhooks/inbound
type InboundWebhookRequest struct {
	Event   string                 `json:"event" validate:"required"`
	UserID  string                 `json:"user_id" validate:"required"`
	Payload map[string]interface{} `json:"payload"`
}

// InboundWebhookHandler handles inbound webhooks from external systems
type InboundWebhookHandler struct {
	appService    usecases.ApplicationService
	userService   usecases.UserService
	workflowSvc   workflow.Service
	logger        *zap.Logger
}

// NewInboundWebhookHandler creates a new inbound webhook handler
func NewInboundWebhookHandler(
	appService usecases.ApplicationService,
	userService usecases.UserService,
	workflowSvc workflow.Service,
	logger *zap.Logger,
) *InboundWebhookHandler {
	return &InboundWebhookHandler{
		appService:  appService,
		userService: userService,
		workflowSvc: workflowSvc,
		logger:     logger,
	}
}

// Receive handles POST /v1/webhooks/inbound
func (h *InboundWebhookHandler) Receive(c *fiber.Ctx) error {
	appID, ok := c.Locals("app_id").(string)
	if !ok || appID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "missing app context"})
	}

	rawBody := c.Body()
	if len(rawBody) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "request body required"})
	}

	// Get app settings for inbound webhook config
	app, err := h.appService.GetByID(c.Context(), appID)
	if err != nil || app == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to load application"})
	}

	cfg := app.Settings.InboundWebhookConfig
	if cfg == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "inbound webhooks not configured. Set inbound_webhook_config in app settings.",
		})
	}

	// Verify HMAC if secret is configured
	if cfg.Secret != "" {
		sigHeader := c.Get("X-Webhook-Signature")
		if sigHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "X-Webhook-Signature header required when webhook secret is configured",
			})
		}
		if !h.verifyHMAC(rawBody, cfg.Secret, sigHeader) {
			h.logger.Warn("Invalid webhook signature", zap.String("app_id", appID))
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid signature"})
		}
	}

	var req InboundWebhookRequest
	if err := json.Unmarshal(rawBody, &req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid JSON"})
	}
	if req.Event == "" || req.UserID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "event and user_id are required",
		})
	}

	// Map event to trigger_id
	triggerID, ok := cfg.EventMapping[req.Event]
	if !ok || triggerID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "event '" + req.Event + "' is not mapped to a workflow. Configure event_mapping in inbound_webhook_config.",
		})
	}

	// Resolve user: try user_id (direct), then external_id, then email
	resolvedUser, err := h.resolveUser(c.Context(), appID, req.UserID)
	if err != nil {
		if pkgerrors.IsNotFound(err) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "user not found: " + req.UserID + " (tried as user_id, external_id, and email)",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	payload := req.Payload
	if payload == nil {
		payload = make(map[string]interface{})
	}

	// Trigger the workflow
	_, triggerErr := h.workflowSvc.Trigger(c.Context(), appID, &workflow.TriggerRequest{
		TriggerID: triggerID,
		UserID:    resolvedUser.UserID,
		Payload:   payload,
	})
	if triggerErr != nil {
		if pkgerrors.IsNotFound(triggerErr) {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "workflow trigger_id '" + triggerID + "' not found",
			})
		}
		h.logger.Error("Failed to trigger workflow from webhook",
			zap.String("app_id", appID),
			zap.String("event", req.Event),
			zap.String("trigger_id", triggerID),
			zap.Error(triggerErr),
		)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": triggerErr.Error()})
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"success": true,
		"message": "webhook processed",
	})
}

func (h *InboundWebhookHandler) verifyHMAC(body []byte, secret, sigHeader string) bool {
	// Support "sha256=<hex>" or bare hex
	expectedMAC := hmac.New(sha256.New, []byte(secret))
	expectedMAC.Write(body)
	expectedHex := hex.EncodeToString(expectedMAC.Sum(nil))

	provided := strings.TrimSpace(sigHeader)
	if strings.HasPrefix(strings.ToLower(provided), "sha256=") {
		provided = provided[6:]
	}
	return hmac.Equal([]byte(provided), []byte(expectedHex))
}

func (h *InboundWebhookHandler) resolveUser(ctx context.Context, appID, identifier string) (*user.User, error) {
	// 1. Try as internal user_id (GetByID)
	u, err := h.userService.GetByID(ctx, identifier)
	if err == nil && u != nil && u.AppID == appID {
		return u, nil
	}

	// 2. Try as external_id
	u, err = h.userService.GetByExternalID(ctx, appID, identifier)
	if err == nil && u != nil {
		return u, nil
	}

	// 3. Try as email
	u, err = h.userService.GetByEmail(ctx, appID, identifier)
	if err == nil && u != nil {
		return u, nil
	}

	return nil, pkgerrors.NotFound("User", identifier)
}
