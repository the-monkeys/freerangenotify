package services

import (
	"context"
	"fmt"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/auth"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"go.uber.org/zap"
)

// PaymentSuccessEvent carries everything needed to notify an admin that a
// payment succeeded and tokens were allocated to their account.
type PaymentSuccessEvent struct {
	TenantID         string
	UserID           string
	Name             string
	Email            string
	Phone            string
	AmountPaisa      int64
	Currency         string
	PaymentID        string
	OrderID          string
	Plan             string
	CreditsAllocated int64
	OccurredAt       time.Time
}

// PaymentNotificationService fans a payment-success event out to whichever
// channels the tenant's admin has enabled (email/SMS/WhatsApp), defaulting
// to all channels enabled with the admin's own account contact details
// when no preferences have been configured yet.
type PaymentNotificationService struct {
	prefsRepo   billing.PaymentNotificationPreferencesRepository
	authRepo    auth.Repository
	emailSender *OTPEmailSender
	smsSender   *TwilioSMSSender
	logger      *zap.Logger
}

func NewPaymentNotificationService(
	prefsRepo billing.PaymentNotificationPreferencesRepository,
	authRepo auth.Repository,
	emailSender *OTPEmailSender,
	smsSender *TwilioSMSSender,
	logger *zap.Logger,
) *PaymentNotificationService {
	return &PaymentNotificationService{
		prefsRepo:   prefsRepo,
		authRepo:    authRepo,
		emailSender: emailSender,
		smsSender:   smsSender,
		logger:      logger,
	}
}

// NotifyAsync dispatches the configured notifications in the background so
// the payment HTTP response is never blocked or failed by a delivery issue.
// Every channel's failure is logged, never returned or panicked on.
func (s *PaymentNotificationService) NotifyAsync(event PaymentSuccessEvent) {
	go func() {
		ctx := context.Background()
		prefs := s.resolvePrefs(ctx, event)

		if prefs.EmailEnabled && prefs.EmailAddress != "" {
			if err := s.emailSender.SendPaymentSuccess(
				prefs.EmailAddress, event.Name, event.AmountPaisa, event.Currency,
				event.PaymentID, event.OrderID, event.Plan, event.CreditsAllocated, event.OccurredAt,
			); err != nil {
				s.logger.Error("payment notify: email delivery failed",
					zap.String("tenant_id", event.TenantID), zap.Error(err))
			}
		}

		body := fmt.Sprintf(
			"Payment received: %s %.2f for plan %s. %d tokens allocated. Payment ID: %s",
			event.Currency, float64(event.AmountPaisa)/100.0, event.Plan, event.CreditsAllocated, event.PaymentID,
		)

		if prefs.SMSEnabled && prefs.PhoneNumber != "" {
			if err := s.smsSender.Send(prefs.PhoneNumber, body); err != nil {
				s.logger.Error("payment notify: sms delivery failed",
					zap.String("tenant_id", event.TenantID), zap.Error(err))
			}
		}

		if prefs.WhatsAppEnabled && prefs.WhatsAppNumber != "" {
			if err := s.smsSender.SendWhatsApp(prefs.WhatsAppNumber, body); err != nil {
				s.logger.Error("payment notify: whatsapp delivery failed",
					zap.String("tenant_id", event.TenantID), zap.Error(err))
			}
		}
	}()
}

// resolvePrefs loads saved preferences, or builds an all-enabled default
// from the admin's own account contact details when none are saved yet.
func (s *PaymentNotificationService) resolvePrefs(ctx context.Context, event PaymentSuccessEvent) billing.PaymentNotificationPreferences {
	defaults := billing.PaymentNotificationPreferences{
		TenantID:        event.TenantID,
		EmailEnabled:    true,
		EmailAddress:    event.Email,
		SMSEnabled:      true,
		PhoneNumber:     event.Phone,
		WhatsAppEnabled: true,
		WhatsAppNumber:  event.Phone,
	}

	if s.prefsRepo == nil {
		return defaults
	}

	saved, err := s.prefsRepo.GetByTenantID(ctx, event.TenantID)
	if err != nil {
		s.logger.Error("payment notify: failed to load notification preferences",
			zap.String("tenant_id", event.TenantID), zap.Error(err))
		return defaults
	}
	if saved == nil {
		return defaults
	}

	if saved.EmailAddress == "" {
		saved.EmailAddress = event.Email
	}
	if saved.PhoneNumber == "" {
		saved.PhoneNumber = event.Phone
	}
	if saved.WhatsAppNumber == "" {
		saved.WhatsAppNumber = event.Phone
	}
	return *saved
}
