package main

import (
	"context"
	"errors"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/providers"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"go.uber.org/zap"
)

// OTP expiry is recorded by the OTP service. Older notifications have no
// deadline and retain their existing retry behavior.
var errNotificationDeliveryExpired = errors.New("notification expired")

func notificationDeliveryDeadline(notif *notification.Notification) (time.Time, bool) {
	if notif == nil || notif.Metadata == nil {
		return time.Time{}, false
	}
	value, ok := notif.Metadata["otp_expires_at"].(string)
	if !ok {
		return time.Time{}, false
	}
	deadline, err := time.Parse(time.RFC3339Nano, value)
	return deadline, err == nil
}

func notificationDeliveryExpired(notif *notification.Notification, at time.Time) bool {
	deadline, ok := notificationDeliveryDeadline(notif)
	return ok && !at.Before(deadline)
}

func (p *NotificationProcessor) handleExpiredNotification(ctx context.Context, notif *notification.Notification) {
	ctx, cancel := notificationAccountingContext(ctx)
	defer cancel()
	if notif.Metadata == nil {
		notif.Metadata = make(map[string]interface{})
	}
	notif.Metadata["failure_code"] = "notification_expired"
	notif.Metadata["failure_stage"] = "delivery"
	notif.Metadata["retryable"] = false
	notif.Status = notification.StatusFailed
	notif.ErrorMessage = "notification expired"
	now := time.Now().UTC()
	notif.FailedAt = &now
	if err := p.notifRepo.Update(ctx, notif); err != nil {
		p.logger.Error("Failed to persist expired notification", zap.Error(err))
	}
	p.publishActivity(ctx, notif.NotificationID, notif.AppID, string(notif.Channel), "failed")
}

// annotateNotificationFailure records safe, optional attempt-time diagnostics.
// Old untyped errors and their public messages retain the existing behavior.
func annotateNotificationFailure(notif *notification.Notification, err error, fallback string) string {
	var providerErr *providers.DeliveryError
	if errors.As(err, &providerErr) {
		if notif.Metadata == nil {
			notif.Metadata = make(map[string]interface{})
		}
		for key, value := range providers.DeliveryErrorMetadata(err) {
			notif.Metadata[key] = value
		}
		return providers.SafeDeliveryErrorMessage(err)
	}
	var unavailable *billing.CreditUnavailableError
	if errors.As(err, &unavailable) {
		if notif.Metadata == nil {
			notif.Metadata = make(map[string]interface{})
		}
		notif.Metadata["failure_code"] = unavailable.Reason
		notif.Metadata["failure_stage"] = "credit_reservation"
		notif.Metadata["retryable"] = unavailable.Reason == billing.CreditUnavailableReasonTemporarilyReserved
		notif.Metadata["credits_required"] = unavailable.CreditsRequired
		notif.Metadata["credits_remaining"] = unavailable.CreditsRemaining
		notif.Metadata["credits_reserved"] = unavailable.CreditsReserved
		notif.Metadata["credits_available"] = unavailable.CreditsAvailable
		notif.Metadata["rate_card_version"] = unavailable.RateCardVersion
		return err.Error()
	}
	if errors.Is(err, services.ErrDailyCapExceeded) || errors.Is(err, services.ErrInsufficientCredits) {
		if notif.Metadata == nil {
			notif.Metadata = make(map[string]interface{})
		}
		code := "insufficient_credits"
		if errors.Is(err, services.ErrDailyCapExceeded) {
			code = "daily_cap_exceeded"
		}
		notif.Metadata["failure_code"] = code
		notif.Metadata["failure_stage"] = "credit_reservation"
		notif.Metadata["retryable"] = false
	}
	return fallback
}

func clearNotificationFailure(notif *notification.Notification) {
	notif.ErrorMessage = ""
	notif.FailedAt = nil
	for _, key := range []string{"failure_code", "failure_stage", "retryable",
		"credits_required", "credits_remaining", "credits_reserved", "credits_available"} {
		delete(notif.Metadata, key)
	}
}

// A retry-store outage must leave the original item in the processing set so
// the queue's visibility-timeout recovery can try again.
type notificationProcessingState struct{ acknowledge bool }
type notificationProcessingStateKey struct{}

func retainNotificationProcessingItem(ctx context.Context) {
	if state, ok := ctx.Value(notificationProcessingStateKey{}).(*notificationProcessingState); ok {
		state.acknowledge = false
	}
}

// Billing cleanup and final expired-state persistence must still run when an
// OTP delivery deadline has elapsed. Keep this independent work bounded.
func notificationAccountingContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
}

// Retain attempt-specific rejection evidence only when no delivery was accepted.
// Unknown send/network outcomes remain available for reconciliation rather than
// becoming a fabricated successful burn or an automatically proven refund.
func notificationDeliveryDefinitelyFailed(err error) bool {
	if errors.Is(err, errNotificationDeliveryExpired) {
		return true
	}
	var delivery *providers.DeliveryError
	if !errors.As(err, &delivery) {
		return false
	}
	if delivery.Stage == "health" || delivery.Stage == "resolve" {
		return true
	}
	if delivery.Provider == "smtp" {
		switch delivery.Stage {
		case "connect", "tls", "auth", "mail", "rcpt":
			// All of these SMTP stages precede DATA acceptance.
			return true
		}
	}
	switch delivery.ErrorType {
	case providers.ErrorTypeAuth, providers.ErrorTypeConfiguration, providers.ErrorTypeInvalid, providers.ErrorTypeRateLimit:
		return true
	}
	return false
}
