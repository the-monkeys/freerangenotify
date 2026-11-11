package handlers

import (
	"context"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/middleware"
	"go.uber.org/zap"
)

type merchantMemberships struct {
	auth.MembershipRepository
	role auth.Role
}

func (r merchantMemberships) GetByAppAndUser(_ context.Context, app, user string) (*auth.AppMembership, error) {
	return &auth.AppMembership{AppID: app, UserID: user, Role: r.role}, nil
}

func TestBizMerchantHandlerWritePermissions(t *testing.T) {
	for _, method := range []string{"GET", "PUT", "DELETE"} {
		handler := NewBizBillingHandler(BizBillingServices{}, zap.NewNop())
		app := fiber.New(fiber.Config{ErrorHandler: handler.handleError})
		app.Use(func(c *fiber.Ctx) error {
			c.Locals("app_id", "owned-app")
			c.Locals("user_id", "viewer")
			return c.Next()
		})
		app.Use(middleware.RequirePermissionForWrites(auth.PermManageBilling, merchantMemberships{role: auth.RoleViewer}, nil, zap.NewNop()))
		app.Get("/connection/:mode", handler.GetMerchantConnection)
		app.Put("/connection/:mode", handler.SaveMerchantConnection)
		app.Delete("/connection/:mode", handler.DisconnectMerchantConnection)
		request := httptest.NewRequest(method, "/connection/test", strings.NewReader(`{"expected_revision":0,"key_secret":"must-never-echo"}`))
		request.Header.Set("Content-Type", "application/json")
		response, err := app.Test(request)
		require.NoError(t, err)
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		require.NotContains(t, string(body), "must-never-echo")
		if method == "GET" {
			require.Equal(t, 503, response.StatusCode, "read access reaches the unavailable service without credentials")
		} else {
			require.Equal(t, 403, response.StatusCode, "viewer cannot connect or disconnect a payment account")
		}
	}
}
