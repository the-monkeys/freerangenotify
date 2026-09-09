package handlers

import (
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
	portalSvc    *services.BizPortalService
	lifecycleSvc *services.BizLifecycleService
	estimateSvc  *services.BizEstimateService
	creditSvc    *services.BizCreditService
	usageSvc     *services.BizUsageService
	contractSvc  *services.BizContractService
	logger       *zap.Logger
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
	pt, err := h.portalSvc.Resolve(c.Context(), c.Params("token"))
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
func (h *BizPortalHandler) ListInvoices(c *fiber.Ctx) error {
	appID, userID, err := h.resolve(c)
	if err != nil {
		return h.handleError(c, err)
	}

	invoices, total, err := h.lifecycleSvc.ListInvoices(c.Context(), bizbilling.InvoiceFilter{
		AppID:  appID,
		UserID: userID,
		Status: bizbilling.InvoiceStatus(c.Query("status")),
		Limit:  50,
	})
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": invoices, "total": total})
}

// GetInvoice handles GET /v1/portal/:token/invoices/:id
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
func (h *BizPortalHandler) PayInvoice(c *fiber.Ctx) error {
	appID, userID, err := h.resolve(c)
	if err != nil {
		return h.handleError(c, err)
	}

	inv, err := h.lifecycleSvc.GetInvoice(c.Context(), appID, c.Params("id"))
	if err != nil || inv.UserID != userID {
		return h.handleError(c, errors.NotFound("invoice", c.Params("id")))
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
func (h *BizPortalHandler) AcceptEstimate(c *fiber.Ctx) error { return h.decideEstimate(c, true) }
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
