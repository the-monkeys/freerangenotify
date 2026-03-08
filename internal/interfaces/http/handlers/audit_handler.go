package handlers

import (
	"strconv"

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
func (h *AuditHandler) List(c *fiber.Ctx) error {
	appIDs, err := h.getAdminAppIDs(c)
	if err != nil {
		return err
	}

	if len(appIDs) == 0 {
		return c.JSON(fiber.Map{"audit_logs": []interface{}{}, "count": 0})
	}

	filter := audit.DefaultFilter()
	filter.AppIDs = appIDs

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

// Get returns a single audit log by ID.
// GET /v1/admin/audit/:id
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
