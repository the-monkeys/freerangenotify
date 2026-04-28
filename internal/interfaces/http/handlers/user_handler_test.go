package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	httpmiddleware "github.com/the-monkeys/freerangenotify/internal/interfaces/http/middleware"
	"github.com/the-monkeys/freerangenotify/internal/usecases"
	"github.com/the-monkeys/freerangenotify/pkg/validator"
	"go.uber.org/zap/zaptest"
)

// ─── Stub UserService ──────────────────────────────────────────────────────

type stubUserService struct {
	users map[string]*user.User // keyed by user_id

	// For UpdateByExternalID: app_id+external_id → user_id
	byExtID map[string]string
}

func newStubUserService() *stubUserService {
	return &stubUserService{
		users:   make(map[string]*user.User),
		byExtID: make(map[string]string),
	}
}

func (s *stubUserService) seed(u *user.User) {
	s.users[u.UserID] = u
	if u.ExternalID != "" {
		s.byExtID[u.AppID+":"+u.ExternalID] = u.UserID
	}
}

// ── Interface implementation ──

var _ usecases.UserService = (*stubUserService)(nil)

func (s *stubUserService) Create(_ context.Context, u *user.User) error { return nil }
func (s *stubUserService) GetByID(_ context.Context, id string) (*user.User, error) {
	if u, ok := s.users[id]; ok {
		return u, nil
	}
	return nil, fmt.Errorf("not found")
}
func (s *stubUserService) GetByEmail(_ context.Context, _, _ string) (*user.User, error) {
	return nil, fmt.Errorf("not found")
}
func (s *stubUserService) GetByExternalID(_ context.Context, appID, extID string) (*user.User, error) {
	if uid, ok := s.byExtID[appID+":"+extID]; ok {
		return s.users[uid], nil
	}
	return nil, fmt.Errorf("not found")
}
func (s *stubUserService) Update(_ context.Context, u *user.User) error {
	s.users[u.UserID] = u
	return nil
}
func (s *stubUserService) UpdateByExternalID(_ context.Context, appID, extID string, apply func(*user.User)) (*user.User, error) {
	uid, ok := s.byExtID[appID+":"+extID]
	if !ok {
		return nil, fmt.Errorf("[NOT_FOUND] User not found: %s", extID)
	}
	u := s.users[uid]
	apply(u)
	s.users[uid] = u
	return u, nil
}
func (s *stubUserService) Delete(_ context.Context, _ string) error { return nil }
func (s *stubUserService) List(_ context.Context, _ user.UserFilter) ([]*user.User, int64, error) {
	return nil, 0, nil
}
func (s *stubUserService) AddDevice(_ context.Context, _ string, _ user.Device) error { return nil }
func (s *stubUserService) RemoveDevice(_ context.Context, _, _ string) error          { return nil }
func (s *stubUserService) GetDevices(_ context.Context, _ string) ([]user.Device, error) {
	return nil, nil
}
func (s *stubUserService) UpdatePreferences(_ context.Context, _ string, _ user.Preferences) error {
	return nil
}
func (s *stubUserService) GetPreferences(_ context.Context, _ string) (*user.Preferences, error) {
	return nil, nil
}
func (s *stubUserService) BulkCreate(_ context.Context, _ []*user.User) error { return nil }

// ─── Test helpers ──────────────────────────────────────────────────────────

func setupUserHandlerTestApp(t *testing.T, svc *stubUserService, appID string) *fiber.App {
	t.Helper()
	logger := zaptest.NewLogger(t)
	handler := NewUserHandler(svc, validator.New(), logger)

	app := fiber.New(fiber.Config{
		ErrorHandler: httpmiddleware.ErrorHandler(logger),
	})
	// Inject app_id into locals as the real APIKeyAuth middleware would.
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("app_id", appID)
		return c.Next()
	})

	app.Put("/users/by-external-id/:external_id", handler.UpdateByExternalID)
	app.Put("/users/:id", handler.Update)
	return app
}

func doJSON(t *testing.T, app *fiber.App, method, path string, body interface{}) (*http.Response, map[string]interface{}) {
	t.Helper()
	payload, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, 5000)
	require.NoError(t, err)

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	resp.Body.Close()
	return resp, result
}

// ─── Tests: UpdateByExternalID handler ─────────────────────────────────────

func TestUserHandler_UpdateByExternalID_Success(t *testing.T) {
	svc := newStubUserService()
	svc.seed(&user.User{
		UserID:     "uid-1",
		AppID:      "app-1",
		ExternalID: "ext-42",
		FullName:   "Old Name",
		Email:      "old@example.com",
	})

	app := setupUserHandlerTestApp(t, svc, "app-1")

	resp, body := doJSON(t, app, http.MethodPut, "/users/by-external-id/ext-42", map[string]interface{}{
		"full_name": "New Name",
		"email":     "new@example.com",
	})

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, true, body["success"])

	data := body["data"].(map[string]interface{})
	assert.Equal(t, "New Name", data["full_name"])
	assert.Equal(t, "new@example.com", data["email"])
	assert.Equal(t, "uid-1", data["user_id"])
}

func TestUserHandler_UpdateByExternalID_NotFound(t *testing.T) {
	svc := newStubUserService()
	app := setupUserHandlerTestApp(t, svc, "app-1")

	resp, body := doJSON(t, app, http.MethodPut, "/users/by-external-id/nonexistent", map[string]interface{}{
		"full_name": "Doesn't matter",
	})

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode) // stub returns plain error, not AppError
	assert.Equal(t, false, body["success"])
}

func TestUserHandler_UpdateByExternalID_EmptyExternalID(t *testing.T) {
	svc := newStubUserService()
	app := setupUserHandlerTestApp(t, svc, "app-1")

	// Fiber won't match the route with empty param — this hits 404 at router level.
	req := httptest.NewRequest(http.MethodPut, "/users/by-external-id/", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, 5000)
	require.NoError(t, err)
	resp.Body.Close()

	// Empty param either returns 404 (no route match) or 400 (handler validation).
	assert.True(t, resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest,
		"expected 404 or 400, got %d", resp.StatusCode)
}

func TestUserHandler_UpdateByExternalID_InvalidJSON(t *testing.T) {
	svc := newStubUserService()
	svc.seed(&user.User{
		UserID:     "uid-1",
		AppID:      "app-1",
		ExternalID: "ext-42",
	})

	app := setupUserHandlerTestApp(t, svc, "app-1")

	req := httptest.NewRequest(http.MethodPut, "/users/by-external-id/ext-42", bytes.NewReader([]byte(`not json`)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, 5000)
	require.NoError(t, err)
	resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestUserHandler_UpdateByExternalID_InvalidEmail(t *testing.T) {
	svc := newStubUserService()
	svc.seed(&user.User{
		UserID:     "uid-1",
		AppID:      "app-1",
		ExternalID: "ext-42",
	})

	app := setupUserHandlerTestApp(t, svc, "app-1")

	resp, body := doJSON(t, app, http.MethodPut, "/users/by-external-id/ext-42", map[string]interface{}{
		"email": "not-an-email",
	})

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, false, body["success"])
}

func TestUserHandler_UpdateByExternalID_PartialUpdate(t *testing.T) {
	svc := newStubUserService()
	svc.seed(&user.User{
		UserID:     "uid-1",
		AppID:      "app-1",
		ExternalID: "ext-42",
		FullName:   "Keep This",
		Email:      "keep@example.com",
		Phone:      "+1234567890",
	})

	app := setupUserHandlerTestApp(t, svc, "app-1")

	// Only update phone — other fields should remain unchanged.
	resp, body := doJSON(t, app, http.MethodPut, "/users/by-external-id/ext-42", map[string]interface{}{
		"phone": "+0987654321",
	})

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	data := body["data"].(map[string]interface{})
	assert.Equal(t, "Keep This", data["full_name"])
	assert.Equal(t, "keep@example.com", data["email"])
	assert.Equal(t, "+0987654321", data["phone"])
}

func TestUserHandler_UpdateByExternalID_NoAuth(t *testing.T) {
	svc := newStubUserService()
	logger := zaptest.NewLogger(t)
	handler := NewUserHandler(svc, validator.New(), logger)

	// Create app WITHOUT injecting app_id into locals.
	app := fiber.New(fiber.Config{
		ErrorHandler: httpmiddleware.ErrorHandler(logger),
	})
	app.Put("/users/by-external-id/:external_id", handler.UpdateByExternalID)

	resp, body := doJSON(t, app, http.MethodPut, "/users/by-external-id/ext-42", map[string]interface{}{
		"full_name": "Hacker",
	})

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, false, body["success"])
}
