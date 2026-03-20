package services

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/agentdebug"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"github.com/the-monkeys/freerangenotify/internal/domain/dashboard_notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/tenant"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"github.com/the-monkeys/freerangenotify/pkg/jwt"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type authService struct {
	repo                auth.Repository
	membershipRepo      auth.MembershipRepository
	jwtManager          *jwt.Manager
	notificationService notification.Service
	otpRepo             repository.OTPRepository
	otpSender           *OTPEmailSender
	notifier            dashboard_notification.Notifier
	subscriptionRepo    license.Repository
	appRepo             application.Repository
	tenantRepo          tenant.Repository
	tenantMemberRepo    tenant.MemberRepository
	esClient            *elasticsearch.Client
	cascadeDeleter      *CascadeDeleter
	logger              *zap.Logger
}

// NewAuthService creates a new auth service
func NewAuthService(
	repo auth.Repository,
	membershipRepo auth.MembershipRepository,
	jwtManager *jwt.Manager,
	notificationService notification.Service,
	otpRepo repository.OTPRepository,
	otpSender *OTPEmailSender,
	notifier dashboard_notification.Notifier,
	subscriptionRepo license.Repository,
	appRepo application.Repository,
	tenantRepo tenant.Repository,
	tenantMemberRepo tenant.MemberRepository,
	esClient *elasticsearch.Client,
	logger *zap.Logger,
) auth.Service {
	return &authService{
		repo:                repo,
		membershipRepo:      membershipRepo,
		jwtManager:          jwtManager,
		notificationService: notificationService,
		otpRepo:             otpRepo,
		otpSender:           otpSender,
		notifier:            notifier,
		subscriptionRepo:    subscriptionRepo,
		appRepo:             appRepo,
		tenantRepo:          tenantRepo,
		tenantMemberRepo:    tenantMemberRepo,
		esClient:            esClient,
		logger:              logger,
	}
}

// SetCascadeDeleter injects the cascade deleter for account deletion cleanup.
func (s *authService) SetCascadeDeleter(cd *CascadeDeleter) {
	s.cascadeDeleter = cd
}

// SetAuthCascadeDeleter is a package-level helper that injects the cascade
// deleter into an auth.Service backed by the unexported authService type.
func SetAuthCascadeDeleter(svc interface{}, cd *CascadeDeleter) {
	if s, ok := svc.(*authService); ok {
		s.SetCascadeDeleter(cd)
	}
}

// Register initiates registration by sending an OTP to the user's email.
// The account is NOT created until the OTP is verified via VerifyRegistrationOTP.
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

	// Generate 6-digit OTP
	otpCode, err := generateOTP(6)
	if err != nil {
		return nil, fmt.Errorf("failed to generate OTP: %w", err)
	}

	// Store pending registration in Redis
	pending := &auth.PendingRegistration{
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		FullName:     req.FullName,
		OTPCode:      otpCode,
		CreatedAt:    time.Now(),
		Attempts:     0,
	}

	if err := s.otpRepo.StorePendingRegistration(ctx, pending); err != nil {
		return nil, fmt.Errorf("failed to store pending registration: %w", err)
	}

	// Send OTP email
	if err := s.otpSender.SendOTP(req.Email, otpCode); err != nil {
		s.logger.Error("Failed to send OTP email", zap.String("email", req.Email), zap.Error(err))
		// Don't fail the request — the OTP is stored, user can resend
	}

	s.logger.Info("Registration OTP sent",
		zap.String("email", req.Email),
	)

	// Return nil AuthResponse — the frontend uses the HTTP status + OTPResponse DTO
	return nil, nil
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

	wasFirstLogin := user.LastLoginAt.IsZero()

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

	if wasFirstLogin {
		s.notifyInApp(ctx, user.UserID, "Welcome to FreeRange Notify", "Your first login is complete. Start by creating your first app and template.", "auth_first_login", map[string]interface{}{"event_code": "auth.first_login_success"})
	}

	// Claim any pending team invitations sent to this email
	if s.membershipRepo != nil {
		claimedMemberships, err := s.membershipRepo.ClaimByEmail(ctx, user.Email, user.UserID)
		if err != nil {
			s.logger.Warn("Failed to claim pending memberships on login",
				zap.String("email", user.Email), zap.Error(err))
		} else {
			s.notifyInviteAcceptedToInviters(ctx, user, claimedMemberships)
			agentdebug.Log(
				"pre-fix-rbac",
				"H3-claim-on-login",
				"internal/usecases/services/auth_service_impl.go:Login",
				"claimed pending memberships on login",
				map[string]any{
					"user_id": user.UserID,
					"email":   user.Email,
				},
			)
		}
	}

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

	s.notifyInApp(ctx, user.UserID, "Password changed", "Your password was reset successfully.", "security", map[string]interface{}{"event_code": "auth.password_changed", "change_type": "reset"})
	if s.otpSender != nil {
		if mailErr := s.otpSender.SendPasswordChanged(user.Email, user.FullName); mailErr != nil {
			s.logger.Warn("Failed to send password changed confirmation email",
				zap.String("user_id", user.UserID),
				zap.String("email", user.Email),
				zap.Error(mailErr),
			)
		}
	}

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

	s.notifyInApp(ctx, user.UserID, "Password changed", "Your password was updated successfully.", "security", map[string]interface{}{"event_code": "auth.password_changed", "change_type": "change"})
	if s.otpSender != nil {
		if mailErr := s.otpSender.SendPasswordChanged(user.Email, user.FullName); mailErr != nil {
			s.logger.Warn("Failed to send password changed confirmation email",
				zap.String("user_id", user.UserID),
				zap.String("email", user.Email),
				zap.Error(mailErr),
			)
		}
	}

	return nil
}

// DeleteOwnAccount deletes the authenticated user's account and owned data.
// Safety constraints:
// 1) self-scope only (userID from JWT)
// 2) password re-auth required
// 3) explicit destructive confirmation text required
func (s *authService) DeleteOwnAccount(ctx context.Context, userID string, req *auth.DeleteAccountRequest) error {
	if req == nil {
		return errors.BadRequest("Invalid request")
	}
	if req.ConfirmText != "DELETE MY ACCOUNT" {
		return errors.BadRequest("Confirmation text mismatch")
	}

	adminUser, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get account: %w", err)
	}
	if adminUser == nil {
		return errors.NotFound("user", "User not found")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(adminUser.PasswordHash), []byte(req.Password)); err != nil {
		return errors.Unauthorized("Invalid password")
	}

	return s.deleteAccountCascade(ctx, userID, adminUser)
}

// DeleteAccountByAdmin deletes a user account and owned data without password challenge.
// This path is intended strictly for privileged backend operations.
func (s *authService) DeleteAccountByAdmin(ctx context.Context, userID, reason string) error {
	adminUser, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get account: %w", err)
	}
	if adminUser == nil {
		return errors.NotFound("user", "User not found")
	}

	if strings.TrimSpace(reason) != "" {
		s.logger.Info("Admin-initiated account deletion requested",
			zap.String("user_id", userID),
			zap.String("reason", reason),
		)
	}

	return s.deleteAccountCascade(ctx, userID, adminUser)
}

func (s *authService) deleteAccountCascade(ctx context.Context, userID string, adminUser *auth.AdminUser) error {

	if err := s.repo.RevokeAllUserTokens(ctx, userID); err != nil {
		s.logger.Warn("Failed to revoke tokens during account deletion", zap.String("user_id", userID), zap.Error(err))
	}

	tenantIDs := []string{userID} // personal workspace scope
	if s.tenantRepo != nil {
		tOwned, err := s.tenantRepo.ListByCreatedBy(ctx, userID)
		if err != nil {
			return fmt.Errorf("failed to list owned organizations: %w", err)
		}
		for _, t := range tOwned {
			if t != nil && t.ID != "" {
				tenantIDs = append(tenantIDs, t.ID)
			}
		}
	}

	appIDs := make(map[string]struct{})
	if s.appRepo != nil {
		ownedApps, err := s.appRepo.List(ctx, application.ApplicationFilter{AdminUserID: userID, Limit: 5000})
		if err != nil {
			return fmt.Errorf("failed to list owned applications: %w", err)
		}
		for _, app := range ownedApps {
			if app != nil && app.AppID != "" {
				appIDs[app.AppID] = struct{}{}
			}
		}

		tenantApps, err := s.appRepo.List(ctx, application.ApplicationFilter{TenantIDs: tenantIDs, Limit: 5000})
		if err == nil {
			for _, app := range tenantApps {
				if app != nil && app.AppID != "" {
					appIDs[app.AppID] = struct{}{}
				}
			}
		}
	}

	appIDList := make([]string, 0, len(appIDs))
	for id := range appIDs {
		appIDList = append(appIDList, id)
	}

	// Purge app-scoped data: use cascade deleter for proper resource adoption.
	if len(appIDList) > 0 && s.cascadeDeleter != nil {
		for _, appID := range appIDList {
			if err := s.cascadeDeleter.DeleteAppResources(ctx, appID); err != nil {
				s.logger.Warn("Failed to cascade-delete app resources during account purge",
					zap.String("app_id", appID), zap.String("user_id", userID), zap.Error(err))
			}
		}
	} else if len(appIDList) > 0 {
		// Fallback: direct ES deletion if cascade deleter not wired.
		indicesByApp := []string{
			"notifications", "users", "templates", "workflows",
			"workflow_executions", "workflow_schedules", "digest_rules",
			"topics", "topic_subscriptions", "environments",
			"app_resource_links", "audits", "audit_logs", "app_memberships",
		}
		for _, indexName := range indicesByApp {
			if err := s.deleteByTerms(ctx, indexName, "app_id", appIDList); err != nil {
				s.logger.Warn("failed to purge index by app scope",
					zap.String("index", indexName), zap.String("user_id", userID), zap.Error(err))
			}
		}
		// Also clean up resource links by source and target.
		for _, appID := range appIDList {
			_ = s.deleteByQuery(ctx, "app_resource_links", map[string]interface{}{
				"query": map[string]interface{}{
					"bool": map[string]interface{}{
						"should": []map[string]interface{}{
							{"term": map[string]interface{}{"source_app_id": appID}},
							{"term": map[string]interface{}{"target_app_id": appID}},
						},
						"minimum_should_match": 1,
					},
				},
			})
		}
	}

	// Purge tenant-scoped data.
	if len(tenantIDs) > 0 {
		_ = s.deleteByTerms(ctx, "tenant_members", "tenant_id", tenantIDs)
		_ = s.deleteByTerms(ctx, "subscriptions", "tenant_id", tenantIDs)
	}

	// Purge user-scoped membership and dashboard records.
	_ = s.deleteByTerm(ctx, "app_memberships", "user_id", userID)
	_ = s.deleteByTerm(ctx, "dashboard_notifications", "user_id", userID)
	_ = s.deleteByTerm(ctx, "dashboard_notifications", "recipient_id", userID)

	// Delete applications after child records are purged.
	if s.appRepo != nil {
		for _, appID := range appIDList {
			if err := s.appRepo.Delete(ctx, appID); err != nil {
				s.logger.Warn("Failed to delete app during account purge", zap.String("app_id", appID), zap.Error(err))
			}
		}
	}

	// Delete owned tenants.
	if s.tenantRepo != nil {
		tOwned, _ := s.tenantRepo.ListByCreatedBy(ctx, userID)
		for _, t := range tOwned {
			if t != nil && t.ID != "" {
				if err := s.tenantRepo.Delete(ctx, t.ID); err != nil {
					s.logger.Warn("Failed to delete tenant during account purge", zap.String("tenant_id", t.ID), zap.Error(err))
				}
			}
		}
	}

	// Remove user from any remaining tenant memberships.
	if s.tenantMemberRepo != nil {
		memberships, err := s.tenantMemberRepo.ListByUser(ctx, userID)
		if err == nil {
			for _, m := range memberships {
				if m != nil && m.ID != "" {
					if err := s.tenantMemberRepo.Delete(ctx, m.ID); err != nil {
						s.logger.Warn("Failed to delete tenant member row", zap.String("membership_id", m.ID), zap.Error(err))
					}
				}
			}
		}
	}

	if err := s.repo.DeleteUser(ctx, userID); err != nil {
		return fmt.Errorf("failed to delete auth user: %w", err)
	}

	if s.otpSender != nil {
		if err := s.otpSender.SendAccountDeleted(adminUser.Email, adminUser.FullName); err != nil {
			s.logger.Warn("Failed to send account deletion confirmation email",
				zap.String("user_id", userID),
				zap.String("email", adminUser.Email),
				zap.Error(err),
			)
		}
	}

	s.logger.Info("Account and owned data deleted", zap.String("user_id", userID), zap.Int("apps_purged", len(appIDList)))
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

func (s *authService) deleteByTerm(ctx context.Context, indexName, field, value string) error {
	if value == "" {
		return nil
	}
	return s.deleteByQuery(ctx, indexName, map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{field: value},
		},
	})
}

func (s *authService) deleteByTerms(ctx context.Context, indexName, field string, values []string) error {
	if len(values) == 0 {
		return nil
	}
	return s.deleteByQuery(ctx, indexName, map[string]interface{}{
		"query": map[string]interface{}{
			"terms": map[string]interface{}{field: values},
		},
	})
}

func (s *authService) deleteByQuery(ctx context.Context, indexName string, query map[string]interface{}) error {
	if s.esClient == nil || indexName == "" {
		return nil
	}
	body, err := json.Marshal(query)
	if err != nil {
		return fmt.Errorf("marshal delete query: %w", err)
	}
	req := esapi.DeleteByQueryRequest{
		Index:             []string{indexName},
		Body:              bytes.NewReader(body),
		Refresh:           esapi.BoolPtr(true),
		Conflicts:         "proceed",
		AllowNoIndices:    esapi.BoolPtr(true),
		IgnoreUnavailable: esapi.BoolPtr(true),
	}
	res, err := req.Do(ctx, s.esClient)
	if err != nil {
		return fmt.Errorf("delete-by-query request failed: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() && res.StatusCode != 404 {
		return fmt.Errorf("delete-by-query failed for %s: %s", indexName, res.Status())
	}
	return nil
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
		smtpFromName = "FreeRange Notify"
	}

	// Build the reset link - get frontend URL from environment or use default
	frontendURL := os.Getenv("FREERANGE_FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}
	resetLink := fmt.Sprintf("%s/reset-password?token=%s", frontendURL, resetToken)

	// HTML email body
	subject := "Reset Your Password - FreeRange Notify"
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
		<p>We received a request to reset your password for your FreeRange Notify account.</p>
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
		<p>© 2026 FreeRange Notify. All rights reserved.</p>
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

// VerifyRegistrationOTP verifies the OTP and creates the user account.
func (s *authService) VerifyRegistrationOTP(ctx context.Context, req *auth.VerifyOTPRequest) (*auth.AuthResponse, error) {
	pending, err := s.otpRepo.GetPendingRegistration(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending registration: %w", err)
	}
	if pending == nil {
		return nil, errors.BadRequest("No pending registration found. Please register again.")
	}

	// Check OTP match
	if pending.OTPCode != req.OTPCode {
		attempts, incErr := s.otpRepo.IncrementAttempts(ctx, req.Email)
		if incErr != nil {
			s.logger.Error("Failed to increment OTP attempts", zap.Error(incErr))
		}
		if attempts >= 5 {
			return nil, errors.BadRequest("Too many failed attempts. Please register again.")
		}
		return nil, errors.BadRequest("Invalid verification code")
	}

	// OTP is correct — create the actual user
	user := &auth.AdminUser{
		UserID:       uuid.New().String(),
		Email:        pending.Email,
		PasswordHash: pending.PasswordHash,
		FullName:     pending.FullName,
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := s.repo.CreateUser(ctx, user); err != nil {
		rechecked, recheckErr := s.repo.GetUserByEmail(ctx, req.Email)
		if recheckErr == nil && rechecked != nil && rechecked.UserID != user.UserID {
			return nil, errors.BadRequest("User with this email already exists")
		}
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Race condition guard
	duplicateUser, _ := s.repo.GetUserByEmail(ctx, req.Email)
	if duplicateUser != nil && duplicateUser.UserID != user.UserID {
		_ = s.repo.DeleteUser(ctx, user.UserID)
		return nil, errors.BadRequest("User with this email already exists")
	}

	// Delete the pending registration from Redis
	_ = s.otpRepo.DeletePendingRegistration(ctx, req.Email)

	// Provision 30-day free trial subscription for the new user.
	// The user's own ID serves as their personal workspace tenant ID.
	if s.subscriptionRepo != nil {
		now := time.Now().UTC()
		sub := &license.Subscription{
			ID:                 uuid.New().String(),
			TenantID:           user.UserID,
			Plan:               "free_trial",
			Status:             license.SubscriptionStatusTrial,
			CurrentPeriodStart: now,
			CurrentPeriodEnd:   now.AddDate(0, 1, 0),
			Metadata: map[string]interface{}{
				"message_limit":      10000,
				"messages_sent":      0,
				"trial_activated_at": now.Format(time.RFC3339),
			},
		}
		if err := s.subscriptionRepo.Create(ctx, sub); err != nil {
			// Non-fatal: log and continue. User can have trial manually activated.
			s.logger.Error("Failed to provision free trial subscription",
				zap.String("user_id", user.UserID),
				zap.Error(err),
			)
		}
	}

	// Generate tokens
	tokens, err := s.generateTokenPair(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tokens: %w", err)
	}

	s.logger.Info("User registered successfully via OTP verification",
		zap.String("user_id", user.UserID),
		zap.String("email", user.Email),
	)

	s.notifyInApp(ctx, user.UserID, "Welcome to FreeRange Notify", "Your account is ready. Start by setting up your first notification flow.", "auth_welcome", map[string]interface{}{"event_code": "auth.registration_completed"})
	if s.otpSender != nil {
		if mailErr := s.otpSender.SendWelcome(user.Email, user.FullName); mailErr != nil {
			s.logger.Warn("Failed to send welcome email",
				zap.String("user_id", user.UserID),
				zap.String("email", user.Email),
				zap.Error(mailErr),
			)
		}
	}

	// Claim any pending team invitations
	if s.membershipRepo != nil {
		claimedMemberships, err := s.membershipRepo.ClaimByEmail(ctx, user.Email, user.UserID)
		if err != nil {
			s.logger.Warn("Failed to claim pending memberships on register",
				zap.String("email", user.Email), zap.Error(err))
		} else {
			s.notifyInviteAcceptedToInviters(ctx, user, claimedMemberships)
			agentdebug.Log(
				"pre-fix-rbac",
				"H2-claim-on-register",
				"internal/usecases/services/auth_service_impl.go:VerifyRegistrationOTP",
				"claimed pending memberships on register",
				map[string]any{
					"user_id": user.UserID,
					"email":   user.Email,
				},
			)
		}
	}

	user.PasswordHash = ""

	return &auth.AuthResponse{
		User:   user,
		Tokens: tokens,
	}, nil
}

// ResendRegistrationOTP regenerates and resends the OTP for a pending registration.
func (s *authService) ResendRegistrationOTP(ctx context.Context, req *auth.ResendOTPRequest) error {
	pending, err := s.otpRepo.GetPendingRegistration(ctx, req.Email)
	if err != nil {
		return fmt.Errorf("failed to get pending registration: %w", err)
	}
	if pending == nil {
		return errors.BadRequest("No pending registration found. Please register again.")
	}

	// Rate limit: 30-second cooldown
	if time.Since(pending.CreatedAt) < 30*time.Second {
		return errors.BadRequest("Please wait before requesting a new code")
	}

	// Generate new OTP
	otpCode, err := generateOTP(6)
	if err != nil {
		return fmt.Errorf("failed to generate OTP: %w", err)
	}

	// Update pending registration with new OTP and reset timestamp
	pending.OTPCode = otpCode
	pending.CreatedAt = time.Now()
	pending.Attempts = 0

	if err := s.otpRepo.StorePendingRegistration(ctx, pending); err != nil {
		return fmt.Errorf("failed to update pending registration: %w", err)
	}

	// Send new OTP email
	if err := s.otpSender.SendOTP(req.Email, otpCode); err != nil {
		s.logger.Error("Failed to resend OTP email", zap.String("email", req.Email), zap.Error(err))
		return fmt.Errorf("failed to send verification email: %w", err)
	}

	s.logger.Info("Registration OTP resent", zap.String("email", req.Email))
	return nil
}

// generateOTP generates a cryptographically secure numeric OTP of the given length.
func generateOTP(length int) (string, error) {
	max := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(length)), nil)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%0*d", length, n), nil
}

// SSOLogin handles authentication via Single Sign-On (OIDC)
func (s *authService) SSOLogin(ctx context.Context, email, name string) (*auth.AuthResponse, error) {
	// Check if user already exists
	user, err := s.repo.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if user == nil {
		// Auto-register (JIT provisioning) for SSO users
		// We'll generate a random complex password since they won't use it directly
		randomPassword, err := generateSecureToken(32)
		if err != nil {
			return nil, fmt.Errorf("failed to generate random password for SSO user: %w", err)
		}

		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(randomPassword), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("failed to hash random password: %w", err)
		}

		user = &auth.AdminUser{
			UserID:       uuid.New().String(),
			Email:        email,
			PasswordHash: string(hashedPassword),
			FullName:     name,
			IsActive:     true,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		if err := s.repo.CreateUser(ctx, user); err != nil {
			return nil, fmt.Errorf("failed to auto-register SSO user: %w", err)
		}

		// Race condition guard: if a concurrent Register or SSOLogin created
		// a duplicate user with the same email, GetUserByEmail (sorted by
		// created_at ASC) returns the canonical (oldest) one. If that's not
		// us, delete our duplicate and use the canonical user instead.
		canonical, _ := s.repo.GetUserByEmail(ctx, email)
		if canonical != nil && canonical.UserID != user.UserID {
			_ = s.repo.DeleteUser(ctx, user.UserID)
			user = canonical
			s.logger.Info("SSO login resolved to existing user (race-condition duplicate removed)",
				zap.String("user_id", user.UserID),
				zap.String("email", user.Email),
			)
		} else {
			s.logger.Info("User auto-registered via SSO",
				zap.String("user_id", user.UserID),
				zap.String("email", user.Email),
			)
		}
	} else {
		// User exists, just check if they are active
		if !user.IsActive {
			return nil, errors.Unauthorized("Account is deactivated")
		}

		// Update name if changed on IdP and user hasn't explicitly set it or it's empty
		if name != "" && user.FullName == "" {
			user.FullName = name
			user.UpdatedAt = time.Now()
			if updateErr := s.repo.UpdateUser(ctx, user); updateErr != nil {
				s.logger.Warn("Failed to update user's FullName from SSO", zap.Error(updateErr))
			}
		}
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

	s.logger.Info("User logged in successfully via SSO",
		zap.String("user_id", user.UserID),
		zap.String("email", user.Email),
	)

	if user.LastLoginAt.IsZero() {
		s.notifyInApp(ctx, user.UserID, "Welcome to FreeRange Notify", "Your first SSO login is complete. You can now access your dashboard.", "auth_first_login", map[string]interface{}{"event_code": "auth.first_login_success", "auth_method": "sso"})
	}

	// Claim any pending team invitations sent to this email
	if s.membershipRepo != nil {
		claimedMemberships, err := s.membershipRepo.ClaimByEmail(ctx, user.Email, user.UserID)
		if err != nil {
			s.logger.Warn("Failed to claim pending memberships on SSO login",
				zap.String("email", user.Email), zap.Error(err))
		} else {
			s.notifyInviteAcceptedToInviters(ctx, user, claimedMemberships)
			agentdebug.Log(
				"pre-fix-rbac",
				"H4-claim-on-sso",
				"internal/usecases/services/auth_service_impl.go:SSOLogin",
				"claimed pending memberships on SSO login",
				map[string]any{
					"user_id": user.UserID,
					"email":   user.Email,
				},
			)
		}
	}

	// Don't return password hash
	user.PasswordHash = ""

	return &auth.AuthResponse{
		User:   user,
		Tokens: tokens,
	}, nil
}

func (s *authService) notifyInApp(ctx context.Context, userID, title, body, category string, data map[string]interface{}) {
	if s.notifier == nil || strings.TrimSpace(userID) == "" {
		return
	}
	if err := s.notifier.NotifyUser(ctx, userID, title, body, category, data); err != nil {
		s.logger.Warn("Failed to create in-app dashboard notification",
			zap.String("user_id", userID),
			zap.String("category", category),
			zap.Error(err),
		)
	}
}

func (s *authService) notifyInviteAcceptedToInviters(ctx context.Context, user *auth.AdminUser, memberships []*auth.AppMembership) {
	if s.notifier == nil || user == nil || len(memberships) == 0 {
		return
	}

	for _, membership := range memberships {
		if membership == nil || strings.TrimSpace(membership.InvitedBy) == "" || strings.TrimSpace(membership.AppID) == "" {
			continue
		}

		appName := membership.AppID
		if s.appRepo != nil {
			if app, err := s.appRepo.GetByID(ctx, membership.AppID); err == nil && app != nil && strings.TrimSpace(app.AppName) != "" {
				appName = app.AppName
			}
		}

		title := "Invite accepted"
		body := fmt.Sprintf("%s accepted your invite to %s.", strings.TrimSpace(user.Email), appName)
		s.notifyInApp(ctx, membership.InvitedBy, title, body, "invite_accepted", map[string]interface{}{
			"event_code":     "org.invite_accepted",
			"membership_id":  membership.MembershipID,
			"app_id":         membership.AppID,
			"accepted_user":  user.UserID,
			"accepted_email": user.Email,
		})
	}
}
