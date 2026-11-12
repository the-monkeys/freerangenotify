package handlers

import (
	stderrors "errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// BizBillingServices bundles every service the billing handler depends on.
type BizBillingServices struct {
	Product   *services.BizProductService
	Plan      *services.BizPlanService
	Config    *services.BizConfigService
	Lifecycle *services.BizLifecycleService
	Estimate  *services.BizEstimateService
	Coupon    *services.BizCouponService
	Credit    *services.BizCreditService
	Contract  *services.BizContractService
	Usage     *services.BizUsageService
	Expense   *services.BizExpenseService
	Portal    *services.BizPortalService
}

// BizBillingHandler handles HTTP requests for the business billing module.
type BizBillingHandler struct {
	outbox        *services.BizOutboxService
	recurring     *services.BizRecurringService
	gatewayEvents *services.BizGatewayEventService
	checkout      *services.BizCheckoutService
	merchants     *services.BizGatewayConnectionService
	migration     *services.BizMigrationService
	operations    *services.BizOperationService
	gate          *services.BizLifecycleGate
	productSvc    *services.BizProductService
	planSvc       *services.BizPlanService
	configSvc     *services.BizConfigService
	lifecycleSvc  *services.BizLifecycleService
	estimateSvc   *services.BizEstimateService
	couponSvc     *services.BizCouponService
	creditSvc     *services.BizCreditService
	contractSvc   *services.BizContractService
	usageSvc      *services.BizUsageService
	expenseSvc    *services.BizExpenseService
	portalSvc     *services.BizPortalService
	analyticsSvc  *services.BizAnalyticsService
	connectorSvc  *services.BizConnectorService
	logger        *zap.Logger
}

// NewBizBillingHandler creates a new BizBillingHandler.
func NewBizBillingHandler(svcs BizBillingServices, logger *zap.Logger) *BizBillingHandler {
	return &BizBillingHandler{
		productSvc:   svcs.Product,
		planSvc:      svcs.Plan,
		configSvc:    svcs.Config,
		lifecycleSvc: svcs.Lifecycle,
		estimateSvc:  svcs.Estimate,
		couponSvc:    svcs.Coupon,
		creditSvc:    svcs.Credit,
		contractSvc:  svcs.Contract,
		usageSvc:     svcs.Usage,
		expenseSvc:   svcs.Expense,
		portalSvc:    svcs.Portal,
		logger:       logger,
	}
}

// getAppID extracts the authenticated app_id from Fiber context.
func (h *BizBillingHandler) getAppID(c *fiber.Ctx) (string, error) {
	appID, ok := c.Locals("app_id").(string)
	if !ok || appID == "" {
		return "", errors.Unauthorized("Application not authenticated")
	}
	return appID, nil
}

// ─── Product Endpoints ───────────────────────────────────────────────────

// CreateProduct handles POST /v1/biz/products
// @Summary Create Product
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Success 201 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/products [post]
// @Accept json
// @Param body body bizbilling.CreateProductRequest true "Request body"
func (h *BizBillingHandler) CreateProduct(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	var req bizbilling.CreateProductRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	product, err := h.productSvc.Create(c.UserContext(), appID, req)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    product,
	})
}

// ListProducts handles GET /v1/biz/products
// @Summary List Products
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/products [get]
func (h *BizBillingHandler) ListProducts(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	filter := bizbilling.ProductFilter{
		AppID:  appID,
		Search: c.Query("search"),
		Limit:  h.queryInt(c, "limit", 20),
		Offset: h.queryInt(c, "offset", 0),
	}
	if c.Query("active") != "" {
		active := c.Query("active") == "true"
		filter.Active = &active
	}

	products, total, err := h.productSvc.List(c.UserContext(), filter)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    products,
		"total":   total,
	})
}

// GetProduct handles GET /v1/biz/products/:id
// @Summary Get Product
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/products/{id} [get]
func (h *BizBillingHandler) GetProduct(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	product, err := h.productSvc.GetByID(c.UserContext(), c.Params("id"))
	if err != nil {
		return h.handleError(c, err)
	}

	// Ownership check
	if product.AppID != appID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "product not found"})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    product,
	})
}

// UpdateProduct handles PUT /v1/biz/products/:id
// @Summary Update Product
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/products/{id} [put]
// @Accept json
// @Param body body bizbilling.UpdateProductRequest true "Request body"
func (h *BizBillingHandler) UpdateProduct(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	// Verify ownership
	product, err := h.productSvc.GetByID(c.UserContext(), c.Params("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	if product.AppID != appID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "product not found"})
	}

	var req bizbilling.UpdateProductRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	updated, err := h.productSvc.Update(c.UserContext(), c.Params("id"), req)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    updated,
	})
}

// DeleteProduct handles DELETE /v1/biz/products/:id
// @Summary Delete Product
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/products/{id} [delete]
func (h *BizBillingHandler) DeleteProduct(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	// Verify ownership
	product, err := h.productSvc.GetByID(c.UserContext(), c.Params("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	if product.AppID != appID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "product not found"})
	}

	if err := h.productSvc.Delete(c.UserContext(), c.Params("id")); err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "product archived",
	})
}

// ─── Plan Endpoints ──────────────────────────────────────────────────────

// CreatePlan handles POST /v1/biz/plans
// @Summary Create Plan
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Success 201 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/plans [post]
// @Accept json
// @Param body body bizbilling.CreatePlanRequest true "Request body"
func (h *BizBillingHandler) CreatePlan(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	var req bizbilling.CreatePlanRequest
	if err := c.BodyParser(&req); err != nil {
		h.logger.Error("CreatePlan body parse error", zap.Error(err), zap.String("body", string(c.Body())))
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body", "details": err.Error()})
	}

	plan, err := h.planSvc.Create(c.UserContext(), appID, req)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    plan,
	})
}

// ListPlans handles GET /v1/biz/plans
// @Summary List Plans
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/plans [get]
func (h *BizBillingHandler) ListPlans(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	filter := bizbilling.PlanFilter{
		AppID:     appID,
		ProductID: c.Query("product_id"),
		Search:    c.Query("search"),
		Limit:     h.queryInt(c, "limit", 20),
		Offset:    h.queryInt(c, "offset", 0),
	}
	if c.Query("active") != "" {
		active := c.Query("active") == "true"
		filter.Active = &active
	}

	plans, total, err := h.planSvc.List(c.UserContext(), filter)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    plans,
		"total":   total,
	})
}

// GetPlan handles GET /v1/biz/plans/:id
// @Summary Get Plan
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/plans/{id} [get]
func (h *BizBillingHandler) GetPlan(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	plan, err := h.planSvc.GetByID(c.UserContext(), c.Params("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	if plan.AppID != appID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "plan not found"})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    plan,
	})
}

// UpdatePlan handles PUT /v1/biz/plans/:id
// @Summary Update Plan
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/plans/{id} [put]
// @Accept json
// @Param body body bizbilling.UpdatePlanRequest true "Request body"
func (h *BizBillingHandler) UpdatePlan(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	plan, err := h.planSvc.GetByID(c.UserContext(), c.Params("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	if plan.AppID != appID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "plan not found"})
	}

	var req bizbilling.UpdatePlanRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	updated, err := h.planSvc.Update(c.UserContext(), c.Params("id"), req)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    updated,
	})
}

// DeletePlan handles DELETE /v1/biz/plans/:id
// @Summary Delete Plan
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/plans/{id} [delete]
func (h *BizBillingHandler) DeletePlan(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	plan, err := h.planSvc.GetByID(c.UserContext(), c.Params("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	if plan.AppID != appID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "plan not found"})
	}

	if err := h.planSvc.Archive(c.UserContext(), c.Params("id")); err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "plan archived",
	})
}

// AddPlanAddon handles POST /v1/biz/plans/:id/addons
// @Summary Add Plan Addon
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 201 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/plans/{id}/addons [post]
// @Accept json
// @Param body body bizbilling.CreatePlanAddonRequest true "Request body"
func (h *BizBillingHandler) AddPlanAddon(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	var req bizbilling.CreatePlanAddonRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	addon, err := h.planSvc.AddAddon(c.UserContext(), appID, c.Params("id"), req)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    addon,
	})
}

// RemovePlanAddon handles DELETE /v1/biz/plans/:id/addons/:addon_id
// @Summary Remove Plan Addon
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Param addon_id path string true "addon_id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/plans/{id}/addons/{addon_id} [delete]
func (h *BizBillingHandler) RemovePlanAddon(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	if err := h.planSvc.RemoveAddon(c.UserContext(), appID, c.Params("id"), c.Params("addon_id")); err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "addon removed",
	})
}

// ListPlanAddons handles GET /v1/biz/plans/:id/addons
// @Summary List Plan Addons
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/plans/{id}/addons [get]
func (h *BizBillingHandler) ListPlanAddons(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	addons, err := h.planSvc.ListAddons(c.UserContext(), appID, c.Params("id"))
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    addons,
	})
}

// ─── Config Endpoints ────────────────────────────────────────────────────

// GetConfig handles GET /v1/biz/config/:config_type
// @Summary Get Config
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param config_type path string true "config_type"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/config/{config_type} [get]
func (h *BizBillingHandler) GetConfig(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	configType := h.configTypeParam(c)
	cfg, err := h.configSvc.GetConfig(c.UserContext(), appID, configType)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    cfg,
	})
}

// UpdateConfig handles PUT /v1/biz/config/:config_type
// @Summary Update Config
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param config_type path string true "config_type"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/config/{config_type} [put]
func (h *BizBillingHandler) UpdateConfig(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	configType := h.configTypeParam(c)

	var data map[string]interface{}
	if err := c.BodyParser(&data); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	cfg, err := h.configSvc.UpdateConfig(c.UserContext(), appID, configType, data)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    cfg,
	})
}

// ─── Helpers ─────────────────────────────────────────────────────────────

// handleError converts domain errors to HTTP status codes.
func (h *BizBillingHandler) handleError(c *fiber.Ctx, err error) error {
	var operation *bizbilling.OperationFailure
	stderrors.As(err, &operation)
	status, code, message, retryable := fiber.StatusInternalServerError, "INTERNAL_ERROR", "internal server error", false
	switch {
	case stderrors.Is(err, bizbilling.ErrModeMismatch):
		status, code, message = 409, "BILLING_MODE_MISMATCH", "Test funds and live billing must remain separate."
	case stderrors.Is(err, bizbilling.ErrInvalidGatewayProof):
		status, code, message = 400, "BILLING_PAYMENT_NOT_VERIFIED", "The checkout result could not be verified. Check the invoice or payment status before trying again."
	case stderrors.Is(err, bizbilling.ErrGatewayPending):
		status, code, message, retryable = 503, "BILLING_GATEWAY_PENDING", "The payment provider outcome is pending verification. Retry this operation; do not create another payment.", true
	case stderrors.Is(err, bizbilling.ErrMerchantAuthentication):
		status, code, message = 400, "BILLING_MERCHANT_AUTHENTICATION_FAILED", "Payment account credentials could not be authenticated."
	case stderrors.Is(err, bizbilling.ErrMerchantUnavailable):
		status, code, message, retryable = 503, "BILLING_MERCHANT_UNAVAILABLE", "Payment account verification is unavailable. Retry shortly.", true
	case stderrors.Is(err, bizbilling.ErrMerchantNotReady):
		status, code, message = 409, "BILLING_MERCHANT_NOT_READY", "Connect and verify the business payment account before online collection."
	case stderrors.Is(err, bizbilling.ErrLegacyReviewRequired):
		status, code, message = 409, "BILLING_LEGACY_REVIEW_REQUIRED", "Review this historical credit before spending or refunding it."
	case stderrors.Is(err, bizbilling.ErrCouponExhausted):
		status, code, message = 400, "BILLING_COUPON_UNAVAILABLE", "This coupon is unavailable or its usage limit has been reached."

	case stderrors.Is(err, bizbilling.ErrCurrencyMismatch):
		status, code, message = 409, "BILLING_CURRENCY_MISMATCH", "The request currency does not match this record."
	case stderrors.Is(err, bizbilling.ErrIdempotencyConflict):
		status, code, message = 409, "BILLING_IDEMPOTENCY_CONFLICT", "This idempotency key was already used for a different request."
	case stderrors.Is(err, bizbilling.ErrAutomaticCollectionInProgress):
		status, code, message = 409, "BILLING_COLLECTION_LOCKED", "Stop automatic collection and confirm its status before changing this invoice."
	case stderrors.Is(err, bizbilling.ErrActiveLegacyWriter):
		status, code, message, retryable = 503, "BILLING_UPGRADE_PENDING", "Billing upgrade is in progress. Retry shortly.", true
	case stderrors.Is(err, bizbilling.ErrReconciliationRequired):
		status, code, message, retryable = 503, "BILLING_RECONCILIATION_PENDING", "Billing needs reconciliation. Check the operation status before retrying.", true
	case stderrors.Is(err, bizbilling.ErrConflict) || stderrors.Is(err, bizbilling.ErrStaleMutationPermit):
		status, code, message, retryable = 409, "BILLING_CONFLICT", "Billing changed. Retry using the same idempotency key.", true
	case stderrors.Is(err, bizbilling.ErrInvalidMoney) || stderrors.Is(err, bizbilling.ErrMoneyOverflow):
		status, code, message = 400, "BILLING_INVALID_AMOUNT", "Invalid amount, rate, or financial state."
	default:
		if appErr, ok := errors.AsAppError(err); ok {
			status, code, message, retryable = appErr.GetHTTPStatus(), appErr.Code, err.Error(), appErr.Retryable
			if operation != nil {
				message = appErr.Message
			}
		} else if operation != nil {
			status, code, message, retryable = 503, "BILLING_OPERATION_PENDING", "Billing could not confirm completion. Check the operation status before retrying.", true
		}
	}
	response := fiber.Map{"error": message, "code": code, "retryable": retryable}
	if operation != nil && operation.Result.OperationID != "" {
		response["operation_id"] = operation.Result.OperationID
		response["operation_state"] = operation.Result.State
		c.Set("Operation-Id", operation.Result.OperationID)
	}
	if retryable {
		c.Set("Retry-After", "1")
	}
	if status >= 500 {
		h.logger.Warn("Billing request needs recovery", zap.String("failure_code", code))
	}
	return c.Status(status).JSON(response)
}

// queryInt extracts an integer query parameter with a default value.
func (h *BizBillingHandler) queryInt(c *fiber.Ctx, key string, defaultVal int) int {
	val := c.Query(key)
	if val == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return n
}

func (h *BizBillingHandler) configTypeParam(c *fiber.Ctx) bizbilling.ConfigType {
	return bizbilling.ConfigType(strings.ReplaceAll(c.Params("config_type"), "-", "_"))
}

func (h *BizBillingHandler) SetLifecycleGate(g *services.BizLifecycleGate) { h.gate = g }

// GuardMutations holds one permit for the complete authorized request.
func (h *BizBillingHandler) GuardMutations(c *fiber.Ctx) error {
	if c.Method() == fiber.MethodGet || c.Method() == fiber.MethodHead || c.Method() == fiber.MethodOptions {
		return c.Next()
	}
	key := c.Get("Idempotency-Key")
	if len(key) > 200 {
		return c.Status(400).JSON(fiber.Map{"error": "Idempotency-Key must be at most 200 characters", "code": "BILLING_INVALID_IDEMPOTENCY_KEY"})
	}
	ctxWithKey := services.WithBizOperationKey(c.UserContext(), key)
	actor, _ := c.Locals("user_id").(string)
	ctxWithKey = services.WithBizOperationActor(ctxWithKey, actor)
	c.SetUserContext(ctxWithKey)
	app, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}
	ctx, done, err := services.BeginBizMutation(c.UserContext(), h.gate, app)
	if err != nil {
		return h.handleError(c, err)
	}
	c.SetUserContext(ctx)
	defer func() {
		if err := done(); err != nil {
			h.logger.Warn("billing request permit requires reconciliation", zap.Error(err))
		}
	}()
	return c.Next()
}
