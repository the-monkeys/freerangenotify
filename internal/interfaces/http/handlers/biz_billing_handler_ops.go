package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
)

// Handlers for operational entities: contracts, usage metering, expenses.

// ─── Contracts ───────────────────────────────────────────────────────────

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
