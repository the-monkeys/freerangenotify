package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestOpsAuth_AllowsValidOpsToken(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler(zap.NewNop())})
	app.Use(OpsAuth("top-secret", 5*time.Minute, zap.NewNop()))
	app.Get("/ops", func(c *fiber.Ctx) error {
		plane, _ := c.Locals("auth_plane").(string)
		return c.JSON(fiber.Map{"ok": true, "auth_plane": plane})
	})

	req := httptest.NewRequest(fiber.MethodGet, "/ops", nil)
	setSignedOpsHeaders(req, "top-secret")
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestOpsAuth_RejectsJWTBearerToken(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler(zap.NewNop())})
	app.Use(OpsAuth("top-secret", 5*time.Minute, zap.NewNop()))
	app.Get("/ops", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	req := httptest.NewRequest(fiber.MethodGet, "/ops", nil)
	req.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9")
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestOpsAuth_RejectsMissingAuthorization(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler(zap.NewNop())})
	app.Use(OpsAuth("top-secret", 5*time.Minute, zap.NewNop()))
	app.Get("/ops", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	req := httptest.NewRequest(fiber.MethodGet, "/ops", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestOpsAuth_RejectsWrongSecret(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler(zap.NewNop())})
	app.Use(OpsAuth("top-secret", 5*time.Minute, zap.NewNop()))
	app.Get("/ops", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	req := httptest.NewRequest(fiber.MethodGet, "/ops", nil)
	setSignedOpsHeaders(req, "not-right")
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestOpsAuth_RejectsWhenMisconfigured(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler(zap.NewNop())})
	app.Use(OpsAuth("", 5*time.Minute, zap.NewNop()))
	app.Get("/ops", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	req := httptest.NewRequest(fiber.MethodGet, "/ops", nil)
	setSignedOpsHeaders(req, "any")
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestOpsAuth_RejectsReplayNonce(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler(zap.NewNop())})
	app.Use(OpsAuth("top-secret", 5*time.Minute, zap.NewNop()))
	app.Get("/ops", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	req1 := httptest.NewRequest(fiber.MethodGet, "/ops", nil)
	setSignedOpsHeaders(req1, "top-secret")
	resp1, err := app.Test(req1)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp1.StatusCode)

	req2 := httptest.NewRequest(fiber.MethodGet, "/ops", nil)
	req2.Header = req1.Header.Clone()
	resp2, err := app.Test(req2)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusUnauthorized, resp2.StatusCode)
}

func TestOpsAuth_RejectsStaleTimestamp(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler(zap.NewNop())})
	app.Use(OpsAuth("top-secret", 1*time.Second, zap.NewNop()))
	app.Get("/ops", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	req := httptest.NewRequest(fiber.MethodGet, "/ops", nil)
	setSignedOpsHeadersWithTime(req, "top-secret", time.Now().UTC().Add(-10*time.Second))
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func setSignedOpsHeaders(req *http.Request, secret string) {
	setSignedOpsHeadersWithTime(req, secret, time.Now().UTC())
}

func setSignedOpsHeadersWithTime(req *http.Request, secret string, ts time.Time) {
	timestamp := ts.UTC().Unix()
	nonce := "unit-test-nonce-" + strconv.FormatInt(time.Now().UTC().UnixNano(), 10)

	sig := signOpsMessage(secret, req.Method, req.URL.RequestURI(), strconv.FormatInt(timestamp, 10), nonce)
	req.Header.Set("Authorization", "Bearer ops:"+secret)
	req.Header.Set("X-Ops-Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("X-Ops-Nonce", nonce)
	req.Header.Set("X-Ops-Signature", sig)
}
