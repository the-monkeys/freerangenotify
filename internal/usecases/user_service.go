package usecases

import (
	"context"

	"github.com/the-monkeys/freerangenotify/internal/domain/user"
)

// UserService defines the interface for user-related business logic
type UserService interface {
	// User CRUD operations
	Create(ctx context.Context, user *user.User) error
	GetByID(ctx context.Context, userID string) (*user.User, error)
	GetByExternalID(ctx context.Context, appID, externalUserID string) (*user.User, error)
	GetByEmail(ctx context.Context, appID, email string) (*user.User, error)
	Update(ctx context.Context, user *user.User) error
	Delete(ctx context.Context, userID string) error
	List(ctx context.Context, filter user.UserFilter) ([]*user.User, int64, error)

	// Device management
	AddDevice(ctx context.Context, userID string, device user.Device) error
	RemoveDevice(ctx context.Context, userID, deviceID string) error
	GetDevices(ctx context.Context, userID string) ([]user.Device, error)

	// Preferences management
	UpdatePreferences(ctx context.Context, userID string, preferences user.Preferences) error
	GetPreferences(ctx context.Context, userID string) (*user.Preferences, error)

	// Bulk operations
	BulkCreate(ctx context.Context, users []*user.User) error
}
