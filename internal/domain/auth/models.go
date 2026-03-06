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
	DeleteUser(ctx context.Context, userID string) error

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

	// SSO
	SSOLogin(ctx context.Context, email, name string) (*AuthResponse, error)
}

// ─── Phase 2: RBAC Types ───────────────────────────────────────────────

// Role represents a named role within an application.
type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleEditor Role = "editor"
	RoleViewer Role = "viewer"
)

// Permission represents a granular permission within an application.
type Permission string

const (
	PermManageApp         Permission = "manage_app"
	PermManageMembers     Permission = "manage_members"
	PermManageTemplates   Permission = "manage_templates"
	PermSendNotifications Permission = "send_notifications"
	PermViewLogs          Permission = "view_logs"
	PermViewAudit         Permission = "view_audit"
)

// RolePermissions maps each role to its set of permissions.
// Roles are cumulative — higher roles inherit lower-role permissions.
var RolePermissions = map[Role][]Permission{
	RoleOwner:  {PermManageApp, PermManageMembers, PermManageTemplates, PermSendNotifications, PermViewLogs, PermViewAudit},
	RoleAdmin:  {PermManageMembers, PermManageTemplates, PermSendNotifications, PermViewLogs, PermViewAudit},
	RoleEditor: {PermManageTemplates, PermSendNotifications, PermViewLogs},
	RoleViewer: {PermViewLogs},
}

// HasPermission returns true if the given role grants the specified permission.
func HasPermission(role Role, perm Permission) bool {
	perms, ok := RolePermissions[role]
	if !ok {
		return false
	}
	for _, p := range perms {
		if p == perm {
			return true
		}
	}
	return false
}

// AppMembership links a user to an application with a specific role.
type AppMembership struct {
	MembershipID string    `json:"membership_id" es:"membership_id"`
	AppID        string    `json:"app_id" es:"app_id"`
	UserID       string    `json:"user_id" es:"user_id"`
	UserEmail    string    `json:"user_email" es:"user_email"`
	Role         Role      `json:"role" es:"role"`
	InvitedBy    string    `json:"invited_by" es:"invited_by"`
	CreatedAt    time.Time `json:"created_at" es:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" es:"updated_at"`
}

// InviteMemberRequest represents a request to invite a member to an app.
type InviteMemberRequest struct {
	Email string `json:"email" validate:"required,email"`
	Role  Role   `json:"role" validate:"required"`
}

// UpdateMemberRoleRequest represents a request to change a member's role.
type UpdateMemberRoleRequest struct {
	Role Role `json:"role" validate:"required"`
}

// MembershipRepository defines data access for app memberships.
type MembershipRepository interface {
	Create(ctx context.Context, m *AppMembership) error
	GetByID(ctx context.Context, id string) (*AppMembership, error)
	GetByAppAndUser(ctx context.Context, appID, userID string) (*AppMembership, error)
	ListByApp(ctx context.Context, appID string) ([]*AppMembership, error)
	ListByUser(ctx context.Context, userID string) ([]*AppMembership, error)
	Update(ctx context.Context, m *AppMembership) error
	Delete(ctx context.Context, id string) error
	// ClaimByEmail links pending invitations to an actual user ID.
	// It finds memberships where user_email matches and user_id still holds the
	// email (i.e. the invite hasn't been claimed yet) and updates user_id to the
	// real UUID.
	ClaimByEmail(ctx context.Context, email, actualUserID string) error
}

// TeamService defines the business logic interface for team/membership management.
type TeamService interface {
	InviteMember(ctx context.Context, appID string, req *InviteMemberRequest, inviterID string) (*AppMembership, error)
	UpdateRole(ctx context.Context, appID, membershipID string, req *UpdateMemberRoleRequest) (*AppMembership, error)
	RemoveMember(ctx context.Context, appID, membershipID string) error
	ListMembers(ctx context.Context, appID string) ([]*AppMembership, error)
	GetMembership(ctx context.Context, appID, userID string) (*AppMembership, error)
}
