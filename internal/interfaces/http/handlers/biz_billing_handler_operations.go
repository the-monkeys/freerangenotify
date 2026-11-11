package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
)

func (h *BizBillingHandler) SetOperationService(service *services.BizOperationService) {
	h.operations = service
}

// GetBillingOperation checks a previously accepted billing action.
// @Summary Check a billing action
// @Description Use the operation_id from a pending request. Results belong to the authenticated application and contain no payment credentials.
// @Tags BizBilling
// @Produce json
// @Param id path string true "Billing operation ID"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 503 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/operations/{id} [get]
func (h *BizBillingHandler) GetBillingOperation(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}
	if h.operations == nil {
		return h.handleError(c, bizbilling.ErrReconciliationRequired)
	}
	result, err := h.operations.Get(c.UserContext(), appID, c.Params("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": result})
}

// ResumeBillingOperation continues the immutable saved action.
// @Summary Resume a saved billing action
// @Description Continues the saved action using its original details. A new request body cannot change the action. Pending work may need reconciliation before it can continue.
// @Tags BizBilling
// @Produce json
// @Param id path string true "Billing operation ID"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Failure 503 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/operations/{id}/resume [post]
func (h *BizBillingHandler) ResumeBillingOperation(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}
	if h.operations == nil {
		return h.handleError(c, bizbilling.ErrReconciliationRequired)
	}
	result, err := h.operations.Resume(c.UserContext(), appID, c.Params("id"))
	if err != nil {
		if result.OperationID != "" {
			err = &bizbilling.OperationFailure{Result: result, Cause: err}
		}
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": result})
}

// GetCustomerBalance reads completed customer credit in one currency.
// @Summary Get a customer's credit balance
// @Description Returns available and reserved amounts in minor currency units. Omit currency to use the application's primary billing currency. Requires a reviewed business billing upgrade.
// @Tags BizBilling
// @Produce json
// @Param user_id path string true "Customer user ID"
// @Param currency query string false "Currency code, such as INR"
// @Success 200 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Failure 503 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/users/{user_id}/balance [get]
func (h *BizBillingHandler) GetCustomerBalance(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}
	if h.creditSvc == nil {
		return h.handleError(c, bizbilling.ErrReconciliationRequired)
	}
	balance, err := h.creditSvc.CurrentBalance(c.UserContext(), appID, c.Params("user_id"), c.Query("currency"))
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": balance})
}
