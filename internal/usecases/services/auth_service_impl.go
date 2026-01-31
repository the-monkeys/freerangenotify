package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/smtp"
	"os"
	"strconv"
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
		ExpiresAt: time.Now().Add(5 * time.Minute), // 5 minutes expiry
		Used:      false,
		CreatedAt: time.Now(),
	}

	if err := s.repo.CreateResetToken(ctx, token); err != nil {
		return fmt.Errorf("failed to create reset token: %w", err)
	}

	// Send password reset email
	if err := s.sendPasswordResetEmail(ctx, user, resetToken); err != nil {
		s.logger.Error("Failed to send password reset email",
			zap.String("user_id", user.UserID),
			zap.String("email", user.Email),
			zap.Error(err),
		)
		// Don't fail the request if email fails - token is still valid
		// User can check logs or contact support
	}

	s.logger.Info("Password reset token generated and email sent",
		zap.String("user_id", user.UserID),
		zap.String("email", user.Email),
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

// sendPasswordResetEmail sends a password reset email to the user
func (s *authService) sendPasswordResetEmail(ctx context.Context, user *auth.AdminUser, resetToken string) error {
	// Get SMTP configuration from environment
	smtpHost := os.Getenv("FREERANGE_PROVIDERS_SMTP_HOST")
	smtpPortStr := os.Getenv("FREERANGE_PROVIDERS_SMTP_PORT")
	smtpUsername := os.Getenv("FREERANGE_PROVIDERS_SMTP_USERNAME")
	smtpPassword := os.Getenv("FREERANGE_PROVIDERS_SMTP_PASSWORD")
	smtpFromEmail := os.Getenv("FREERANGE_PROVIDERS_SMTP_FROM_EMAIL")
	smtpFromName := os.Getenv("FREERANGE_PROVIDERS_SMTP_FROM_NAME")

	// Check if SMTP is configured
	if smtpHost == "" || smtpUsername == "" || smtpPassword == "" || smtpFromEmail == "" {
		s.logger.Warn("SMTP not configured, password reset email not sent",
			zap.String("user_id", user.UserID),
			zap.String("token", resetToken),
			zap.String("missing_config", fmt.Sprintf("host=%v, username=%v, password=%v, from=%v",
				smtpHost == "", smtpUsername == "", smtpPassword == "", smtpFromEmail == "")),
		)
		return fmt.Errorf("SMTP not configured - check SMTP environment variables")
	}

	// Parse port
	smtpPort := 587
	if smtpPortStr != "" {
		if port, err := strconv.Atoi(smtpPortStr); err == nil {
			smtpPort = port
		}
	}

	if smtpFromName == "" {
		smtpFromName = "FreeRangeNotify"
	}

	// Build the reset link - get frontend URL from environment or use default
	frontendURL := os.Getenv("FREERANGE_FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}
	resetLink := fmt.Sprintf("%s/reset-password?token=%s", frontendURL, resetToken)

	// HTML email body
	subject := "Reset Your Password - FreeRangeNotify"
	emailBody := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Password Reset</title>
</head>
<body style="font-family: Arial, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px;">
    <div style="background-color: #f8f9fa; border-radius: 10px; padding: 30px; margin-bottom: 20px;">
        <h1 style="color: #2563eb; margin-top: 0;">Password Reset Request</h1>
        <p>Hello,</p>
        <p>We received a request to reset your password for your FreeRangeNotify account.</p>
        <p>Click the button below to reset your password. This link will expire in <strong>5 minutes</strong>.</p>
        <div style="text-align: center; margin: 30px 0;">
            <a href="%s" 
               style="background-color: #2563eb; color: white; padding: 12px 30px; text-decoration: none; border-radius: 5px; display: inline-block; font-weight: bold;">
                Reset Password
            </a>
        </div>
        <p>Or copy and paste this link into your browser:</p>
        <p style="background-color: #e9ecef; padding: 10px; border-radius: 5px; word-break: break-all; font-family: monospace; font-size: 12px;">
            %s
        </p>
        <hr style="border: none; border-top: 1px solid #dee2e6; margin: 30px 0;">
        <p style="color: #6c757d; font-size: 14px;">
            If you didn't request a password reset, you can safely ignore this email. Your password will remain unchanged.
        </p>
        <p style="color: #6c757d; font-size: 14px;">
            For security reasons, this link will expire in 5 minutes.
        </p>
    </div>
    <div style="text-align: center; color: #6c757d; font-size: 12px;">
        <p>© 2026 FreeRangeNotify. All rights reserved.</p>
    </div>
</body>
</html>
`, resetLink, resetLink)

	// Construct the email message
	message := fmt.Sprintf("From: %s <%s>\r\n", smtpFromName, smtpFromEmail)
	message += fmt.Sprintf("To: %s\r\n", user.Email)
	message += fmt.Sprintf("Subject: %s\r\n", subject)
	message += "MIME-Version: 1.0\r\n"
	message += "Content-Type: text/html; charset=\"UTF-8\"\r\n"
	message += "\r\n"
	message += emailBody

	// Set up authentication
	auth := smtp.PlainAuth("", smtpUsername, smtpPassword, smtpHost)

	// Send the email
	addr := fmt.Sprintf("%s:%d", smtpHost, smtpPort)
	err := smtp.SendMail(addr, auth, smtpFromEmail, []string{user.Email}, []byte(message))
	if err != nil {
		s.logger.Error("Failed to send password reset email via SMTP",
			zap.String("user_id", user.UserID),
			zap.String("email", user.Email),
			zap.String("smtp_host", smtpHost),
			zap.Int("smtp_port", smtpPort),
			zap.Error(err),
		)
		return fmt.Errorf("failed to send email: %w", err)
	}

	s.logger.Info("Password reset email sent successfully",
		zap.String("user_id", user.UserID),
		zap.String("email", user.Email),
		zap.String("smtp_host", smtpHost),
	)

	return nil
}
