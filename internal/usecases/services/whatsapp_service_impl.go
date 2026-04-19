package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"github.com/the-monkeys/freerangenotify/internal/domain/workflow"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/sse"
	"go.uber.org/zap"
)

// whatsappService implements whatsapp.Service.
type whatsappService struct {
	repo        whatsapp.Repository
	appRepo     application.Repository
	notifRepo   notification.Repository
	sseBroadc   *sse.Broadcaster
	workflowSvc workflow.Service
	redis       *redis.Client
	logger      *zap.Logger
}

// NewWhatsAppService creates a new WhatsApp service.
func NewWhatsAppService(
	repo whatsapp.Repository,
	appRepo application.Repository,
	notifRepo notification.Repository,
	sseBroadcaster *sse.Broadcaster,
	workflowSvc workflow.Service,
	redisClient *redis.Client,
	logger *zap.Logger,
) whatsapp.Service {
	return &whatsappService{
		repo:        repo,
		appRepo:     appRepo,
		notifRepo:   notifRepo,
		sseBroadc:   sseBroadcaster,
		workflowSvc: workflowSvc,
		redis:       redisClient,
		logger:      logger,
	}
}

func (s *whatsappService) HandleInbound(ctx context.Context, appID string, msg *whatsapp.InboundMessage) error {
	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}
	msg.AppID = appID
	msg.Direction = whatsapp.DirectionInbound
	msg.CreatedAt = time.Now().UTC()

	if err := s.repo.StoreMessage(ctx, msg); err != nil {
		s.logger.Error("Failed to store inbound WhatsApp message",
			zap.String("app_id", appID),
			zap.String("meta_message_id", msg.MetaMessageID),
			zap.Error(err))
		return err
	}

	s.trackCSW(ctx, appID, msg.ContactWAID)

	s.publishSSE("whatsapp_inbound", map[string]interface{}{
		"app_id":        appID,
		"message_id":    msg.ID,
		"contact_wa_id": msg.ContactWAID,
		"contact_name":  msg.ContactName,
		"message_type":  msg.MessageType,
		"text_body":     msg.TextBody,
		"timestamp":     msg.Timestamp,
	})

	app, err := s.appRepo.GetByID(ctx, appID)
	if err != nil {
		s.logger.Warn("Could not load app for inbound routing", zap.String("app_id", appID), zap.Error(err))
		return nil
	}

	if app.Settings.WhatsAppInbound == nil || !app.Settings.WhatsAppInbound.Enabled {
		return nil
	}

	cfg := app.Settings.WhatsAppInbound
	switch cfg.RouteAction {
	case whatsapp.RouteWorkflow:
		s.triggerWorkflow(ctx, appID, cfg.WorkflowTriggerID, msg)
	case whatsapp.RouteWebhookForward:
		s.forwardToWebhook(ctx, cfg.WebhookForwardURL, msg)
	case whatsapp.RouteAutoReply:
		s.logger.Info("Auto-reply configured but send not yet implemented",
			zap.String("app_id", appID),
			zap.String("contact_wa_id", msg.ContactWAID))
	case whatsapp.RouteInbox:
		// Messages are already stored; inbox UI queries them directly.
	}

	return nil
}

func (s *whatsappService) HandleStatus(ctx context.Context, status *whatsapp.DeliveryStatus) error {
	if status.MetaMessageID == "" {
		return nil
	}

	statusOrder := map[string]int{"sent": 1, "delivered": 2, "read": 3, "failed": 0}
	newOrder, known := statusOrder[status.Status]
	if !known {
		s.logger.Debug("Ignoring unknown delivery status", zap.String("status", status.Status))
		return nil
	}

	filter := &notification.NotificationFilter{
		Channel:           notification.ChannelWhatsApp,
		ProviderMessageID: status.MetaMessageID,
		PageSize:          1,
	}

	notifs, err := s.notifRepo.List(ctx, filter)
	if err != nil || len(notifs) == 0 {
		s.logger.Debug("No notification found for meta_message_id (may be inbound-only contact)",
			zap.String("meta_message_id", status.MetaMessageID))
		return nil
	}

	notif := notifs[0]

	currentOrder := statusOrder[string(notif.Status)]
	if newOrder <= currentOrder && status.Status != "failed" {
		return nil
	}

	var newStatus notification.Status
	switch status.Status {
	case "sent":
		newStatus = notification.StatusSent
	case "delivered":
		newStatus = notification.StatusDelivered
	case "read":
		newStatus = notification.StatusRead
	case "failed":
		newStatus = notification.StatusFailed
	}

	if err := s.notifRepo.UpdateStatus(ctx, notif.NotificationID, newStatus); err != nil {
		s.logger.Error("Failed to update notification delivery status",
			zap.String("notification_id", notif.NotificationID),
			zap.String("new_status", status.Status),
			zap.Error(err))
		return err
	}

	s.publishSSE("delivery_status", map[string]interface{}{
		"notification_id": notif.NotificationID,
		"app_id":          notif.AppID,
		"status":          status.Status,
		"timestamp":       status.Timestamp,
		"meta_message_id": status.MetaMessageID,
	})

	s.logger.Info("Delivery status updated",
		zap.String("notification_id", notif.NotificationID),
		zap.String("status", status.Status),
		zap.String("meta_message_id", status.MetaMessageID))

	return nil
}

func (s *whatsappService) ListMessages(ctx context.Context, filter *whatsapp.MessageFilter) ([]*whatsapp.InboundMessage, int64, error) {
	return s.repo.List(ctx, filter)
}

func (s *whatsappService) IsCSWOpen(ctx context.Context, appID, contactWAID string) bool {
	key := fmt.Sprintf("csw:%s:%s", appID, contactWAID)
	_, err := s.redis.Get(ctx, key).Result()
	return err == nil
}

func (s *whatsappService) trackCSW(ctx context.Context, appID, contactWAID string) {
	key := fmt.Sprintf("csw:%s:%s", appID, contactWAID)
	if err := s.redis.Set(ctx, key, time.Now().Unix(), 24*time.Hour).Err(); err != nil {
		s.logger.Warn("Failed to track CSW in Redis", zap.String("key", key), zap.Error(err))
	}
}

func (s *whatsappService) publishSSE(eventType string, data map[string]interface{}) {
	if s.sseBroadc == nil {
		return
	}
	_ = s.sseBroadc.PublishMessage(&sse.SSEMessage{
		Type: eventType,
		Data: data,
	})
}

func (s *whatsappService) triggerWorkflow(ctx context.Context, appID, triggerID string, msg *whatsapp.InboundMessage) {
	if s.workflowSvc == nil || triggerID == "" {
		return
	}
	userID := msg.UserID
	if userID == "" {
		userID = msg.ContactWAID
	}
	payload := map[string]interface{}{
		"contact_wa_id":   msg.ContactWAID,
		"contact_name":    msg.ContactName,
		"message_type":    msg.MessageType,
		"text_body":       msg.TextBody,
		"meta_message_id": msg.MetaMessageID,
	}
	_, err := s.workflowSvc.Trigger(ctx, appID, &workflow.TriggerRequest{
		TriggerID: triggerID,
		UserID:    userID,
		Payload:   payload,
	})
	if err != nil {
		s.logger.Error("Failed to trigger workflow from inbound WhatsApp",
			zap.String("app_id", appID),
			zap.String("trigger_id", triggerID),
			zap.Error(err))
	}
}

func (s *whatsappService) ListConversations(ctx context.Context, appID string, limit, offset int) ([]*whatsapp.Conversation, int64, error) {
	convos, total, err := s.repo.ListConversations(ctx, appID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	for _, c := range convos {
		c.CSWOpen = s.IsCSWOpen(ctx, appID, c.ContactWAID)
	}
	return convos, total, nil
}

func (s *whatsappService) Reply(ctx context.Context, appID, contactWAID, text, templateName string) error {
	app, err := s.appRepo.GetByID(ctx, appID)
	if err != nil {
		return fmt.Errorf("failed to load app: %w", err)
	}
	wa := app.Settings.WhatsApp
	if wa == nil || wa.Provider != "meta" || wa.MetaAccessToken == "" {
		return fmt.Errorf("WhatsApp Meta not configured for app %s", appID)
	}

	var payload map[string]interface{}

	if s.IsCSWOpen(ctx, appID, contactWAID) && text != "" {
		payload = map[string]interface{}{
			"messaging_product": "whatsapp",
			"recipient_type":    "individual",
			"to":                contactWAID,
			"type":              "text",
			"text": map[string]interface{}{
				"preview_url": false,
				"body":        text,
			},
		}
	} else if templateName != "" {
		payload = map[string]interface{}{
			"messaging_product": "whatsapp",
			"recipient_type":    "individual",
			"to":                contactWAID,
			"type":              "template",
			"template": map[string]interface{}{
				"name":     templateName,
				"language": map[string]interface{}{"code": "en_US"},
			},
		}
	} else {
		return fmt.Errorf("CSW is closed and no template specified — cannot send free-form text outside the 24h window")
	}

	body, _ := json.Marshal(payload)
	apiURL := fmt.Sprintf("https://graph.facebook.com/%s/%s/messages",
		"v23.0", wa.MetaPhoneNumberID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to build Meta API request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+wa.MetaAccessToken)

	httpClient := &http.Client{Timeout: 15 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("Meta API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Meta API error %d: %s", resp.StatusCode, string(respBody))
	}

	s.logger.Info("Reply sent via Meta WhatsApp",
		zap.String("app_id", appID),
		zap.String("to", contactWAID))

	return nil
}

func (s *whatsappService) MarkRead(ctx context.Context, appID, contactWAID string) error {
	app, err := s.appRepo.GetByID(ctx, appID)
	if err != nil {
		return fmt.Errorf("failed to load app: %w", err)
	}
	wa := app.Settings.WhatsApp
	if wa == nil || wa.Provider != "meta" || wa.MetaAccessToken == "" {
		return fmt.Errorf("WhatsApp Meta not configured for app %s", appID)
	}

	// Get the latest inbound message from this contact to mark it as read
	msgs, _, err := s.repo.List(ctx, &whatsapp.MessageFilter{
		AppID:       appID,
		ContactWAID: contactWAID,
		Direction:   "inbound",
		Limit:       1,
	})
	if err != nil || len(msgs) == 0 {
		return nil
	}

	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"status":            "read",
		"message_id":        msgs[0].MetaMessageID,
	}

	body, _ := json.Marshal(payload)
	apiURL := fmt.Sprintf("https://graph.facebook.com/%s/%s/messages",
		"v23.0", wa.MetaPhoneNumberID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to build read receipt request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+wa.MetaAccessToken)

	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("read receipt request failed: %w", err)
	}
	defer resp.Body.Close()

	s.logger.Debug("Read receipt sent", zap.String("contact", contactWAID))
	return nil
}

func (s *whatsappService) forwardToWebhook(ctx context.Context, url string, msg *whatsapp.InboundMessage) {
	if url == "" {
		return
	}
	body, err := json.Marshal(msg)
	if err != nil {
		s.logger.Error("Failed to marshal inbound message for webhook forward", zap.Error(err))
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		s.logger.Error("Failed to create webhook forward request", zap.Error(err))
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		s.logger.Error("Webhook forward failed", zap.String("url", url), zap.Error(err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		s.logger.Warn("Webhook forward returned error status",
			zap.String("url", url), zap.Int("status", resp.StatusCode))
	}
}
