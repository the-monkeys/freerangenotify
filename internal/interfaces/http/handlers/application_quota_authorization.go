package handlers

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/dto"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
)

// Match the permissions enforced by the endpoint before admitting its HTTP
// request. Membership in the app alone does not authorize every admin action.
func (h *ApplicationHandler) authorizeAppQuotaOperation(c *fiber.Ctx, app *application.Application, role auth.Role, userID string) error {
	suffix := strings.TrimPrefix(strings.TrimSuffix(c.Path(), "/"), "/v1/apps/"+c.Params("id"))
	readOnly := c.Method() == fiber.MethodGet || c.Method() == fiber.MethodHead || c.Method() == fiber.MethodOptions
	if (suffix == "" && c.Method() == fiber.MethodDelete) || (suffix == "/regenerate-key" && c.Method() == fiber.MethodPost) {
		if role != auth.RoleOwner {
			return errors.Forbidden("only the application owner can perform this action")
		}
	}
	if c.Method() == fiber.MethodPut && (suffix == "" || suffix == "/settings") {
		if role != auth.RoleOwner && role != auth.RoleAdmin {
			return errors.Forbidden("admin or owner role required")
		}
		if suffix == "" {
			var update dto.UpdateApplicationRequest
			if len(c.Body()) > 0 && c.BodyParser(&update) == nil && update.TenantID != nil && *update.TenantID != app.TenantID {
				if role != auth.RoleOwner {
					return errors.Forbidden("only the application owner can change the organization")
				}
				if *update.TenantID != "" && h.tenantService != nil {
					allowed, targetRole, err := h.tenantService.HasAccess(c.Context(), *update.TenantID, userID)
					if err != nil {
						return err
					}
					if !allowed || (targetRole != "owner" && targetRole != "admin") {
						return errors.Forbidden("only tenant owners/admins can move an application")
					}
				}
			}
		}
	}
	provider := strings.HasPrefix(suffix, "/providers")
	team := strings.HasPrefix(suffix, "/team")
	imports := suffix == "/import" || strings.HasPrefix(suffix, "/links")
	if provider || team || imports {
		// These endpoints currently authorize app membership, not organization
		// membership; granting an org role here would still fail after admission.
		appRole := auth.RoleOwner
		if app.AdminUserID != userID {
			appRole = ""
			if h.membershipRepo != nil {
				m, err := h.membershipRepo.GetByAppAndUser(c.Context(), app.AppID, userID)
				if err == nil && m != nil {
					appRole = m.Role
				}
			}
		}
		if appRole == "" || ((!readOnly || team || imports) && appRole != auth.RoleOwner && appRole != auth.RoleAdmin) {
			return errors.Forbidden("application permission required")
		}
		if suffix == "/import" && c.Method() == fiber.MethodPost {
			var request struct {
				SourceAppID string `json:"source_app_id"`
			}
			if c.BodyParser(&request) == nil && request.SourceAppID != "" {
				source, err := h.service.GetByID(c.Context(), request.SourceAppID)
				if err != nil {
					return err
				}
				if source == nil {
					return errors.Forbidden("source application unavailable")
				}
				if source.AdminUserID != userID {
					if h.membershipRepo == nil {
						return errors.Forbidden("source application permission required")
					}
					m, err := h.membershipRepo.GetByAppAndUser(c.Context(), source.AppID, userID)
					if err != nil || m == nil || (m.Role != auth.RoleOwner && m.Role != auth.RoleAdmin) {
						return errors.Forbidden("source application permission required")
					}
				}
			}
		}
	}
	return nil
}
