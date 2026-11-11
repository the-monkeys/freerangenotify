package handlers

import (
	"context"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/audit"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/middleware"
	"go.uber.org/zap"
)

func TestAuditDateBounds(t *testing.T) {
	start, end, exclusive, err := parseAuditDateRange("2026-11-11", "2026-11-11")
	require.NoError(t, err)
	require.Equal(t, "2026-11-11T00:00:00Z", start.Format(time.RFC3339))
	require.Equal(t, "2026-11-12T00:00:00Z", end.Format(time.RFC3339))
	require.True(t, exclusive)
	_, end, exclusive, err = parseAuditDateRange("2026-11-11T10:00:00+05:30", "2026-11-11T11:00:00.123+05:30")
	require.NoError(t, err)
	require.Equal(t, "2026-11-11T05:30:00.123Z", end.Format(time.RFC3339Nano))
	require.False(t, exclusive)
	for _, pair := range [][2]string{{"invalid", ""}, {"", "2026-02-30"}, {"2026-11-12", "2026-11-11"}, {"2026-11-11T12:00:00Z", "2026-11-11T11:59:59Z"}, {"", "9999-12-31"}} {
		_, _, _, err := parseAuditDateRange(pair[0], pair[1])
		require.Error(t, err, pair)
	}
	start, end, exclusive, err = parseAuditDateRange("", "")
	require.NoError(t, err)
	require.Nil(t, start)
	require.Nil(t, end)
	require.False(t, exclusive)
}

type auditDateApps struct{ application.Repository }

func (a auditDateApps) List(context.Context, application.ApplicationFilter) ([]*application.Application, error) {
	return []*application.Application{{AppID: "owned"}}, nil
}

type auditDateService struct {
	audit.Service
	received *audit.Filter
}

func (s *auditDateService) List(_ context.Context, f audit.Filter) ([]*audit.AuditLog, error) {
	s.received = &f
	return []*audit.AuditLog{}, nil
}

func TestAuditDateHTTPValidationAndScope(t *testing.T) {
	for _, test := range []struct {
		query  string
		status int
	}{{"from_date=invalid", 400}, {"from_date=2026-11-12&to_date=2026-11-11", 400}, {"app_id=foreign&to_date=2026-11-11", 403}, {"app_id=owned&from_date=2026-11-11&to_date=2026-11-11&limit=7&offset=2", 200}, {"to_date=" + url.QueryEscape("2026-11-11T23:00:00+05:30"), 200}} {
		svc := &auditDateService{}
		handler := NewAuditHandler(svc, auditDateApps{}, zap.NewNop())
		app := fiber.New(fiber.Config{ErrorHandler: middleware.ErrorHandler(zap.NewNop())})
		app.Use(func(c *fiber.Ctx) error { c.Locals("user_id", "admin"); return c.Next() })
		app.Get("/audit", handler.List)
		res, err := app.Test(httptest.NewRequest("GET", "/audit?"+test.query, nil))
		require.NoError(t, err)
		res.Body.Close()
		require.Equal(t, test.status, res.StatusCode, test.query)
		if test.status == 200 {
			require.Equal(t, []string{"owned"}, svc.received.AppIDs)
			require.NotNil(t, svc.received.ToDate)
		} else {
			require.Nil(t, svc.received)
		}
	}
}
