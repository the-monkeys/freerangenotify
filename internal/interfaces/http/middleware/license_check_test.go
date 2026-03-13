package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
	"go.uber.org/zap"
)

type fakeChecker struct {
	enabled  bool
	mode     license.Mode
	decision license.Decision
	err      error
}

func (f *fakeChecker) Enabled() bool      { return f.enabled }
func (f *fakeChecker) Mode() license.Mode { return f.mode }
func (f *fakeChecker) Check(_ context.Context, _ *application.Application) (license.Decision, error) {
	if f.err != nil {
		return license.Decision{}, f.err
	}
	return f.decision, nil
}

func TestLicenseCheck_AllowsWhenDisabled(t *testing.T) {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("app", &application.Application{AppID: "app-1"})
		return c.Next()
	})
	app.Post("/licensed", LicenseCheck(&fakeChecker{enabled: false}, zap.NewNop()), func(c *fiber.Ctx) error {
		return c.SendStatus(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/licensed", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestLicenseCheck_BlocksHostedWith402(t *testing.T) {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("app", &application.Application{AppID: "app-1"})
		return c.Next()
	})
	app.Post("/licensed", LicenseCheck(&fakeChecker{
		enabled: true,
		mode:    license.ModeHosted,
		decision: license.Decision{
			Allowed:   false,
			Mode:      license.ModeHosted,
			State:     license.StateUnlicensed,
			Reason:    "subscription_required",
			CheckedAt: time.Now().UTC(),
		},
	}, zap.NewNop()), func(c *fiber.Ctx) error {
		return c.SendStatus(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/licensed", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusPaymentRequired, resp.StatusCode)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "subscription_required", body["code"])
}

func TestLicenseCheck_BlocksSelfHostedWith402(t *testing.T) {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("app", &application.Application{AppID: "app-1"})
		return c.Next()
	})
	app.Post("/licensed", LicenseCheck(&fakeChecker{
		enabled: true,
		mode:    license.ModeSelfHosted,
		decision: license.Decision{
			Allowed:   false,
			Mode:      license.ModeSelfHosted,
			State:     license.StateUnlicensed,
			Reason:    "license_required",
			CheckedAt: time.Now().UTC(),
		},
	}, zap.NewNop()), func(c *fiber.Ctx) error {
		return c.SendStatus(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/licensed", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusPaymentRequired, resp.StatusCode)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "license_required", body["code"])
}

func TestLicenseCheck_Returns500OnCheckerError(t *testing.T) {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("app", &application.Application{AppID: "app-1"})
		return c.Next()
	})
	app.Post("/licensed", LicenseCheck(&fakeChecker{enabled: true, mode: license.ModeHosted, err: errors.New("boom")}, zap.NewNop()), func(c *fiber.Ctx) error {
		return c.SendStatus(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/licensed", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}
