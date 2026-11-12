package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
)

func (h *BizPortalHandler) SetOperationService(service *services.BizOperationService) {
	h.operations = service
}

// ListSubscriptions returns this link's customer subscriptions only.
// @Summary List customer subscriptions
// @Tags BizPortal
// @Produce json
// @Param token path string true "Customer portal token"
// @Param offset query int false "Page offset"
// @Success 200 {object} map[string]interface{}
// @Router /v1/portal/{token}/subscriptions [get]
func (h *BizPortalHandler) ListSubscriptions(c *fiber.Ctx) error {
	app, user, err := h.resolve(c)
	if err != nil {
		return h.handleError(c, err)
	}
	rows, total, err := h.lifecycleSvc.ListSubscriptions(c.UserContext(), bizbilling.SubscriptionFilter{AppID: app, UserID: user, Limit: 20, Offset: portalOffset(c)})
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": rows, "total": total})
}

// @Summary Get customer subscription
// @Tags BizPortal
// @Produce json
// @Param token path string true "Customer portal token"
// @Param id path string true "Subscription ID"
// @Success 200 {object} map[string]interface{}
// @Router /v1/portal/{token}/subscriptions/{id} [get]
func (h *BizPortalHandler) GetSubscription(c *fiber.Ctx) error {
	app, user, err := h.resolve(c)
	if err != nil {
		return h.handleError(c, err)
	}
	row, err := h.lifecycleSvc.GetSubscription(c.UserContext(), app, c.Params("id"))
	if err != nil || row.UserID != user {
		return h.handleError(c, apperrors.NotFound("subscription", ""))
	}
	return c.JSON(fiber.Map{"success": true, "data": row})
}

// @Summary Get issued customer invoice document
// @Tags BizPortal
// @Produce json
// @Param token path string true "Customer portal token"
// @Param id path string true "Invoice ID"
// @Success 200 {object} map[string]interface{}
// @Router /v1/portal/{token}/invoices/{id}/document [get]
func (h *BizPortalHandler) GetInvoiceDocument(c *fiber.Ctx) error {
	app, user, err := h.resolve(c)
	if err != nil {
		return h.handleError(c, err)
	}
	doc, err := services.NewBizInvoiceService(h.lifecycleSvc).Document(c.UserContext(), app, user, c.Params("id"))
	if err != nil {
		return (&BizBillingHandler{logger: h.logger}).handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": doc})
}

// @Summary Get a customer payment receipt
// @Tags BizPortal
// @Produce json
// @Param token path string true "Customer portal token"
// @Param id path string true "Payment ID"
// @Success 200 {object} map[string]interface{}
// @Router /v1/portal/{token}/payments/{id}/receipt [get]
func (h *BizPortalHandler) GetPaymentReceipt(c *fiber.Ctx) error {
	app, user, err := h.resolve(c)
	if err != nil {
		return h.handleError(c, err)
	}
	payment, err := h.lifecycleSvc.GetPayment(c.UserContext(), app, c.Params("id"))
	if err != nil || payment.UserID != user {
		return h.handleError(c, apperrors.NotFound("payment", ""))
	}
	return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"id": payment.ID, "amount_paisa": payment.AmountPaisa, "currency": payment.Currency, "method": payment.Method, "reference": payment.Reference, "status": payment.Status, "refunded_paisa": payment.RefundedPaisa, "created_at": payment.CreatedAt, "invoice_ids": payment.InvoiceIDs}})
}

// @Summary Check a customer payment or authorization action
// @Tags BizPortal
// @Produce json
// @Param token path string true "Customer portal token"
// @Param id path string true "Operation ID"
// @Success 200 {object} map[string]interface{}
// @Router /v1/portal/{token}/operations/{id} [get]
func (h *BizPortalHandler) GetOperation(c *fiber.Ctx) error {
	app, user, err := h.resolve(c)
	if err != nil {
		return h.handleError(c, err)
	}
	if h.operations != nil {
		result, lookup := h.operations.GetCustomer(c.UserContext(), app, user, c.Params("id"))
		if lookup == nil {
			return c.JSON(fiber.Map{"success": true, "data": result})
		}
		if !apperrors.IsNotFound(lookup) {
			return (&BizBillingHandler{logger: h.logger}).handleError(c, lookup)
		}
	}
	if h.checkout != nil {
		effect, lookup := h.checkout.GetEffect(c.UserContext(), app, c.Params("id"))
		if lookup == nil && effect.Scope.UserID == user {
			return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"operation_id": effect.ID, "state": effect.State, "kind": effect.Kind, "result": effect.Result}})
		}
	}
	return h.handleError(c, apperrors.NotFound("billing action", ""))
}

func portalLimit(c *fiber.Ctx, fallback int) int {
	limit := c.QueryInt("limit", fallback)
	if limit < 1 {
		return fallback
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func portalOffset(c *fiber.Ctx) int {
	offset := c.QueryInt("offset", 0)
	if offset < 0 || offset > 100000 {
		return 0
	}
	return offset
}

// @Summary Revoke a customer portal link
// @Tags BizBilling
// @Produce json
// @Security ApiKeyAuth
// @Security BearerAuth
// @Param id path string true "Portal token ID returned when creating the link"
// @Success 200 {object} map[string]interface{}
// @Router /v1/biz/portal-tokens/{id}/revoke [post]
func (h *BizBillingHandler) RevokePortalToken(c *fiber.Ctx) error {
	app, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}
	if err = h.portalSvc.Revoke(c.UserContext(), app, c.Params("id")); err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true})
}
