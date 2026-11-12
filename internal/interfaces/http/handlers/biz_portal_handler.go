package handlers

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// BizPortalHandler serves the customer self-service portal. Every route is
// authenticated by an opaque, time-limited portal token in the URL, which
// resolves to (app_id, user_id) — all data access is scoped to both.
type BizPortalHandler struct {
	operations   *services.BizOperationService
	recurring    *services.BizRecurringService
	checkout     *services.BizCheckoutService
	portalSvc    *services.BizPortalService
	lifecycleSvc *services.BizLifecycleService
	estimateSvc  *services.BizEstimateService
	creditSvc    *services.BizCreditService
	usageSvc     *services.BizUsageService
	contractSvc  *services.BizContractService
	logger       *zap.Logger
}

func (h *BizPortalHandler) SetCheckoutService(service *services.BizCheckoutService) {
	h.checkout = service
}

// NewBizPortalHandler creates a new BizPortalHandler.
func NewBizPortalHandler(svcs BizBillingServices, logger *zap.Logger) *BizPortalHandler {
	return &BizPortalHandler{
		portalSvc:    svcs.Portal,
		lifecycleSvc: svcs.Lifecycle,
		estimateSvc:  svcs.Estimate,
		creditSvc:    svcs.Credit,
		usageSvc:     svcs.Usage,
		contractSvc:  svcs.Contract,
		logger:       logger,
	}
}

// resolve validates the :token path param and returns (appID, userID).
func (h *BizPortalHandler) resolve(c *fiber.Ctx) (string, string, error) {
	pt, err := h.portalSvc.ResolveScope(c.UserContext(), c.Params("token"))
	if err != nil {
		return "", "", err
	}
	return pt.AppID, pt.UserID, nil
}

func (h *BizPortalHandler) handleError(c *fiber.Ctx, err error) error {
	if appErr, ok := errors.AsAppError(err); ok {
		return c.Status(appErr.GetHTTPStatus()).JSON(fiber.Map{"error": err.Error()})
	}
	h.logger.Error("Internal error in biz portal handler", zap.Error(err))
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal server error"})
}

// Overview handles GET /v1/portal/:token — statement summary + open invoices.
// @Summary Overview
// @Description Public endpoint with its route-specific token, signature or authentication flow.
// @Tags BizPortal
// @Produce json
// @Param token path string true "token"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /v1/portal/{token} [get]
func (h *BizPortalHandler) Overview(c *fiber.Ctx) error {
	appID, userID, err := h.resolve(c)
	if err != nil {
		return h.handleError(c, err)
	}

	statement, err := h.creditSvc.Statement(c.Context(), appID, userID)
	if err != nil {
		return h.handleError(c, err)
	}

	openInvoices, _, err := h.lifecycleSvc.ListInvoices(c.Context(), bizbilling.InvoiceFilter{
		AppID: appID, UserID: userID, Status: bizbilling.InvoiceStatusOpen, Limit: 20,
	})
	if err != nil {
		return h.handleError(c, err)
	}

	subs, _, err := h.lifecycleSvc.ListSubscriptions(c.Context(), bizbilling.SubscriptionFilter{
		AppID: appID, UserID: userID, Limit: 20,
	})
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(fiber.Map{"success": true, "data": fiber.Map{
		"statement":     statement,
		"open_invoices": openInvoices,
		"subscriptions": subs,
	}})
}

// ListInvoices handles GET /v1/portal/:token/invoices
// @Summary List Invoices
// @Description Public endpoint with its route-specific token, signature or authentication flow.
// @Tags BizPortal
// @Produce json
// @Param token path string true "token"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /v1/portal/{token}/invoices [get]
func (h *BizPortalHandler) ListInvoices(c *fiber.Ctx) error {
	appID, userID, err := h.resolve(c)
	if err != nil {
		return h.handleError(c, err)
	}

	invoices, total, err := h.lifecycleSvc.ListInvoices(c.Context(), bizbilling.InvoiceFilter{
		AppID:  appID,
		UserID: userID,
		Status: bizbilling.InvoiceStatus(c.Query("status")),
		Limit:  portalLimit(c, 50),
		Offset: portalOffset(c),
	})
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": invoices, "total": total})
}

// GetInvoice handles GET /v1/portal/:token/invoices/:id
// @Summary Get Invoice
// @Description Public endpoint with its route-specific token, signature or authentication flow.
// @Tags BizPortal
// @Produce json
// @Param token path string true "token"
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /v1/portal/{token}/invoices/{id} [get]
func (h *BizPortalHandler) GetInvoice(c *fiber.Ctx) error {
	appID, userID, err := h.resolve(c)
	if err != nil {
		return h.handleError(c, err)
	}

	inv, err := h.lifecycleSvc.GetInvoice(c.Context(), appID, c.Params("id"))
	if err != nil || inv.UserID != userID {
		return h.handleError(c, errors.NotFound("invoice", c.Params("id")))
	}

	return c.JSON(fiber.Map{"success": true, "data": inv})
}

// PayInvoice handles POST /v1/portal/:token/invoices/:id/pay — returns
// gateway checkout parameters for a payable invoice.
// @Summary Pay Invoice
// @Description Public endpoint with its route-specific token, signature or authentication flow.
// @Tags BizPortal
// @Produce json
// @Param token path string true "token"
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /v1/portal/{token}/invoices/{id}/pay [post]
func (h *BizPortalHandler) PayInvoice(c *fiber.Ctx) error {
	appID, userID, err := h.resolve(c)
	if err != nil {
		return h.handleError(c, err)
	}

	inv, err := h.lifecycleSvc.GetInvoice(c.Context(), appID, c.Params("id"))
	if err != nil || inv.UserID != userID {
		return h.handleError(c, errors.NotFound("invoice", c.Params("id")))
	}
	if h.checkout != nil {
		upgraded, stateErr := h.checkout.IsUpgraded(c.UserContext(), appID)
		if stateErr != nil {
			return (&BizBillingHandler{logger: h.logger}).handleError(c, stateErr)
		}
		if upgraded {
			mode := inv.GatewayMode
			if mode == "" {
				mode = bizbilling.GatewayLive
			}
			key := c.Get("Idempotency-Key")
			if key == "" {
				key = "portal-checkout-" + inv.ID + "-" + strconv.FormatInt(inv.LastSequence, 10)
			}
			descriptor, checkoutErr := h.checkout.Start(c.UserContext(), appID, userID, inv.ID, mode, key)
			if checkoutErr != nil {
				if descriptor.OperationID != "" {
					checkoutErr = &bizbilling.OperationFailure{Result: bizbilling.OperationResult{OperationID: descriptor.OperationID, State: bizbilling.OperationAccepted}, Cause: checkoutErr}
				}
				return (&BizBillingHandler{logger: h.logger}).handleError(c, checkoutErr)
			}
			return c.JSON(fiber.Map{"success": true, "data": descriptor})
		}
	}
	if !bizbilling.PayableStatuses[inv.Status] {
		return h.handleError(c, errors.BadRequest("invoice is not payable in its current status"))
	}
	if inv.GatewayOrderID == "" {
		return h.handleError(c, errors.BadRequest("no payment gateway order exists for this invoice — contact the business"))
	}

	return c.JSON(fiber.Map{"success": true, "data": fiber.Map{
		"gateway_order_id": inv.GatewayOrderID,
		"amount_due_paisa": inv.AmountDuePaisa,
		"currency":         inv.Currency,
		"invoice_number":   inv.InvoiceNumber,
	}})
}

// ListEstimates handles GET /v1/portal/:token/estimates
// @Summary List Estimates
// @Description Public endpoint with its route-specific token, signature or authentication flow.
// @Tags BizPortal
// @Produce json
// @Param token path string true "token"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /v1/portal/{token}/estimates [get]
func (h *BizPortalHandler) ListEstimates(c *fiber.Ctx) error {
	appID, userID, err := h.resolve(c)
	if err != nil {
		return h.handleError(c, err)
	}

	estimates, total, err := h.estimateSvc.List(c.Context(), bizbilling.EstimateFilter{
		AppID: appID, UserID: userID, Limit: 50,
	})
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": estimates, "total": total})
}

// DecideEstimate handles POST /v1/portal/:token/estimates/:id/accept|reject.
// @Summary Accept Estimate
// @Description Public endpoint with its route-specific token, signature or authentication flow.
// @Tags BizPortal
// @Produce json
// @Param token path string true "token"
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /v1/portal/{token}/estimates/{id}/accept [post]
func (h *BizPortalHandler) AcceptEstimate(c *fiber.Ctx) error { return h.decideEstimate(c, true) }

// @Summary Reject Estimate
// @Description Public endpoint with its route-specific token, signature or authentication flow.
// @Tags BizPortal
// @Produce json
// @Param token path string true "token"
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /v1/portal/{token}/estimates/{id}/reject [post]
func (h *BizPortalHandler) RejectEstimate(c *fiber.Ctx) error { return h.decideEstimate(c, false) }

func (h *BizPortalHandler) decideEstimate(c *fiber.Ctx, accepted bool) error {
	appID, userID, err := h.resolve(c)
	if err != nil {
		return h.handleError(c, err)
	}

	est, err := h.estimateSvc.SetDecision(c.Context(), appID, userID, c.Params("id"), accepted)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": est})
}

// AcceptContract handles POST /v1/portal/:token/contracts/:id/accept
// @Summary Accept Contract
// @Description Public endpoint with its route-specific token, signature or authentication flow.
// @Tags BizPortal
// @Produce json
// @Param token path string true "token"
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /v1/portal/{token}/contracts/{id}/accept [post]
func (h *BizPortalHandler) AcceptContract(c *fiber.Ctx) error {
	appID, userID, err := h.resolve(c)
	if err != nil {
		return h.handleError(c, err)
	}

	contract, err := h.contractSvc.Accept(c.Context(), appID, userID, c.Params("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": contract})
}

// GetStatement handles GET /v1/portal/:token/statement
// @Summary Get Statement
// @Description Public endpoint with its route-specific token, signature or authentication flow.
// @Tags BizPortal
// @Produce json
// @Param token path string true "token"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /v1/portal/{token}/statement [get]
func (h *BizPortalHandler) GetStatement(c *fiber.Ctx) error {
	appID, userID, err := h.resolve(c)
	if err != nil {
		return h.handleError(c, err)
	}

	statement, err := h.creditSvc.Statement(c.Context(), appID, userID)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": statement})
}

// GetUsage handles GET /v1/portal/:token/usage — current-month usage.
// @Summary Get Usage
// @Description Public endpoint with its route-specific token, signature or authentication flow.
// @Tags BizPortal
// @Produce json
// @Param token path string true "token"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Router /v1/portal/{token}/usage [get]
func (h *BizPortalHandler) GetUsage(c *fiber.Ctx) error {
	appID, userID, err := h.resolve(c)
	if err != nil {
		return h.handleError(c, err)
	}

	now := time.Now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	summaries, err := h.usageSvc.Summary(c.Context(), appID, userID, "", monthStart, now)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": summaries})
}
