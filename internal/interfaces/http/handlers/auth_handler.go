package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/dto"
	"github.com/the-monkeys/freerangenotify/pkg/errors"
	"github.com/the-monkeys/freerangenotify/pkg/validator"
	"go.uber.org/zap"
)

// AuthHandler handles authentication HTTP requests
type AuthHandler struct {
	authService auth.Service
	validator   *validator.Validator
	logger      *zap.Logger
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(authService auth.Service, validator *validator.Validator, logger *zap.Logger) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		validator:   validator,
		logger:      logger,
	}
}

// Register handles user registration
// @Summary Register a new user
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body dto.RegisterRequest true "Registration details"
// @Success 201 {object} dto.AuthResponse
// @Failure 400 {object} errors.APIError
// @Failure 500 {object} errors.APIError
// @Router /v1/auth/register [post]
func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req dto.RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return errors.BadRequest("Invalid request body")
	}

	if err := h.validator.Validate(req); err != nil {
		return errors.Validation("Validation failed", validator.FormatValidationErrors(err))
	}

	authReq := &auth.RegisterRequest{
		Email:    req.Email,
		Password: req.Password,
		FullName: req.FullName,
	}

	response, err := h.authService.Register(c.Context(), authReq)
	if err != nil {
		h.logger.Error("Failed to register user", zap.Error(err))
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(dto.AuthResponse{
		User: &dto.AdminUserResponse{
			UserID:      response.User.UserID,
			Email:       response.User.Email,
			FullName:    response.User.FullName,
			IsActive:    response.User.IsActive,
			CreatedAt:   response.User.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:   response.User.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
			LastLoginAt: formatTimePtr(response.User.LastLoginAt),
		},
		AccessToken:  response.Tokens.AccessToken,
		RefreshToken: response.Tokens.RefreshToken,
		ExpiresAt:    response.Tokens.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// Login handles user login
// @Summary Login a user
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body dto.LoginRequest true "Login credentials"
// @Success 200 {object} dto.AuthResponse
// @Failure 400 {object} errors.APIError
// @Failure 401 {object} errors.APIError
// @Failure 500 {object} errors.APIError
// @Router /v1/auth/login [post]
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req dto.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return errors.BadRequest("Invalid request body")
	}

	if err := h.validator.Validate(req); err != nil {
		return errors.Validation("Validation failed", validator.FormatValidationErrors(err))
	}

	authReq := &auth.LoginRequest{
		Email:    req.Email,
		Password: req.Password,
	}

	response, err := h.authService.Login(c.Context(), authReq)
	if err != nil {
		h.logger.Error("Failed to login user", zap.Error(err))
		return err
	}

	return c.JSON(dto.AuthResponse{
		User: &dto.AdminUserResponse{
			UserID:      response.User.UserID,
			Email:       response.User.Email,
			FullName:    response.User.FullName,
			IsActive:    response.User.IsActive,
			CreatedAt:   response.User.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:   response.User.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
			LastLoginAt: formatTimePtr(response.User.LastLoginAt),
		},
		AccessToken:  response.Tokens.AccessToken,
		RefreshToken: response.Tokens.RefreshToken,
		ExpiresAt:    response.Tokens.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// RefreshToken handles token refresh
// @Summary Refresh access token
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body dto.RefreshTokenRequest true "Refresh token"
// @Success 200 {object} dto.TokenPairResponse
// @Failure 400 {object} errors.APIError
// @Failure 401 {object} errors.APIError
// @Failure 500 {object} errors.APIError
// @Router /v1/auth/refresh [post]
func (h *AuthHandler) RefreshToken(c *fiber.Ctx) error {
	var req dto.RefreshTokenRequest
	if err := c.BodyParser(&req); err != nil {
		return errors.BadRequest("Invalid request body")
	}

	if err := h.validator.Validate(req); err != nil {
		return errors.Validation("Validation failed", validator.FormatValidationErrors(err))
	}

	tokens, err := h.authService.RefreshAccessToken(c.Context(), req.RefreshToken)
	if err != nil {
		h.logger.Error("Failed to refresh token", zap.Error(err))
		return err
	}

	return c.JSON(dto.TokenPairResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresAt:    tokens.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// Logout handles user logout
// @Summary Logout a user
// @Tags Auth
// @Security BearerAuth
// @Success 200 {object} dto.MessageResponse
// @Failure 401 {object} errors.APIError
// @Failure 500 {object} errors.APIError
// @Router /v1/auth/logout [post]
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	if err := h.authService.Logout(c.Context(), userID); err != nil {
		h.logger.Error("Failed to logout user", zap.Error(err))
		return err
	}

	return c.JSON(dto.MessageResponse{
		Message: "Logged out successfully",
	})
}

// ForgotPassword handles forgot password request
// @Summary Request password reset
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body dto.ForgotPasswordRequest true "Email address"
// @Success 200 {object} dto.MessageResponse
// @Failure 400 {object} errors.APIError
// @Failure 500 {object} errors.APIError
// @Router /v1/auth/forgot-password [post]
func (h *AuthHandler) ForgotPassword(c *fiber.Ctx) error {
	var req dto.ForgotPasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return errors.BadRequest("Invalid request body")
	}

	if err := h.validator.Validate(req); err != nil {
		return errors.Validation("Validation failed", validator.FormatValidationErrors(err))
	}

	authReq := &auth.ForgotPasswordRequest{
		Email: req.Email,
	}

	if err := h.authService.ForgotPassword(c.Context(), authReq); err != nil {
		h.logger.Error("Failed to process forgot password", zap.Error(err))
		return err
	}

	return c.JSON(dto.MessageResponse{
		Message: "Password reset instructions have been sent to your email",
	})
}

// ResetPassword handles password reset
// @Summary Reset password with token
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body dto.ResetPasswordRequest true "Reset token and new password"
// @Success 200 {object} dto.MessageResponse
// @Failure 400 {object} errors.APIError
// @Failure 500 {object} errors.APIError
// @Router /v1/auth/reset-password [post]
func (h *AuthHandler) ResetPassword(c *fiber.Ctx) error {
	var req dto.ResetPasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return errors.BadRequest("Invalid request body")
	}

	if err := h.validator.Validate(req); err != nil {
		return errors.Validation("Validation failed", validator.FormatValidationErrors(err))
	}

	authReq := &auth.ResetPasswordRequest{
		Token:       req.Token,
		NewPassword: req.NewPassword,
	}

	if err := h.authService.ResetPassword(c.Context(), authReq); err != nil {
		h.logger.Error("Failed to reset password", zap.Error(err))
		return err
	}

	return c.JSON(dto.MessageResponse{
		Message: "Password has been reset successfully",
	})
}

// ChangePassword handles password change
// @Summary Change password
// @Tags Auth
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param request body dto.ChangePasswordRequest true "Old and new password"
// @Success 200 {object} dto.MessageResponse
// @Failure 400 {object} errors.APIError
// @Failure 401 {object} errors.APIError
// @Failure 500 {object} errors.APIError
// @Router /v1/auth/change-password [post]
func (h *AuthHandler) ChangePassword(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	var req dto.ChangePasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return errors.BadRequest("Invalid request body")
	}

	if err := h.validator.Validate(req); err != nil {
		return errors.Validation("Validation failed", validator.FormatValidationErrors(err))
	}

	authReq := &auth.ChangePasswordRequest{
		OldPassword: req.OldPassword,
		NewPassword: req.NewPassword,
	}

	if err := h.authService.ChangePassword(c.Context(), userID, authReq); err != nil {
		h.logger.Error("Failed to change password", zap.Error(err))
		return err
	}

	return c.JSON(dto.MessageResponse{
		Message: "Password has been changed successfully",
	})
}

// GetCurrentUser returns the current authenticated user
// @Summary Get current user
// @Tags Auth
// @Security BearerAuth
// @Produce json
// @Success 200 {object} dto.UserResponse
// @Failure 401 {object} errors.APIError
// @Failure 500 {object} errors.APIError
// @Router /v1/auth/me [get]
func (h *AuthHandler) GetCurrentUser(c *fiber.Ctx) error {
	userID := c.Locals("user_id").(string)

	user, err := h.authService.GetCurrentUser(c.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to get current user", zap.Error(err))
		return err
	}

	return c.JSON(dto.AdminUserResponse{
		UserID:      user.UserID,
		Email:       user.Email,
		FullName:    user.FullName,
		IsActive:    user.IsActive,
		CreatedAt:   user.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:   user.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		LastLoginAt: formatTimePtr(user.LastLoginAt),
	})
}

// Helper function to format time pointer
func formatTimePtr(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02T15:04:05Z07:00")
}
