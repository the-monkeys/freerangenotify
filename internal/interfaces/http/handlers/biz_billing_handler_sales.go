package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
)

// Handlers for the sales documents: estimates, coupons, credit notes,
// retainers, and customer statements / credit balance.

// ─── Estimates ───────────────────────────────────────────────────────────

// @Summary Create Estimate
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Success 201 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/estimates [post]
// @Accept json
// @Param body body bizbilling.CreateEstimateRequest true "Request body"
func (h *BizBillingHandler) CreateEstimate(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	var req bizbilling.CreateEstimateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	est, err := h.estimateSvc.Create(c.Context(), appID, &req)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": est})
}

// @Summary List Estimates
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/estimates [get]
func (h *BizBillingHandler) ListEstimates(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	filter := bizbilling.EstimateFilter{
		AppID:  appID,
		UserID: c.Query("user_id"),
		Status: bizbilling.EstimateStatus(c.Query("status")),
		Limit:  h.queryInt(c, "limit", 20),
		Offset: h.queryInt(c, "offset", 0),
	}

	estimates, total, err := h.estimateSvc.List(c.Context(), filter)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": estimates, "total": total})
}

// @Summary Get Estimate
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/estimates/{id} [get]
func (h *BizBillingHandler) GetEstimate(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	est, err := h.estimateSvc.Get(c.Context(), appID, c.Params("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": est})
}

// @Summary Update Estimate
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/estimates/{id} [put]
// @Accept json
// @Param body body bizbilling.UpdateEstimateRequest true "Request body"
func (h *BizBillingHandler) UpdateEstimate(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	var req bizbilling.UpdateEstimateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	est, err := h.estimateSvc.Update(c.Context(), appID, c.Params("id"), &req)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": est})
}

// @Summary Delete Estimate
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/estimates/{id} [delete]
func (h *BizBillingHandler) DeleteEstimate(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	if err := h.estimateSvc.Delete(c.Context(), appID, c.Params("id")); err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "message": "estimate deleted"})
}

// @Summary Send Estimate
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/estimates/{id}/send [post]
func (h *BizBillingHandler) SendEstimate(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	est, err := h.estimateSvc.Send(c.Context(), appID, c.Params("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": est})
}

// AcceptEstimate / RejectEstimate record the decision on behalf of the
// customer (dashboard action). The portal flow uses the portal handler.
// @Summary Accept Estimate
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/estimates/{id}/accept [post]
func (h *BizBillingHandler) AcceptEstimate(c *fiber.Ctx) error {
	return h.decideEstimate(c, true)
}

// @Summary Reject Estimate
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/estimates/{id}/reject [post]
func (h *BizBillingHandler) RejectEstimate(c *fiber.Ctx) error {
	return h.decideEstimate(c, false)
}

func (h *BizBillingHandler) decideEstimate(c *fiber.Ctx, accepted bool) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	est, err := h.estimateSvc.SetDecision(c.Context(), appID, "", c.Params("id"), accepted)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": est})
}

// ConvertEstimate handles POST /v1/biz/estimates/:id/convert
// @Summary Convert Estimate
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 201 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/estimates/{id}/convert [post]
func (h *BizBillingHandler) ConvertEstimate(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	var req struct {
		LineIndexes []int `json:"line_indexes,omitempty"` // empty = all lines
	}
	_ = c.BodyParser(&req)

	inv, err := h.estimateSvc.Convert(c.Context(), appID, c.Params("id"), req.LineIndexes)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": inv})
}

// ─── Coupons ─────────────────────────────────────────────────────────────

// @Summary Create Coupon
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Success 201 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/coupons [post]
// @Accept json
// @Param body body bizbilling.CreateCouponRequest true "Request body"
func (h *BizBillingHandler) CreateCoupon(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	var req bizbilling.CreateCouponRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	coupon, err := h.couponSvc.Create(c.Context(), appID, &req)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": coupon})
}

// @Summary Bulk Generate Coupons
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Success 201 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/coupons/bulk-generate [post]
// @Accept json
// @Param body body bizbilling.BulkGenerateCouponsRequest true "Request body"
func (h *BizBillingHandler) BulkGenerateCoupons(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	var req bizbilling.BulkGenerateCouponsRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	coupons, err := h.couponSvc.BulkGenerate(c.Context(), appID, &req)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": coupons, "total": len(coupons)})
}

// @Summary List Coupons
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/coupons [get]
func (h *BizBillingHandler) ListCoupons(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	filter := bizbilling.CouponFilter{
		AppID:  appID,
		Code:   c.Query("code"),
		Limit:  h.queryInt(c, "limit", 20),
		Offset: h.queryInt(c, "offset", 0),
	}
	if c.Query("active") != "" {
		active := c.Query("active") == "true"
		filter.Active = &active
	}

	coupons, total, err := h.couponSvc.List(c.Context(), filter)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": coupons, "total": total})
}

// @Summary Get Coupon
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/coupons/{id} [get]
func (h *BizBillingHandler) GetCoupon(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	coupon, err := h.couponSvc.Get(c.Context(), appID, c.Params("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": coupon})
}

// @Summary Update Coupon
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/coupons/{id} [put]
// @Accept json
// @Param body body bizbilling.UpdateCouponRequest true "Request body"
func (h *BizBillingHandler) UpdateCoupon(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	var req bizbilling.UpdateCouponRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	coupon, err := h.couponSvc.Update(c.Context(), appID, c.Params("id"), &req)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": coupon})
}

// @Summary Delete Coupon
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/coupons/{id} [delete]
func (h *BizBillingHandler) DeleteCoupon(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	if err := h.couponSvc.Delete(c.Context(), appID, c.Params("id")); err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "message": "coupon deleted"})
}

// ─── Credit Notes ────────────────────────────────────────────────────────

// @Summary Create Credit Note
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Success 201 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/credit-notes [post]
// @Accept json
// @Param body body bizbilling.CreateCreditNoteRequest true "Request body"
func (h *BizBillingHandler) CreateCreditNote(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	var req bizbilling.CreateCreditNoteRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	note, err := h.creditSvc.CreateCreditNote(c.Context(), appID, &req)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": note})
}

// @Summary List Credit Notes
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/credit-notes [get]
func (h *BizBillingHandler) ListCreditNotes(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	filter := bizbilling.CreditNoteFilter{
		AppID:  appID,
		UserID: c.Query("user_id"),
		Status: bizbilling.CreditNoteStatus(c.Query("status")),
		Limit:  h.queryInt(c, "limit", 20),
		Offset: h.queryInt(c, "offset", 0),
	}

	notes, total, err := h.creditSvc.ListCreditNotes(c.Context(), filter)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": notes, "total": total})
}

// @Summary Get Credit Note
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/credit-notes/{id} [get]
func (h *BizBillingHandler) GetCreditNote(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	note, err := h.creditSvc.GetCreditNote(c.Context(), appID, c.Params("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": note})
}

// @Summary Refund Credit Note
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/credit-notes/{id}/refund [post]
func (h *BizBillingHandler) RefundCreditNote(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	note, err := h.creditSvc.RefundCreditNote(c.Context(), appID, c.Params("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": note})
}

// ─── Retainers ───────────────────────────────────────────────────────────

// @Summary Create Retainer
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Success 201 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/retainers [post]
// @Accept json
// @Param body body bizbilling.CreateRetainerRequest true "Request body"
func (h *BizBillingHandler) CreateRetainer(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	var req bizbilling.CreateRetainerRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	retainer, err := h.creditSvc.CreateRetainer(c.Context(), appID, &req)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": retainer})
}

// @Summary List Retainers
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/retainers [get]
func (h *BizBillingHandler) ListRetainers(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	filter := bizbilling.RetainerFilter{
		AppID:  appID,
		UserID: c.Query("user_id"),
		Status: bizbilling.RetainerStatus(c.Query("status")),
		Limit:  h.queryInt(c, "limit", 20),
		Offset: h.queryInt(c, "offset", 0),
	}

	retainers, total, err := h.creditSvc.ListRetainers(c.Context(), filter)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": retainers, "total": total})
}

// @Summary Get Retainer
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/retainers/{id} [get]
func (h *BizBillingHandler) GetRetainer(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	retainer, err := h.creditSvc.GetRetainer(c.Context(), appID, c.Params("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": retainer})
}

// @Summary Mark Retainer Paid
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/retainers/{id}/mark-paid [post]
func (h *BizBillingHandler) MarkRetainerPaid(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	retainer, err := h.creditSvc.MarkRetainerPaid(c.Context(), appID, c.Params("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": retainer})
}

// ─── Customer Billing (statement, balance, portal link) ─────────────────

// GetUserStatement handles GET /v1/biz/users/:user_id/statement
// @Summary Get User Statement
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param user_id path string true "user_id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/users/{user_id}/statement [get]
func (h *BizBillingHandler) GetUserStatement(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	statement, err := h.creditSvc.Statement(c.Context(), appID, c.Params("user_id"))
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": statement})
}

// AdjustUserBalance handles POST /v1/biz/users/:user_id/balance
// @Summary Adjust User Balance
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param user_id path string true "user_id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/users/{user_id}/balance [post]
// @Accept json
// @Param body body bizbilling.AdjustBalanceRequest true "Request body"
func (h *BizBillingHandler) AdjustUserBalance(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	var req bizbilling.AdjustBalanceRequest
	if err := c.BodyParser(&req); err != nil || req.AmountPaisa == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "non-zero amount_paisa is required"})
	}

	balance, err := h.creditSvc.AdjustBalance(c.Context(), appID, c.Params("user_id"), req.AmountPaisa)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"balance_paisa": balance}})
}

// CreatePortalLink handles POST /v1/biz/users/:user_id/portal-link
// @Summary Create Portal Link
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param user_id path string true "user_id"
// @Success 201 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/users/{user_id}/portal-link [post]
func (h *BizBillingHandler) CreatePortalLink(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	token, err := h.portalSvc.CreateToken(c.Context(), appID, c.Params("user_id"), 0)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"url":        h.portalSvc.PortalLink(token.Token),
			"expires_at": token.ExpiresAt,
		},
	})
}
