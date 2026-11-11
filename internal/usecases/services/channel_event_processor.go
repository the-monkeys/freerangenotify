package services

import (
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"github.com/the-monkeys/freerangenotify/internal/domain/workflow"
	"time"
)

// AcceptVerifiedEvent persists identity before any callback business action.
func (s *whatsappService) AcceptVerifiedEvent(ctx context.Context, binding *InboundSenderBinding, event *whatsapp.VerifiedEvent) (*whatsapp.EventReceipt, error) {
	if s.events == nil {
		return nil, fmt.Errorf("durable callback store unavailable")
	}
	event.Binding = *binding
	event.Binding.SigningSecret = ""
	identity := event.ID
	if identity == "" {
		if event.Message != nil {
			identity = event.Message.MetaMessageID
		}
		if event.Status != nil {
			identity = event.Status.MetaMessageID + ":" + event.Status.Status + ":" + event.Status.Timestamp.Format(time.RFC3339Nano)
		}
		if event.Template != nil {
			identity = event.Template.TemplateID + ":" + event.Template.Status + ":" + event.Template.Reason
		}
	}
	if identity == "" || binding.AppID == "" {
		return nil, fmt.Errorf("verified callback identity required")
	}
	id := fmt.Sprintf("%x", sha256.Sum256([]byte(binding.Provider+"\x00"+binding.ID+"\x00"+binding.AppID+"\x00"+event.Kind+"\x00"+identity)))
	now := time.Now().UTC()
	receipt := &whatsapp.EventReceipt{ID: id, Event: *event, State: "pending", Steps: map[string]string{}, CreatedAt: now, UpdatedAt: now, NextAttempt: now, LeaseUntil: now}
	if err := s.events.AcceptEvent(ctx, receipt); err != nil {
		return nil, err
	}
	return receipt, nil
}
func (s *whatsappService) ProcessEventReceipt(ctx context.Context, id string) error {
	receipt, err := s.events.ClaimEvent(ctx, id, time.Now().UTC())
	if err != nil {
		return err
	}
	if receipt == nil {
		return nil
	}
	if receipt.Event.Kind == "template" {
		if s.applyTemplate == nil || receipt.Event.Template == nil {
			err = fmt.Errorf("template recovery unavailable")
		} else {
			err = s.applyTemplate(ctx, &receipt.Event.Binding, receipt.Event.Template)
		}
	} else if receipt.Event.Kind == "status" {
		err = s.processStatusReceipt(ctx, receipt)
	} else {
		err = s.processInboundReceipt(ctx, receipt)
	}
	if err != nil {
		receipt.LastError = "callback processing unavailable"
		if workflow.IsBeforeDispatch(err) {
			receipt.Steps["action"] = "pending"
		}
		if receipt.Steps["action"] == "dispatching" {
			receipt.State = "unknown"
		} else if receipt.Attempts >= 5 {
			receipt.State = "dead_letter"
		} else {
			receipt.State = "pending"
			receipt.NextAttempt = time.Now().UTC().Add(time.Duration(receipt.Attempts) * 10 * time.Second)
		}
	} else {
		receipt.State = "done"
	}
	receipt.LeaseUntil = time.Time{}
	if saveErr := s.events.SaveEvent(ctx, receipt); saveErr != nil {
		return saveErr
	}
	return err
}
func (s *whatsappService) processStatusReceipt(ctx context.Context, receipt *whatsapp.EventReceipt) error {
	status := receipt.Event.Status
	if status == nil || status.MetaMessageID == "" {
		return fmt.Errorf("provider status identity required")
	}
	if receipt.Steps["status"] != "done" {
		if err := s.applyDeliveryStatus(ctx, status); err != nil {
			return err
		}
		receipt.Steps["status"] = "done"
		if err := s.events.SaveEvent(ctx, receipt); err != nil {
			return err
		}
	}
	if receipt.Steps["sse"] == "done" {
		return nil
	}
	rows, err := s.notifRepo.List(ctx, &notification.NotificationFilter{AppID: receipt.Event.Binding.AppID, Channel: notification.ChannelWhatsApp, ProviderMessageID: status.MetaMessageID, PageSize: 1})
	if err != nil || len(rows) == 0 {
		return fmt.Errorf("delivery refresh state unavailable")
	}
	row, err := s.notifRepo.GetByID(ctx, rows[0].NotificationID)
	if err != nil || row == nil || row.AppID != receipt.Event.Binding.AppID {
		return fmt.Errorf("delivery refresh state unavailable")
	}
	if receipt.Steps["inbox"] != "done" {
		if err := RepairOutboundConversation(ctx, s.repo, row); err != nil {
			return err
		}
		receipt.Steps["inbox"] = "done"
		if err := s.events.SaveEvent(ctx, receipt); err != nil {
			return err
		}
	}
	if err := s.publishSSE("delivery_status", map[string]interface{}{"notification_id": row.NotificationID, "app_id": row.AppID, "status": string(row.Status), "timestamp": status.Timestamp, "meta_message_id": status.MetaMessageID}); err != nil {
		return err
	}
	receipt.Steps["sse"] = "done"
	return s.events.SaveEvent(ctx, receipt)
}
func (s *whatsappService) processInboundReceipt(ctx context.Context, receipt *whatsapp.EventReceipt) error {
	msg := receipt.Event.Message
	if msg == nil {
		return fmt.Errorf("inbound message required")
	}
	msg.ID = receipt.ID
	msg.AppID = receipt.Event.Binding.AppID
	msg.EnvironmentID = receipt.Event.Binding.EnvironmentID
	ctx = channelconnection.WithEnvironment(ctx, msg.EnvironmentID)
	msg.Direction = whatsapp.DirectionInbound
	msg.CreatedAt = receipt.CreatedAt
	if receipt.Steps["stored"] != "done" {
		if err := s.repo.StoreMessage(ctx, msg); err != nil {
			return err
		}
		receipt.Steps["stored"] = "done"
		if err := s.events.SaveEvent(ctx, receipt); err != nil {
			return err
		}
	}
	if err := s.trackInboundWindow(ctx, msg); err != nil {
		return err
	}
	app, err := s.appRepo.GetByID(ctx, msg.AppID)
	if err != nil || app == nil {
		return fmt.Errorf("inbound application unavailable")
	}
	if receipt.Steps["action"] != "done" {
		cfg := app.Settings.WhatsAppInbound
		unsupportedReply := cfg != nil && cfg.Enabled && cfg.RouteAction == whatsapp.RouteAutoReply
		if unsupportedReply {
			receipt.LastError = "automatic replies are unavailable; message retained in inbox"
		}
		if cfg != nil && cfg.Enabled && (cfg.RouteAction == whatsapp.RouteWorkflow || cfg.RouteAction == whatsapp.RouteWebhookForward) {
			receipt.Steps["action"] = "dispatching"
			if err := s.events.SaveEvent(ctx, receipt); err != nil {
				return err
			}
			if cfg.RouteAction == whatsapp.RouteWorkflow {
				err = s.triggerWorkflow(ctx, msg.AppID, cfg.WorkflowTriggerID, msg)
			} else {
				err = s.forwardToWebhook(ctx, cfg.WebhookForwardURL, msg)
			}
			if err != nil {
				return err
			}
		}
		receipt.Steps["action"] = "done"
		if unsupportedReply {
			receipt.Steps["action"] = "unavailable"
		}
		if err := s.events.SaveEvent(ctx, receipt); err != nil {
			return err
		}
	}
	if receipt.Steps["sse"] != "done" {
		if err := s.publishSSE("whatsapp_inbound", map[string]interface{}{"app_id": msg.AppID, "message_id": msg.ID, "contact_wa_id": msg.ContactWAID, "contact_name": msg.ContactName, "text_body": msg.TextBody, "timestamp": msg.Timestamp, "message_type": msg.MessageType}); err != nil {
			return err
		}
		receipt.Steps["sse"] = "done"
		if err := s.events.SaveEvent(ctx, receipt); err != nil {
			return err
		}
	}
	return nil
}
func (s *whatsappService) SetTemplateStatusApplier(apply func(context.Context, *InboundSenderBinding, *whatsapp.TemplateStatusEvent) error) {
	s.applyTemplate = apply
}
func (s *whatsappService) RecoverChannelEvents(ctx context.Context) error {
	ids, err := s.events.PendingEvents(ctx, time.Now().UTC(), 100)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := s.ProcessEventReceipt(ctx, id); err != nil {
			s.logger.Warn("Callback recovery deferred")
		}
	}
	return nil
}

func (s *whatsappService) trackInboundWindow(ctx context.Context, msg *whatsapp.InboundMessage) error {
	if s.redis == nil {
		return nil
	}
	now := time.Now().UTC()
	timestamp := msg.Timestamp
	if msg.Provider == "twilio" {
		// Programmable Messaging callbacks omit the provider creation time.
		// Receipt time is for display only and cannot open a billable window.
		raw, _ := msg.RawPayload["DateCreated"].(string)
		if raw == "" {
			return nil
		}
		var err error
		timestamp, err = time.Parse(time.RFC3339, raw)
		if err != nil {
			timestamp, err = time.Parse(time.RFC1123Z, raw)
		}
		if err != nil {
			return fmt.Errorf("invalid provider creation time")
		}
	}
	if timestamp.IsZero() || timestamp.After(now.Add(time.Minute)) {
		return fmt.Errorf("invalid inbound timestamp")
	}
	expiry := timestamp.Add(24 * time.Hour)
	if !expiry.After(now) {
		return nil
	}
	const script = `local previous = tonumber(redis.call('GET', KEYS[1]) or '0'); local current = tonumber(ARGV[1]); if current > previous then redis.call('SET', KEYS[1], ARGV[1], 'PXAT', ARGV[2]); end; return 1`
	if err := s.redis.Eval(ctx, script, []string{channelconnection.WindowKey(msg.AppID, msg.EnvironmentID, msg.Provider, msg.AccountID, msg.SenderID, msg.ContactWAID)}, timestamp.UnixMilli(), expiry.UnixMilli()).Err(); err != nil {
		return err
	}
	// Legacy display readers remain compatible; dispatch never authorizes from this key.
	return s.redis.Eval(ctx, script, []string{fmt.Sprintf("csw:%s:%s", msg.AppID, msg.ContactWAID)}, timestamp.UnixMilli(), expiry.UnixMilli()).Err()
}
