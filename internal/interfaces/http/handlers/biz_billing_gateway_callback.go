package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"go.uber.org/zap"
)

type BizBillingGatewayCallbackHandler struct {
	service *services.BizGatewayEventService
	logger  *zap.Logger
}

func NewBizBillingGatewayCallbackHandler(service *services.BizGatewayEventService, logger *zap.Logger) *BizBillingGatewayCallbackHandler {
	return &BizBillingGatewayCallbackHandler{service: service, logger: logger}
}

// Handle verifies the exact request body and persists before acknowledgment.
// @Summary Accept a signed business Razorpay callback
// @Description Dedicated business-account inbox; separate from platform credit purchases. Requires a valid HMAC-SHA256 over the exact raw body. A 202 means durable receipt, not that the invoice or refund is complete. Unsigned event headers never determine financial idempotency.
// @Tags BizBilling
// @Accept json
// @Produce json
// @Param connection_id path string true "Business merchant connection ID"
// @Param X-Razorpay-Signature header string true "Raw-body webhook signature"
// @Param X-Razorpay-Event-Id header string false "Provider event reference"
// @Success 202 {object} bizbilling.GatewayEventAcknowledgment
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 503 {object} map[string]interface{}
// @Router /v1/webhooks/biz/razorpay/{connection_id} [post]
func (h *BizBillingGatewayCallbackHandler) Handle(c *fiber.Ctx) error {
	if h.service == nil {
		return c.Status(503).JSON(fiber.Map{"error": "Billing callback storage is unavailable", "retryable": true})
	}
	receipt, err := h.service.Accept(c.UserContext(), c.Params("connection_id"), c.Get("X-Razorpay-Event-Id"), c.Body(), c.Get("X-Razorpay-Signature"))
	if errors.Is(err, bizbilling.ErrGatewaySignature) {
		return c.Status(401).JSON(fiber.Map{"error": "Invalid payment callback signature"})
	}
	if errors.Is(err, bizbilling.ErrInvalidGatewayProof) {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid payment callback"})
	}
	if err != nil {
		h.logger.Warn("Billing callback not acknowledged", zap.String("failure_code", "inbox_unavailable"))
		c.Set("Retry-After", "1")
		return c.Status(503).JSON(fiber.Map{"error": "Payment callback could not be durably accepted. Retry delivery.", "retryable": true})
	}
	return c.Status(202).JSON(fiber.Map{"success": true, "data": bizbilling.GatewayEventAcknowledgment{ID: receipt.ID, State: receipt.State, HeaderConflict: receipt.HeaderConflict}})
}
