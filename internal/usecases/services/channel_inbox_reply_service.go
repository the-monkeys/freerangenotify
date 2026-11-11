package services

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/queue"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
	"regexp"
	"strings"
	"time"
)

type ChannelInboxReplyService struct {
	Dispatch      *ChannelDispatchService
	Notifications notification.Repository
	Queue         queue.Queue
	Inbox         whatsapp.Repository
}

func (s *ChannelInboxReplyService) EnqueueReply(ctx context.Context, app, contact, text, template, key string) (*notification.Notification, error) {
	phone := notification.NormalizePhone(contact)
	if !regexp.MustCompile(`^\+[1-9][0-9]{6,14}$`).MatchString(phone) || text == "" && template == "" || key == "" || len(key) > 128 {
		return nil, apperrors.BadRequest("Phone, reply content and idempotency key required")
	}
	env := channelconnection.EnvironmentID(ctx)
	id := fmt.Sprintf("wa-reply-%x", sha256.Sum256([]byte(strings.Join([]string{app, env, phone, key}, "\x00"))))
	old, err := s.Notifications.GetByID(ctx, id)
	var appErr *apperrors.AppError
	missing := errors.Is(err, notification.ErrNotificationNotFound) || errors.As(err, &appErr) && appErr.Code == string(apperrors.ErrCodeNotFound) || err == nil && old == nil
	if err != nil && !missing {
		return nil, fmt.Errorf("reply state unavailable")
	}
	if old != nil {
		if old.AppID != app || old.Metadata["inbox_reply_text"] != text || old.Metadata["inbox_reply_template"] != template {
			return nil, apperrors.Conflict("Reply key already used for different content")
		}
		if old.Status == notification.StatusQueued || old.Status == notification.StatusPending {
			if err := s.enqueue(ctx, old); err != nil {
				return nil, err
			}
		}
		return old, nil
	}
	snap, err := s.Dispatch.Connections.ResolveDeliveryConnection(ctx, app, env, notification.ChannelWhatsApp)
	if err != nil {
		return nil, err
	}
	data := map[string]interface{}{}
	if template != "" {
		if snap.Provider == "meta" {
			data["whatsapp_template"] = map[string]interface{}{"name": template, "language": "en_US"}
		} else {
			sid := template
			if !strings.HasPrefix(sid, "HX") {
				if s.Dispatch.RichTemplates == nil {
					return nil, apperrors.BadRequest("Twilio template_name must identify an approved Content SID or local template")
				}
				tpl, e := s.Dispatch.RichTemplates.GetByName(ctx, app, template)
				if e != nil {
					return nil, e
				}
				data["whatsapp_rich"] = map[string]interface{}{"template_id": tpl.ID}
			}
			if strings.HasPrefix(sid, "HX") {
				data["content_sid"] = sid
			}
		}
	}
	now := time.Now().UTC()
	n := &notification.Notification{NotificationID: id, AppID: app, EnvironmentID: env, UserID: contact, Channel: notification.ChannelWhatsApp, Priority: notification.PriorityNormal, Status: notification.StatusQueued, Content: notification.Content{Body: text, Data: data}, CreatedAt: now, UpdatedAt: now, Metadata: map[string]interface{}{"inbox_reply_phone": phone, "inbox_reply_contact": contact, "inbox_reply_text": text, "inbox_reply_template": template, "inbox_reply_key": key}}
	if _, err := s.Dispatch.PrepareChannelDelivery(ctx, n, &user.User{Phone: phone, AppID: app}); err != nil {
		return nil, apperrors.BadRequest(err.Error())
	}
	creator, ok := s.Notifications.(interface {
		CreateIfAbsent(context.Context, *notification.Notification) (bool, error)
	})
	if !ok {
		return nil, fmt.Errorf("durable reply creation unavailable")
	}
	created, err := creator.CreateIfAbsent(ctx, n)
	if err != nil {
		return nil, fmt.Errorf("reply storage unavailable")
	}
	if !created {
		return s.EnqueueReply(ctx, app, contact, text, template, key)
	}
	if err := s.enqueue(ctx, n); err != nil {
		return nil, err
	}
	return n, nil
}
func (s *ChannelInboxReplyService) enqueue(ctx context.Context, n *notification.Notification) error {
	if s.Inbox == nil || s.Queue == nil {
		return fmt.Errorf("reply queue or conversation storage unavailable")
	}
	phone, _ := n.Metadata["inbox_reply_phone"].(string)
	if err := RecordOutboundConversation(ctx, s.Inbox, n, &user.User{Phone: phone}); err != nil {
		return fmt.Errorf("outbound conversation storage unavailable")
	}
	if err := s.Queue.Enqueue(ctx, queue.NotificationQueueItem{NotificationID: n.NotificationID, AppID: n.AppID, Priority: n.Priority, EnqueuedAt: time.Now().UTC()}); err != nil {
		return fmt.Errorf("reply queue admission unavailable; retry the same key")
	}
	return nil
}

// Only deterministic server-created reply IDs can bypass subscriber lookup.
func InboxReplyRecipient(n *notification.Notification) (string, bool) {
	if n == nil || n.Channel != notification.ChannelWhatsApp {
		return "", false
	}
	phone, _ := n.Metadata["inbox_reply_phone"].(string)
	key, _ := n.Metadata["inbox_reply_key"].(string)
	env := n.EnvironmentID
	if env == "" {
		env = "default"
	}
	expected := fmt.Sprintf("wa-reply-%x", sha256.Sum256([]byte(strings.Join([]string{n.AppID, env, phone, key}, "\x00"))))
	return phone, key != "" && phone != "" && n.NotificationID == expected
}

func RecordOutboundConversation(ctx context.Context, repo whatsapp.Repository, n *notification.Notification, u *user.User) error {
	if repo == nil || n == nil || n.Channel != notification.ChannelWhatsApp || u == nil {
		return nil
	}
	b, err := json.Marshal(n.Metadata[channelconnection.DeliveryMetadataKey])
	if err != nil {
		return err
	}
	var snap channelconnection.Snapshot
	if json.Unmarshal(b, &snap) != nil || snap.AppID != n.AppID {
		return fmt.Errorf("outbound connection binding unavailable")
	}
	contact := strings.TrimPrefix(u.Phone, "whatsapp:")
	if snap.Provider == "meta" {
		contact = strings.TrimPrefix(contact, "+")
	}
	if value, ok := n.Metadata["inbox_reply_contact"].(string); ok {
		contact = value
	}
	providerID, _ := n.Metadata["provider_message_id"].(string)
	created := n.CreatedAt
	if created.IsZero() {
		created = time.Now().UTC()
	}
	stamp := created
	if n.SentAt != nil {
		stamp = *n.SentAt
	}
	kind := "text"
	templateName, _ := n.Metadata["inbox_reply_template"].(string)
	if templateName != "" || n.Content.Data["content_sid"] != nil || n.Content.Data["whatsapp_template"] != nil || n.Content.Data["whatsapp_rich"] != nil {
		kind = "template"
	}
	m := &whatsapp.InboundMessage{ID: "outbound:" + n.NotificationID, NotificationID: n.NotificationID, AppID: n.AppID, EnvironmentID: n.EnvironmentID, ConnectionID: snap.ConnectionID, Provider: snap.Provider, AccountID: snap.AccountID, SenderID: snap.SenderID, ContactWAID: contact, UserID: n.UserID, Direction: whatsapp.DirectionOutbound, MessageType: kind, TextBody: n.Content.Body, MetaMessageID: providerID, Timestamp: stamp, CreatedAt: created, DeliveryState: string(n.Status), TemplateName: templateName}
	if writer, ok := repo.(interface {
		StoreOutboundMessage(context.Context, *whatsapp.InboundMessage) error
	}); ok {
		return writer.StoreOutboundMessage(ctx, m)
	}
	return repo.StoreMessage(ctx, m)
}

// Replayable projection of the durable notification. Old rows without an
// admitted binding remain owned by the legacy inbox reader.
func RepairOutboundConversation(ctx context.Context, repo whatsapp.Repository, n *notification.Notification) error {
	if n == nil || n.Channel != notification.ChannelWhatsApp || n.Metadata[channelconnection.DeliveryMetadataKey] == nil {
		return nil
	}
	phone, _ := n.Metadata["channel_destination"].(string)
	if phone == "" {
		phone, _ = n.Metadata["inbox_reply_phone"].(string)
	}
	if phone == "" {
		return fmt.Errorf("admitted inbox destination unavailable")
	}
	return RecordOutboundConversation(ctx, repo, n, &user.User{Phone: phone})
}
