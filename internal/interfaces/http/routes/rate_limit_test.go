package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/config"
	"github.com/the-monkeys/freerangenotify/internal/container"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/tenant"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/handlers"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/middleware"
	"github.com/the-monkeys/freerangenotify/internal/usecases"
	"go.uber.org/zap"
	"net/http/httptest"
	"testing"
	"time"
)

type quotaAppService struct{ usecases.ApplicationService }

func (*quotaAppService) GetByID(context.Context, string) (*application.Application, error) {
	return &application.Application{AppID: "app-1", AdminUserID: "owner", Settings: application.Settings{RateLimit: 2}}, nil
}

func (*quotaAppService) ValidateAPIKey(_ context.Context, key string) (*application.Application, error) {
	if key == "invalid" {
		return nil, fmt.Errorf("invalid key")
	}
	return &application.Application{AppID: "app-1", AdminUserID: "owner", Settings: application.Settings{RateLimit: 2}}, nil
}

type quotaBroadcast struct {
	notification.Service
	calls int
}

func (s *quotaBroadcast) Broadcast(context.Context, notification.BroadcastRequest) (*notification.BroadcastResult, error) {
	s.calls++
	return &notification.BroadcastResult{Triggered: 6000}, nil
}

type quotaLimiter struct {
	calls   int
	keys    []string
	windows []time.Duration
}

func (l *quotaLimiter) Allow(_ context.Context, key string, limit int, window time.Duration) (bool, error) {
	l.calls++
	l.keys = append(l.keys, key)
	l.windows = append(l.windows, window)
	return l.calls <= limit, nil
}
func (*quotaLimiter) IncrementAndCheckDailyLimit(context.Context, string, int) (bool, error) {
	return true, nil
}
func (*quotaLimiter) ResetDailyLimit(context.Context, string) error { return nil }
func TestAppHTTPQuotaCountsBroadcastOnceAndSharesKeys(t *testing.T) {
	l := &quotaLimiter{}
	svc := &quotaBroadcast{}
	cfg := &config.Config{}
	cfg.Security.AppRateLimitEnabled = true
	c := &container.Container{Config: cfg, Logger: zap.NewNop(), ApplicationService: &quotaAppService{}, Limiter: l, NotificationHandler: handlers.NewNotificationHandler(svc, zap.NewNop())}
	app := fiber.New(fiber.Config{ErrorHandler: middleware.ErrorHandler(zap.NewNop())})
	setupProtectedRoutes(app.Group("/v1"), c)
	body, _ := json.Marshal(map[string]string{"channel": "email", "priority": "normal", "template_id": "fixture"})
	for i, key := range []string{"invalid", "key-a", "key-b", "key-b"} {
		req := httptest.NewRequest("POST", "/v1/notifications/broadcast", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", key)
		res, err := app.Test(req)
		require.NoError(t, err)
		expected := 202
		if i == 0 {
			expected = 401
		}
		if i == 3 {
			expected = 429
		}
		require.Equal(t, expected, res.StatusCode)
		if i == 3 {
			require.NotEmpty(t, res.Header.Get("Retry-After"))
		}
		res.Body.Close()
	}
	require.Equal(t, 2, svc.calls)
	require.Equal(t, 3, l.calls)
	require.Equal(t, []string{"app:app-1", "app:app-1", "app:app-1"}, l.keys)
	for _, window := range l.windows {
		require.Equal(t, time.Hour, window)
	}
}

type quotaAuthService struct{ auth.Service }

func (*quotaAuthService) ValidateToken(context.Context, string) (*auth.AdminUser, error) {
	return &auth.AdminUser{UserID: "viewer"}, nil
}

type quotaMembers struct{ auth.MembershipRepository }

func (*quotaMembers) GetByAppAndUser(context.Context, string, string) (*auth.AppMembership, error) {
	return &auth.AppMembership{Role: auth.RoleViewer}, nil
}

type quotaAppRepo struct{ application.Repository }

func (*quotaAppRepo) GetByID(context.Context, string) (*application.Application, error) {
	return &application.Application{AppID: "app-1", AdminUserID: "owner"}, nil
}

type quotaDeny struct{ quotaLimiter }

func (l *quotaDeny) Allow(context.Context, string, int, time.Duration) (bool, error) {
	l.calls++
	return false, nil
}
func TestDirectAppRoutesAuthorizeDashboardBeforeQuota(t *testing.T) {
	l := &quotaDeny{}
	cfg := &config.Config{}
	cfg.Security.AppRateLimitEnabled = true
	c := &container.Container{Config: cfg, Logger: zap.NewNop(), ApplicationService: &quotaAppService{}, AuthService: &quotaAuthService{}, MembershipRepo: &quotaMembers{}, AppRepo: &quotaAppRepo{}, Limiter: l}
	app := fiber.New(fiber.Config{ErrorHandler: middleware.ErrorHandler(zap.NewNop())})
	setupProtectedRoutes(app.Group("/v1"), c)
	for _, path := range []string{"/v1/quick-send", "/v1/media/upload", "/v1/sse/tokens"} {
		req := httptest.NewRequest("POST", path, nil)
		req.Header.Set("X-API-Key", "key")
		req.Header.Set("Authorization", "Bearer jwt-fixture")
		res, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, 403, res.StatusCode)
		res.Body.Close()
	}
	require.Zero(t, l.calls, "unauthorized writes must not consume application quota")
}

type quotaTokenAuth struct{ auth.Service }

func (*quotaTokenAuth) ValidateToken(_ context.Context, token string) (*auth.AdminUser, error) {
	return &auth.AdminUser{UserID: token}, nil
}
func TestJWTAppQuotaRejectsOutsiderBeforeAdmission(t *testing.T) {
	l := &quotaLimiter{}
	cfg := &config.Config{}
	cfg.Security.AppRateLimitEnabled = true
	svc := &quotaAppService{}
	c := &container.Container{Config: cfg, Logger: zap.NewNop(), ApplicationService: svc, AuthService: &quotaTokenAuth{}, Limiter: l, ApplicationHandler: handlers.NewApplicationHandler(svc, nil, nil, nil, nil, zap.NewNop())}
	app := fiber.New(fiber.Config{ErrorHandler: middleware.ErrorHandler(zap.NewNop())})
	setupAdminRoutes(app.Group("/v1"), c)
	for i, token := range []string{"outsider", "owner", "owner", "owner"} {
		req := httptest.NewRequest("GET", "/v1/apps/app-1", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		res, err := app.Test(req)
		require.NoError(t, err)
		expected := 200
		if i == 0 {
			expected = 403
		}
		if i == 3 {
			expected = 429
		}
		require.Equal(t, expected, res.StatusCode)
		res.Body.Close()
	}
	require.Equal(t, 3, l.calls)
}

type rejectedWebhookProvider struct{ billing.Provider }

func (*rejectedWebhookProvider) VerifyWebhook([]byte, string) (billing.WebhookEvent, error) {
	return billing.WebhookEvent{}, fmt.Errorf("invalid signature")
}
func TestPublicAuthQuotaExcludesSignedPaymentCallback(t *testing.T) {
	l := &quotaDeny{}
	cfg := &config.Config{}
	cfg.Security.RateLimitEnabled = true
	cfg.Security.RateLimit = 100
	cfg.Security.RateLimitWindow = 3600
	c := &container.Container{Config: cfg, Logger: zap.NewNop(), Limiter: l, PaymentHandler: handlers.NewPaymentHandler(&rejectedWebhookProvider{}, nil, nil, nil, true, zap.NewNop())}
	app := fiber.New(fiber.Config{ErrorHandler: middleware.ErrorHandler(zap.NewNop())})
	setupPublicRoutes(app.Group("/v1"), c)
	res, err := app.Test(httptest.NewRequest("POST", "/v1/auth/login", nil))
	require.NoError(t, err)
	require.Equal(t, 429, res.StatusCode)
	res.Body.Close()
	require.Equal(t, 1, l.calls)
	req := httptest.NewRequest("POST", "/v1/billing/webhook", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("X-Razorpay-Signature", "invalid")
	res, err = app.Test(req)
	require.NoError(t, err)
	require.Equal(t, 401, res.StatusCode)
	res.Body.Close()
	require.Equal(t, 1, l.calls, "payment callbacks are outside the public auth IP bucket")
}

type quotaOrgAppRepo struct{ application.Repository }

func (*quotaOrgAppRepo) GetByID(context.Context, string) (*application.Application, error) {
	return &application.Application{AppID: "app-1", AdminUserID: "owner", TenantID: "org-1"}, nil
}

type quotaNoMembers struct{ auth.MembershipRepository }

func (*quotaNoMembers) GetByAppAndUser(context.Context, string, string) (*auth.AppMembership, error) {
	return nil, nil
}

type quotaOrgAccess struct {
	tenant.Service
	role string
}

func (s *quotaOrgAccess) HasAccess(context.Context, string, string) (bool, string, error) {
	return true, s.role, nil
}
func TestDirectAppQuotaPreservesOrganizationRoles(t *testing.T) {
	for _, role := range []string{"admin", "member"} {
		t.Run(role, func(t *testing.T) {
			l := &quotaDeny{}
			cfg := &config.Config{}
			cfg.Security.AppRateLimitEnabled = true
			c := &container.Container{Config: cfg, Logger: zap.NewNop(), ApplicationService: &quotaAppService{}, AuthService: &quotaAuthService{}, MembershipRepo: &quotaNoMembers{}, AppRepo: &quotaOrgAppRepo{}, TenantService: &quotaOrgAccess{role: role}, Limiter: l}
			app := fiber.New(fiber.Config{ErrorHandler: middleware.ErrorHandler(zap.NewNop())})
			setupProtectedRoutes(app.Group("/v1"), c)
			req := httptest.NewRequest("POST", "/v1/quick-send", nil)
			req.Header.Set("X-API-Key", "key")
			req.Header.Set("Authorization", "Bearer jwt-fixture")
			res, err := app.Test(req)
			require.NoError(t, err)
			defer res.Body.Close()
			if role == "admin" {
				require.Equal(t, 429, res.StatusCode)
				require.Equal(t, 1, l.calls)
			} else {
				require.Equal(t, 403, res.StatusCode)
				require.Zero(t, l.calls)
			}
		})
	}
}
