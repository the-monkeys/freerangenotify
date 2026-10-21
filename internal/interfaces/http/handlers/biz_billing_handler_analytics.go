package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"go.uber.org/zap"
)

// SetAnalyticsService injects the analytics service (feature-wired in container).
func (h *BizBillingHandler) SetAnalyticsService(svc *services.BizAnalyticsService) {
	h.analyticsSvc = svc
}

// SetConnectorService injects the connector service.
func (h *BizBillingHandler) SetConnectorService(svc *services.BizConnectorService) {
	h.connectorSvc = svc
}

// ─── Analytics ───────────────────────────────────────────────────────────

// analyticsCall runs one analytics query with the shared auth/error handling.
func (h *BizBillingHandler) analyticsCall(c *fiber.Ctx, fn func(appID string) (interface{}, error)) error {
	if h.analyticsSvc == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "analytics service unavailable"})
	}
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}
	data, err := fn(appID)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": data})
}

// @Summary Analytics Revenue
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/analytics/revenue [get]
func (h *BizBillingHandler) AnalyticsRevenue(c *fiber.Ctx) error {
	return h.analyticsCall(c, func(appID string) (interface{}, error) {
		return h.analyticsSvc.Revenue(c.Context(), appID)
	})
}

// @Summary Analytics Subscriptions
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/analytics/subscriptions [get]
func (h *BizBillingHandler) AnalyticsSubscriptions(c *fiber.Ctx) error {
	return h.analyticsCall(c, func(appID string) (interface{}, error) {
		return h.analyticsSvc.Subscriptions(c.Context(), appID)
	})
}

// @Summary Analytics Invoices
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/analytics/invoices [get]
func (h *BizBillingHandler) AnalyticsInvoices(c *fiber.Ctx) error {
	return h.analyticsCall(c, func(appID string) (interface{}, error) {
		return h.analyticsSvc.Invoices(c.Context(), appID)
	})
}

// @Summary Analytics Aging
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/analytics/aging [get]
func (h *BizBillingHandler) AnalyticsAging(c *fiber.Ctx) error {
	return h.analyticsCall(c, func(appID string) (interface{}, error) {
		return h.analyticsSvc.Aging(c.Context(), appID)
	})
}

// @Summary Analytics Customers
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/analytics/customers [get]
func (h *BizBillingHandler) AnalyticsCustomers(c *fiber.Ctx) error {
	return h.analyticsCall(c, func(appID string) (interface{}, error) {
		return h.analyticsSvc.Customers(c.Context(), appID, h.queryInt(c, "limit", 10))
	})
}

// @Summary Analytics Dunning
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/analytics/dunning [get]
func (h *BizBillingHandler) AnalyticsDunning(c *fiber.Ctx) error {
	return h.analyticsCall(c, func(appID string) (interface{}, error) {
		return h.analyticsSvc.Dunning(c.Context(), appID)
	})
}

// @Summary Analytics Revenue Recognition
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/analytics/revenue-recognition [get]
func (h *BizBillingHandler) AnalyticsRevenueRecognition(c *fiber.Ctx) error {
	return h.analyticsCall(c, func(appID string) (interface{}, error) {
		return h.analyticsSvc.RevenueRecognition(c.Context(), appID)
	})
}

// @Summary Analytics Tax Summary
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/analytics/tax-summary [get]
func (h *BizBillingHandler) AnalyticsTaxSummary(c *fiber.Ctx) error {
	return h.analyticsCall(c, func(appID string) (interface{}, error) {
		return h.analyticsSvc.TaxSummary(c.Context(), appID)
	})
}

// ─── Connectors ──────────────────────────────────────────────────────────

// @Summary Create Connector
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Success 201 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/connectors [post]
// @Accept json
// @Param body body bizbilling.CreateConnectorRequest true "Request body"
func (h *BizBillingHandler) CreateConnector(c *fiber.Ctx) error {
	if h.connectorSvc == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "connector service unavailable"})
	}
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	var req bizbilling.CreateConnectorRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	req.AppID = appID // never trust the body for scoping

	connector, err := h.connectorSvc.Create(c.Context(), appID, &req)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": connector})
}

// @Summary List Connectors
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/connectors [get]
func (h *BizBillingHandler) ListConnectors(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	connectors, err := h.connectorSvc.List(c.Context(), appID)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": connectors})
}

// @Summary Update Connector
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/connectors/{id} [put]
// @Accept json
// @Param body body bizbilling.UpdateConnectorRequest true "Request body"
func (h *BizBillingHandler) UpdateConnector(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	var req bizbilling.UpdateConnectorRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	connector, err := h.connectorSvc.Update(c.Context(), appID, c.Params("id"), &req)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": connector})
}

// @Summary Delete Connector
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/connectors/{id} [delete]
func (h *BizBillingHandler) DeleteConnector(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	if err := h.connectorSvc.Delete(c.Context(), appID, c.Params("id")); err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "message": "connector deleted"})
}

// HandleConnectorWebhook handles POST /v1/webhooks/billing/:connector_id
// (public — authenticated by connector ID + HMAC signature).
// @Summary Handle Connector Webhook
// @Description Inbound provider callback. Requires the configured provider signature or handshake verification; application API keys are not callback credentials.
// @Tags BizBilling
// @Produce json
// @Param connector_id path string true "connector_id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /v1/webhooks/billing/{connector_id} [post]
func (h *BizBillingHandler) HandleConnectorWebhook(c *fiber.Ctx) error {
	signature := c.Get("X-Webhook-Signature")
	if signature == "" {
		signature = c.Get("X-Razorpay-Signature")
	}

	event, err := h.connectorSvc.HandleWebhook(c.Context(), c.Params("connector_id"), c.Body(), signature)
	if err != nil {
		return h.handleError(c, err)
	}

	h.logger.Info("bizbilling: connector webhook processed",
		zap.String("connector_id", c.Params("connector_id")),
		zap.String("event_type", event.EventType))
	return c.JSON(fiber.Map{"success": true, "event_type": event.EventType})
}
