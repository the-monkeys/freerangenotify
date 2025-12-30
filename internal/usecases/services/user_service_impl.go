package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"github.com/the-monkeys/freerangenotify/pkg/utils"
	"go.uber.org/zap"
)

// UserServiceImpl implements the UserService interface
type UserServiceImpl struct {
	repo   user.Repository
	logger *zap.Logger
}

// NewUserService creates a new UserService
func NewUserService(repo user.Repository, logger *zap.Logger) *UserServiceImpl {
	return &UserServiceImpl{
		repo:   repo,
		logger: logger,
	}
}

// Create creates a new user
func (s *UserServiceImpl) Create(ctx context.Context, u *user.User) error {
	s.logger.Info("Creating user", zap.String("app_id", u.AppID))

	// Validate required fields
	if u.AppID == "" {
		return errors.BadRequest("app_id is required")
	}

	// Check if user already exists
	if u.Email != "" {
		existing, err := s.repo.GetByEmail(ctx, u.AppID, u.Email)
		if err == nil && existing != nil {
			return errors.Conflict("User with this email already exists")
		}
	}

	// Generate UUID if not provided
	if u.UserID == "" {
		u.UserID = uuid.New().String()
	}
	now := time.Now()
	u.CreatedAt = now
	u.UpdatedAt = now

	// Set default preferences if not provided
	if u.Preferences.EmailEnabled == nil && u.Preferences.PushEnabled == nil && u.Preferences.SMSEnabled == nil {
		u.Preferences.EmailEnabled = utils.BoolPtr(true)
		u.Preferences.PushEnabled = utils.BoolPtr(true)
		u.Preferences.SMSEnabled = utils.BoolPtr(true)
	}

	if err := s.repo.Create(ctx, u); err != nil {
		s.logger.Error("Failed to create user", zap.Error(err))
		return errors.DatabaseError("create user", err)
	}

	s.logger.Info("User created successfully", zap.String("user_id", u.UserID))
	return nil
}

// GetByID retrieves a user by ID
func (s *UserServiceImpl) GetByID(ctx context.Context, userID string) (*user.User, error) {
	if userID == "" {
		return nil, errors.BadRequest("user_id is required")
	}

	u, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get user", zap.String("user_id", userID), zap.Error(err))
		return nil, errors.DatabaseError("get user", err)
	}
	if u == nil {
		return nil, errors.NotFound("User", userID)
	}

	return u, nil
}

// GetByEmail retrieves a user by email
func (s *UserServiceImpl) GetByEmail(ctx context.Context, appID, email string) (*user.User, error) {
	if appID == "" || email == "" {
		return nil, errors.BadRequest("app_id and email are required")
	}

	u, err := s.repo.GetByEmail(ctx, appID, email)
	if err != nil {
		s.logger.Error("Failed to get user by email", zap.Error(err))
		return nil, errors.DatabaseError("get user by email", err)
	}
	if u == nil {
		return nil, errors.NotFound("User", email)
	}

	return u, nil
}

// Update updates a user
func (s *UserServiceImpl) Update(ctx context.Context, u *user.User) error {
	if u.UserID == "" {
		return errors.BadRequest("user_id is required")
	}

	// Check if user exists
	existing, err := s.repo.GetByID(ctx, u.UserID)
	if err != nil {
		return errors.DatabaseError("check user existence", err)
	}
	if existing == nil {
		return errors.NotFound("User", u.UserID)
	}

	u.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, u); err != nil {
		s.logger.Error("Failed to update user", zap.String("user_id", u.UserID), zap.Error(err))
		return errors.DatabaseError("update user", err)
	}

	s.logger.Info("User updated successfully", zap.String("user_id", u.UserID))
	return nil
}

// Delete deletes a user
func (s *UserServiceImpl) Delete(ctx context.Context, userID string) error {
	if userID == "" {
		return errors.BadRequest("user_id is required")
	}

	// Check if user exists
	existing, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return errors.DatabaseError("check user existence", err)
	}
	if existing == nil {
		return errors.NotFound("User", userID)
	}

	if err := s.repo.Delete(ctx, userID); err != nil {
		s.logger.Error("Failed to delete user", zap.String("user_id", userID), zap.Error(err))
		return errors.DatabaseError("delete user", err)
	}

	s.logger.Info("User deleted successfully", zap.String("user_id", userID))
	return nil
}

// List retrieves users with filtering
func (s *UserServiceImpl) List(ctx context.Context, filter user.UserFilter) ([]*user.User, int64, error) {
	users, err := s.repo.List(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to list users", zap.Error(err))
		return nil, 0, errors.DatabaseError("list users", err)
	}

	// Get total count
	count, err := s.repo.Count(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to count users", zap.Error(err))
		return users, 0, errors.DatabaseError("count users", err)
	}

	return users, count, nil
}

// AddDevice adds a device to a user
func (s *UserServiceImpl) AddDevice(ctx context.Context, userID string, device user.Device) error {
	if userID == "" {
		return errors.BadRequest("user_id is required")
	}
	if device.DeviceID == "" {
		device.DeviceID = uuid.New().String()
	}
	if device.Platform == "" {
		return errors.BadRequest("device platform is required")
	}
	if device.Token == "" {
		return errors.BadRequest("device token is required")
	}

	device.Active = true
	device.RegisteredAt = time.Now()

	if err := s.repo.AddDevice(ctx, userID, device); err != nil {
		s.logger.Error("Failed to add device", zap.String("user_id", userID), zap.Error(err))
		return errors.DatabaseError("add device", err)
	}

	s.logger.Info("Device added successfully", zap.String("user_id", userID), zap.String("device_id", device.DeviceID))
	return nil
}

// RemoveDevice removes a device from a user
func (s *UserServiceImpl) RemoveDevice(ctx context.Context, userID, deviceID string) error {
	if userID == "" || deviceID == "" {
		return errors.BadRequest("user_id and device_id are required")
	}

	if err := s.repo.RemoveDevice(ctx, userID, deviceID); err != nil {
		s.logger.Error("Failed to remove device", zap.String("user_id", userID), zap.String("device_id", deviceID), zap.Error(err))
		return errors.DatabaseError("remove device", err)
	}

	s.logger.Info("Device removed successfully", zap.String("user_id", userID), zap.String("device_id", deviceID))
	return nil
}

// GetDevices retrieves all devices for a user
func (s *UserServiceImpl) GetDevices(ctx context.Context, userID string) ([]user.Device, error) {
	if userID == "" {
		return nil, errors.BadRequest("user_id is required")
	}

	u, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, errors.DatabaseError("get user", err)
	}
	if u == nil {
		return nil, errors.NotFound("User", userID)
	}

	return u.Devices, nil
}

// UpdatePreferences updates user preferences
func (s *UserServiceImpl) UpdatePreferences(ctx context.Context, userID string, preferences user.Preferences) error {
	if userID == "" {
		return errors.BadRequest("user_id is required")
	}

	if err := s.repo.UpdatePreferences(ctx, userID, preferences); err != nil {
		s.logger.Error("Failed to update preferences", zap.String("user_id", userID), zap.Error(err))
		return errors.DatabaseError("update preferences", err)
	}

	s.logger.Info("Preferences updated successfully", zap.String("user_id", userID))
	return nil
}

// GetPreferences retrieves user preferences
func (s *UserServiceImpl) GetPreferences(ctx context.Context, userID string) (*user.Preferences, error) {
	if userID == "" {
		return nil, errors.BadRequest("user_id is required")
	}

	u, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, errors.DatabaseError("get user", err)
	}
	if u == nil {
		return nil, errors.NotFound("User", userID)
	}

	return &u.Preferences, nil
}

// BulkCreate creates multiple users
func (s *UserServiceImpl) BulkCreate(ctx context.Context, users []*user.User) error {
	if len(users) == 0 {
		return errors.BadRequest("users array is empty")
	}

	// Validate and set IDs for all users
	now := time.Now()
	for _, u := range users {
		if u.AppID == "" {
			return errors.BadRequest("app_id is required for all users")
		}
		if u.UserID == "" {
			u.UserID = uuid.New().String()
		}
		u.CreatedAt = now
		u.UpdatedAt = now

		// Set default preferences if not provided
		if u.Preferences.EmailEnabled == nil && u.Preferences.PushEnabled == nil && u.Preferences.SMSEnabled == nil {
			u.Preferences.EmailEnabled = utils.BoolPtr(true)
			u.Preferences.PushEnabled = utils.BoolPtr(true)
			u.Preferences.SMSEnabled = utils.BoolPtr(true)
		}
	}

	if err := s.repo.BulkCreate(ctx, users); err != nil {
		s.logger.Error("Failed to bulk create users", zap.Int("count", len(users)), zap.Error(err))
		return errors.DatabaseError("bulk create users", err)
	}

	s.logger.Info("Users created successfully", zap.Int("count", len(users)))
	return nil
}
