package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"time"

	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"go.uber.org/zap"
)

// ApplicationServiceImpl implements the ApplicationService interface
type ApplicationServiceImpl struct {
	repo   application.Repository
	logger *zap.Logger
}

// NewApplicationService creates a new ApplicationService
func NewApplicationService(repo application.Repository, logger *zap.Logger) *ApplicationServiceImpl {
	return &ApplicationServiceImpl{
		repo:   repo,
		logger: logger,
	}
}

// Create creates a new application
func (s *ApplicationServiceImpl) Create(ctx context.Context, app *application.Application) error {
	s.logger.Info("Creating application", zap.String("app_name", app.AppName))

	// Validate required fields
	if app.AppName == "" {
		return errors.BadRequest("app_name is required")
	}

	// Generate UUID for app ID
	app.AppID = uuid.New().String()

	// Generate API key
	apiKey, err := generateAPIKey()
	if err != nil {
		s.logger.Error("Failed to generate API key", zap.Error(err))
		return errors.Internal("Failed to generate API key", err)
	}
	app.APIKey = apiKey
	app.APIKeyGeneratedAt = time.Now()

	now := time.Now()
	app.CreatedAt = now
	app.UpdatedAt = now

	// Set default settings if not provided
	if app.Settings.RateLimit == 0 {
		app.Settings.RateLimit = 1000 // Default: 1000 requests per minute
	}
	if app.Settings.RetryAttempts == 0 {
		app.Settings.RetryAttempts = 3 // Default: 3 retry attempts
	}

	if err := s.repo.Create(ctx, app); err != nil {
		s.logger.Error("Failed to create application", zap.Error(err))
		return errors.DatabaseError("create application", err)
	}

	s.logger.Info("Application created successfully", zap.String("app_id", app.AppID))
	return nil
}

// GetByID retrieves an application by ID
func (s *ApplicationServiceImpl) GetByID(ctx context.Context, appID string) (*application.Application, error) {
	if appID == "" {
		return nil, errors.BadRequest("app_id is required")
	}

	app, err := s.repo.GetByID(ctx, appID)
	if err != nil {
		s.logger.Error("Failed to get application", zap.String("app_id", appID), zap.Error(err))
		return nil, errors.DatabaseError("get application", err)
	}
	if app == nil {
		return nil, errors.NotFound("Application", appID)
	}

	return app, nil
}

// GetByAPIKey retrieves an application by API key
func (s *ApplicationServiceImpl) GetByAPIKey(ctx context.Context, apiKey string) (*application.Application, error) {
	if apiKey == "" {
		return nil, errors.BadRequest("api_key is required")
	}

	app, err := s.repo.GetByAPIKey(ctx, apiKey)
	if err != nil {
		s.logger.Error("Failed to get application by API key", zap.Error(err))
		return nil, errors.DatabaseError("get application by API key", err)
	}
	if app == nil {
		return nil, errors.New(errors.ErrCodeInvalidAPIKey, "Invalid API key")
	}

	return app, nil
}

// Update updates an application
func (s *ApplicationServiceImpl) Update(ctx context.Context, app *application.Application) error {
	if app.AppID == "" {
		return errors.BadRequest("app_id is required")
	}

	// Check if application exists
	existing, err := s.repo.GetByID(ctx, app.AppID)
	if err != nil {
		return errors.DatabaseError("check application existence", err)
	}
	if existing == nil {
		return errors.NotFound("Application", app.AppID)
	}

	app.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, app); err != nil {
		s.logger.Error("Failed to update application", zap.String("app_id", app.AppID), zap.Error(err))
		return errors.DatabaseError("update application", err)
	}

	s.logger.Info("Application updated successfully", zap.String("app_id", app.AppID))
	return nil
}

// Delete deletes an application
func (s *ApplicationServiceImpl) Delete(ctx context.Context, appID string) error {
	if appID == "" {
		return errors.BadRequest("app_id is required")
	}

	// Check if application exists
	existing, err := s.repo.GetByID(ctx, appID)
	if err != nil {
		return errors.DatabaseError("check application existence", err)
	}
	if existing == nil {
		return errors.NotFound("Application", appID)
	}

	if err := s.repo.Delete(ctx, appID); err != nil {
		s.logger.Error("Failed to delete application", zap.String("app_id", appID), zap.Error(err))
		return errors.DatabaseError("delete application", err)
	}

	s.logger.Info("Application deleted successfully", zap.String("app_id", appID))
	return nil
}

// List retrieves applications with filtering
func (s *ApplicationServiceImpl) List(ctx context.Context, filter application.ApplicationFilter) ([]*application.Application, int64, error) {
	apps, err := s.repo.List(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to list applications", zap.Error(err))
		return nil, 0, errors.DatabaseError("list applications", err)
	}

	// Get total count
	count := int64(len(apps))

	return apps, count, nil
}

// RegenerateAPIKey generates a new API key for an application
func (s *ApplicationServiceImpl) RegenerateAPIKey(ctx context.Context, appID string) (string, error) {
	if appID == "" {
		return "", errors.BadRequest("app_id is required")
	}

	// Check if application exists
	app, err := s.repo.GetByID(ctx, appID)
	if err != nil {
		return "", errors.DatabaseError("check application existence", err)
	}
	if app == nil {
		return "", errors.NotFound("Application", appID)
	}

	newAPIKey, err := s.repo.RegenerateAPIKey(ctx, appID)
	if err != nil {
		s.logger.Error("Failed to regenerate API key", zap.String("app_id", appID), zap.Error(err))
		return "", errors.DatabaseError("regenerate API key", err)
	}

	s.logger.Info("API key regenerated successfully", zap.String("app_id", appID))
	return newAPIKey, nil
}

// ValidateAPIKey validates an API key and returns the associated application
func (s *ApplicationServiceImpl) ValidateAPIKey(ctx context.Context, apiKey string) (*application.Application, error) {
	if apiKey == "" {
		return nil, errors.New(errors.ErrCodeInvalidAPIKey, "API key is required")
	}

	app, err := s.repo.GetByAPIKey(ctx, apiKey)
	if err != nil {
		s.logger.Error("Failed to validate API key", zap.Error(err))
		return nil, errors.DatabaseError("validate API key", err)
	}
	if app == nil {
		return nil, errors.New(errors.ErrCodeInvalidAPIKey, "Invalid API key")
	}

	return app, nil
}

// UpdateSettings updates application settings
func (s *ApplicationServiceImpl) UpdateSettings(ctx context.Context, appID string, settings application.Settings) error {
	if appID == "" {
		return errors.BadRequest("app_id is required")
	}

	// Check if application exists
	app, err := s.repo.GetByID(ctx, appID)
	if err != nil {
		return errors.DatabaseError("check application existence", err)
	}
	if app == nil {
		return errors.NotFound("Application", appID)
	}

	app.Settings = settings
	app.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, app); err != nil {
		s.logger.Error("Failed to update settings", zap.String("app_id", appID), zap.Error(err))
		return errors.DatabaseError("update settings", err)
	}

	s.logger.Info("Settings updated successfully", zap.String("app_id", appID))
	return nil
}

// GetSettings retrieves application settings
func (s *ApplicationServiceImpl) GetSettings(ctx context.Context, appID string) (*application.Settings, error) {
	if appID == "" {
		return nil, errors.BadRequest("app_id is required")
	}

	app, err := s.repo.GetByID(ctx, appID)
	if err != nil {
		return nil, errors.DatabaseError("get application", err)
	}
	if app == nil {
		return nil, errors.NotFound("Application", appID)
	}

	return &app.Settings, nil
}

// generateAPIKey generates a random API key
func generateAPIKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "frn_" + base64.URLEncoding.EncodeToString(b), nil
}
