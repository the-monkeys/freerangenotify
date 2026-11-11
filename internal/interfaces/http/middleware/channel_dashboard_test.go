package middleware

import (
	"context"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"github.com/the-monkeys/freerangenotify/internal/domain/tenant"
	"github.com/the-monkeys/freerangenotify/internal/usecases"
	"go.uber.org/zap"
	"net/http/httptest"
	"testing"
)

type channelDashboardApp struct{ usecases.ApplicationService }

func (*channelDashboardApp) GetByID(context.Context, string) (*application.Application, error) {
	return &application.Application{AppID: "a", AdminUserID: "owner"}, nil
}

type channelRoleMembers struct{ auth.MembershipRepository }

func (*channelRoleMembers) GetByAppAndUser(context.Context, string, string) (*auth.AppMembership, error) {
	return &auth.AppMembership{Role: auth.RoleEditor}, nil
}

type channelRoleTenant struct{ tenant.Service }

func (*channelRoleTenant) HasAccess(context.Context, string, string) (bool, string, error) {
	return true, "member", nil
}

type channelRoleAppRepo struct{ application.Repository }

func (*channelRoleAppRepo) GetByID(context.Context, string) (*application.Application, error) {
	return &application.Application{AppID: "a", TenantID: "t", AdminUserID: "owner"}, nil
}
func TestChannelAuthorizationSharedEditorRole(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler(zap.NewNop())})
	app.Use(func(c *fiber.Ctx) error { c.Locals("user_id", "editor"); c.Locals("app_id", "a"); return c.Next() })
	app.Use(DashboardRBAC(&channelRoleMembers{}, &channelRoleAppRepo{}, zap.NewNop(), &channelRoleTenant{}))
	app.Post("/", func(c *fiber.Ctx) error { return c.SendStatus(200) })
	res, err := app.Test(httptest.NewRequest("POST", "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("explicit app editor shadowed by tenant membership: %d", res.StatusCode)
	}
}
func (*channelDashboardApp) ValidateAPIKey(_ context.Context, key string) (*application.Application, error) {
	if key != "fixture-key" {
		return nil, fmt.Errorf("invalid")
	}
	return &application.Application{AppID: "a", AdminUserID: "owner"}, nil
}

type channelDashboardAuth struct{ auth.Service }

func (*channelDashboardAuth) ValidateToken(_ context.Context, token string) (*auth.AdminUser, error) {
	if token == "owner" {
		return &auth.AdminUser{UserID: "owner"}, nil
	}
	return nil, fmt.Errorf("invalid")
}
func TestChannelAuthorizationDashboard(t *testing.T) {
	for _, tc := range []struct {
		key, jwt string
		want     int
	}{{"dashboard:a", "owner", 200}, {"dashboard:a", "", 401}, {"dashboard:a", "invalid", 401}, {"fixture-key", "", 200}} {
		app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler(zap.NewNop())})
		app.Use(APIKeyAuth(&channelDashboardApp{}, zap.NewNop(), WithAuthService(&channelDashboardAuth{})))
		app.Get("/", func(c *fiber.Ctx) error { return c.SendStatus(200) })
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-API-Key", tc.key)
		if tc.jwt != "" {
			req.Header.Set("Authorization", "Bearer "+tc.jwt)
		}
		res, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != tc.want {
			t.Errorf("key=%s jwt=%s status=%d want=%d", tc.key, tc.jwt, res.StatusCode, tc.want)
		}
	}
}
