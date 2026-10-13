package handlers

import (
	"context"
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/environment"
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
	app := fiber.New()
	app.Get("/apps/:id/environments", h.List)
	res, err := app.Test(httptest.NewRequest("GET", "/apps/a/environments", nil))
	if err != nil {
		t.Fatal(err)
	}
	out, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if strings.Contains(string(out), "fixture-environment-key") {
		t.Fatal("environment read exposes bearer credential")
	}
}
