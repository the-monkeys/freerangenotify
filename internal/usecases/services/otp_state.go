package services

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/otp"
)

func cloneOTP(r *otp.Request) *otp.Request {
	raw, _ := json.Marshal(r)
	var next otp.Request
	_ = json.Unmarshal(raw, &next)
	return &next
}

func (s *OTPService) swapOTP(ctx context.Context, before, after *otp.Request) error {
	repo, ok := s.repo.(otp.AtomicRepository)
	if !ok {
		return otp.ErrStateUnavailable
	}
	err := repo.CompareAndSwap(ctx, before, after)
	if err != nil && !errors.Is(err, otp.ErrConflict) && !errors.Is(err, otp.ErrNotFound) && !errors.Is(err, otp.ErrExpired) {
		return otp.ErrStateUnavailable
	}
	return err
}

func (s *OTPService) loadOTP(ctx context.Context, id string) (*otp.Request, error) {
	r, err := s.repo.Get(ctx, strings.TrimSpace(id))
	if err != nil {
		if errors.Is(err, otp.ErrNotFound) {
			return nil, err
		}
		return nil, otp.ErrStateUnavailable
	}
	if !otpRequestInScope(ctx, r) {
		return nil, otp.ErrNotFound
	}
	if !time.Now().Before(r.ExpiresAt) {
		return nil, otp.ErrExpired
	}
	return r, nil
}

func otpPending(r *otp.Request) bool { return r.Delivery != nil && r.Delivery.State != "accepted" }

func (s *OTPService) verifyOTP(ctx context.Context, in otp.VerifyInput) (*otp.VerifyResult, error) {
	code := strings.TrimSpace(in.Code)
	if strings.TrimSpace(in.RequestID) == "" || code == "" {
		return nil, otp.ErrInvalidCode
	}
	for retry := 0; retry < 8; retry++ {
		r, err := s.loadOTP(ctx, in.RequestID)
		if err != nil {
			return nil, err
		}
		if otpPending(r) {
			return nil, otp.ErrOperationPending
		}
		if r.CodeHash == "" {
			return nil, otp.ErrAttemptsExhausted
		}
		valid := subtle.ConstantTimeCompare([]byte(hashCode(code, r.Salt)), []byte(r.CodeHash)) == 1
		if r.Verified {
			if !valid {
				return nil, otp.ErrInvalidCode
			}
			return &otp.VerifyResult{Verified: true, AttemptsRemaining: r.MaxAttempts - r.Attempts, VerifiedAt: r.VerifiedAt}, nil
		}
		next := cloneOTP(r)
		if r.Attempts >= r.MaxAttempts {
			// Fence all writers before removing the exhausted record.
			next.CodeHash = ""
			next.Delivery = nil
			if err = s.swapOTP(ctx, r, next); errors.Is(err, otp.ErrConflict) {
				continue
			}
			if err != nil {
				return nil, err
			}
			_ = s.repo.Delete(ctx, r.RequestID)
			return nil, otp.ErrAttemptsExhausted
		}
		next.Attempts++
		if valid {
			now := time.Now().UTC()
			next.Verified = true
			next.VerifiedAt = &now
		}
		if err = s.swapOTP(ctx, r, next); errors.Is(err, otp.ErrConflict) {
			continue
		}
		if err != nil {
			return nil, err
		}
		result := &otp.VerifyResult{Verified: next.Verified, AttemptsRemaining: next.MaxAttempts - next.Attempts, VerifiedAt: next.VerifiedAt}
		if !valid {
			return result, otp.ErrInvalidCode
		}
		return result, nil
	}
	return nil, otp.ErrStateUnavailable
}

// AuthorizeDelivery also recovers an enqueue whose acknowledgement was lost.
// The worker supplies the actual dequeued identity; it never enqueues a retry here.
func (s *OTPService) AuthorizeDelivery(ctx context.Context, scope otp.Scope, requestID, operationID, notificationID string) error {
	ctx = otp.WithScope(ctx, scope)
	for retry := 0; retry < 8; retry++ {
		r, err := s.loadOTP(ctx, requestID)
		if err != nil {
			return err
		}
		if r.Verified {
			return otp.ErrAlreadyVerified
		}
		op := r.Delivery
		if op == nil || op.ID != operationID || op.NotificationID != notificationID {
			return otp.ErrNotFound
		}
		if op.State == "accepted" {
			return nil
		}
		next := cloneOTP(r)
		next.CodeHash = op.CodeHash
		next.Salt = op.Salt
		next.Attempts = 0
		next.LastSentAt = op.StartedAt
		next.Delivery.State = "accepted"
		if err = s.swapOTP(ctx, r, next); errors.Is(err, otp.ErrConflict) {
			continue
		}
		return err
	}
	return otp.ErrStateUnavailable
}

var _ otp.DeliveryAuthorizer = (*OTPService)(nil)
