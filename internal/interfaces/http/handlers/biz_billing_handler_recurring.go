package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
)

func (h *BizBillingHandler) SetRecurringService(service *services.BizRecurringService) {
	h.recurring = service
}
func (h *BizPortalHandler) SetRecurringService(service *services.BizRecurringService) {
	h.recurring = service
}

// @Summary Choose subscription collection
// @Description Invoice collection supports cash, direct UPI and bank payments without a gateway. Automatic collection requires explicit customer authorization for future periods. Switching back waits for verified cancellation of future automatic debits. Retry an identical request with the same key after a pending outcome.
// @Tags BizBilling
// @Accept json
// @Produce json
// @Param id path string true "Subscription ID"
// @Param Idempotency-Key header string true "Stable key for this request"
// @Param body body bizbilling.ChangeCollectionRequest true "Collection choice and explicit automatic cycle count"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Failure 503 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/subscriptions/{id}/collection-mode [post]
func (h *BizBillingHandler) ChangeSubscriptionCollection(c *fiber.Ctx) error {
	app, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}
	if h.recurring == nil {
		return h.handleError(c, bizbilling.ErrMerchantUnavailable)
	}
	var request bizbilling.ChangeCollectionRequest
	if c.BodyParser(&request) != nil {
		return h.handleError(c, apperrors.BadRequest("Choose invoice or automatic collection"))
	}
	result, err := h.recurring.ConfigureCollection(c.UserContext(), app, c.Params("id"), request, c.Get("Idempotency-Key"))
	if err != nil {
		if result.OperationID != "" {
			err = &bizbilling.OperationFailure{Result: result, Cause: err}
		}
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": result})
}

// @Summary Authorize future subscription payments
// @Description The portal token must belong to this subscription's customer. Returns public Razorpay SDK parameters after the business explicitly selected automatic collection. Existing arrears are not charged. Authorization is not an invoice payment; the server verifies provider state and account linkage.
// @Tags BizPortal
// @Accept json
// @Produce json
// @Param token path string true "Customer portal token"
// @Param id path string true "Subscription ID"
// @Param Idempotency-Key header string true "Reuse this key when retrying authorization"
// @Param body body bizbilling.AuthorizeSubscriptionRequest true "Test or live mode matching the subscription"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Failure 503 {object} map[string]interface{}
// @Router /v1/portal/{token}/subscriptions/{id}/authorize [post]
func (h *BizPortalHandler) AuthorizeSubscription(c *fiber.Ctx) error {
	app, user, err := h.resolve(c)
	if err != nil {
		return h.handleError(c, err)
	}
	mapper := &BizBillingHandler{logger: h.logger}
	if h.recurring == nil {
		return mapper.handleError(c, bizbilling.ErrMerchantUnavailable)
	}
	var request bizbilling.AuthorizeSubscriptionRequest
	if c.BodyParser(&request) != nil {
		return mapper.handleError(c, apperrors.BadRequest("Choose test or live payment mode"))
	}
	descriptor, err := h.recurring.Authorize(c.UserContext(), app, user, c.Params("id"), request.Mode, c.Get("Idempotency-Key"))
	if err != nil {
		if descriptor.OperationID != "" {
			err = &bizbilling.OperationFailure{Result: bizbilling.OperationResult{OperationID: descriptor.OperationID, State: bizbilling.OperationAccepted}, Cause: err}
		}
		return mapper.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": descriptor})
}

// @Summary Verify subscription authorization
// @Description Verify the Razorpay checkout signature against the saved subscription and its original business key, then fetch provider state. Authorization never records an invoice payment.
// @Tags BizPortal
// @Accept json
// @Produce json
// @Param token path string true "Customer portal token"
// @Param id path string true "Subscription ID"
// @Param body body bizbilling.SubscriptionAuthorizationProof true "Razorpay authorization result"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 503 {object} map[string]interface{}
// @Router /v1/portal/{token}/subscriptions/{id}/verify-authorization [post]
func (h *BizPortalHandler) VerifySubscriptionAuthorization(c *fiber.Ctx) error {
	app, user, err := h.resolve(c)
	if err != nil {
		return h.handleError(c, err)
	}
	mapper := &BizBillingHandler{logger: h.logger}
	if h.recurring == nil {
		return mapper.handleError(c, bizbilling.ErrMerchantUnavailable)
	}
	var proof bizbilling.SubscriptionAuthorizationProof
	if c.BodyParser(&proof) != nil {
		return mapper.handleError(c, apperrors.BadRequest("Provide the subscription checkout result"))
	}
	descriptor, err := h.recurring.VerifyAuthorization(c.UserContext(), app, user, c.Params("id"), proof)
	if err != nil {
		return mapper.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": descriptor})
}
