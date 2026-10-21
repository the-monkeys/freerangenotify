package handlers

import (
	"bytes"
	"context"
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/middleware"
	"github.com/the-monkeys/freerangenotify/pkg/validator"
	"go.uber.org/zap"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

type channelSettingsMembers struct {
	auth.MembershipRepository
	role auth.Role
}

func TestChannelSettingsNamedWebhookMaskedRoundTrip(t *testing.T) {
	svc := &fakeApplicationService{app: &application.Application{AppID: "a", AdminUserID: "owner", Webhooks: map[string]string{"ops": "https://hook.example.test/private?token=fixture"}}}
	h := NewApplicationHandler(svc, nil, nil, nil, validator.New(), zap.NewNop())
	app := fiber.New(fiber.Config{ErrorHandler: middleware.ErrorHandler(zap.NewNop())})
	app.Use(func(c *fiber.Ctx) error { c.Locals("user_id", "owner"); return c.Next() })
	app.Put("/apps/:id", h.Update)
	req := httptest.NewRequest("PUT", "/apps/a", strings.NewReader(`{"webhooks":{"ops":"https://hook.example.test"}}`))
	req.Header.Set("Content-Type", "application/json")
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 || svc.app.Webhooks["ops"] != "https://hook.example.test/private?token=fixture" {
		t.Fatal("safe endpoint round-trip erased named webhook credentials")
	}
}

func TestChannelSettingsNestedPatchKeepsOmittedFields(t *testing.T) {
	svc := &fakeApplicationService{app: &application.Application{AppID: "a", AdminUserID: "owner", Settings: application.Settings{WhatsApp: &application.WhatsAppAppConfig{Provider: "meta", MetaWABAID: "waba", MetaPhoneNumberID: "phone", MetaAccessToken: "fixture-old-token"}}}}
	h := NewApplicationHandler(svc, nil, nil, nil, validator.New(), zap.NewNop())
	app := fiber.New(fiber.Config{ErrorHandler: middleware.ErrorHandler(zap.NewNop())})
	app.Use(func(c *fiber.Ctx) error { c.Locals("user_id", "owner"); return c.Next() })
	app.Put("/apps/:id/settings", h.UpdateSettings)
	req := httptest.NewRequest("PUT", "/apps/a/settings", strings.NewReader(`{"whatsapp_config":{"meta_access_token":"fixture-new-token"}}`))
	req.Header.Set("Content-Type", "application/json")
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	cfg := svc.app.Settings.WhatsApp
	if res.StatusCode != 200 || cfg.Provider != "meta" || cfg.MetaWABAID != "waba" || cfg.MetaPhoneNumberID != "phone" || cfg.MetaAccessToken != "fixture-new-token" {
		t.Fatal("nested patch erased omitted sender settings")
	}
}

func (r *channelSettingsMembers) GetByAppAndUser(context.Context, string, string) (*auth.AppMembership, error) {
	return &auth.AppMembership{Role: r.role}, nil
}
func TestChannelAuthorizationReadAndReveal(t *testing.T) {
	for _, role := range []auth.Role{auth.RoleOwner, auth.RoleAdmin, auth.RoleEditor, auth.RoleViewer} {
		t.Run(string(role), func(t *testing.T) {
			svc := &fakeApplicationService{app: &application.Application{AppID: "a", AdminUserID: "owner", APIKey: "fixture-master-key", Settings: application.Settings{SMS: &application.SMSAppConfig{AuthToken: "fixture-channel-token"}}}}
			h := NewApplicationHandler(svc, &channelSettingsMembers{role: role}, nil, nil, validator.New(), zap.NewNop())
			app := fiber.New(fiber.Config{ErrorHandler: middleware.ErrorHandler(zap.NewNop())})
			app.Use(func(c *fiber.Ctx) error {
				user := "member"
				if role == auth.RoleOwner {
					user = "owner"
				}
				c.Locals("user_id", user)
				return c.Next()
			})
			app.Get("/apps/:id", h.GetByID)
			app.Post("/apps/:id/reveal", h.RevealAPIKey)
			res, err := app.Test(httptest.NewRequest("GET", "/apps/a", nil))
			if err != nil {
				t.Fatal(err)
			}
			raw, _ := io.ReadAll(res.Body)
			res.Body.Close()
			if res.StatusCode != 200 || strings.Contains(string(raw), "fixture-master-key") || strings.Contains(string(raw), "fixture-channel-token") {
				t.Fatal("ordinary read exposed credentials or denied allowed role")
			}
			res, err = app.Test(httptest.NewRequest("POST", "/apps/a/reveal", nil))
			if err != nil {
				t.Fatal(err)
			}
			raw, _ = io.ReadAll(res.Body)
			res.Body.Close()
			if role == auth.RoleOwner {
				if res.StatusCode != 200 || res.Header.Get("Cache-Control") != "no-store" || !strings.Contains(string(raw), "fixture-master-key") {
					t.Fatal("explicit owner reveal failed")
				}
			} else if res.StatusCode != 403 || strings.Contains(string(raw), "fixture-master-key") {
				t.Fatal("non-owner revealed master key")
			}
		})
	}
}

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
