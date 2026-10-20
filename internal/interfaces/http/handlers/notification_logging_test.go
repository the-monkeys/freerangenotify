package handlers

import (
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNotificationSensitiveLoggingMalformedBody(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	h := NewNotificationHandler(nil, zap.New(core))
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error { c.Locals("app_id", "app-a"); return c.Next() })
	app.Post("/send", h.Send)
	req := httptest.NewRequest("POST", "/send", strings.NewReader(`{"code":"583917",invalid`))
	req.Header.Set("Content-Type", "application/json")
	res, err := app.Test(req)
	require.NoError(t, err)
	defer res.Body.Close()
	require.Equal(t, 400, res.StatusCode)
	for _, entry := range logs.All() {
		raw, _ := json.Marshal(entry.ContextMap())
		require.NotContains(t, string(raw), "583917")
	}
}
