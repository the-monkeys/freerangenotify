package providers

import (
	"context"
	"fmt"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"go.uber.org/zap"
)

// FCMProvider implements the Provider interface for Firebase Cloud Messaging
type FCMProvider struct {
	config Config
	logger *zap.Logger

	// Firebase credentials
	projectID string

	// TODO: Add Firebase messaging client
	// messagingClient *messaging.Client
}

// FCMConfig holds FCM-specific configuration
type FCMConfig struct {
	Config

	// ProjectID is the Firebase project ID
	ProjectID string

	// CredentialsPath is the path to the service account JSON file
	CredentialsPath string
}

// NewFCMProvider creates a new FCM provider
func NewFCMProvider(config FCMConfig, logger *zap.Logger) (Provider, error) {
	// TODO: Initialize Firebase app and messaging client
	// app, err := firebase.NewApp(ctx, &firebase.Config{
	//     ProjectID: config.ProjectID,
	// }, option.WithCredentialsFile(config.CredentialsPath))
	// if err != nil {
	//     return nil, fmt.Errorf("failed to initialize Firebase app: %w", err)
	// }
	//
	// messagingClient, err := app.Messaging(ctx)
	// if err != nil {
	//     return nil, fmt.Errorf("failed to get messaging client: %w", err)
	// }

	return &FCMProvider{
		config:    config.Config,
		logger:    logger,
		projectID: config.ProjectID,
	}, nil
}

// Send sends a push notification via FCM
func (p *FCMProvider) Send(ctx context.Context, notif *notification.Notification, usr *user.User) (*Result, error) {
	startTime := time.Now()

	p.logger.Info("Sending FCM notification",
		zap.String("notification_id", notif.NotificationID),
		zap.String("user_id", usr.UserID))

	// Get device tokens for user
	tokens := p.getDeviceTokens(usr, "android") // FCM for Android
	if len(tokens) == 0 {
		return NewErrorResult(
			fmt.Errorf("no android device tokens found for user %s", usr.UserID),
			ErrorTypeInvalid,
		), nil
	}

	// TODO: Build FCM message
	// message := &messaging.Message{
	//     Notification: &messaging.Notification{
	//         Title: notif.Content.Title,
	//         Body:  notif.Content.Body,
	//     },
	//     Data: notif.Content.Data,
	//     Token: tokens[0],
	//     Android: &messaging.AndroidConfig{
	//         Priority: p.mapPriority(notif.Priority),
	//     },
	// }
	//
	// // Send message
	// response, err := p.messagingClient.Send(ctx, message)
	// if err != nil {
	//     return p.handleError(err), nil
	// }

	// Simulate sending (remove when real implementation is added)
	time.Sleep(50 * time.Millisecond)

	deliveryTime := time.Since(startTime)

	p.logger.Info("FCM notification sent successfully",
		zap.String("notification_id", notif.NotificationID),
		zap.Duration("delivery_time", deliveryTime))

	result := NewResult("fcm-simulated-"+notif.NotificationID, deliveryTime)
	result.Metadata["token_count"] = len(tokens)
	result.Metadata["platform"] = "android"

	return result, nil
}

// GetName returns the provider name
func (p *FCMProvider) GetName() string {
	return "fcm"
}

// GetSupportedChannel returns the channel this provider supports
func (p *FCMProvider) GetSupportedChannel() notification.Channel {
	return notification.ChannelPush
}

// IsHealthy checks if FCM is healthy
func (p *FCMProvider) IsHealthy(ctx context.Context) bool {
	// TODO: Implement real health check
	// - Check Firebase connection
	// - Verify credentials are valid
	// - Test API connectivity
	return true
}

// Close closes the provider
func (p *FCMProvider) Close() error {
	// TODO: Close Firebase app if needed
	return nil
}

// getDeviceTokens returns device tokens for a specific platform
func (p *FCMProvider) getDeviceTokens(usr *user.User, platform string) []string {
	var tokens []string

	for _, device := range usr.Devices {
		if device.Platform == platform && device.Active && device.Token != "" {
			tokens = append(tokens, device.Token)
		}
	}

	return tokens
}

// mapPriority maps notification priority to FCM priority
func (p *FCMProvider) mapPriority(priority notification.Priority) string {
	switch priority {
	case notification.PriorityCritical, notification.PriorityHigh:
		return "high"
	case notification.PriorityLow:
		return "normal"
	default:
		return "normal"
	}
}

// handleError categorizes FCM errors
func (p *FCMProvider) handleError(err error) *Result {
	// TODO: Implement proper error categorization based on FCM error codes
	// - messaging.IsInternal(err) -> ErrorTypeProviderAPI
	// - messaging.IsInvalidArgument(err) -> ErrorTypeInvalid
	// - messaging.IsUnauthenticated(err) -> ErrorTypeAuth
	// - messaging.IsResourceExhausted(err) -> ErrorTypeRateLimit

	return NewErrorResult(err, ErrorTypeUnknown)
}
