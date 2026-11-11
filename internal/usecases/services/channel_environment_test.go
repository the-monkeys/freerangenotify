package services

import (
	"context"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"testing"
	"time"
)

func TestChannelEnvironmentDailyCap(t *testing.T) {
	rc := phase4Redis(t)
	ctx := context.Background()
	tenant := uuid.NewString()
	s := NewCreditService(nil, nil, &stubLicenseRepo{sub: &license.Subscription{Plan: "free"}}, nil, nil, nil, rc, nil, true)
	for _, quoted := range []bool{false, true} {
		t.Run(map[bool]string{false: "legacy", true: "quoted"}[quoted], func(t *testing.T) {
			tenant += uuid.NewString()
			s.SetEnforceFreeTierDailyCap(false)
			acquire := func(i int) (string, error) {
				if quoted {
					return s.acquireQuotedDailyCap(ctx, tenant, "sms", tenant+string(rune('a'+i)))
				}
				return s.acquireDailyCap(ctx, tenant, "sms")
			}
			for i := 0; i < 5; i++ {
				key, err := acquire(i)
				if err != nil || key != "" {
					t.Fatalf("disabled cap charged admission: %q %v", key, err)
				}
			}
			if rc.Exists(ctx, dailyCapKey(tenant, "sms", time.Now().UTC())).Val() != 0 {
				t.Fatal("disabled cap mutated budget")
			}
			s.SetEnforceFreeTierDailyCap(true)
			for i := 0; i < 3; i++ {
				if _, err := acquire(i); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := acquire(4); err != ErrDailyCapExceeded {
				t.Fatalf("enabled cap bypassed: %v", err)
			}
		})
	}
}

type channelUsageCapture struct {
	billing.UsageRepository
	event *billing.UsageEvent
}

func (r *channelUsageCapture) Store(_ context.Context, e *billing.UsageEvent) error {
	r.event = e
	return nil
}
func TestChannelQuotedUsageFRNCurrency(t *testing.T) {
	repo := &channelUsageCapture{}
	s := &CreditService{usageRepo: repo}
	res := &billing.CreditReservation{ID: "id", TenantID: "tenant", AppID: "app", NotificationID: "n", Channel: "sms", Status: billing.CreditReservationCommitted, CreditsReserved: 3, Quote: &billing.ChannelQuote{VendorCurrency: "GBP"}}
	if err := s.recordQuotedUsage(context.Background(), res); err != nil {
		t.Fatal(err)
	}
	if repo.event.Currency != "INR" || repo.event.Quote.VendorCurrency != "GBP" {
		t.Fatal("FRN currency omitted or confused with vendor estimate")
	}
}

func TestChannelEnvironmentMetaAdmissionSwitch(t *testing.T) {
	connections := connectionTestService()
	connections.Apps.(*connectionApps).app.Settings.WhatsApp = &application.WhatsAppAppConfig{Provider: "meta", MetaWABAID: "waba", MetaPhoneNumberID: "phone", MetaAccessToken: "fixture-token"}
	enabled := false
	s := &ChannelDispatchService{Connections: connections, MetaEnabled: &enabled}
	n := &notification.Notification{AppID: "a", NotificationID: "n", Channel: notification.ChannelWhatsApp, Content: notification.Content{Body: "hello", Data: map[string]interface{}{"whatsapp_template": map[string]interface{}{"name": "hello", "language": "en_US"}}}}
	if _, err := s.PrepareChannelDelivery(context.Background(), n, &user.User{Phone: "+456"}); err == nil {
		t.Fatal("explicitly disabled Meta accepted stale credentials")
	}
	enabled = true
	if _, err := s.PrepareChannelDelivery(context.Background(), n, &user.User{Phone: "+456"}); err != nil {
		t.Fatal(err)
	}
	enabled = false
	if _, err := s.PrepareChannelDelivery(context.Background(), n, &user.User{Phone: "+456"}); err != nil {
		t.Fatalf("admitted snapshot lost on rollback: %v", err)
	}
}
