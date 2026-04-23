package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	httpmiddleware "github.com/the-monkeys/freerangenotify/internal/interfaces/http/middleware"
	"github.com/the-monkeys/freerangenotify/internal/usecases"
	"go.uber.org/zap/zaptest"
)

type fakeApplicationService struct {
	app         *application.Application
	updateCalls int
}

var _ usecases.ApplicationService = (*fakeApplicationService)(nil)

func (f *fakeApplicationService) Create(context.Context, *application.Application) error { return nil }
func (f *fakeApplicationService) GetByID(context.Context, string) (*application.Application, error) {
	return f.app, nil
}
func (f *fakeApplicationService) GetByAPIKey(context.Context, string) (*application.Application, error) {
	return nil, nil
}
func (f *fakeApplicationService) Update(context.Context, *application.Application) error { return nil }
func (f *fakeApplicationService) Delete(context.Context, string) error                   { return nil }
func (f *fakeApplicationService) List(context.Context, application.ApplicationFilter) ([]*application.Application, int64, error) {
	return nil, 0, nil
}
func (f *fakeApplicationService) RegenerateAPIKey(context.Context, string) (string, error) {
	return "", nil
}
func (f *fakeApplicationService) ValidateAPIKey(context.Context, string) (*application.Application, error) {
	return nil, nil
}
func (f *fakeApplicationService) UpdateSettings(_ context.Context, _ string, settings application.Settings) error {
	f.updateCalls++
	f.app.Settings = settings
	return nil
}
func (f *fakeApplicationService) GetSettings(context.Context, string) (*application.Settings, error) {
	return &f.app.Settings, nil
}

func setupCustomProviderTestApp(t *testing.T, svc *fakeApplicationService, adminUserID string) *fiber.App {
	t.Helper()
	logger := zaptest.NewLogger(t)
	handler := NewCustomProviderHandler(svc, nil, logger)

	app := fiber.New(fiber.Config{
		ErrorHandler: httpmiddleware.ErrorHandler(logger),
	})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", adminUserID)
		return c.Next()
	})
	app.Post("/apps/:id/providers", handler.Register)
	app.Post("/apps/:id/providers/:provider_id/test", handler.Test)
	app.Post("/apps/:id/providers/:provider_id/rotate", handler.RotateSigningKey)

	return app
}

func TestCustomProviderHandler_RegisterRejectsNonHTTPSNonLocalhost(t *testing.T) {
	svc := &fakeApplicationService{
		app: &application.Application{
			AppID:       "app-1",
			AdminUserID: "admin-1",
		},
	}
	app := setupCustomProviderTestApp(t, svc, "admin-1")

	body := map[string]interface{}{
		"name":        "bad-http-provider",
		"channel":     "webhook",
		"webhook_url": "http://example.com/hook",
	}
	payload, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/apps/app-1/providers", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCustomProviderHandler_TestEndpoint(t *testing.T) {
	hit := 0
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit++
		w.WriteHeader(http.StatusOK)
	}))
	defer receiver.Close()

	svc := &fakeApplicationService{
		app: &application.Application{
			AppID:       "app-1",
			AdminUserID: "admin-1",
			Settings: application.Settings{
				CustomProviders: []application.CustomProviderConfig{
					{
						ProviderID:       "provider-1",
						Name:             "test-provider",
						Channel:          "webhook",
						Kind:             "generic",
						WebhookURL:       receiver.URL,
						SignatureVersion: "v1",
						Active:           true,
						CreatedAt:        time.Now().UTC().Format(time.RFC3339),
					},
				},
			},
		},
	}
	app := setupCustomProviderTestApp(t, svc, "admin-1")

	req := httptest.NewRequest(http.MethodPost, "/apps/app-1/providers/provider-1/test", nil)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if hit != 1 {
		t.Fatalf("expected receiver hit once, got %d", hit)
	}
}

func TestCustomProviderHandler_TestEndpointProviderNotFound(t *testing.T) {
	svc := &fakeApplicationService{
		app: &application.Application{
			AppID:       "app-1",
			AdminUserID: "admin-1",
			Settings: application.Settings{
				CustomProviders: []application.CustomProviderConfig{},
			},
		},
	}
	app := setupCustomProviderTestApp(t, svc, "admin-1")

	req := httptest.NewRequest(http.MethodPost, "/apps/app-1/providers/missing/test", nil)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCustomProviderHandler_TestEndpointRejectsInactiveProvider(t *testing.T) {
	svc := &fakeApplicationService{
		app: &application.Application{
			AppID:       "app-1",
			AdminUserID: "admin-1",
			Settings: application.Settings{
				CustomProviders: []application.CustomProviderConfig{
					{
						ProviderID: "provider-1",
						Name:       "inactive-provider",
						Channel:    "webhook",
						WebhookURL: "https://example.com/hook",
						Active:     false,
					},
				},
			},
		},
	}
	app := setupCustomProviderTestApp(t, svc, "admin-1")

	req := httptest.NewRequest(http.MethodPost, "/apps/app-1/providers/provider-1/test", nil)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCustomProviderHandler_TestEndpointProviderDeliveryFailure(t *testing.T) {
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer receiver.Close()

	svc := &fakeApplicationService{
		app: &application.Application{
			AppID:       "app-1",
			AdminUserID: "admin-1",
			Settings: application.Settings{
				CustomProviders: []application.CustomProviderConfig{
					{
						ProviderID:       "provider-1",
						Name:             "failing-provider",
						Channel:          "webhook",
						Kind:             "generic",
						WebhookURL:       receiver.URL,
						SignatureVersion: "v1",
						Active:           true,
						CreatedAt:        time.Now().UTC().Format(time.RFC3339),
					},
				},
			},
		},
	}
	app := setupCustomProviderTestApp(t, svc, "admin-1")

	req := httptest.NewRequest(http.MethodPost, "/apps/app-1/providers/provider-1/test", nil)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

func TestCustomProviderHandler_TestEndpointForbiddenForNonAdminNonOwner(t *testing.T) {
	svc := &fakeApplicationService{
		app: &application.Application{
			AppID:       "app-1",
			AdminUserID: "admin-1",
			Settings: application.Settings{
				CustomProviders: []application.CustomProviderConfig{
					{
						ProviderID: "provider-1",
						Name:       "provider",
						Channel:    "webhook",
						WebhookURL: "https://example.com/hook",
						Active:     true,
					},
				},
			},
		},
	}
	app := setupCustomProviderTestApp(t, svc, "non-admin-user")

	req := httptest.NewRequest(http.MethodPost, "/apps/app-1/providers/provider-1/test", nil)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestCustomProviderHandler_RotateSigningKey(t *testing.T) {
	oldKey := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	svc := &fakeApplicationService{
		app: &application.Application{
			AppID:       "app-1",
			AdminUserID: "admin-1",
			Settings: application.Settings{
				CustomProviders: []application.CustomProviderConfig{
					{
						ProviderID: "provider-1",
						Name:       "provider",
						Channel:    "webhook",
						WebhookURL: "https://example.com/hook",
						SigningKey: oldKey,
						Active:     true,
					},
				},
			},
		},
	}
	app := setupCustomProviderTestApp(t, svc, "admin-1")

	req := httptest.NewRequest(http.MethodPost, "/apps/app-1/providers/provider-1/rotate", nil)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if svc.updateCalls != 1 {
		t.Fatalf("expected one UpdateSettings call, got %d", svc.updateCalls)
	}
	newKey := svc.app.Settings.CustomProviders[0].SigningKey
	if newKey == "" || newKey == oldKey {
		t.Fatalf("expected rotated signing key to differ from old one")
	}
}

func TestCustomProviderHandler_RotateSigningKeyProviderNotFound(t *testing.T) {
	svc := &fakeApplicationService{
		app: &application.Application{
			AppID:       "app-1",
			AdminUserID: "admin-1",
			Settings: application.Settings{
				CustomProviders: []application.CustomProviderConfig{},
			},
		},
	}
	app := setupCustomProviderTestApp(t, svc, "admin-1")

	req := httptest.NewRequest(http.MethodPost, "/apps/app-1/providers/missing/rotate", nil)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	if svc.updateCalls != 0 {
		t.Fatalf("expected no UpdateSettings calls, got %d", svc.updateCalls)
	}
}

func TestCustomProviderHandler_RotateSigningKeyReturnsKeyInBody(t *testing.T) {
	oldKey := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	svc := &fakeApplicationService{
		app: &application.Application{
			AppID:       "app-1",
			AdminUserID: "admin-1",
			Settings: application.Settings{
				CustomProviders: []application.CustomProviderConfig{
					{
						ProviderID: "provider-1",
						Name:       "provider",
						Channel:    "webhook",
						WebhookURL: "https://example.com/hook",
						SigningKey: oldKey,
						Active:     true,
					},
				},
			},
		},
	}
	app := setupCustomProviderTestApp(t, svc, "admin-1")

	req := httptest.NewRequest(http.MethodPost, "/apps/app-1/providers/provider-1/rotate", nil)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var payload struct {
		Success bool `json:"success"`
		Data    struct {
			ProviderID string `json:"provider_id"`
			SigningKey string `json:"signing_key"`
		} `json:"data"`
	}
	raw, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		t.Fatalf("failed reading response: %v", readErr)
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("failed parsing response: %v", err)
	}
	if !payload.Success {
		t.Fatalf("expected success response")
	}
	if payload.Data.ProviderID != "provider-1" {
		t.Fatalf("expected provider_id provider-1, got %q", payload.Data.ProviderID)
	}
	if payload.Data.SigningKey == "" || payload.Data.SigningKey == oldKey {
		t.Fatalf("expected a rotated signing key in response")
	}
}

func TestCustomProviderHandler_RotateSigningKeyForbiddenForNonAdminNonOwner(t *testing.T) {
	svc := &fakeApplicationService{
		app: &application.Application{
			AppID:       "app-1",
			AdminUserID: "admin-1",
			Settings: application.Settings{
				CustomProviders: []application.CustomProviderConfig{
					{
						ProviderID: "provider-1",
						Name:       "provider",
						Channel:    "webhook",
						WebhookURL: "https://example.com/hook",
						Active:     true,
					},
				},
			},
		},
	}
	app := setupCustomProviderTestApp(t, svc, "non-admin-user")

	req := httptest.NewRequest(http.MethodPost, "/apps/app-1/providers/provider-1/rotate", nil)
	resp, err := app.Test(req, 5000)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}
