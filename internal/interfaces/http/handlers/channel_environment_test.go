package handlers

import (
	"context"
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/environment"
	"github.com/the-monkeys/freerangenotify/pkg/validator"
	"go.uber.org/zap"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

type channelEnvironments struct{ environment.Service }

func (*channelEnvironments) ListByApp(context.Context, string) ([]environment.Environment, error) {
	return []environment.Environment{{ID: "env", AppID: "a", APIKey: "fixture-environment-key"}}, nil
}
func TestChannelSettingsEnvironmentSecrets(t *testing.T) {
	h := NewEnvironmentHandler(&channelEnvironments{}, nil, zap.NewNop())
	h.SetAppHandler(NewApplicationHandler(&fakeApplicationService{app: &application.Application{AppID: "a", AdminUserID: "owner"}}, nil, nil, nil, validator.New(), zap.NewNop()))
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error { c.Locals("user_id", "owner"); return c.Next() })
	app.Get("/apps/:id/environments", h.List)
	res, err := app.Test(httptest.NewRequest("GET", "/apps/a/environments", nil))
	if err != nil {
		t.Fatal(err)
	}
	out, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || !strings.Contains(string(out), "dashboard:a:env") {
		t.Fatal("authorized environment read did not retain safe reference")
	}
	if strings.Contains(string(out), "fixture-environment-key") {
		t.Fatal("environment read exposes bearer credential")
	}
}
