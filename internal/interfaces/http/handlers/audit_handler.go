package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/audit"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// AuditHandler exposes admin-level endpoints for querying audit logs.
type AuditHandler struct {
	service audit.Service
	logger  *zap.Logger
}

// NewAuditHandler creates a new AuditHandler.
func NewAuditHandler(service audit.Service, logger *zap.Logger) *AuditHandler {
	return &AuditHandler{service: service, logger: logger}
}

// List returns audit logs matching the query parameters.
// GET /v1/admin/audit?app_id=&actor_id=&action=&resource=&limit=&offset=
func (h *AuditHandler) List(c *fiber.Ctx) error {
	filter := audit.DefaultFilter()

	if v := c.Query("app_id"); v != "" {
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

	log, err := h.service.Get(c.Context(), id)
	if err != nil {
		return errors.NotFound("audit_log", id)
	}

	return c.JSON(log)
}
