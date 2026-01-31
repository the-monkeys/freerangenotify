package auth

import (
	"context"
	"time"
)

// AdminUser represents an admin user who can log into the dashboard
type AdminUser struct {
	UserID       string    `json:"user_id" es:"user_id"`
	Email        string    `json:"email" es:"email"`
	PasswordHash string    `json:"password_hash" es:"password_hash"`
	FullName     string    `json:"full_name,omitempty" es:"full_name"`
	IsActive     bool      `json:"is_active" es:"is_active"`
	CreatedAt    time.Time `json:"created_at" es:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" es:"updated_at"`
	LastLoginAt  time.Time `json:"last_login_at,omitempty" es:"last_login_at"`
}

// PasswordResetToken represents a password reset token
type PasswordResetToken struct {
	TokenID   string    `json:"token_id" es:"token_id"`
	UserID    string    `json:"user_id" es:"user_id"`
	Token     string    `json:"token" es:"token"`
	ExpiresAt time.Time `json:"expires_at" es:"expires_at"`
	Used      bool      `json:"used" es:"used"`
	CreatedAt time.Time `json:"created_at" es:"created_at"`
}

// RefreshToken represents a refresh token for JWT
type RefreshToken struct {
	TokenID   string    `json:"token_id" es:"token_id"`
	UserID    string    `json:"user_id" es:"user_id"`
	Token     string    `json:"token" es:"token"`
	ExpiresAt time.Time `json:"expires_at" es:"expires_at"`
	Revoked   bool      `json:"revoked" es:"revoked"`
	CreatedAt time.Time `json:"created_at" es:"created_at"`
}

// LoginRequest represents a login request
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

// RegisterRequest represents a registration request
type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	FullName string `json:"full_name" validate:"required,min=2"`
}

// ForgotPasswordRequest represents a forgot password request
type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// ResetPasswordRequest represents a reset password request
type ResetPasswordRequest struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8"`
}

// ChangePasswordRequest represents a change password request
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8"`
}

// TokenPair represents access and refresh tokens
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// AuthResponse represents authentication response
type AuthResponse struct {
	User   *AdminUser `json:"user"`
	Tokens *TokenPair `json:"tokens"`
}

// Repository defines the interface for auth data operations
type Repository interface {
	// Admin User operations
	CreateUser(ctx context.Context, user *AdminUser) error
	GetUserByID(ctx context.Context, userID string) (*AdminUser, error)
	GetUserByEmail(ctx context.Context, email string) (*AdminUser, error)
	UpdateUser(ctx context.Context, user *AdminUser) error
	UpdateLastLogin(ctx context.Context, userID string, loginTime time.Time) error

	// Password reset operations
	CreateResetToken(ctx context.Context, token *PasswordResetToken) error
	GetResetToken(ctx context.Context, token string) (*PasswordResetToken, error)
	MarkResetTokenUsed(ctx context.Context, tokenID string) error

	// Refresh token operations
	CreateRefreshToken(ctx context.Context, token *RefreshToken) error
	GetRefreshToken(ctx context.Context, token string) (*RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, tokenID string) error
	RevokeAllUserTokens(ctx context.Context, userID string) error
}

// Service defines the business logic interface for authentication
type Service interface {
	// Authentication
	Register(ctx context.Context, req *RegisterRequest) (*AuthResponse, error)
	Login(ctx context.Context, req *LoginRequest) (*AuthResponse, error)
	RefreshAccessToken(ctx context.Context, refreshToken string) (*TokenPair, error)
	Logout(ctx context.Context, userID string) error

	// Password management
	ForgotPassword(ctx context.Context, req *ForgotPasswordRequest) error
	ResetPassword(ctx context.Context, req *ResetPasswordRequest) error
	ChangePassword(ctx context.Context, userID string, req *ChangePasswordRequest) error

	// User operations
	GetCurrentUser(ctx context.Context, userID string) (*AdminUser, error)
	ValidateToken(ctx context.Context, token string) (*AdminUser, error)
}
