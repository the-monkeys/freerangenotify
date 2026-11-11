package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
)

// VerifyInvoicePayment verifies both checkout signature and fetched capture.
// @Summary Verify a customer invoice payment
// @Description Scoped by the portal token and the stored business payment account. Browser success alone never marks an invoice paid. Uncertain outcomes remain pending and can recover through signed callbacks.
// @Tags BizPortal
// @Accept json
// @Produce json
// @Param token path string true "Customer portal token"
// @Param id path string true "Invoice ID"
// @Param body body bizbilling.CheckoutProof true "Razorpay checkout result"
// @Success 200 {object} bizbilling.OperationResult
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Failure 503 {object} map[string]interface{}
// @Router /v1/portal/{token}/invoices/{id}/verify-payment [post]
func (h *BizPortalHandler) VerifyInvoicePayment(c *fiber.Ctx) error {
	app, user, err := h.resolve(c)
	if err != nil {
		return h.handleError(c, err)
	}
	if h.checkout == nil {
		return (&BizBillingHandler{logger: h.logger}).handleError(c, bizbilling.ErrMerchantUnavailable)
	}
	var proof bizbilling.CheckoutProof
	if c.BodyParser(&proof) != nil {
		return h.handleError(c, apperrors.BadRequest("Provide the payment checkout result"))
	}
	result, err := h.checkout.Verify(c.UserContext(), app, user, c.Params("id"), proof)
	if err != nil {
		return (&BizBillingHandler{logger: h.logger}).handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": result})
}
