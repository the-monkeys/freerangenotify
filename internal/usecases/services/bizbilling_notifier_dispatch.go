package services

import (
	"context"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	apperrors "github.com/the-monkeys/freerangenotify/pkg/errors"
)

// Dispatch preserves admission errors and the trusted delivery identity. The
// legacy best-effort Send method and general notification APIs are unchanged.
func (n *BizNotifier) Dispatch(ctx context.Context, intent bizbilling.NotificationIntent, id string, data map[string]interface{}) (*notification.Notification, error) {
	if n == nil || n.notifSvc == nil {
		return nil, apperrors.Conflict("Notification delivery is not configured")
	}
	tmpl, err := n.ensureTemplate(ctx, intent.AppID, intent.TemplateID)
	if err != nil {
		return nil, err
	}
	channel := intent.Channel
	if channel == "" {
		channel = tmpl.Channel
	}
	return n.notifSvc.Send(notification.WithDispatchIdentity(ctx, id), notification.SendRequest{AppID: intent.AppID, UserID: intent.UserID, Channel: notification.Channel(channel), Priority: notification.PriorityNormal, TemplateID: tmpl.ID, Data: data, Metadata: map[string]interface{}{"source": "biz_billing", "billing_delivery_id": id}})
}
func (n *BizNotifier) Get(ctx context.Context, id, app string) (*notification.Notification, error) {
	if n == nil || n.notifSvc == nil {
		return nil, apperrors.Conflict("Notification delivery is not configured")
	}
	return n.notifSvc.Get(ctx, id, app)
}
