package middleware

import (
	"context"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/limiter"
	"go.uber.org/zap"
	"net/http/httptest"
	"testing"
	"time"
)

type requestQuotaFake struct {
	limiter.Limiter
	calls int
	err   error
}

func (f *requestQuotaFake) Allow(context.Context, string, int, time.Duration) (bool, error) {
	f.calls++
	return true, f.err
}
func TestAppQuotaDisabledCompatibilityAndOutage(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		l := &requestQuotaFake{err: fmt.Errorf("redis down")}
		app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler(zap.NewNop())})
		app.Use(func(c *fiber.Ctx) error {
			c.Locals("app", &application.Application{AppID: "app", Settings: application.Settings{RateLimit: 1000}})
			return c.Next()
		})
		app.Use(ApplicationRateLimit(l, enabled, zap.NewNop()))
		app.Post("/broadcast", func(c *fiber.Ctx) error { return c.SendStatus(202) })
		res, err := app.Test(httptest.NewRequest("POST", "/broadcast", nil))
		require.NoError(t, err)
		if enabled {
			require.Equal(t, 503, res.StatusCode)
			require.Equal(t, 1, l.calls)
		} else {
			require.Equal(t, 202, res.StatusCode)
			require.Zero(t, l.calls)
		}
		res.Body.Close()
	}
}
