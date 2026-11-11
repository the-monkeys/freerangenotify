package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
)

// The lifecycle service performs app-ownership checks internally, so every
// handler here simply extracts the authenticated app_id and delegates.

// ─── Subscriptions ───────────────────────────────────────────────────────

// @Summary Create Subscription
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Success 201 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/subscriptions [post]
// @Accept json
// @Param body body bizbilling.CreateSubscriptionRequest true "Request body"
func (h *BizBillingHandler) CreateSubscription(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	var req bizbilling.CreateSubscriptionRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	sub, err := h.lifecycleSvc.CreateSubscription(c.Context(), appID, &req)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": sub})
}

// @Summary List Subscriptions
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/subscriptions [get]
func (h *BizBillingHandler) ListSubscriptions(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	filter := bizbilling.SubscriptionFilter{
		AppID:  appID,
		UserID: c.Query("user_id"),
		PlanID: c.Query("plan_id"),
		Status: bizbilling.SubscriptionStatus(c.Query("status")),
		Limit:  h.queryInt(c, "limit", 20),
		Offset: h.queryInt(c, "offset", 0),
	}

	subs, total, err := h.lifecycleSvc.ListSubscriptions(c.Context(), filter)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": subs, "total": total})
}

// @Summary Get Subscription
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/subscriptions/{id} [get]
func (h *BizBillingHandler) GetSubscription(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	sub, err := h.lifecycleSvc.GetSubscription(c.Context(), appID, c.Params("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": sub})
}

// @Summary Update Subscription
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/subscriptions/{id} [put]
// @Accept json
// @Param body body bizbilling.UpdateSubscriptionRequest true "Request body"
func (h *BizBillingHandler) UpdateSubscription(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	var req bizbilling.UpdateSubscriptionRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	updated, err := h.lifecycleSvc.UpdateSubscription(c.Context(), appID, c.Params("id"), &req)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": updated})
}

// ChangePlan handles POST /v1/biz/subscriptions/:id/change-plan
// @Summary Change Plan
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/subscriptions/{id}/change-plan [post]
func (h *BizBillingHandler) ChangePlan(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	var req struct {
		PlanID   string `json:"plan_id"`
		Quantity *int   `json:"quantity,omitempty"`
	}
	if err := c.BodyParser(&req); err != nil || req.PlanID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "plan_id is required"})
	}

	sub, err := h.lifecycleSvc.ChangePlan(c.Context(), appID, c.Params("id"), req.PlanID, req.Quantity)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": sub})
}

// @Summary Cancel Subscription
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/subscriptions/{id}/cancel [post]
func (h *BizBillingHandler) CancelSubscription(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	atPeriodEnd := c.Query("at_period_end") == "true"
	sub, err := h.lifecycleSvc.CancelSubscription(c.Context(), appID, c.Params("id"), atPeriodEnd)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": sub})
}

// @Summary Pause Subscription
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/subscriptions/{id}/pause [post]
func (h *BizBillingHandler) PauseSubscription(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	sub, err := h.lifecycleSvc.PauseSubscription(c.Context(), appID, c.Params("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": sub})
}

// @Summary Resume Subscription
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/subscriptions/{id}/resume [post]
func (h *BizBillingHandler) ResumeSubscription(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	sub, err := h.lifecycleSvc.ResumeSubscription(c.Context(), appID, c.Params("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": sub})
}

// @Summary Reactivate Subscription
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/subscriptions/{id}/reactivate [post]
func (h *BizBillingHandler) ReactivateSubscription(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	sub, err := h.lifecycleSvc.ReactivateSubscription(c.Context(), appID, c.Params("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": sub})
}

// @Summary Set Non Renewing
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/subscriptions/{id}/non-renewing [post]
func (h *BizBillingHandler) SetNonRenewing(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	var req struct {
		NonRenewing bool `json:"non_renewing"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	sub, err := h.lifecycleSvc.SetNonRenewing(c.Context(), appID, c.Params("id"), req.NonRenewing)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": sub})
}

// ─── Invoices ────────────────────────────────────────────────────────────

// @Summary Create Invoice
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Success 201 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/invoices [post]
// @Accept json
// @Param body body bizbilling.CreateInvoiceRequest true "Request body"
func (h *BizBillingHandler) CreateInvoice(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	var req bizbilling.CreateInvoiceRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	inv, err := h.lifecycleSvc.CreateOneTimeInvoice(c.Context(), appID, &req)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": inv})
}

// @Summary List Invoices
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/invoices [get]
func (h *BizBillingHandler) ListInvoices(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	filter := bizbilling.InvoiceFilter{
		AppID:          appID,
		UserID:         c.Query("user_id"),
		SubscriptionID: c.Query("subscription_id"),
		Status:         bizbilling.InvoiceStatus(c.Query("status")),
		Limit:          h.queryInt(c, "limit", 20),
		Offset:         h.queryInt(c, "offset", 0),
	}

	invoices, total, err := h.lifecycleSvc.ListInvoices(c.Context(), filter)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": invoices, "total": total})
}

// @Summary Get Invoice
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/invoices/{id} [get]
func (h *BizBillingHandler) GetInvoice(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	inv, err := h.lifecycleSvc.GetInvoice(c.Context(), appID, c.Params("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": inv})
}

// @Summary Update Draft Invoice
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/invoices/{id} [put]
// @Accept json
// @Param body body bizbilling.UpdateInvoiceRequest true "Request body"
func (h *BizBillingHandler) UpdateDraftInvoice(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	var req bizbilling.UpdateInvoiceRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	inv, err := h.lifecycleSvc.UpdateDraftInvoice(c.Context(), appID, c.Params("id"), &req)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": inv})
}

// @Summary Issue Invoice
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/invoices/{id}/issue [post]
func (h *BizBillingHandler) IssueInvoice(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	inv, err := h.lifecycleSvc.IssueInvoice(c.Context(), appID, c.Params("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": inv})
}

// SendInvoice handles POST /v1/biz/invoices/:id/send
// @Summary Send Invoice
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/invoices/{id}/send [post]
func (h *BizBillingHandler) SendInvoice(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	inv, err := h.lifecycleSvc.SendInvoice(c.Context(), appID, c.Params("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": inv})
}

// @Summary Void Invoice
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/invoices/{id}/void [post]
func (h *BizBillingHandler) VoidInvoice(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	inv, err := h.lifecycleSvc.VoidInvoice(c.Context(), appID, c.Params("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": inv})
}

// @Summary Write Off Invoice
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/invoices/{id}/write-off [post]
func (h *BizBillingHandler) WriteOffInvoice(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	inv, err := h.lifecycleSvc.WriteOffInvoice(c.Context(), appID, c.Params("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": inv})
}

// ApplyCoupon handles POST /v1/biz/invoices/:id/apply-coupon
// @Summary Apply Coupon
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/invoices/{id}/apply-coupon [post]
// @Accept json
// @Param body body bizbilling.ApplyCouponRequest true "Request body"
func (h *BizBillingHandler) ApplyCoupon(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	var req bizbilling.ApplyCouponRequest
	if err := c.BodyParser(&req); err != nil || req.Code == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "coupon code is required"})
	}

	inv, err := h.lifecycleSvc.ApplyCoupon(c.Context(), appID, c.Params("id"), req.Code)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": inv})
}

// ApplyCredit handles POST /v1/biz/invoices/:id/apply-credit — applies the
// user's stored credit balance, or a specific credit note / retainer.
// @Summary Apply Credit
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/invoices/{id}/apply-credit [post]
func (h *BizBillingHandler) ApplyCredit(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	var req struct {
		CreditNoteID string `json:"credit_note_id,omitempty"`
		RetainerID   string `json:"retainer_id,omitempty"`
	}
	_ = c.BodyParser(&req) // empty body = apply stored balance

	var inv *bizbilling.BizInvoice
	switch {
	case req.CreditNoteID != "":
		inv, err = h.lifecycleSvc.ApplyCreditNoteToInvoice(c.Context(), appID, req.CreditNoteID, c.Params("id"))
	case req.RetainerID != "":
		inv, err = h.lifecycleSvc.ApplyRetainerToInvoice(c.Context(), appID, req.RetainerID, c.Params("id"))
	default:
		inv, err = h.lifecycleSvc.ApplyCredit(c.Context(), appID, c.Params("id"))
	}
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": inv})
}

// AddLateFee handles POST /v1/biz/invoices/:id/late-fee
// @Summary Add Late Fee
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/invoices/{id}/late-fee [post]
func (h *BizBillingHandler) AddLateFee(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	inv, err := h.lifecycleSvc.AddLateFee(c.Context(), appID, c.Params("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": inv})
}

// ─── Payments ────────────────────────────────────────────────────────────

// RecordPayment handles POST /v1/biz/invoices/:id/payments
// @Summary Record Payment
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 201 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/invoices/{id}/payments [post]
// @Accept json
// @Param body body bizbilling.RecordPaymentRequest true "Request body"
func (h *BizBillingHandler) RecordPayment(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	var req bizbilling.RecordPaymentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	payment, err := h.lifecycleSvc.RecordPayment(c.Context(), appID, c.Params("id"), &req)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": payment})
}

// @Summary List Payments
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/payments [get]
func (h *BizBillingHandler) ListPayments(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	filter := bizbilling.PaymentFilter{
		AppID:     appID,
		UserID:    c.Query("user_id"),
		InvoiceID: c.Query("invoice_id"),
		Status:    bizbilling.PaymentStatus(c.Query("status")),
		Limit:     h.queryInt(c, "limit", 20),
		Offset:    h.queryInt(c, "offset", 0),
	}

	payments, total, err := h.lifecycleSvc.ListPayments(c.Context(), filter)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": payments, "total": total})
}

// @Summary Get Payment
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/payments/{id} [get]
func (h *BizBillingHandler) GetPayment(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	payment, err := h.lifecycleSvc.GetPayment(c.Context(), appID, c.Params("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": payment})
}

// RefundPayment handles POST /v1/biz/payments/:id/refund
// @Summary Refund Payment
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/payments/{id}/refund [post]
// @Accept json
// @Param body body bizbilling.RefundPaymentRequest true "Request body"
func (h *BizBillingHandler) RefundPayment(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	var req bizbilling.RefundPaymentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	payment, err := h.lifecycleSvc.RefundPayment(c.Context(), appID, c.Params("id"), &req)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": payment})
}
