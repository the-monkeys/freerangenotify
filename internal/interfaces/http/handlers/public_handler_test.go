package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
)

// fakeAuthRepo is a minimal stub of auth.Repository for testing PublicHandler.
type fakeAuthRepo struct {
	countResult int64
	countErr    error
}

func (f *fakeAuthRepo) CountUsers(_ context.Context) (int64, error) {
	return f.countResult, f.countErr
}

// Unused interface methods — stubs only.
func (f *fakeAuthRepo) CreateUser(_ context.Context, _ *auth.AdminUser) error { return nil }
func (f *fakeAuthRepo) GetUserByID(_ context.Context, _ string) (*auth.AdminUser, error) {
	return nil, nil
}
func (f *fakeAuthRepo) GetUserByEmail(_ context.Context, _ string) (*auth.AdminUser, error) {
	return nil, nil
}
func (f *fakeAuthRepo) UpdateUser(_ context.Context, _ *auth.AdminUser) error    { return nil }
func (f *fakeAuthRepo) UpdateLastLogin(_ context.Context, _ string, _ time.Time) error { return nil }
func (f *fakeAuthRepo) DeleteUser(_ context.Context, _ string) error              { return nil }
func (f *fakeAuthRepo) CreateResetToken(_ context.Context, _ *auth.PasswordResetToken) error {
	return nil
}
func (f *fakeAuthRepo) GetResetToken(_ context.Context, _ string) (*auth.PasswordResetToken, error) {
	return nil, nil
}
func (f *fakeAuthRepo) MarkResetTokenUsed(_ context.Context, _ string) error { return nil }
func (f *fakeAuthRepo) CreateRefreshToken(_ context.Context, _ *auth.RefreshToken) error {
	return nil
}
func (f *fakeAuthRepo) GetRefreshToken(_ context.Context, _ string) (*auth.RefreshToken, error) {
	return nil, nil
}
func (f *fakeAuthRepo) RevokeRefreshToken(_ context.Context, _ string) error    { return nil }
func (f *fakeAuthRepo) RevokeAllUserTokens(_ context.Context, _ string) error { return nil }

func TestPublicHandler_GetStats_ReturnsCount(t *testing.T) {
	repo := &fakeAuthRepo{countResult: 42}
	handler := NewPublicHandler(repo)

	app := fiber.New()
	app.Get("/v1/public/stats", handler.GetStats)

	req := httptest.NewRequest(http.MethodGet, "/v1/public/stats", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, float64(42), body["user_count"])
}

func TestPublicHandler_GetStats_ZeroUsers(t *testing.T) {
	repo := &fakeAuthRepo{countResult: 0}
	handler := NewPublicHandler(repo)

	app := fiber.New()
	app.Get("/v1/public/stats", handler.GetStats)

	req := httptest.NewRequest(http.MethodGet, "/v1/public/stats", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, float64(0), body["user_count"])
}

func TestPublicHandler_GetStats_RepoError(t *testing.T) {
	repo := &fakeAuthRepo{countErr: errors.New("es connection refused")}
	handler := NewPublicHandler(repo)

	app := fiber.New()
	app.Get("/v1/public/stats", handler.GetStats)

	req := httptest.NewRequest(http.MethodGet, "/v1/public/stats", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, "failed to fetch stats", body["error"])
}

func TestPublicHandler_GetStats_LargeCount(t *testing.T) {
	repo := &fakeAuthRepo{countResult: 1_000_000}
	handler := NewPublicHandler(repo)

	app := fiber.New()
	app.Get("/v1/public/stats", handler.GetStats)

	req := httptest.NewRequest(http.MethodGet, "/v1/public/stats", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, float64(1_000_000), body["user_count"])
}
