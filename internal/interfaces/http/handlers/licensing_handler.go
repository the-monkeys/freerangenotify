package handlers

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
	"go.uber.org/zap"
)

// LicensingHandler serves hosted and self-hosted licensing management APIs.
type LicensingHandler struct {
	checker license.Checker
	subRepo license.Repository
	logger  *zap.Logger
}

func NewLicensingHandler(checker license.Checker, subRepo license.Repository, logger *zap.Logger) *LicensingHandler {
	return &LicensingHandler{checker: checker, subRepo: subRepo, logger: logger}
}

// GetStatus handles GET /v1/license/status
func (h *LicensingHandler) GetStatus(c *fiber.Ctx) error {
	if h.checker == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "licensing checker unavailable"})
	}

	app, ok := c.Locals("app").(*application.Application)
	if !ok || app == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "application context missing"})
	}

	decision, err := h.checker.Check(c.Context(), app)
	if err != nil {
		h.logger.Error("license status check failed", zap.Error(err), zap.String("app_id", app.AppID))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "license check failed"})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"app_id":      app.AppID,
			"tenant_id":   app.TenantID,
			"allowed":     decision.Allowed,
			"mode":        string(decision.Mode),
			"state":       string(decision.State),
			"reason":      decision.Reason,
			"source":      decision.Source,
			"checked_at":  decision.CheckedAt,
			"valid_until": decision.ValidUntil,
		},
	})
}

// CreateSubscription handles POST /v1/admin/licensing/subscriptions
func (h *LicensingHandler) CreateSubscription(c *fiber.Ctx) error {
	if h.subRepo == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "subscription repository unavailable"})
	}

	var req struct {
		ID                 string                 `json:"id"`
		TenantID           string                 `json:"tenant_id"`
		AppID              string                 `json:"app_id"`
		Plan               string                 `json:"plan"`
		Status             string                 `json:"status"`
		CurrentPeriodStart time.Time              `json:"current_period_start"`
		CurrentPeriodEnd   time.Time              `json:"current_period_end"`
		Metadata           map[string]interface{} `json:"metadata"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	if req.ID == "" {
		req.ID = uuid.New().String()
	}
	if req.Plan == "" || req.Status == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "plan and status are required"})
	}
	if req.AppID == "" && req.TenantID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "app_id or tenant_id is required"})
	}
	if req.CurrentPeriodStart.IsZero() || req.CurrentPeriodEnd.IsZero() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "current_period_start and current_period_end are required"})
	}

	sub := &license.Subscription{
		ID:                 req.ID,
		TenantID:           req.TenantID,
		AppID:              req.AppID,
		Plan:               req.Plan,
		Status:             license.SubscriptionStatus(req.Status),
		CurrentPeriodStart: req.CurrentPeriodStart,
		CurrentPeriodEnd:   req.CurrentPeriodEnd,
		Metadata:           req.Metadata,
	}

	if err := h.subRepo.Create(c.Context(), sub); err != nil {
		h.logger.Error("failed to create subscription", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create subscription"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": sub})
}

// UpdateSubscription handles PUT /v1/admin/licensing/subscriptions/:id
func (h *LicensingHandler) UpdateSubscription(c *fiber.Ctx) error {
	if h.subRepo == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "subscription repository unavailable"})
	}

	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "subscription id is required"})
	}

	sub, err := h.subRepo.GetByID(c.Context(), id)
	if err != nil || sub == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "subscription not found"})
	}

	var req struct {
		Plan               *string                `json:"plan"`
		Status             *string                `json:"status"`
		CurrentPeriodStart *time.Time             `json:"current_period_start"`
		CurrentPeriodEnd   *time.Time             `json:"current_period_end"`
		Metadata           map[string]interface{} `json:"metadata"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	if req.Plan != nil {
		sub.Plan = *req.Plan
	}
	if req.Status != nil {
		sub.Status = license.SubscriptionStatus(*req.Status)
	}
	if req.CurrentPeriodStart != nil {
		sub.CurrentPeriodStart = *req.CurrentPeriodStart
	}
	if req.CurrentPeriodEnd != nil {
		sub.CurrentPeriodEnd = *req.CurrentPeriodEnd
	}
	if req.Metadata != nil {
		sub.Metadata = req.Metadata
	}

	if err := h.subRepo.Update(c.Context(), sub); err != nil {
		h.logger.Error("failed to update subscription", zap.Error(err), zap.String("id", id))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to update subscription"})
	}

	return c.JSON(fiber.Map{"success": true, "data": sub})
}

// GetSubscription handles GET /v1/admin/licensing/subscriptions/:id
func (h *LicensingHandler) GetSubscription(c *fiber.Ctx) error {
	if h.subRepo == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "subscription repository unavailable"})
	}

	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "subscription id is required"})
	}

	sub, err := h.subRepo.GetByID(c.Context(), id)
	if err != nil || sub == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "subscription not found"})
	}

	return c.JSON(fiber.Map{"success": true, "data": sub})
}

// ListSubscriptions handles GET /v1/admin/licensing/subscriptions
func (h *LicensingHandler) ListSubscriptions(c *fiber.Ctx) error {
	if h.subRepo == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "subscription repository unavailable"})
	}

	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))
	status := c.Query("status")

	filter := license.SubscriptionFilter{
		TenantID: c.Query("tenant_id"),
		AppID:    c.Query("app_id"),
		Limit:    limit,
		Offset:   offset,
	}
	if status != "" {
		filter.Statuses = []license.SubscriptionStatus{license.SubscriptionStatus(status)}
	}

	subs, err := h.subRepo.List(c.Context(), filter)
	if err != nil {
		h.logger.Error("failed to list subscriptions", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to list subscriptions"})
	}

	return c.JSON(fiber.Map{"success": true, "data": subs})
}

// RequestLicense handles POST /v1/admin/licensing/request
func (h *LicensingHandler) RequestLicense(c *fiber.Ctx) error {
	if h.checker == nil || h.checker.Mode() != license.ModeSelfHosted {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "license request is only available in self_hosted mode"})
	}

	requestID := uuid.New().String()

	var req map[string]interface{}
	if err := c.BodyParser(&req); err != nil {
		req = map[string]interface{}{}
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"request_id": requestID,
			"mode":       string(h.checker.Mode()),
			"submitted":  req,
			"message":    "Submit this request payload to licensing authority to obtain a signed license artifact",
		},
	})
}

// ActivateLicense handles POST /v1/admin/licensing/activate
func (h *LicensingHandler) ActivateLicense(c *fiber.Ctx) error {
	var req struct {
		LicenseKey string `json:"license_key"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.LicenseKey == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "license_key is required"})
	}

	type selfHostedSetter interface {
		SetLicenseKey(string)
	}

	setter, ok := h.checker.(selfHostedSetter)
	if !ok || h.checker.Mode() != license.ModeSelfHosted {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "license activation is only available in self_hosted mode"})
	}

	setter.SetLicenseKey(req.LicenseKey)

	decision, err := h.checker.Check(c.Context(), nil)
	if err != nil {
		h.logger.Error("license activation check failed", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "license activation check failed"})
	}
	if !decision.Allowed {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":  fmt.Sprintf("license not accepted: %s", decision.Reason),
			"result": decision,
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"message": "license activated in memory (persist with frn license patch for restart safety)",
			"result":  decision,
		},
	})
}

// GetLicense handles GET /v1/admin/licensing
func (h *LicensingHandler) GetLicense(c *fiber.Ctx) error {
	if h.checker == nil || h.checker.Mode() != license.ModeSelfHosted {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "license details endpoint is only available in self_hosted mode"})
	}

	decision, err := h.checker.Check(c.Context(), nil)
	if err != nil {
		h.logger.Error("license fetch check failed", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "license check failed"})
	}

	return c.JSON(fiber.Map{"success": true, "data": decision})
}

// ValidateLicense handles POST /v1/admin/licensing/validate
func (h *LicensingHandler) ValidateLicense(c *fiber.Ctx) error {
	if h.checker == nil || h.checker.Mode() != license.ModeSelfHosted {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "license validation endpoint is only available in self_hosted mode"})
	}

	type cacheResetter interface {
		ClearCache()
	}

	if resetter, ok := h.checker.(cacheResetter); ok {
		resetter.ClearCache()
	}

	decision, err := h.checker.Check(c.Context(), nil)
	if err != nil {
		h.logger.Error("license revalidation failed", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "license revalidation failed"})
	}

	return c.JSON(fiber.Map{"success": true, "data": decision})
}
