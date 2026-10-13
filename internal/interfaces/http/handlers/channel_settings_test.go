package handlers

import (
	"bytes"
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/middleware"
	"github.com/the-monkeys/freerangenotify/pkg/validator"
	"go.uber.org/zap"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestChannelSettingsSecretsPreserved(t *testing.T) {
	for _, body := range []string{`{"sms_config":{"from_number":"+456"}}`, `{"sms_config":{"from_number":"+456","auth_token":""}}`, `{"sms_config":{"from_number":"+456","auth_token":"****"}}`} {
		svc := &fakeApplicationService{app: &application.Application{AppID: "a", AdminUserID: "owner", Settings: application.Settings{SMS: &application.SMSAppConfig{AccountSID: "account", AuthToken: "fixture-token", FromNumber: "+123"}}}}
		h := NewApplicationHandler(svc, nil, nil, nil, validator.New(), zap.NewNop())
		app := fiber.New(fiber.Config{ErrorHandler: middleware.ErrorHandler(zap.NewNop())})
		app.Use(func(c *fiber.Ctx) error { c.Locals("user_id", "owner"); return c.Next() })
		app.Put("/apps/:id/settings", h.UpdateSettings)
		app.Get("/apps/:id/settings", h.GetSettings)
		req := httptest.NewRequest("PUT", "/apps/a/settings", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		res, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 200 || svc.app.Settings.SMS.AuthToken != "fixture-token" || svc.app.Settings.SMS.AccountSID != "account" || svc.app.Settings.SMS.FromNumber != "+456" {
			t.Fatalf("secret lost or replaced: status=%d", res.StatusCode)
		}
		res, err = app.Test(httptest.NewRequest("GET", "/apps/a/settings", nil))
		if err != nil {
			t.Fatal(err)
		}
		out, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if strings.Contains(string(out), "fixture-token") {
			t.Fatal("settings GET exposes token")
		}
	}
}

func TestChannelSettingsExplicitClear(t *testing.T) {
	svc := &fakeApplicationService{app: &application.Application{AppID: "a", AdminUserID: "owner", Settings: application.Settings{SMS: &application.SMSAppConfig{AccountSID: "account", AuthToken: "fixture-token"}}}}
	h := NewApplicationHandler(svc, nil, nil, nil, validator.New(), zap.NewNop())
	app := fiber.New(fiber.Config{ErrorHandler: middleware.ErrorHandler(zap.NewNop())})
	app.Use(func(c *fiber.Ctx) error { c.Locals("user_id", "owner"); return c.Next() })
	app.Put("/apps/:id/settings", h.UpdateSettings)
	req := httptest.NewRequest("PUT", "/apps/a/settings", strings.NewReader(`{"clear_secrets":["sms_config.auth_token"]}`))
	req.Header.Set("Content-Type", "application/json")
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 || svc.app.Settings.SMS.AuthToken != "" {
		t.Fatal("explicit clear was not applied")
	}
}
