package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"github.com/the-monkeys/freerangenotify/pkg/jwt"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type authService struct {
	repo                auth.Repository
	jwtManager          *jwt.Manager
	notificationService notification.Service
	logger              *zap.Logger
}

// NewAuthService creates a new auth service
func NewAuthService(
	repo auth.Repository,
	jwtManager *jwt.Manager,
	notificationService notification.Service,
	logger *zap.Logger,
) auth.Service {
	return &authService{
		repo:                repo,
		jwtManager:          jwtManager,
		notificationService: notificationService,
		logger:              logger,
	}
}

// Register registers a new admin user
func (s *authService) Register(ctx context.Context, req *auth.RegisterRequest) (*auth.AuthResponse, error) {
	// Check if user already exists
	existingUser, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}

	if existingUser != nil {
		return nil, errors.BadRequest("User with this email already exists")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user
	user := &auth.AdminUser{
		UserID:       uuid.New().String(),
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		FullName:     req.FullName,
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := s.repo.CreateUser(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Generate tokens
	tokens, err := s.generateTokenPair(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	s.logger.Info("User registered successfully",
		zap.String("user_id", user.UserID),
		zap.String("email", user.Email),
	)

	// Don't return password hash
	user.PasswordHash = ""

	return &auth.AuthResponse{
		User:   user,
		Tokens: tokens,
	}, nil
}

// Login authenticates a user
func (s *authService) Login(ctx context.Context, req *auth.LoginRequest) (*auth.AuthResponse, error) {
	// Get user by email
	user, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if user == nil {
		return nil, errors.Unauthorized("Invalid email or password")
	}

	// Check if user is active
	if !user.IsActive {
		return nil, errors.Unauthorized("Account is deactivated")
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, errors.Unauthorized("Invalid email or password")
	}

	// Update last login
	if err := s.repo.UpdateLastLogin(ctx, user.UserID, time.Now()); err != nil {
		s.logger.Error("Failed to update last login", zap.Error(err))
	}

	// Generate tokens
	tokens, err := s.generateTokenPair(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	s.logger.Info("User logged in successfully",
		zap.String("user_id", user.UserID),
		zap.String("email", user.Email),
	)

	// Don't return password hash
	user.PasswordHash = ""

	return &auth.AuthResponse{
		User:   user,
		Tokens: tokens,
	}, nil
}

// RefreshAccessToken refreshes the access token using a refresh token
func (s *authService) RefreshAccessToken(ctx context.Context, refreshToken string) (*auth.TokenPair, error) {
	// Validate refresh token
	claims, err := s.jwtManager.ValidateToken(refreshToken)
	if err != nil {
		return nil, errors.Unauthorized("Invalid refresh token")
	}

	// Check if refresh token exists and is valid
	storedToken, err := s.repo.GetRefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get refresh token: %w", err)
	}

	if storedToken == nil || storedToken.Revoked || time.Now().After(storedToken.ExpiresAt) {
		return nil, errors.Unauthorized("Refresh token is invalid or expired")
	}

	// Get user
	user, err := s.repo.GetUserByID(ctx, claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if user == nil || !user.IsActive {
		return nil, errors.Unauthorized("User not found or inactive")
	}

	// Generate new tokens
	tokens, err := s.generateTokenPair(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	// Revoke old refresh token
	if err := s.repo.RevokeRefreshToken(ctx, storedToken.TokenID); err != nil {
		s.logger.Error("Failed to revoke old refresh token", zap.Error(err))
	}

	return tokens, nil
}

// Logout logs out a user by revoking all their tokens
func (s *authService) Logout(ctx context.Context, userID string) error {
	if err := s.repo.RevokeAllUserTokens(ctx, userID); err != nil {
		return fmt.Errorf("failed to revoke tokens: %w", err)
	}

	s.logger.Info("User logged out", zap.String("user_id", userID))
	return nil
}

// ForgotPassword initiates password reset process
func (s *authService) ForgotPassword(ctx context.Context, req *auth.ForgotPasswordRequest) error {
	// Get user by email
	user, err := s.repo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	// Don't reveal if user exists or not for security
	if user == nil {
		s.logger.Info("Password reset requested for non-existent email", zap.String("email", req.Email))
		return nil
	}

	// Generate reset token
	resetToken, err := generateSecureToken(32)
	if err != nil {
		return fmt.Errorf("failed to generate reset token: %w", err)
	}

	// Store reset token
	token := &auth.PasswordResetToken{
		TokenID:   uuid.New().String(),
		UserID:    user.UserID,
		Token:     resetToken,
		ExpiresAt: time.Now().Add(1 * time.Hour), // 1 hour expiry
		Used:      false,
		CreatedAt: time.Now(),
	}

	if err := s.repo.CreateResetToken(ctx, token); err != nil {
		return fmt.Errorf("failed to create reset token: %w", err)
	}

	// TODO: Send email with reset link
	// For now, just log the token
	s.logger.Info("Password reset token generated",
		zap.String("user_id", user.UserID),
		zap.String("email", user.Email),
		zap.String("token", resetToken),
	)

	return nil
}

// ResetPassword resets a user's password using a reset token
func (s *authService) ResetPassword(ctx context.Context, req *auth.ResetPasswordRequest) error {
	// Get reset token
	token, err := s.repo.GetResetToken(ctx, req.Token)
	if err != nil {
		return fmt.Errorf("failed to get reset token: %w", err)
	}

	if token == nil || token.Used || time.Now().After(token.ExpiresAt) {
		return errors.BadRequest("Invalid or expired reset token")
	}

	// Get user
	user, err := s.repo.GetUserByID(ctx, token.UserID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	if user == nil {
		return errors.NotFound("user", "User not found")
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update password
	user.PasswordHash = string(hashedPassword)
	user.UpdatedAt = time.Now()

	if err := s.repo.UpdateUser(ctx, user); err != nil {
		return fmt.Errorf("failed to update user password: %w", err)
	}

	// Mark token as used
	if err := s.repo.MarkResetTokenUsed(ctx, token.TokenID); err != nil {
		s.logger.Error("Failed to mark reset token as used", zap.Error(err))
	}

	// Revoke all existing tokens for security
	if err := s.repo.RevokeAllUserTokens(ctx, user.UserID); err != nil {
		s.logger.Error("Failed to revoke user tokens after password reset", zap.Error(err))
	}

	s.logger.Info("Password reset successfully",
		zap.String("user_id", user.UserID),
		zap.String("email", user.Email),
	)

	return nil
}

// ChangePassword changes a user's password
func (s *authService) ChangePassword(ctx context.Context, userID string, req *auth.ChangePasswordRequest) error {
	// Get user
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	if user == nil {
		return errors.NotFound("user", "User not found")
	}

	// Verify old password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.OldPassword)); err != nil {
		return errors.Unauthorized("Invalid old password")
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update password
	user.PasswordHash = string(hashedPassword)
	user.UpdatedAt = time.Now()

	if err := s.repo.UpdateUser(ctx, user); err != nil {
		return fmt.Errorf("failed to update user password: %w", err)
	}

	// Revoke all existing tokens for security
	if err := s.repo.RevokeAllUserTokens(ctx, userID); err != nil {
		s.logger.Error("Failed to revoke user tokens after password change", zap.Error(err))
	}

	s.logger.Info("Password changed successfully",
		zap.String("user_id", userID),
		zap.String("email", user.Email),
	)

	return nil
}

// GetCurrentUser gets the current authenticated user
func (s *authService) GetCurrentUser(ctx context.Context, userID string) (*auth.AdminUser, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if user == nil {
		return nil, errors.NotFound("user", "User not found")
	}

	// Don't return password hash
	user.PasswordHash = ""

	return user, nil
}

// ValidateToken validates a JWT token and returns the user
func (s *authService) ValidateToken(ctx context.Context, token string) (*auth.AdminUser, error) {
	claims, err := s.jwtManager.ValidateToken(token)
	if err != nil {
		return nil, errors.Unauthorized("Invalid token")
	}

	user, err := s.repo.GetUserByID(ctx, claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if user == nil || !user.IsActive {
		return nil, errors.Unauthorized("User not found or inactive")
	}

	// Don't return password hash
	user.PasswordHash = ""

	return user, nil
}

// generateTokenPair generates access and refresh tokens for a user
func (s *authService) generateTokenPair(ctx context.Context, user *auth.AdminUser) (*auth.TokenPair, error) {
	// Generate access token
	accessToken, accessExpiresAt, err := s.jwtManager.GenerateAccessToken(user.UserID, user.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	// Generate refresh token
	refreshToken, refreshExpiresAt, err := s.jwtManager.GenerateRefreshToken(user.UserID, user.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Store refresh token
	token := &auth.RefreshToken{
		TokenID:   uuid.New().String(),
		UserID:    user.UserID,
		Token:     refreshToken,
		ExpiresAt: refreshExpiresAt,
		Revoked:   false,
		CreatedAt: time.Now(),
	}

	if err := s.repo.CreateRefreshToken(ctx, token); err != nil {
		return nil, fmt.Errorf("failed to store refresh token: %w", err)
	}

	return &auth.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    accessExpiresAt,
	}, nil
}

// generateSecureToken generates a cryptographically secure random token
func generateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
