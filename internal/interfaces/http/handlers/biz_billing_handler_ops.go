package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
)

// Handlers for operational entities: contracts, usage metering, expenses.

// ─── Contracts ───────────────────────────────────────────────────────────

// @Summary Create Contract
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Success 201 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/contracts [post]
// @Accept json
// @Param body body bizbilling.CreateContractRequest true "Request body"
func (h *BizBillingHandler) CreateContract(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	var req bizbilling.CreateContractRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	contract, err := h.contractSvc.Create(c.Context(), appID, &req)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": contract})
}

// @Summary List Contracts
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/contracts [get]
func (h *BizBillingHandler) ListContracts(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	filter := bizbilling.ContractFilter{
		AppID:  appID,
		UserID: c.Query("user_id"),
		Status: bizbilling.ContractStatus(c.Query("status")),
		Limit:  h.queryInt(c, "limit", 20),
		Offset: h.queryInt(c, "offset", 0),
	}

	contracts, total, err := h.contractSvc.List(c.Context(), filter)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": contracts, "total": total})
}

// @Summary Get Contract
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/contracts/{id} [get]
func (h *BizBillingHandler) GetContract(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	contract, err := h.contractSvc.Get(c.Context(), appID, c.Params("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": contract})
}

// @Summary Update Contract
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/contracts/{id} [put]
// @Accept json
// @Param body body bizbilling.UpdateContractRequest true "Request body"
func (h *BizBillingHandler) UpdateContract(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	var req bizbilling.UpdateContractRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	contract, err := h.contractSvc.Update(c.Context(), appID, c.Params("id"), &req)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": contract})
}

// @Summary Send Contract
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/contracts/{id}/send [post]
func (h *BizBillingHandler) SendContract(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	contract, err := h.contractSvc.Send(c.Context(), appID, c.Params("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": contract})
}

// AcceptContract records digital acceptance from the dashboard.
// @Summary Accept Contract
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/contracts/{id}/accept [post]
func (h *BizBillingHandler) AcceptContract(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	contract, err := h.contractSvc.Accept(c.Context(), appID, "", c.Params("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": contract})
}

// @Summary Amend Contract
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/contracts/{id}/amend [post]
// @Accept json
// @Param body body bizbilling.AmendContractRequest true "Request body"
func (h *BizBillingHandler) AmendContract(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	var req bizbilling.AmendContractRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	contract, err := h.contractSvc.Amend(c.Context(), appID, c.Params("id"), &req)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": contract})
}

// @Summary Renew Contract
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/contracts/{id}/renew [post]
func (h *BizBillingHandler) RenewContract(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	var req struct {
		NewEndDate *time.Time `json:"new_end_date,omitempty"`
	}
	_ = c.BodyParser(&req)

	contract, err := h.contractSvc.Renew(c.Context(), appID, c.Params("id"), req.NewEndDate)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": contract})
}

// @Summary Terminate Contract
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/contracts/{id}/terminate [post]
func (h *BizBillingHandler) TerminateContract(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	contract, err := h.contractSvc.Terminate(c.Context(), appID, c.Params("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": contract})
}

// ─── Usage Metering ──────────────────────────────────────────────────────

// @Summary Create Usage Meter
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Success 201 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/usage/meters [post]
// @Accept json
// @Param body body bizbilling.CreateUsageMeterRequest true "Request body"
func (h *BizBillingHandler) CreateUsageMeter(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	var req bizbilling.CreateUsageMeterRequest
	if err := c.BodyParser(&req); err != nil || req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "meter name is required"})
	}

	meter, err := h.usageSvc.CreateMeter(c.Context(), appID, &req)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": meter})
}

// @Summary List Usage Meters
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/usage/meters [get]
func (h *BizBillingHandler) ListUsageMeters(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	meters, err := h.usageSvc.ListMeters(c.Context(), appID)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": meters})
}

// ReportUsage handles POST /v1/biz/usage/events
// @Summary Report Usage
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Success 202 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/usage/events [post]
// @Accept json
// @Param body body bizbilling.ReportUsageRequest true "Request body"
func (h *BizBillingHandler) ReportUsage(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	var req bizbilling.ReportUsageRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	accepted, err := h.usageSvc.Report(c.Context(), appID, &req)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"success": true, "accepted": accepted})
}

// GetUsageSummary handles GET /v1/biz/usage/summary?user_id=&subscription_id=&from=&to=
// @Summary Get Usage Summary
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/usage/summary [get]
func (h *BizBillingHandler) GetUsageSummary(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	userID := c.Query("user_id")
	if userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "user_id is required"})
	}

	now := time.Now().UTC()
	from := h.queryTime(c, "from", now.AddDate(0, -1, 0))
	to := h.queryTime(c, "to", now)

	summaries, err := h.usageSvc.Summary(c.Context(), appID, userID, c.Query("subscription_id"), from, to)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": summaries})
}

// ─── Expenses ────────────────────────────────────────────────────────────

// @Summary Create Expense
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Success 201 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/expenses [post]
// @Accept json
// @Param body body bizbilling.CreateExpenseRequest true "Request body"
func (h *BizBillingHandler) CreateExpense(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	var req bizbilling.CreateExpenseRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	expense, err := h.expenseSvc.Create(c.Context(), appID, &req)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": expense})
}

// @Summary List Expenses
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/expenses [get]
func (h *BizBillingHandler) ListExpenses(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	filter := bizbilling.ExpenseFilter{
		AppID:    appID,
		Category: c.Query("category"),
		UserID:   c.Query("user_id"),
		Limit:    h.queryInt(c, "limit", 20),
		Offset:   h.queryInt(c, "offset", 0),
	}
	if c.Query("billable") != "" {
		billable := c.Query("billable") == "true"
		filter.Billable = &billable
	}
	if from := c.Query("from"); from != "" {
		t := h.queryTime(c, "from", time.Time{})
		filter.From = &t
	}
	if to := c.Query("to"); to != "" {
		t := h.queryTime(c, "to", time.Time{})
		filter.To = &t
	}

	expenses, total, err := h.expenseSvc.List(c.Context(), filter)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": expenses, "total": total})
}

// @Summary Get Expense
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/expenses/{id} [get]
func (h *BizBillingHandler) GetExpense(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	expense, err := h.expenseSvc.Get(c.Context(), appID, c.Params("id"))
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": expense})
}

// @Summary Update Expense
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/expenses/{id} [put]
// @Accept json
// @Param body body bizbilling.UpdateExpenseRequest true "Request body"
func (h *BizBillingHandler) UpdateExpense(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	var req bizbilling.UpdateExpenseRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	expense, err := h.expenseSvc.Update(c.Context(), appID, c.Params("id"), &req)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": expense})
}

// @Summary Delete Expense
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/expenses/{id} [delete]
func (h *BizBillingHandler) DeleteExpense(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	if err := h.expenseSvc.Delete(c.Context(), appID, c.Params("id")); err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "message": "expense deleted"})
}

// ConvertExpensesToInvoice handles POST /v1/biz/expenses/convert
// @Summary Convert Expenses To Invoice
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Success 201 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/expenses/convert [post]
func (h *BizBillingHandler) ConvertExpensesToInvoice(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	var req struct {
		UserID     string   `json:"user_id"`
		ExpenseIDs []string `json:"expense_ids"`
	}
	if err := c.BodyParser(&req); err != nil || req.UserID == "" || len(req.ExpenseIDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "user_id and expense_ids are required"})
	}

	inv, err := h.expenseSvc.ConvertToInvoice(c.Context(), appID, req.UserID, req.ExpenseIDs)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"success": true, "data": inv})
}

// GetExpenseSummary handles GET /v1/biz/expenses/summary
// @Summary Get Expense Summary
// @Description Use an application API key on your backend. Feature and provider configuration must be enabled for this operation.
// @Tags BizBilling
// @Produce json
// @Success 200 {object} map[string]interface{} "Response; see the operation guide for the resource envelope"
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/expenses/summary [get]
func (h *BizBillingHandler) GetExpenseSummary(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}

	var from, to *time.Time
	if c.Query("from") != "" {
		t := h.queryTime(c, "from", time.Time{})
		from = &t
	}
	if c.Query("to") != "" {
		t := h.queryTime(c, "to", time.Time{})
		to = &t
	}

	summary, err := h.expenseSvc.CategorySummary(c.Context(), appID, from, to)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": summary})
}

// queryTime parses an RFC3339 or date-only query parameter.
func (h *BizBillingHandler) queryTime(c *fiber.Ctx, key string, defaultVal time.Time) time.Time {
	val := c.Query(key)
	if val == "" {
		return defaultVal
	}
	if t, err := time.Parse(time.RFC3339, val); err == nil {
		return t.UTC()
	}
	if t, err := time.Parse("2006-01-02", val); err == nil {
		return t.UTC()
	}
	return defaultVal
}
