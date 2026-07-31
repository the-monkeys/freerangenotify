package handlers

import (
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
	productSvc   *services.BizProductService
	planSvc      *services.BizPlanService
	configSvc    *services.BizConfigService
	lifecycleSvc *services.BizLifecycleService
	estimateSvc  *services.BizEstimateService
	couponSvc    *services.BizCouponService
	creditSvc    *services.BizCreditService
	contractSvc  *services.BizContractService
	usageSvc     *services.BizUsageService
	expenseSvc   *services.BizExpenseService
	portalSvc    *services.BizPortalService
	analyticsSvc *services.BizAnalyticsService
	connectorSvc *services.BizConnectorService
	logger       *zap.Logger
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
func (h *BizBillingHandler) CreateProduct(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	var req bizbilling.CreateProductRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	product, err := h.productSvc.Create(c.Context(), appID, req)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    product,
	})
}

// ListProducts handles GET /v1/biz/products
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

	products, total, err := h.productSvc.List(c.Context(), filter)
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
func (h *BizBillingHandler) GetProduct(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	product, err := h.productSvc.GetByID(c.Context(), c.Params("id"))
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
func (h *BizBillingHandler) UpdateProduct(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	// Verify ownership
	product, err := h.productSvc.GetByID(c.Context(), c.Params("id"))
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

	updated, err := h.productSvc.Update(c.Context(), c.Params("id"), req)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    updated,
	})
}

// DeleteProduct handles DELETE /v1/biz/products/:id
func (h *BizBillingHandler) DeleteProduct(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	// Verify ownership
	product, err := h.productSvc.GetByID(c.Context(), c.Params("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	if product.AppID != appID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "product not found"})
	}

	if err := h.productSvc.Delete(c.Context(), c.Params("id")); err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "product archived",
	})
}

// ─── Plan Endpoints ──────────────────────────────────────────────────────

// CreatePlan handles POST /v1/biz/plans
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

	plan, err := h.planSvc.Create(c.Context(), appID, req)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    plan,
	})
}

// ListPlans handles GET /v1/biz/plans
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

	plans, total, err := h.planSvc.List(c.Context(), filter)
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
func (h *BizBillingHandler) GetPlan(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	plan, err := h.planSvc.GetByID(c.Context(), c.Params("id"))
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
func (h *BizBillingHandler) UpdatePlan(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	plan, err := h.planSvc.GetByID(c.Context(), c.Params("id"))
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

	updated, err := h.planSvc.Update(c.Context(), c.Params("id"), req)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    updated,
	})
}

// DeletePlan handles DELETE /v1/biz/plans/:id
func (h *BizBillingHandler) DeletePlan(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	plan, err := h.planSvc.GetByID(c.Context(), c.Params("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	if plan.AppID != appID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "plan not found"})
	}

	if err := h.planSvc.Archive(c.Context(), c.Params("id")); err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "plan archived",
	})
}

// AddPlanAddon handles POST /v1/biz/plans/:id/addons
func (h *BizBillingHandler) AddPlanAddon(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	var req bizbilling.CreatePlanAddonRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	addon, err := h.planSvc.AddAddon(c.Context(), appID, c.Params("id"), req)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    addon,
	})
}

// RemovePlanAddon handles DELETE /v1/biz/plans/:id/addons/:addon_id
func (h *BizBillingHandler) RemovePlanAddon(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	if err := h.planSvc.RemoveAddon(c.Context(), appID, c.Params("id"), c.Params("addon_id")); err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"message": "addon removed",
	})
}

// ListPlanAddons handles GET /v1/biz/plans/:id/addons
func (h *BizBillingHandler) ListPlanAddons(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	addons, err := h.planSvc.ListAddons(c.Context(), appID, c.Params("id"))
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
func (h *BizBillingHandler) GetConfig(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	}

	configType := h.configTypeParam(c)
	cfg, err := h.configSvc.GetConfig(c.Context(), appID, configType)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    cfg,
	})
}

// UpdateConfig handles PUT /v1/biz/config/:config_type
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

	cfg, err := h.configSvc.UpdateConfig(c.Context(), appID, configType, data)
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
	if appErr, ok := errors.AsAppError(err); ok {
		return c.Status(appErr.GetHTTPStatus()).JSON(fiber.Map{"error": err.Error()})
	}

	h.logger.Error("Internal error in biz billing handler", zap.Error(err))
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
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
