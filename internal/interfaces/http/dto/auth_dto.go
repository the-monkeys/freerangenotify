package dto

// LoginRequest represents a login request DTO
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

// RegisterRequest represents a registration request DTO
type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	FullName string `json:"full_name" validate:"required,min=2"`
}

// ForgotPasswordRequest represents a forgot password request DTO
type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// ResetPasswordRequest represents a reset password request DTO
type ResetPasswordRequest struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8"`
}

// ChangePasswordRequest represents a change password request DTO
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=8"`
}

// DeleteAccountRequest represents a self-service account deletion request.
type DeleteAccountRequest struct {
	Password    string `json:"password" validate:"required,min=8"`
	ConfirmText string `json:"confirm_text" validate:"required"`
}

// RefreshTokenRequest represents a refresh token request DTO
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// AuthResponse represents authentication response DTO
type AuthResponse struct {
	User                *AdminUserResponse `json:"user"`
	AccessToken         string             `json:"access_token"`
	RefreshToken        string             `json:"refresh_token"`
	ExpiresAt           string             `json:"expires_at"`
	RequireTrialWelcome bool               `json:"require_trial_welcome"`
}

// AdminUserResponse represents admin user data in response
type AdminUserResponse struct {
	UserID        string `json:"user_id"`
	Email         string `json:"email"`
	FullName      string `json:"full_name"`
	Phone         string `json:"phone,omitempty"`
	PhoneVerified bool   `json:"phone_verified"`
	IsActive      bool   `json:"is_active"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
	LastLoginAt   string `json:"last_login_at,omitempty"`
}

// TokenPairResponse represents token pair response
type TokenPairResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    string `json:"expires_at"`
}

// MessageResponse represents a simple message response
type MessageResponse struct {
	Message string `json:"message"`
}

// VerifyOTPRequest represents a request to verify a registration OTP
type VerifyOTPRequest struct {
	Email   string `json:"email" validate:"required,email"`
	OTPCode string `json:"otp_code" validate:"required,len=6"`
}

// ResendOTPRequest represents a request to resend a registration OTP
type ResendOTPRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// OTPResponse represents the response after initiating or resending an OTP
type OTPResponse struct {
	Message   string `json:"message"`
	ExpiresIn int    `json:"expires_in"`
}

// PhoneOTPRequest represents a request to send a phone verification OTP (DTO)
type PhoneOTPRequest struct {
	Phone string `json:"phone" validate:"required"`
}

// PhoneVerifyRequest represents a request to verify a phone OTP (DTO)
type PhoneVerifyRequest struct {
	Phone   string `json:"phone" validate:"required"`
	OTPCode string `json:"otp_code" validate:"required,len=6"`
}
