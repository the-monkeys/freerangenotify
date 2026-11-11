package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
)

type MerchantConnectionRequest struct {
	ExpectedRevision *int64 `json:"expected_revision"`
	bizbilling.MerchantPatch
}

type MerchantDisconnectRequest struct {
	ExpectedRevision *int64 `json:"expected_revision"`
}

func (h *BizBillingHandler) SetMerchantService(service *services.BizGatewayConnectionService) {
	h.merchants = service
}

// GetMerchantConnection reads account readiness without exposing credentials.
// @Summary Get business payment account readiness
// @Description Optional for manual cash/UPI billing. API authentication is separate from verified account linkage and automatic-collection capability. Secrets are never returned.
// @Tags BizBilling
// @Produce json
// @Param mode path string true "test or live"
// @Success 200 {object} bizbilling.MerchantSnapshot
// @Failure 400 {object} map[string]interface{}
// @Failure 503 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/gateway-connections/{mode} [get]
func (h *BizBillingHandler) GetMerchantConnection(c *fiber.Ctx) error {
	app, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}
	if h.merchants == nil {
		return h.handleError(c, bizbilling.ErrMerchantUnavailable)
	}
	snapshot, err := h.merchants.Snapshot(c.UserContext(), app, bizbilling.GatewayMode(c.Params("mode")))
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": snapshot})
}

// SaveMerchantConnection encrypts credentials and retains old account revisions.
// @Summary Connect or rotate a business payment account
// @Description Requires billing-write permission and the current slot revision. Credentials are write-only. A different account creates a new connection; old orders retain their original binding. Authentication uses a read-only provider API and does not enable automatic collection by itself.
// @Tags BizBilling
// @Accept json
// @Produce json
// @Param mode path string true "test or live"
// @Param body body MerchantConnectionRequest true "Expected revision and write-only credentials"
// @Success 200 {object} bizbilling.MerchantSnapshot
// @Failure 400 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Failure 503 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/gateway-connections/{mode} [put]
func (h *BizBillingHandler) SaveMerchantConnection(c *fiber.Ctx) error {
	app, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}
	if h.merchants == nil {
		return h.handleError(c, bizbilling.ErrMerchantUnavailable)
	}
	var request MerchantConnectionRequest
	if c.BodyParser(&request) != nil || request.ExpectedRevision == nil {
		return h.handleError(c, apperrors.BadRequest("Provide the current payment account revision"))
	}
	snapshot, err := h.merchants.Save(c.UserContext(), app, bizbilling.GatewayMode(c.Params("mode")), *request.ExpectedRevision, request.MerchantPatch)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": snapshot})
}

// DisconnectMerchantConnection removes the active slot, not historical bindings.
// @Summary Disconnect a business payment account
// @Description Stops using this slot for new collection. Retains encrypted revisions needed by existing orders and late events; does not cancel provider mandates or delete payment history.
// @Tags BizBilling
// @Accept json
// @Produce json
// @Param mode path string true "test or live"
// @Param body body MerchantDisconnectRequest true "Current slot revision"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/gateway-connections/{mode} [delete]
func (h *BizBillingHandler) DisconnectMerchantConnection(c *fiber.Ctx) error {
	app, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}
	if h.merchants == nil {
		return h.handleError(c, bizbilling.ErrMerchantUnavailable)
	}
	var request MerchantDisconnectRequest
	if c.BodyParser(&request) != nil || request.ExpectedRevision == nil {
		return h.handleError(c, apperrors.BadRequest("Provide the current payment account revision"))
	}
	if err := h.merchants.Disconnect(c.UserContext(), app, bizbilling.GatewayMode(c.Params("mode")), *request.ExpectedRevision); err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true})
}
