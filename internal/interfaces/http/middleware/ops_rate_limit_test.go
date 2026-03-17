package middleware

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

type stubLimiter struct {
	allow bool
	err   error
}

func (s *stubLimiter) Allow(_ context.Context, _ string, _ int, _ time.Duration) (bool, error) {
	return s.allow, s.err
}

func (s *stubLimiter) IncrementAndCheckDailyLimit(_ context.Context, _ string, _ int) (bool, error) {
	return true, nil
}

func (s *stubLimiter) ResetDailyLimit(_ context.Context, _ string) error { return nil }

func TestOpsRateLimit_AllowsRequest(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler(zap.NewNop())})
	app.Use(OpsRateLimit(&stubLimiter{allow: true}, 2, time.Minute, zap.NewNop()))
	app.Get("/ops", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	req := httptest.NewRequest(fiber.MethodGet, "/ops", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestOpsRateLimit_BlocksRequest(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler(zap.NewNop())})
	app.Use(OpsRateLimit(&stubLimiter{allow: false}, 1, time.Minute, zap.NewNop()))
	app.Get("/ops", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	req := httptest.NewRequest(fiber.MethodGet, "/ops", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusTooManyRequests, resp.StatusCode)
	assert.Equal(t, "60", resp.Header.Get("Retry-After"))
}

func TestOpsRateLimit_PropagatesLimiterError(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler(zap.NewNop())})
	app.Use(OpsRateLimit(&stubLimiter{err: errors.New("redis down")}, 1, time.Minute, zap.NewNop()))
	app.Get("/ops", func(c *fiber.Ctx) error { return c.SendStatus(fiber.StatusOK) })

	req := httptest.NewRequest(fiber.MethodGet, "/ops", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}
