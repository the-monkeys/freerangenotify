package handlers

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/dto"
	"github.com/the-monkeys/freerangenotify/internal/usecases"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"github.com/the-monkeys/freerangenotify/pkg/validator"
	"go.uber.org/zap"
)

type billingProfileService struct {
	usecases.UserService
	record *user.User
}

func (s billingProfileService) GetByID(context.Context, string) (*user.User, error) {
	copy := *s.record
	return &copy, nil
}

func TestUserBillingReadUsesCompletedBalance(t *testing.T) {
	for _, upgraded := range []bool{true, false} {
		record := &user.User{UserID: "4cf45f80-a936-4d8d-864c-8ad010e6968a", AppID: "app", Email: "fixture@example.invalid", BalancePaisa: 900}
		handler := NewUserHandler(billingProfileService{record: record}, nil, zap.NewNop())
		handler.SetBillingBalanceReader(func(_ context.Context, profile *user.User) (int64, bool, error) {
			require.Equal(t, record.UserID, profile.UserID)
			return 100, upgraded, nil
		})
		app := fiber.New()
		app.Use(func(c *fiber.Ctx) error { c.Locals("app_id", "app"); return c.Next() })
		app.Get("/users/:id", handler.GetByID)
		res, err := app.Test(httptest.NewRequest("GET", "/users/"+record.UserID, nil))
		require.NoError(t, err)
		defer res.Body.Close()
		require.Equal(t, 200, res.StatusCode)
		var response struct {
			Data dto.UserResponse `json:"data"`
		}
		require.NoError(t, json.NewDecoder(res.Body).Decode(&response))
		if upgraded {
			require.Equal(t, int64(100), response.Data.BalancePaisa)
		} else {
			require.Equal(t, int64(900), response.Data.BalancePaisa)
		}
		require.Equal(t, int64(900), record.BalancePaisa, "reading must not mutate a shared profile")
	}
}

type billingMutableService struct {
	usecases.UserService
	saved *user.User
	rows  []*user.User
}

func (s *billingMutableService) GetByEmail(context.Context, string, string) (*user.User, error) {
	return nil, apperrors.NotFound("user", "fixture")
}
func (s *billingMutableService) GetByID(context.Context, string) (*user.User, error) {
	if s.saved == nil {
		return nil, apperrors.NotFound("user", "fixture")
	}
	copy := *s.saved
	return &copy, nil
}
func (s *billingMutableService) Create(_ context.Context, u *user.User) error {
	copy := *u
	s.saved = &copy
	return nil
}
func (s *billingMutableService) Update(_ context.Context, u *user.User) error {
	copy := *u
	s.saved = &copy
	return nil
}

func TestUserBillingExplicitBalanceUsesWriter(t *testing.T) {
	service := &billingMutableService{}
	handler := NewUserHandler(service, validator.New(), zap.NewNop())
	calls := 0
	handler.SetBillingBalanceWriter(func(_ context.Context, appID string, profile *user.User, target int64, reason, fingerprint string) (*user.BillingBalanceReceipt, error) {
		calls++
		require.Equal(t, "app", appID)
		require.Equal(t, int64(100), target)
		require.NotEmpty(t, reason)
		require.NotEmpty(t, fingerprint)
		return &user.BillingBalanceReceipt{BalancePaisa: 100, Sequence: int64(calls), Currency: "INR"}, nil
	})
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error { c.Locals("app_id", "app"); return c.Next() })
	app.Post("/users", handler.Create)
	app.Put("/users/:id", handler.Update)
	for _, method := range []string{"POST", "PUT"} {
		path := "/users"
		body := `{"user_id":"4cf45f80-a936-4d8d-864c-8ad010e6968a","email":"fixture@example.invalid","balance_paisa":100}`
		if method == "PUT" {
			path += "/4cf45f80-a936-4d8d-864c-8ad010e6968a"
			body = `{"full_name":"Updated profile","balance_paisa":100}`
		}
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", method+"-fixture")
		res, err := app.Test(req)
		require.NoError(t, err)
		defer res.Body.Close()
		require.Contains(t, []int{200, 201}, res.StatusCode)
		var result struct {
			Data dto.UserResponse `json:"data"`
		}
		require.NoError(t, json.NewDecoder(res.Body).Decode(&result))
		require.Equal(t, int64(100), result.Data.BalancePaisa)
	}
	require.Equal(t, 2, calls)
}

func (s *billingMutableService) BulkCreate(_ context.Context, profiles []*user.User) error {
	s.rows = profiles
	return nil
}
func TestUserBillingImportOnlyInitializesExplicitCredit(t *testing.T) {
	service := &billingMutableService{}
	handler := NewUserHandler(service, validator.New(), zap.NewNop())
	calls := 0
	handler.SetBillingBalanceWriter(func(_ context.Context, appID string, profile *user.User, target int64, reason, fingerprint string) (*user.BillingBalanceReceipt, error) {
		calls++
		require.Equal(t, "funded@example.invalid", profile.Email)
		return &user.BillingBalanceReceipt{BalancePaisa: target, Sequence: 1, Currency: "INR"}, nil
	})
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error { c.Locals("app_id", "app"); return c.Next() })
	app.Post("/users/bulk", handler.BulkCreate)
	req := httptest.NewRequest("POST", "/users/bulk", strings.NewReader(`{"users":[{"email":"normal@example.invalid"},{"email":"funded@example.invalid","balance_paisa":100}]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "import")
	res, err := app.Test(req)
	require.NoError(t, err)
	defer res.Body.Close()
	require.Equal(t, 201, res.StatusCode)
	require.Equal(t, 1, calls)
	require.Len(t, service.rows, 2)
	require.Zero(t, service.rows[0].BizBalanceSequence)
	require.Equal(t, int64(1), service.rows[1].BizBalanceSequence)
}
