package handlers

import (
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
)

type BillingUpgradeActivateRequest struct {
	ReportID string `json:"report_id"`
}

type BillingUpgradeReviewRequest struct {
	DryRun           *bool  `json:"dry_run,omitempty"`
	ExpectedReportID string `json:"expected_report_id,omitempty"`
	PrimaryCurrency  string `json:"primary_currency"`
}

func (h *BizBillingHandler) SetMigrationService(service *services.BizMigrationService) {
	h.migration = service
}

// PrepareBillingUpgrade inventories original billing records without changing them.
// @Summary Review an application billing upgrade
// @Description Defaults to dry run. Set dry_run=false and expected_report_id to persist a review. Only an operator can attest the writer rollout; client input cannot enable it.
// @Tags BizBilling
// @Accept json
// @Produce json
// @Param body body BillingUpgradeReviewRequest false "Review options; dry_run defaults to true"
// @Success 200 {object} bizbilling.MigrationReport
// @Failure 400 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/lifecycle/prepare [post]
func (h *BizBillingHandler) PrepareBillingUpgrade(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}
	if h.migration == nil {
		return h.handleError(c, bizbilling.ErrReconciliationRequired)
	}
	request := BillingUpgradeReviewRequest{}
	if len(c.Body()) > 0 {
		if err = c.BodyParser(&request); err != nil {
			return h.handleError(c, apperrors.BadRequest("Invalid billing review options"))
		}
	}
	options := services.MigrationOptions{DryRun: true, ExpectedReportID: request.ExpectedReportID, PrimaryCurrency: request.PrimaryCurrency}
	if request.DryRun != nil {
		options.DryRun = *request.DryRun
	}
	report, err := h.migration.Prepare(c.UserContext(), appID, options)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": report})
}

// ResolveBillingUpgrade records an accountable currency decision in a sidecar.
// @Summary Resolve a historical billing currency
// @Description Requires the current source hash and a reason. The authenticated actor is recorded; original documents are retained.
// @Tags BizBilling
// @Accept json
// @Produce json
// @Param id path string true "Persisted review ID"
// @Param body body bizbilling.LegacyResolution true "Reviewed decision"
// @Success 200 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/lifecycle/reviews/{id}/resolve [post]
func (h *BizBillingHandler) ResolveBillingUpgrade(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}
	if h.migration == nil {
		return h.handleError(c, bizbilling.ErrReconciliationRequired)
	}
	var decision services.LegacyResolution
	if err = c.BodyParser(&decision); err != nil {
		return h.handleError(c, apperrors.BadRequest("Invalid review decision"))
	}
	actor, _ := c.Locals("user_id").(string)
	if actor == "" {
		actor = "application:" + appID
	}
	decision.ActorID = actor
	if err = h.migration.Resolve(c.UserContext(), appID, c.Params("id"), decision); err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true})
}

// ActivateBillingUpgrade activates only an operator-attested, current review.
// @Summary Activate a reviewed billing upgrade
// @Description Requires prior operator confirmation that older writers were drained. Unresolved sources, stale reports, active writers and unfinished billing features block activation.
// @Tags BizBilling
// @Produce json
// @Accept json
// @Param body body BillingUpgradeActivateRequest true "Operator-attested review ID"
// @Success 200 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Failure 503 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/lifecycle/activate [post]
func (h *BizBillingHandler) ActivateBillingUpgrade(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}
	if h.migration == nil {
		return h.handleError(c, bizbilling.ErrReconciliationRequired)
	}
	var request BillingUpgradeActivateRequest
	if err = c.BodyParser(&request); err != nil || request.ReportID == "" {
		return h.handleError(c, apperrors.BadRequest("A reviewed report_id is required"))
	}
	if err = h.migration.Activate(c.UserContext(), appID, request.ReportID); err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true})
}

// GetBillingLifecycle reports the current lifecycle and rollout readiness.
// @Summary Check business billing lifecycle
// @Tags BizBilling
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/lifecycle [get]
func (h *BizBillingHandler) GetBillingLifecycle(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}
	if h.migration == nil {
		return h.handleError(c, bizbilling.ErrReconciliationRequired)
	}
	state, enabled, err := h.migration.Lifecycle(c.UserContext(), appID)
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": fiber.Map{"app_id": appID, "lifecycle_version": state.LifecycleVersion, "status": state.Status, "primary_currency": state.PrimaryCurrency, "new_collection_enabled": enabled, "active_writers": len(state.MutationPermits), "migration_report_id": state.MigrationReportID}})
}

// ListBillingReviews lists review summaries without private source documents.
// @Summary List billing upgrade reviews
// @Tags BizBilling
// @Produce json
// @Param after query string false "Exclusive report ID cursor"
// @Param limit query int false "Page size, at most 25"
// @Success 200 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /v1/biz/lifecycle/reviews [get]
func (h *BizBillingHandler) ListBillingReviews(c *fiber.Ctx) error {
	appID, err := h.getAppID(c)
	if err != nil {
		return h.handleError(c, err)
	}
	if h.migration == nil {
		return h.handleError(c, bizbilling.ErrReconciliationRequired)
	}
	rows, next, err := h.migration.Reviews(c.UserContext(), appID, c.Query("after"), h.queryInt(c, "limit", 10))
	if err != nil {
		return h.handleError(c, err)
	}
	return c.JSON(fiber.Map{"success": true, "data": rows, "next_cursor": next})
}
