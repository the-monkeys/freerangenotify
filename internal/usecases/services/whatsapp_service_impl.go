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

	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"github.com/the-monkeys/freerangenotify/internal/domain/workflow"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/sse"
	"go.uber.org/zap"
)

// whatsappService implements whatsapp.Service.
type whatsappService struct {
	applyTemplate func(context.Context, *InboundSenderBinding, *whatsapp.TemplateStatusEvent) error
	events        whatsapp.EventRepository
	repo          whatsapp.Repository
	appRepo       application.Repository
	notifRepo     notification.Repository
	sseBroadc     interface{ PublishMessage(*sse.SSEMessage) error }
	workflowSvc   workflow.Service
	redis         *redis.Client
	logger        *zap.Logger
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
	service := &whatsappService{
		repo:        repo,
		appRepo:     appRepo,
		notifRepo:   notifRepo,
		workflowSvc: workflowSvc,
		redis:       redisClient,
		logger:      logger,
	}
	if sseBroadcaster != nil {
		service.sseBroadc = sseBroadcaster
	}
	if events, ok := repo.(whatsapp.EventRepository); ok {
		service.events = events
	}
	return service
}

func (s *whatsappService) HandleInbound(ctx context.Context, appID string, msg *whatsapp.InboundMessage) error {
	binding := &InboundSenderBinding{ID: msg.ConnectionID, AppID: appID, EnvironmentID: msg.EnvironmentID, Provider: msg.Provider, AccountID: msg.AccountID, SenderID: msg.SenderID}
	if binding.ID == "" {
		binding.ID = appID
	}
	if binding.EnvironmentID == "" {
		binding.EnvironmentID = "default"
	}
	if binding.Provider == "" {
		binding.Provider = "meta"
	}
	_, err := s.AcceptVerifiedEvent(ctx, binding, &whatsapp.VerifiedEvent{Kind: "inbound", Message: msg})
	return err
}

func (s *whatsappService) HandleStatus(ctx context.Context, status *whatsapp.DeliveryStatus) error {
	binding := &InboundSenderBinding{ID: status.ConnectionID, AppID: status.AppID, EnvironmentID: status.EnvironmentID, Provider: status.Provider, AccountID: status.AccountID, SenderID: status.SenderID}
	_, err := s.AcceptVerifiedEvent(ctx, binding, &whatsapp.VerifiedEvent{Kind: "status", Status: status})
	return err
}

func (s *whatsappService) applyDeliveryStatus(ctx context.Context, status *whatsapp.DeliveryStatus) error {
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
		AppID:             status.AppID,
		Channel:           notification.ChannelWhatsApp,
		ProviderMessageID: status.MetaMessageID,
		PageSize:          1,
	}

	notifs, err := s.notifRepo.List(ctx, filter)
	if err != nil || len(notifs) == 0 {
		s.logger.Debug("No notification found for meta_message_id (may be inbound-only contact)",
			zap.String("meta_message_id", status.MetaMessageID))
		return fmt.Errorf("delivery notification not yet available")
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

	transition, ok := s.notifRepo.(interface {
		ApplyProviderDeliveryStatus(context.Context, string, string, string, notification.Status, time.Time) (bool, error)
	})
	if !ok {
		return fmt.Errorf("conditional delivery transitions unavailable")
	}
	changed, transitionErr := transition.ApplyProviderDeliveryStatus(ctx, notif.NotificationID, status.AppID, status.MetaMessageID, newStatus, status.Timestamp)
	if transitionErr != nil {
		s.logger.Error("Failed to update notification delivery status",
			zap.String("notification_id", notif.NotificationID),
			zap.String("new_status", status.Status),
			zap.Error(transitionErr))
		return transitionErr
	}
	if !changed {
		return nil
	}

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
	if s.redis == nil {
		return false
	}
	key := fmt.Sprintf("csw:%s:%s", appID, contactWAID)
	_, err := s.redis.Get(ctx, key).Result()
	return err == nil
}

func (s *whatsappService) publishSSE(eventType string, data map[string]interface{}) error {
	if s.sseBroadc == nil {
		return nil
	}
	return s.sseBroadc.PublishMessage(&sse.SSEMessage{
		AppID: fmt.Sprint(data["app_id"]),
		Type:  eventType,
		Data:  data,
	})
}

// classifyInboundEvent returns the high-level event name corresponding to
// an InboundMessage. Used to populate `event_name` in the workflow trigger
// payload so authors can branch on a single field rather than reverse-
// engineering message_type / interactive_type combinations.
//
// Event names align with the WhatsApp dev doc taxonomy:
//
//	whatsapp.text                — free-form text reply
//	whatsapp.media               — image / video / audio / document upload
//	whatsapp.location            — location share
//	whatsapp.button_clicked      — template URL/QR button tap
//	whatsapp.list_reply          — list-picker selection
//	whatsapp.quick_reply_clicked — interactive button reply
//	whatsapp.reaction            — emoji reaction on a prior outbound message
func classifyInboundEvent(msg *whatsapp.InboundMessage) string {
	if msg.ReactionEmoji != "" {
		return "whatsapp.reaction"
	}
	switch msg.InteractiveType {
	case "button_reply":
		return "whatsapp.quick_reply_clicked"
	case "list_reply":
		return "whatsapp.list_reply"
	}
	if msg.ButtonPayload != "" {
		return "whatsapp.button_clicked"
	}
	switch msg.MessageType {
	case "image", "video", "audio", "document", "sticker":
		return "whatsapp.media"
	case "location":
		return "whatsapp.location"
	case "text":
		return "whatsapp.text"
	}
	return ""
}

func (s *whatsappService) triggerWorkflow(ctx context.Context, appID, triggerID string, msg *whatsapp.InboundMessage) error {
	if s.workflowSvc == nil || triggerID == "" {
		return &workflow.BeforeDispatch{Err: fmt.Errorf("workflow trigger unavailable")}
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
	// Phase 6 of WHATSAPP_RICH_INTERACTIVE_PLAN.md — surface parsed
	// interactive / template-button click data so workflows can branch on
	// `payload.button_payload == "REORDER"` / `payload.reply_id` / etc.
	// without re-parsing raw_payload.
	if msg.InteractiveType != "" {
		payload["interactive_type"] = msg.InteractiveType
	}
	if msg.ReplyID != "" {
		payload["reply_id"] = msg.ReplyID
	}
	if msg.ReplyTitle != "" {
		payload["reply_title"] = msg.ReplyTitle
	}
	if msg.ReplyDescription != "" {
		payload["reply_description"] = msg.ReplyDescription
	}
	if msg.ButtonPayload != "" {
		payload["button_payload"] = msg.ButtonPayload
	}
	if msg.ReactionEmoji != "" {
		payload["reaction_emoji"] = msg.ReactionEmoji
	}
	// Synthesise a higher-level event name workflows can match on. Letting
	// workflow conditions filter by `event_name == "button_clicked"` is
	// dramatically simpler than asking authors to "check if interactive_type
	// is button_reply or button is set".
	if eventName := classifyInboundEvent(msg); eventName != "" {
		payload["event_name"] = eventName
	}
	req := &workflow.TriggerRequest{
		TriggerID:     triggerID,
		TransactionID: "channel-event:" + msg.ID,
		UserID:        userID,
		Payload:       payload,
	}
	var err error
	if dispatcher, ok := s.workflowSvc.(interface {
		TriggerWithOutcome(context.Context, string, *workflow.TriggerRequest) (*workflow.WorkflowExecution, bool, error)
	}); ok {
		var attempted bool
		_, attempted, err = dispatcher.TriggerWithOutcome(ctx, appID, req)
		if err != nil && !attempted {
			err = &workflow.BeforeDispatch{Err: err}
		}
	} else {
		_, err = s.workflowSvc.Trigger(ctx, appID, req)
	}
	if err != nil {
		s.logger.Error("Failed to trigger workflow from inbound WhatsApp",
			zap.String("app_id", appID),
			zap.String("trigger_id", triggerID),
			zap.Error(err))
	}
	return err
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
	marker, ok := s.repo.(interface {
		MarkContactRead(context.Context, string, string, time.Time) error
	})
	if !ok {
		return fmt.Errorf("local inbox read tracking unavailable")
	}
	if err := marker.MarkContactRead(ctx, appID, contactWAID, time.Now().UTC()); err != nil {
		return err
	}
	wa := app.Settings.WhatsApp
	if wa == nil || wa.Provider != "meta" || wa.MetaAccessToken == "" {
		return nil
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
		return fmt.Errorf("provider read receipt unavailable")
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("provider read receipt rejected")
	}
	s.logger.Debug("Read receipt sent", zap.String("contact", contactWAID))
	return nil
}

func (s *whatsappService) forwardToWebhook(ctx context.Context, url string, msg *whatsapp.InboundMessage) error {
	if url == "" {
		return fmt.Errorf("webhook forward URL required")
	}
	body, err := json.Marshal(msg)
	if err != nil {
		s.logger.Error("Failed to marshal inbound message for webhook forward", zap.Error(err))
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		s.logger.Error("Failed to create webhook forward request", zap.Error(err))
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "channel-event:"+msg.ID)

	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		s.logger.Error("Webhook forward unavailable")
		return fmt.Errorf("webhook forward unavailable")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		s.logger.Warn("Webhook forward returned error status",
			zap.Int("status", resp.StatusCode))
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook forwarding rejected")
	}
	return nil
}
