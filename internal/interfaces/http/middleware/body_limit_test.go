package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUploadBodyBoundaries(t *testing.T) {
	require.Equal(t, 51<<20, UploadTransportLimit(50<<20))
	require.Equal(t, 16<<20, UploadTransportLimit(1<<20))
	require.Equal(t, 51<<20, UploadTransportLimit(100<<20))
	for _, tc := range []struct {
		path, method, mime, app string
		status                  int
	}{
		{"/v1/files", "POST", "multipart/form-data; boundary=test", "app-a", 204},
		{"/v1/files/", "POST", "multipart/form-data; boundary=test", "app-a", 204},
		{"/v1/files", "POST", "multipart/form-data; boundary=test", "", 413},
		{"/v1/files", "POST", "application/json", "app-a", 413},
		{"/v1/notifications", "POST", "multipart/form-data; boundary=test", "app-a", 413},
	} {
		t.Run(tc.path+tc.mime+tc.app, func(t *testing.T) {
			app := fiber.New(fiber.Config{BodyLimit: UploadTransportLimit(50 << 20), DisablePreParseMultipartForm: true})
			app.Use(func(c *fiber.Ctx) error { c.Locals("app_id", tc.app); return c.Next() })
			app.Use(NonUploadBodyLimit(16 << 20))
			app.All("*", func(c *fiber.Ctx) error { return c.SendStatus(204) })
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(strings.Repeat("x", 17<<20)))
			req.Header.Set("Content-Type", tc.mime)
			res, err := app.Test(req, 5000)
			require.NoError(t, err)
			defer res.Body.Close()
			require.Equal(t, tc.status, res.StatusCode)
		})
	}
}
