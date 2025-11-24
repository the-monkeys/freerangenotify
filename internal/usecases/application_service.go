package usecases

import (
	"context"

	"github.com/the-monkeys/freerangenotify/internal/domain/application"
)

// ApplicationService defines the interface for application-related business logic
type ApplicationService interface {
	// Application CRUD operations
	Create(ctx context.Context, app *application.Application) error
	GetByID(ctx context.Context, appID string) (*application.Application, error)
	GetByAPIKey(ctx context.Context, apiKey string) (*application.Application, error)
	Update(ctx context.Context, app *application.Application) error
	Delete(ctx context.Context, appID string) error
	List(ctx context.Context, filter application.ApplicationFilter) ([]*application.Application, int64, error)

	// API Key management
	RegenerateAPIKey(ctx context.Context, appID string) (string, error)
	ValidateAPIKey(ctx context.Context, apiKey string) (*application.Application, error)

	// Settings management
	UpdateSettings(ctx context.Context, appID string, settings application.Settings) error
	GetSettings(ctx context.Context, appID string) (*application.Settings, error)
}
