package handlers

import (
	"context"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	httpmiddleware "github.com/the-monkeys/freerangenotify/internal/interfaces/http/middleware"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"github.com/the-monkeys/freerangenotify/pkg/validator"
	"go.uber.org/zap"
	"net/http"
	"testing"
)

type importUserService struct {
	*stubUserService
	writes     []*user.User
	lookupErr  error
	itemErrors []error
}

func (s *importUserService) GetByEmail(context.Context, string, string) (*user.User, error) {
	if s.lookupErr != nil {
		return nil, s.lookupErr
	}
	return nil, apperrors.NotFound("User", "fixture")
}
func (s *importUserService) BulkCreate(_ context.Context, rows []*user.User) error {
	s.writes = rows
	return nil
}
func (s *importUserService) BulkCreateResults(_ context.Context, rows []*user.User) ([]error, error) {
	s.writes = rows
	if s.itemErrors != nil {
		return s.itemErrors, nil
	}
	return make([]error, len(rows)), nil
}
func bulkTestApp(s *importUserService) *fiber.App {
	h := NewUserHandler(s, validator.New(), zap.NewNop())
	app := fiber.New(fiber.Config{ErrorHandler: httpmiddleware.ErrorHandler(zap.NewNop())})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("app_id", "app-1")
		c.Locals("environment_id", "staging")
		return c.Next()
	})
	app.Post("/users/bulk", h.BulkCreate)
	return app
}
func TestBulkImportDeduplicatesAndScopesNewUsers(t *testing.T) {
	s := &importUserService{stubUserService: newStubUserService()}
	resp, body := doJSON(t, bulkTestApp(s), http.MethodPost, "/users/bulk", map[string]any{"skip_existing": true, "users": []map[string]string{{"email": "same@example.com"}, {"email": "same@example.com"}}})
	require.Equal(t, 201, resp.StatusCode)
	require.Equal(t, float64(1), body["created"])
	require.Equal(t, float64(1), body["skipped"])
	require.Len(t, s.writes, 1)
	require.Equal(t, "staging", s.writes[0].EnvironmentID)
}
func TestBulkImportDoesNotCreateOnLookupOutage(t *testing.T) {
	s := &importUserService{stubUserService: newStubUserService(), lookupErr: fmt.Errorf("database unavailable")}
	resp, _ := doJSON(t, bulkTestApp(s), http.MethodPost, "/users/bulk", map[string]any{"skip_existing": true, "users": []map[string]string{{"email": "a@example.com"}}})
	require.Equal(t, 500, resp.StatusCode)
	require.Empty(t, s.writes)
}
func TestBulkImportReportsOnlyConfirmedWrites(t *testing.T) {
	s := &importUserService{stubUserService: newStubUserService(), itemErrors: []error{nil, fmt.Errorf("write rejected")}}
	resp, body := doJSON(t, bulkTestApp(s), http.MethodPost, "/users/bulk", map[string]any{"users": []map[string]string{{"email": "a@example.com"}, {"email": "b@example.com"}}})
	require.Equal(t, 201, resp.StatusCode)
	require.Equal(t, float64(1), body["created"])
	require.Equal(t, float64(2), body["total"])
	rows := body["errors"].([]any)
	require.Len(t, rows, 1)
	require.Equal(t, float64(1), rows[0].(map[string]any)["index"])
}
