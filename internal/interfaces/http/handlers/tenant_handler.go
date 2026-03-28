package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/tenant"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"github.com/the-monkeys/freerangenotify/pkg/validator"
	"go.uber.org/zap"
)

// TenantHandler handles tenant/organization HTTP requests.
type TenantHandler struct {
	service   tenant.Service
	validator *validator.Validator
	logger    *zap.Logger
}

// NewTenantHandler creates a new TenantHandler.
func NewTenantHandler(service tenant.Service, v *validator.Validator, logger *zap.Logger) *TenantHandler {
	return &TenantHandler{
		service:   service,
		validator: v,
		logger:    logger,
	}
}

// Create handles POST /v1/tenants
func (h *TenantHandler) Create(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return errors.Unauthorized("User not authenticated")
	}

	var req tenant.CreateRequest
	if err := c.BodyParser(&req); err != nil {
		return errors.BadRequest("Invalid request body")
	}
	if err := h.validator.Validate(req); err != nil {
		return errors.Validation("Validation failed", validator.FormatValidationErrors(err))
	}

	t, err := h.service.Create(c.Context(), req, userID)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    t,
	})
}

// List handles GET /v1/tenants
func (h *TenantHandler) List(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return errors.Unauthorized("User not authenticated")
	}

	tenants, err := h.service.ListByUser(c.Context(), userID)
	if err != nil {
		return err
	}
	if tenants == nil {
		tenants = []*tenant.Tenant{}
	}
	return c.JSON(fiber.Map{
		"success": true,
		"data":    tenants,
	})
}

// GetByID handles GET /v1/tenants/:id
func (h *TenantHandler) GetByID(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return errors.Unauthorized("User not authenticated")
	}

	tenantID := c.Params("id")
	if tenantID == "" {
		return errors.BadRequest("tenant id is required")
	}

	hasAccess, _, err := h.service.HasAccess(c.Context(), tenantID, userID)
	if err != nil {
		return err
	}
	if !hasAccess {
		return errors.Forbidden("You do not have access to this tenant")
	}

	t, err := h.service.GetByID(c.Context(), tenantID)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{
		"success": true,
		"data":    t,
	})
}

// Update handles PUT /v1/tenants/:id
func (h *TenantHandler) Update(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return errors.Unauthorized("User not authenticated")
	}

	tenantID := c.Params("id")
	if tenantID == "" {
		return errors.BadRequest("tenant id is required")
	}

	var req tenant.UpdateRequest
	if err := c.BodyParser(&req); err != nil {
		return errors.BadRequest("Invalid request body")
	}

	t, err := h.service.Update(c.Context(), tenantID, req, userID)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{
		"success": true,
		"data":    t,
	})
}

// Delete handles DELETE /v1/tenants/:id
func (h *TenantHandler) Delete(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return errors.Unauthorized("User not authenticated")
	}

	tenantID := c.Params("id")
	if tenantID == "" {
		return errors.BadRequest("tenant id is required")
	}

	if err := h.service.Delete(c.Context(), tenantID, userID); err != nil {
		return err
	}
	return c.JSON(fiber.Map{
		"success": true,
		"message": "Tenant deleted successfully",
	})
}

// ListMembers handles GET /v1/tenants/:id/members
func (h *TenantHandler) ListMembers(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return errors.Unauthorized("User not authenticated")
	}

	tenantID := c.Params("id")
	if tenantID == "" {
		return errors.BadRequest("tenant id is required")
	}

	hasAccess, _, err := h.service.HasAccess(c.Context(), tenantID, userID)
	if err != nil || !hasAccess {
		return errors.Forbidden("You do not have access to this tenant")
	}

	members, err := h.service.ListMembers(c.Context(), tenantID)
	if err != nil {
		return err
	}
	if members == nil {
		members = []*tenant.TenantMember{}
	}
	return c.JSON(fiber.Map{
		"success": true,
		"data":    members,
	})
}

// InviteMember handles POST /v1/tenants/:id/members
func (h *TenantHandler) InviteMember(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return errors.Unauthorized("User not authenticated")
	}

	tenantID := c.Params("id")
	if tenantID == "" {
		return errors.BadRequest("tenant id is required")
	}

	var req tenant.InviteMemberRequest
	if err := c.BodyParser(&req); err != nil {
		return errors.BadRequest("Invalid request body")
	}
	if err := h.validator.Validate(req); err != nil {
		return errors.Validation("Validation failed", validator.FormatValidationErrors(err))
	}

	member, err := h.service.InviteMember(c.Context(), tenantID, req, userID)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data":    member,
	})
}

// GetBilling handles GET /v1/tenants/:id/billing
func (h *TenantHandler) GetBilling(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return errors.Unauthorized("User not authenticated")
	}

	tenantID := c.Params("id")
	if tenantID == "" {
		return errors.BadRequest("tenant id is required")
	}

	hasAccess, _, err := h.service.HasAccess(c.Context(), tenantID, userID)
	if err != nil || !hasAccess {
		return errors.Forbidden("You do not have access to this tenant")
	}

	t, err := h.service.GetByID(c.Context(), tenantID)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"billing_tier":   t.BillingTier,
			"valid_until":    t.ValidUntil,
			"max_apps":       t.MaxApps,
			"max_throughput": t.MaxThroughput,
		},
	})
}

// Checkout handles POST /v1/tenants/:id/billing/checkout
func (h *TenantHandler) Checkout(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return errors.Unauthorized("User not authenticated")
	}

	tenantID := c.Params("id")
	if tenantID == "" {
		return errors.BadRequest("tenant id is required")
	}

	hasAccess, role, err := h.service.HasAccess(c.Context(), tenantID, userID)
	if err != nil || !hasAccess {
		return errors.Forbidden("You do not have access to this tenant")
	}
	if role != "owner" && role != "admin" {
		return errors.Forbidden("Only owners and admins can manage billing")
	}
	// Route the checkout logic to the dedicated PaymentHandler
	// (Keeping the endpoint /v1/tenants/:id/billing/checkout for backward compat)

	// Create request payload for PaymentHandler.CreateOrder
	reqBody := struct {
		Tier string `json:"tier"`
	}{}
	if err := c.BodyParser(&reqBody); err == nil && reqBody.Tier != "" {
		// Just passing the tier through. If error, the handler will catch it or default
	}

	// Make an internal fast-forward call rather than duplicating logic
	// We'll replace the Locals with just user_id which PaymentHandler needs
	return h.createInternalOrder(c, userID, reqBody.Tier)
}

func (h *TenantHandler) createInternalOrder(c *fiber.Ctx, userID, tier string) error {
	// Call to container's PaymentHandler (we're going to use the route instead so this is cleaner)
	// Returning a redirection error to specify they should use the new billing API
	return errors.BadRequest("Please use the new /v1/billing/checkout endpoint")
}

// UpdateMemberRole handles PUT /v1/tenants/:id/members/:memberId
func (h *TenantHandler) UpdateMemberRole(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return errors.Unauthorized("User not authenticated")
	}

	tenantID := c.Params("id")
	memberID := c.Params("memberId")
	if tenantID == "" || memberID == "" {
		return errors.BadRequest("tenant id and member id are required")
	}

	var req tenant.UpdateMemberRoleRequest
	if err := c.BodyParser(&req); err != nil {
		return errors.BadRequest("Invalid request body")
	}

	member, err := h.service.UpdateMemberRole(c.Context(), tenantID, memberID, req, userID)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{
		"success": true,
		"data":    member,
	})
}

// RemoveMember handles DELETE /v1/tenants/:id/members/:memberId
func (h *TenantHandler) RemoveMember(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return errors.Unauthorized("User not authenticated")
	}

	tenantID := c.Params("id")
	memberID := c.Params("memberId")
	if tenantID == "" || memberID == "" {
		return errors.BadRequest("tenant id and member id are required")
	}

	if err := h.service.RemoveMember(c.Context(), tenantID, memberID, userID); err != nil {
		return err
	}
	return c.JSON(fiber.Map{
		"success": true,
		"message": "Member removed",
	})
}
