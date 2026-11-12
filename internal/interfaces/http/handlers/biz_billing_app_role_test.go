package handlers

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"github.com/the-monkeys/freerangenotify/internal/usecases"
	"github.com/the-monkeys/freerangenotify/pkg/validator"
	"go.uber.org/zap"
)

type billingRoleApps struct{ usecases.ApplicationService }

func (billingRoleApps) GetByID(context.Context, string) (*application.Application, error) {
	return &application.Application{AppID: "billing-app", AdminUserID: "owner", AppName: "Business"}, nil
}

type billingRoleMembers struct {
	auth.MembershipRepository
	role auth.Role
}

func (r billingRoleMembers) GetByAppAndUser(_ context.Context, app, user string) (*auth.AppMembership, error) {
	return &auth.AppMembership{AppID: app, UserID: user, Role: r.role}, nil
}

func TestBizApplicationReadCarriesResolvedBillingRole(t *testing.T) {
	for _, role := range []auth.Role{auth.RoleOwner, auth.RoleAdmin, auth.RoleEditor, auth.RoleViewer} {
		t.Run(string(role), func(t *testing.T) {
			h := NewApplicationHandler(billingRoleApps{}, billingRoleMembers{role: role}, nil, nil, validator.New(), zap.NewNop())
			app := fiber.New()
			app.Use(func(c *fiber.Ctx) error { c.Locals("user_id", string(role)); return c.Next() })
			app.Get("/apps/:id", h.GetByID)
			res, err := app.Test(httptest.NewRequest("GET", "/apps/billing-app", nil))
			require.NoError(t, err)
			defer res.Body.Close()
			require.Equal(t, 200, res.StatusCode)
			var body struct {
				Data struct {
					Role  string `json:"role"`
					AppID string `json:"app_id"`
				} `json:"data"`
			}
			require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
			require.Equal(t, string(role), body.Data.Role)
			require.Equal(t, "billing-app", body.Data.AppID)
		})
	}
}
