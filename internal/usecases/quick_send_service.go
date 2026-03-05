package usecases

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	templateDomain "github.com/the-monkeys/freerangenotify/internal/domain/template"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/interfaces/http/dto"
	"go.uber.org/zap"
)

// QuickSendService orchestrates the simplified quick-send flow:
// resolve recipient, resolve template, delegate to notification service.
type QuickSendService struct {
	notificationService notification.Service
	userRepo            user.Repository
	templateRepo        templateDomain.Repository
	templateService     *TemplateService
	logger              *zap.Logger
}

// NewQuickSendService creates a new QuickSendService.
func NewQuickSendService(
	notifSvc notification.Service,
	userRepo user.Repository,
	tmplRepo templateDomain.Repository,
	tmplSvc *TemplateService,
	logger *zap.Logger,
) *QuickSendService {
	return &QuickSendService{
		notificationService: notifSvc,
		userRepo:            userRepo,
		templateRepo:        tmplRepo,
		templateService:     tmplSvc,
		logger:              logger,
	}
}

// Send resolves human-readable identifiers and delegates to the notification service.
func (s *QuickSendService) Send(ctx context.Context, appID string, req *dto.QuickSendRequest) (*dto.QuickSendResponse, error) {
	// 1. Resolve recipient
	userID, err := s.resolveRecipient(ctx, appID, req.To)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve recipient %q: %w", req.To, err)
	}

	// 2. Resolve template (or use inline content)
	var templateID string
	var channel notification.Channel

	if req.Template != "" {
		tmpl, err := s.resolveTemplate(ctx, appID, req.Template)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve template %q: %w", req.Template, err)
		}
		templateID = tmpl.ID
		channel = notification.Channel(tmpl.Channel)
	} else if req.Body != "" {
		// Inline content: create a transient template
		tmpl, err := s.createTransientTemplate(ctx, appID, req)
		if err != nil {
			return nil, fmt.Errorf("failed to create inline template: %w", err)
		}
		templateID = tmpl.ID
		channel = notification.Channel(tmpl.Channel)
	} else {
		return nil, fmt.Errorf("either 'template' or 'body' must be provided")
	}

	// 3. Channel: explicit > inferred from template
	if req.Channel != "" {
		channel = notification.Channel(req.Channel)
	}

	// 4. Priority: default to "normal"
	priority := notification.PriorityNormal
	if req.Priority != "" {
		priority = notification.Priority(req.Priority)
	}

	// 5. Build send request and delegate
	sendReq := notification.SendRequest{
		AppID:       appID,
		UserID:      userID,
		Channel:     channel,
		Priority:    priority,
		TemplateID:  templateID,
		Data:        req.Data,
		ScheduledAt: req.ScheduledAt,
	}

	// Pass webhook URL through Data if provided
	if req.WebhookURL != "" {
		if sendReq.Data == nil {
			sendReq.Data = make(map[string]interface{})
		}
		sendReq.Data["webhook_url"] = req.WebhookURL
	}

	notif, err := s.notificationService.Send(ctx, sendReq)
	if err != nil {
		return nil, err
	}

	return &dto.QuickSendResponse{
		NotificationID: notif.NotificationID,
		Status:         string(notif.Status),
		UserID:         userID,
		Channel:        string(channel),
		Message:        "Notification accepted for delivery",
	}, nil
}

// resolveRecipient resolves a "to" value to an internal user UUID.
// Accepts: email address or UUID.
// If email and user doesn't exist, auto-creates one.
func (s *QuickSendService) resolveRecipient(ctx context.Context, appID, to string) (string, error) {
	// Check if it's a UUID (existing internal ID)
	if _, err := uuid.Parse(to); err == nil {
		if _, err := s.userRepo.GetByID(ctx, to); err == nil {
			return to, nil
		}
	}

	// Check if it's an email
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	if emailRegex.MatchString(to) {
		// Try to find existing user by email
		existing, err := s.userRepo.GetByEmail(ctx, appID, to)
		if err == nil && existing != nil {
			return existing.UserID, nil
		}

		// Auto-create user
		now := time.Now()
		newUser := &user.User{
			UserID: uuid.New().String(),
			AppID:  appID,
			Email:  to,
			Preferences: user.Preferences{
				EmailEnabled: boolPtr(true),
				PushEnabled:  boolPtr(true),
				SMSEnabled:   boolPtr(true),
			},
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := s.userRepo.Create(ctx, newUser); err != nil {
			return "", fmt.Errorf("failed to auto-create user: %w", err)
		}
		s.logger.Info("Auto-created user for quick-send",
			zap.String("email", to),
			zap.String("user_id", newUser.UserID))
		return newUser.UserID, nil
	}

	return "", fmt.Errorf("recipient %q not found; use an email address (auto-creates user) or internal UUID", to)
}

// resolveTemplate resolves a template reference by name or UUID.
func (s *QuickSendService) resolveTemplate(ctx context.Context, appID, ref string) (*templateDomain.Template, error) {
	// Try UUID first
	if _, err := uuid.Parse(ref); err == nil {
		tmpl, err := s.templateRepo.GetByID(ctx, ref)
		if err == nil && tmpl.AppID == appID {
			return tmpl, nil
		}
	}

	// Try by name (latest active, default locale "en")
	tmpl, err := s.templateRepo.GetByAppAndName(ctx, appID, ref, "en")
	if err == nil {
		return tmpl, nil
	}

	// Try with empty locale (catch-all)
	tmpl, err = s.templateRepo.GetByAppAndName(ctx, appID, ref, "")
	if err == nil {
		return tmpl, nil
	}

	return nil, fmt.Errorf("template %q not found", ref)
}

// createTransientTemplate creates a system-managed template for inline content.
func (s *QuickSendService) createTransientTemplate(ctx context.Context, appID string, req *dto.QuickSendRequest) (*templateDomain.Template, error) {
	ch := req.Channel
	if ch == "" {
		ch = "email" // Default channel for inline content
	}

	// Generate a unique name for the inline template
	name := fmt.Sprintf("_inline_%s", uuid.New().String()[:8])

	createReq := &templateDomain.CreateRequest{
		AppID:     appID,
		Name:      name,
		Channel:   ch,
		Subject:   req.Subject,
		Body:      req.Body,
		Locale:    "en",
		CreatedBy: "system:quick-send",
	}

	return s.templateService.Create(ctx, createReq)
}

func boolPtr(b bool) *bool { return &b }
