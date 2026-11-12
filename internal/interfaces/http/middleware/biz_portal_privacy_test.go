package middleware

import (
	"bytes"
	"context"
	"github.com/gofiber/contrib/otelfiber/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/stretchr/testify/require"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBizPortalSecretsNeverLogged(t *testing.T) {
	var logs bytes.Buffer
	z := zap.New(zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), zapcore.AddSync(&logs), zap.DebugLevel))
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	old := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	defer otel.SetTracerProvider(old)
	defer provider.Shutdown(context.Background())
	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler(z)})
	app.Use(BizPortalPrivacy(&portalTestLimiter{}, z))
	app.Use(BizPortalTracing(otelfiber.Middleware()))
	app.Use(logger.New(logger.Config{Next: IsBizPortalRequest, Output: &logs, Format: "${path} ${queryParams}\n"}))
	app.Get("/v1/portal/:token/invoices/:id", func(c *fiber.Ctx) error { return apperrors.BadRequest("invalid invoice") })
	raw := "pt2_fixture-bearer-never-in-logs"
	response, err := app.Test(httptest.NewRequest("GET", "/v1/portal/"+raw+"/invoices/fixture?secret_query=not-visible", nil))
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, 400, response.StatusCode)
	require.Equal(t, "no-referrer", response.Header.Get("Referrer-Policy"))
	require.Equal(t, "no-store", response.Header.Get("Cache-Control"))
	require.NotContains(t, logs.String(), raw)
	require.NotContains(t, logs.String(), "not-visible")
	require.Contains(t, logs.String(), "/v1/portal/:token")
	revoked, err := app.Test(httptest.NewRequest("POST", "/v1/biz/portal-tokens/"+raw+"/revoke", nil))
	require.NoError(t, err)
	revoked.Body.Close()
	require.NotContains(t, logs.String(), raw)
	require.Contains(t, logs.String(), "/v1/biz/portal-tokens/:id/revoke")
	require.Empty(t, exporter.GetSpans(), "actual automatic exporter must see no token-bearing span")
	app.Get("/ordinary", func(c *fiber.Ctx) error { return c.SendStatus(200) })
	ordinary, err := app.Test(httptest.NewRequest("GET", "/ordinary", nil))
	require.NoError(t, err)
	ordinary.Body.Close()
	require.NotEmpty(t, exporter.GetSpans(), "ordinary tracing remains active")
}

func TestBizPortalPrivacyRateLimitAndUnknownRoute(t *testing.T) {
	app := fiber.New()
	app.Use(BizPortalPrivacy(&portalTestLimiter{}, nil))
	app.Get("/v1/portal/:token", func(c *fiber.Ctx) error { return c.SendStatus(401) })
	for i := 0; i < 121; i++ {
		r, err := app.Test(httptest.NewRequest("GET", "/v1/portal/invalid", nil))
		require.NoError(t, err)
		r.Body.Close()
		require.Equal(t, "no-store", r.Header.Get("Cache-Control"))
		if i == 120 {
			require.Equal(t, 429, r.StatusCode)
		}
	}
	unknown := fiber.New()
	unknown.Use(BizPortalPrivacy(&portalTestLimiter{}, nil))
	r, err := unknown.Test(httptest.NewRequest("GET", "/v1/portal/unknown/missing", nil))
	require.NoError(t, err)
	defer r.Body.Close()
	require.Equal(t, 404, r.StatusCode)
	require.Equal(t, "no-referrer", r.Header.Get("Referrer-Policy"))
}

type portalTestLimiter struct{ count int }

func (l *portalTestLimiter) Allow(context.Context, string, int, time.Duration) (bool, error) {
	l.count++
	return l.count <= 120, nil
}
func (l *portalTestLimiter) IncrementAndCheckDailyLimit(context.Context, string, int) (bool, error) {
	return true, nil
}
func (l *portalTestLimiter) ResetDailyLimit(context.Context, string) error { return nil }
