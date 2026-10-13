package providers

import (
	"context"
	"testing"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"go.uber.org/zap"
)

type usageCapture struct{ events chan billing.UsageEvent }

func (c *usageCapture) Emit(_ context.Context, event *billing.UsageEvent) error {
	c.events <- *event
	return nil
}

func TestUsageEventAttributesCreditsToTenantRatherThanApp(t *testing.T) {
	emitter := &usageCapture{events: make(chan billing.UsageEvent, 1)}
	m := NewManager(nil, nil, zap.NewNop()).WithBillingEmitter(true, emitter)
	notif := &notification.Notification{AppID: "app-1", NotificationID: "notif-1", Channel: notification.ChannelEmail,
		Metadata: map[string]interface{}{"tenant_id": "tenant-1", "credits_used": int64(5), "rate_card_version": "reserved-version"}}
	m.emitUsageEvent(notif, &Result{Success: true, Metadata: map[string]interface{}{"credential_source": "byoc", "billing_channel": "email"}}, "smtp")
	select {
	case event := <-emitter.events:
		if event.TenantID != "tenant-1" || event.AppID != "app-1" || event.CreditsUsed != 5 || event.RateCardVersion != "reserved-version" {
			t.Fatalf("metered usage differs from billed identity or reservation: %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("usage event missing")
	}
}
