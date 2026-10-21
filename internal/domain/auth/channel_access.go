package auth

import (
	"context"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/tenant"
)

// ApplicationRole is shared by channel configuration, callbacks' SSE audiences,
// and JWT-only dashboard requests. Repository failures never grant access.
func ApplicationRole(ctx context.Context, app *application.Application, userID string, members MembershipRepository, tenants tenant.Service) Role {
	if app == nil || userID == "" {
		return ""
	}
	if app.AdminUserID == userID {
		return RoleOwner
	}
	if members != nil {
		m, err := members.GetByAppAndUser(ctx, app.AppID, userID)
		if err == nil && m != nil {
			return m.Role
		}
	}
	if tenants != nil && app.TenantID != "" {
		allowed, role, err := tenants.HasAccess(ctx, app.TenantID, userID)
		if err == nil && allowed {
			if role == "owner" || role == "admin" {
				return RoleAdmin
			}
			return RoleViewer
		}
	}
	return ""
}
