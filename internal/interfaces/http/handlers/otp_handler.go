package handlers

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/the-monkeys/freerangenotify/internal/domain/otp"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/dto"
	"github.com/the-monkeys/freerangenotify/pkg/validator"
)

// OTPHandler exposes the public OTP-as-a-service endpoints under /v1/otp/*.
// AppID is sourced from the API-key middleware (`c.Locals("app_id")`), never
// from the request body, so callers cannot send on behalf of another tenant.
type OTPHandler struct {
	service   otp.Service
	validator *validator.Validator
	logger    *zap.Logger
}

// NewOTPHandler constructs an OTPHandler.
func NewOTPHandler(service otp.Service, v *validator.Validator, logger *zap.Logger) *OTPHandler {
	return &OTPHandler{service: service, validator: v, logger: logger}
}

// Send handles POST /v1/otp/send.
// @Summary Send an OTP
// @Description Generates a one-time passcode and dispatches it through SMS, WhatsApp, or email via the standard notification pipeline. Subject to standard per-channel credit billing.
// @Tags OTP
// @Accept json
// @Produce json
// @Param body body dto.OTPSendRequest true "OTP send request"
// @Success 202 {object} dto.OTPSendResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 429 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /v1/otp/send [post]
func (h *OTPHandler) Send(c *fiber.Ctx) error {
	appID, ok := c.Locals("app_id").(string)
	if !ok || appID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "application not authenticated"})
	}

	var req dto.OTPSendRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if err := h.validator.Validate(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	res, err := h.service.Send(c.Context(), otp.SendInput{
		AppID:        appID,
		Channel:      otp.Channel(req.Channel),
		Recipient:    req.Recipient,
		UserID:       req.UserID,
		ExternalID:   req.ExternalID,
		Length:       req.Length,
		Alphanumeric: req.Alphanumeric,
		TTLSeconds:   req.TTLSeconds,
		MaxAttempts:  req.MaxAttempts,
		TemplateBody: req.TemplateBody,
		TemplateData: req.TemplateData,
	})
	if err != nil {
		return h.writeSendError(c, appID, err)
	}

	return c.Status(fiber.StatusAccepted).JSON(toSendResponse(res))
}

// Verify handles POST /v1/otp/verify.
// @Summary Verify an OTP
// @Description Verifies a code against a previously-issued request_id. Uses constant-time comparison and atomic attempt counting.
// @Tags OTP
// @Accept json
// @Produce json
// @Param body body dto.OTPVerifyRequest true "OTP verify request"
// @Success 200 {object} dto.OTPVerifyResponse
// @Failure 400 {object} dto.OTPVerifyResponse
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} dto.OTPVerifyResponse
// @Failure 410 {object} dto.OTPVerifyResponse
// @Security ApiKeyAuth
// @Router /v1/otp/verify [post]
func (h *OTPHandler) Verify(c *fiber.Ctx) error {
	if _, ok := c.Locals("app_id").(string); !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "application not authenticated"})
	}

	var req dto.OTPVerifyRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if err := h.validator.Validate(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	res, err := h.service.Verify(c.Context(), otp.VerifyInput{
		RequestID: req.RequestID,
		Code:      req.Code,
	})
	return h.writeVerifyOutcome(c, req.RequestID, res, err)
}

// Resend handles POST /v1/otp/resend.
// @Summary Resend an OTP
// @Description Re-issues a new code for an existing request_id (subject to 60s cooldown). The previous code is invalidated and attempt counter is reset.
// @Tags OTP
// @Accept json
// @Produce json
// @Param body body dto.OTPResendRequest true "OTP resend request"
// @Success 202 {object} dto.OTPSendResponse
// @Failure 400 {object} map[string]interface{}
// @Failure 401 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Failure 409 {object} map[string]interface{}
// @Failure 429 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /v1/otp/resend [post]
func (h *OTPHandler) Resend(c *fiber.Ctx) error {
	appID, ok := c.Locals("app_id").(string)
	if !ok || appID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "application not authenticated"})
	}

	var req dto.OTPResendRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if err := h.validator.Validate(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	res, err := h.service.Resend(c.Context(), req.RequestID)
	if err != nil {
		return h.writeSendError(c, appID, err)
	}
	return c.Status(fiber.StatusAccepted).JSON(toSendResponse(res))
}

// writeSendError centralises the OTP send/resend error→status mapping so the
// two handlers stay consistent. The HTTP status reflects the semantics:
//   - 404 for missing request
//   - 410 for expired
//   - 409 for already-verified or cooldown
//   - 429 for rate limit
//   - 400 for validation / invalid recipient
func (h *OTPHandler) writeSendError(c *fiber.Ctx, appID string, err error) error {
	switch {
	case errors.Is(err, otp.ErrNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, otp.ErrExpired):
		return c.Status(fiber.StatusGone).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, otp.ErrAlreadyVerified), errors.Is(err, otp.ErrResendCooldown):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, otp.ErrRateLimited):
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, otp.ErrInvalidChannel),
		errors.Is(err, otp.ErrInvalidRecipient),
		errors.Is(err, otp.ErrInvalidLength),
		errors.Is(err, otp.ErrInvalidTTL),
		errors.Is(err, otp.ErrInvalidAttempts),
		errors.Is(err, otp.ErrTemplateMissingCode),
		errors.Is(err, otp.ErrAmbiguousRecipient),
		errors.Is(err, otp.ErrUserMissingChannelAddress):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, otp.ErrUserNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	}
	h.logger.Error("OTP send failed", zap.String("app_id", appID), zap.Error(err))
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
}

func (h *OTPHandler) writeVerifyOutcome(c *fiber.Ctx, requestID string, res *otp.VerifyResult, err error) error {
	if err == nil {
		out := dto.OTPVerifyResponse{
			Verified:          res.Verified,
			RequestID:         requestID,
			AttemptsRemaining: res.AttemptsRemaining,
		}
		if res.VerifiedAt != nil {
			out.VerifiedAt = res.VerifiedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
		}
		return c.Status(fiber.StatusOK).JSON(out)
	}

	resp := dto.OTPVerifyResponse{Verified: false, RequestID: requestID}
	switch {
	case errors.Is(err, otp.ErrInvalidCode):
		resp.Error = "invalid_code"
		if res != nil {
			resp.AttemptsRemaining = res.AttemptsRemaining
		}
		return c.Status(fiber.StatusBadRequest).JSON(resp)
	case errors.Is(err, otp.ErrAttemptsExhausted):
		resp.Error = "attempts_exhausted"
		return c.Status(fiber.StatusBadRequest).JSON(resp)
	case errors.Is(err, otp.ErrExpired):
		resp.Error = "expired"
		return c.Status(fiber.StatusGone).JSON(resp)
	case errors.Is(err, otp.ErrNotFound):
		resp.Error = "not_found"
		return c.Status(fiber.StatusNotFound).JSON(resp)
	}
	h.logger.Error("OTP verify failed", zap.String("request_id", requestID), zap.Error(err))
	resp.Error = "internal_error"
	return c.Status(fiber.StatusInternalServerError).JSON(resp)
}

func toSendResponse(r *otp.SendResult) dto.OTPSendResponse {
	return dto.OTPSendResponse{
		RequestID:      r.RequestID,
		NotificationID: r.NotificationID,
		Channel:        string(r.Channel),
		ExpiresAt:      r.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		TTLSeconds:     r.TTLSeconds,
		MaxAttempts:    r.MaxAttempts,
	}
}
