package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/otp"
)

func (s *OTPService) recipientBudget(ctx context.Context, r *otp.Request) error {
	count, err := s.repo.RecipientRateLimit(ctx, r.AppID, r.Recipient, 3600)
	if err != nil {
		return otp.ErrStateUnavailable
	}
	if count > s.recipientLimitPerHr {
		return otp.ErrRateLimited
	}
	return nil
}
func newOTPOperation(code, salt string) *otp.DeliveryOperation {
	return &otp.DeliveryOperation{ID: uuid.NewString(), NotificationID: uuid.NewString(), State: "pending", CodeHash: hashCode(code, salt), Salt: salt, StartedAt: time.Now().UTC()}
}
func (s *OTPService) sendOTP(ctx context.Context, in otp.SendInput) (*otp.SendResult, error) {
	if scope, ok := otp.ScopeFromContext(ctx); ok {
		if scope.AppID == "" || scope.AppID != in.AppID {
			return nil, otp.ErrNotFound
		}
		in.EnvironmentID = scope.EnvironmentID
	}
	if err := s.validateSendInput(&in); err != nil {
		return nil, err
	}
	userID, address, err := s.resolveRecipient(ctx, &in)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	r := &otp.Request{RequestID: "otp_" + strings.ReplaceAll(uuid.NewString(), "-", ""), AppID: in.AppID, EnvironmentID: in.EnvironmentID, Channel: in.Channel, Recipient: address, Length: in.Length, Alphanumeric: in.Alphanumeric, MaxAttempts: in.MaxAttempts, ExpiresAt: now.Add(time.Duration(in.TTLSeconds) * time.Second), CreatedAt: now, ResolvedUserID: userID, TemplateBody: in.TemplateBody, TemplateData: in.TemplateData}
	if err = s.recipientBudget(ctx, r); err != nil {
		return nil, err
	}
	if userID == "" {
		userID, err = s.resolveOrCreateRecipientUser(ctx, r.AppID, r.Channel, r.Recipient)
		if err != nil {
			return nil, err
		}
		r.ResolvedUserID = userID
	}
	code, err := generateCode(r.Length, r.Alphanumeric)
	if err != nil {
		return nil, err
	}
	salt, err := randomHex(16)
	if err != nil {
		return nil, err
	}
	r.Delivery = newOTPOperation(code, salt)
	if _, ok := s.repo.(otp.AtomicRepository); !ok {
		return nil, otp.ErrStateUnavailable
	}
	if err = s.repo.Create(ctx, r); err != nil {
		return nil, otp.ErrStateUnavailable
	}
	return s.deliverOTP(ctx, r, nil, code)
}
func (s *OTPService) resendOTP(ctx context.Context, id string) (*otp.SendResult, error) {
	for retry := 0; retry < 8; retry++ {
		r, err := s.loadOTP(ctx, id)
		if err != nil {
			return nil, err
		}
		if r.Verified {
			return nil, otp.ErrAlreadyVerified
		}
		if otpPending(r) {
			return nil, otp.ErrOperationPending
		}
		if r.CodeHash == "" {
			return nil, otp.ErrAttemptsExhausted
		}
		if time.Now().Before(r.LastSentAt.Add(time.Duration(s.resendCooldownSec) * time.Second)) {
			return nil, otp.ErrResendCooldown
		}
		code, err := generateCode(r.Length, r.Alphanumeric)
		if err != nil {
			return nil, err
		}
		salt, err := randomHex(16)
		if err != nil {
			return nil, err
		}
		next := cloneOTP(r)
		next.Delivery = newOTPOperation(code, salt)
		if err = s.swapOTP(ctx, r, next); errors.Is(err, otp.ErrConflict) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if err = s.recipientBudget(ctx, next); err != nil {
			s.abortOTP(ctx, next, r)
			return nil, err
		}
		return s.deliverOTP(ctx, next, r, code)
	}
	return nil, otp.ErrStateUnavailable
}
func (s *OTPService) deliverOTP(ctx context.Context, r, previous *otp.Request, code string) (*otp.SendResult, error) {
	if r.EnvironmentID != "" {
		ctx = channelconnection.WithEnvironment(ctx, r.EnvironmentID)
	}
	id, err := s.dispatchNotification(ctx, r, r.ResolvedUserID, code, r.TemplateBody, r.TemplateData)
	if err != nil {
		var dispatch *notification.DispatchError
		if errors.As(err, &dispatch) && dispatch.MayBeEnqueued {
			s.markOTPUncertain(ctx, r)
			return nil, otp.ErrStateUnavailable
		}
		s.abortOTP(ctx, r, previous)
		return nil, err
	}
	if id != r.Delivery.NotificationID {
		s.markOTPUncertain(ctx, r)
		return nil, otp.ErrStateUnavailable
	}
	if err = s.AuthorizeDelivery(ctx, otp.Scope{AppID: r.AppID, EnvironmentID: r.EnvironmentID}, r.RequestID, r.Delivery.ID, id); err != nil {
		return nil, err
	}
	return &otp.SendResult{RequestID: r.RequestID, NotificationID: id, Channel: r.Channel, ExpiresAt: r.ExpiresAt, TTLSeconds: int(time.Until(r.ExpiresAt).Seconds() + 0.999), MaxAttempts: r.MaxAttempts}, nil
}
func (s *OTPService) abortOTP(ctx context.Context, claim, previous *otp.Request) {
	for retry := 0; retry < 8; retry++ {
		r, err := s.loadOTP(ctx, claim.RequestID)
		if err != nil || r.Verified || r.Delivery == nil || r.Delivery.ID != claim.Delivery.ID || r.Delivery.State == "accepted" {
			return
		}
		next := cloneOTP(r)
		next.Delivery = nil
		if previous != nil {
			next.Delivery = previous.Delivery
		}
		if err = s.swapOTP(ctx, r, next); errors.Is(err, otp.ErrConflict) {
			continue
		}
		return
	}
}
func (s *OTPService) markOTPUncertain(ctx context.Context, claim *otp.Request) {
	for retry := 0; retry < 8; retry++ {
		r, err := s.loadOTP(ctx, claim.RequestID)
		if err != nil || r.Delivery == nil || r.Delivery.ID != claim.Delivery.ID || r.Delivery.State == "accepted" {
			return
		}
		next := cloneOTP(r)
		next.Delivery.State = "uncertain"
		if err = s.swapOTP(ctx, r, next); errors.Is(err, otp.ErrConflict) {
			continue
		}
		return
	}
}
