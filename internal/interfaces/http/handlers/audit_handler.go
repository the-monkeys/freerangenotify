package handlers

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/audit"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// AuditHandler exposes admin-level endpoints for querying audit logs.
type AuditHandler struct {
	service audit.Service
	appRepo application.Repository
	logger  *zap.Logger
}

// NewAuditHandler creates a new AuditHandler.
func NewAuditHandler(service audit.Service, appRepo application.Repository, logger *zap.Logger) *AuditHandler {
	return &AuditHandler{service: service, appRepo: appRepo, logger: logger}
}

// getAdminAppIDs returns the list of app IDs owned by the authenticated admin user.
func (h *AuditHandler) getAdminAppIDs(c *fiber.Ctx) ([]string, error) {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return nil, fiber.NewError(fiber.StatusUnauthorized, "authentication required")
	}

	apps, err := h.appRepo.List(c.Context(), application.ApplicationFilter{AdminUserID: userID})
	if err != nil {
		return nil, fiber.NewError(fiber.StatusInternalServerError, "failed to fetch admin apps")
	}

	appIDs := make([]string, len(apps))
	for i, app := range apps {
		appIDs[i] = app.AppID
	}
	return appIDs, nil
}

// List returns audit logs matching the query parameters.
// GET /v1/admin/audit?app_id=&actor_id=&action=&resource=&limit=&offset=
// @Summary List audit logs
// @Description Query audit logs with filters for app, actor, action, resource, and pagination
// @Tags Audit
// @Produce json
// @Param app_id query string false "Filter by application ID"
// @Param actor_id query string false "Filter by actor ID"
// @Param action query string false "Filter by action"
// @Param resource query string false "Filter by resource type"
// @Param resource_id query string false "Filter by resource ID"
// @Param environment_id query string false "Filter by environment ID"
// @Param from_date query string false "Start date (YYYY-MM-DD in UTC) or RFC3339 timestamp, inclusive"
// @Param to_date query string false "End date (YYYY-MM-DD in UTC, whole day) or inclusive RFC3339 timestamp"
// @Param limit query int false "Limit results" default(50)
// @Param offset query int false "Offset for pagination" default(0)
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 403 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /v1/admin/audit [get]
// @Security BearerAuth
func (h *AuditHandler) List(c *fiber.Ctx) error {
	appIDs, err := h.getAdminAppIDs(c)
	if err != nil {
		return err
	}
	from, to, exclusive, err := parseAuditDateRange(c.Query("from_date"), c.Query("to_date"))
	if err != nil {
		return err
	}

	if len(appIDs) == 0 {
		return c.JSON(fiber.Map{"audit_logs": []interface{}{}, "count": 0})
	}

	filter := audit.DefaultFilter()
	filter.AppIDs = appIDs
	filter.FromDate, filter.ToDate, filter.ToDateExclusive = from, to, exclusive

	// If a specific app_id is requested, validate it belongs to this admin
	if v := c.Query("app_id"); v != "" {
		owned := false
		for _, id := range appIDs {
			if id == v {
				owned = true
				break
			}
		}
		if !owned {
			return errors.Forbidden("you do not have access to this application's audit logs")
		}
		filter.AppID = v
	}

	if v := c.Query("actor_id"); v != "" {
		filter.ActorID = v
	}
	if v := c.Query("action"); v != "" {
		filter.Action = v
	}
	if v := c.Query("resource"); v != "" {
		filter.Resource = v
	}
	if v := c.Query("resource_id"); v != "" {
		filter.ResourceID = v
	}
	if v := c.Query("environment_id"); v != "" {
		filter.EnvironmentID = v
	}
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			filter.Limit = n
		}
	}
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			filter.Offset = n
		}
	}

	logs, err := h.service.List(c.Context(), filter)
	if err != nil {
		return errors.Internal("failed to list audit logs", err)
	}

	return c.JSON(fiber.Map{
		"audit_logs": logs,
		"count":      len(logs),
	})
}

func parseAuditDateRange(from, to string) (*time.Time, *time.Time, bool, error) {
	parse := func(value string, end bool) (*time.Time, bool, error) {
		if value == "" {
			return nil, false, nil
		}
		if len(value) == len("2006-01-02") {
			parsed, err := time.Parse("2006-01-02", value)
			if err != nil {
				return nil, false, errors.BadRequest("invalid audit date; use YYYY-MM-DD or RFC3339")
			}
			if end {
				parsed = parsed.AddDate(0, 0, 1)
				if parsed.Year() > 9999 {
					return nil, false, errors.BadRequest("audit end date exceeds supported range")
				}
			}
			return &parsed, end, nil
		}
		parsed, err := time.Parse(time.RFC3339Nano, value)
		if err != nil {
			return nil, false, errors.BadRequest("invalid audit date; use YYYY-MM-DD or RFC3339")
		}
		parsed = parsed.UTC()
		return &parsed, false, nil
	}
	start, _, err := parse(from, false)
	if err != nil {
		return nil, nil, false, err
	}
	end, exclusive, err := parse(to, true)
	if err != nil {
		return nil, nil, false, err
	}
	if start != nil && end != nil && (start.After(*end) || (exclusive && start.Equal(*end))) {
		return nil, nil, false, errors.BadRequest("audit start date must not be after end date")
	}
	return start, end, exclusive, nil
}

// Get returns a single audit log by ID.
// GET /v1/admin/audit/:id
// @Summary Get an audit log entry
// @Description Retrieve a single audit log entry by its ID
// @Tags Audit
// @Produce json
// @Param id path string true "Audit Log ID"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /v1/admin/audit/{id} [get]
// @Security BearerAuth
func (h *AuditHandler) Get(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return errors.BadRequest("audit log id is required")
	}

	appIDs, err := h.getAdminAppIDs(c)
	if err != nil {
		return err
	}

	log, err := h.service.Get(c.Context(), id)
	if err != nil {
		return errors.NotFound("audit_log", id)
	}

	// Verify the audit log belongs to one of the admin's apps
	owned := false
	for _, appID := range appIDs {
		if appID == log.AppID {
			owned = true
			break
		}
	}
	if !owned {
		return errors.NotFound("audit_log", id)
	}

	return c.JSON(log)
}
