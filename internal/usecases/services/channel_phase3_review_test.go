package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/providers"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestPhase3ReviewDestinationWithoutBilling(t *testing.T) {
	c := connectionTestService()
	c.Apps.(*connectionApps).app.Settings.SMS = &application.SMSAppConfig{AccountSID: "account", AuthToken: "fixture", FromNumber: "+123"}
	s := &ChannelDispatchService{Connections: c}
	n := &notification.Notification{AppID: "a", Channel: notification.ChannelSMS, Content: notification.Content{Body: "hello"}}
	if _, err := s.PrepareChannelDelivery(context.Background(), n, &user.User{Phone: "+456"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PrepareChannelDelivery(context.Background(), n, &user.User{Phone: "+789"}); err == nil {
		t.Fatal("queued destination changed with billing disabled")
	}
}

func TestPhase3ReviewLegacyPending(t *testing.T) {
	c := connectionTestService()
	c.Apps.(*connectionApps).app.Settings.WhatsApp.ConnectionStatus = "pending"
	s := &ChannelDispatchService{Connections: c}
	n := &notification.Notification{AppID: "a", Channel: notification.ChannelWhatsApp, Content: notification.Content{Data: map[string]interface{}{"content_sid": "HXfixture"}}}
	if _, err := s.PrepareChannelDelivery(context.Background(), n, &user.User{Phone: "+456"}); err == nil {
		t.Fatal("explicit pending onboarding sent before validation")
	}
}

func TestPhase3ReviewPlatformMetaRoute(t *testing.T) {
	c := connectionTestService()
	c.Apps.(*connectionApps).app.Settings.WhatsApp = nil
	c.System = map[notification.Channel]*channelconnection.Snapshot{
		notification.ChannelWhatsApp:          {Provider: "twilio"},
		notification.Channel("meta_whatsapp"): {Provider: "meta", AccountID: "waba", SenderID: "phone", Credentials: map[string]string{"access_token": "fixture"}},
	}
	s := &ChannelDispatchService{Connections: c}
	n := &notification.Notification{AppID: "a", Channel: notification.ChannelWhatsApp, Content: notification.Content{Data: map[string]interface{}{"whatsapp_template": map[string]interface{}{"name": "hello"}}}}
	for i := 0; i < 2; i++ {
		d, err := s.PrepareChannelDelivery(context.Background(), n, &user.User{Phone: "+456"})
		if err != nil {
			t.Fatal(err)
		}
		if d.Snapshot.Provider != "meta" || d.Snapshot.CredentialSource != "system" {
			t.Fatal("explicit Meta payload lost platform route")
		}
	}
}

func TestPhase3ReviewEmptyMetaSettingsRetainPlatformRoute(t *testing.T) {
	c := connectionTestService()
	c.Apps.(*connectionApps).app.Settings.WhatsApp = &application.WhatsAppAppConfig{Provider: "meta"}
	c.System = map[notification.Channel]*channelconnection.Snapshot{notification.Channel("meta_whatsapp"): {Provider: "meta", AccountID: "waba", SenderID: "phone", Credentials: map[string]string{"access_token": "fixture"}}}
	s := &ChannelDispatchService{Connections: c}
	n := &notification.Notification{AppID: "a", Channel: notification.ChannelWhatsApp, Content: notification.Content{Data: map[string]interface{}{"whatsapp_template": map[string]interface{}{"name": "hello"}}}}
	d, e := s.PrepareChannelDelivery(context.Background(), n, &user.User{Phone: "+456"})
	if e != nil || d.Snapshot.CredentialSource != "system" {
		t.Fatalf("empty app fields prevented explicit system Meta route: %+v %v", d, e)
	}
}

func TestPhase3ReviewVonageRejectionAndRedaction(t *testing.T) {
	for _, response := range []struct {
		status int
		body   string
	}{{400, `{"message":"123456 fixture-secret"}`}, {200, `{"messages":[{"status":"4","error-text":"123456 fixture-secret"}]}`}} {
		t.Run(fmt.Sprint(response.status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(response.status)
				fmt.Fprint(w, response.body)
			}))
			defer server.Close()
			core, logs := observer.New(zap.DebugLevel)
			s := &ChannelDispatchService{Client: &http.Client{Transport: dispatchTransport{server.URL}}, Logger: zap.New(core)}
			d := &channelconnection.PreparedDelivery{Snapshot: &channelconnection.Snapshot{Channel: notification.ChannelSMS, Provider: "vonage", CredentialSource: "system", AccountID: "account", SenderID: "sender", Credentials: map[string]string{"api_secret": "fixture-secret"}}, Notification: &notification.Notification{Content: notification.Content{Body: "OTP 123456"}}, User: &user.User{Phone: "+919876543210"}}
			_, err := s.DispatchPrepared(context.Background(), d)
			if !providers.IsDefinitiveDeliveryFailure(err) {
				t.Errorf("known rejection quarantined: %v", err)
			}
			for _, log := range logs.All() {
				if strings.Contains(fmt.Sprint(log.ContextMap()), "123456") || strings.Contains(fmt.Sprint(log.ContextMap()), "fixture-secret") {
					t.Error("OTP/secret leaked in provider log")
				}
			}
		})
	}
}

func TestPhase3ReviewTwilioRejectsMetaShapes(t *testing.T) {
	for _, key := range []string{"whatsapp_interactive", "whatsapp_location", "whatsapp_video"} {
		n := &notification.Notification{Content: notification.Content{Body: "fallback", Data: map[string]interface{}{key: map[string]interface{}{"fixture": true}}}}
		if _, err := providers.WhatsAppPayloadKind(n, "twilio"); err == nil {
			t.Errorf("%s silently discarded", key)
		}
	}
}

func TestPhase3ReviewResolvedCarouselAdmission(t *testing.T) {
	svc, _, _, _, cleanup := newServiceWithMockProviders(t)
	defer cleanup()
	tpl, err := svc.Create(context.Background(), &whatsapp.RichTemplate{Name: "carousel", AppID: "app-1", Language: "en_US", Category: "MARKETING", Kind: whatsapp.KindCarousel, Body: "hello", Cards: []whatsapp.CarouselCard{{HeaderImageURL: "https://fixture/1.jpg", Body: "one", Buttons: []whatsapp.Button{{Type: whatsapp.ButtonURL, Text: "View", URL: "https://fixture/one"}}}, {HeaderImageURL: "https://fixture/2.jpg", Body: "two", Buttons: []whatsapp.Button{{Type: whatsapp.ButtonURL, Text: "View", URL: "https://fixture/two"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	// Delivery requires the provider callback to approve the submitted template.
	tpl.Providers.Meta.Status = "APPROVED"
	c := connectionTestService()
	c.Apps.(*connectionApps).app = &application.Application{AppID: "app-1", Settings: application.Settings{WhatsApp: &application.WhatsAppAppConfig{Provider: "meta", MetaWABAID: "waba", MetaPhoneNumberID: "phone", MetaAccessToken: "fixture"}}}
	s := &ChannelDispatchService{Connections: c, RichTemplates: svc}
	n := &notification.Notification{AppID: "app-1", Channel: notification.ChannelWhatsApp, Content: notification.Content{Data: map[string]interface{}{"whatsapp_rich": map[string]interface{}{"template_id": tpl.ID}}}}
	d, err := s.PrepareChannelDelivery(context.Background(), n, &user.User{Phone: "+456"})
	if err != nil {
		t.Fatal(err)
	}
	if d.PayloadKind != "template" {
		t.Fatal("carousel no longer a template")
	}
}

func TestPhase3ReviewInboxCallbackHistory(t *testing.T) {
	events := &channelMemoryEvents{rows: map[string]*whatsapp.EventReceipt{}}
	inbox := &channelMessages{rows: map[string]*whatsapp.InboundMessage{}}
	snap := channelconnection.Snapshot{AppID: "a", Provider: "meta", AccountID: "account", SenderID: "sender"}
	b, _ := json.Marshal(snap)
	var safe map[string]interface{}
	json.Unmarshal(b, &safe)
	notifs := &channelStatusRepo{row: &notification.Notification{NotificationID: "n", AppID: "a", Channel: notification.ChannelWhatsApp, Status: notification.StatusSent, Metadata: map[string]interface{}{"provider_message_id": "upstream", "channel_destination": "+919876543210", channelconnection.DeliveryMetadataKey: safe}}}
	svc := &whatsappService{events: events, repo: inbox, notifRepo: notifs, sseBroadc: &channelSignalPublisher{}, logger: zap.NewNop()}
	receipt, err := svc.AcceptVerifiedEvent(context.Background(), &InboundSenderBinding{ID: "binding", AppID: "a", Provider: "meta"}, &whatsapp.VerifiedEvent{Kind: "status", Status: &whatsapp.DeliveryStatus{AppID: "a", MetaMessageID: "upstream", Status: "read", Timestamp: time.Now()}})
	if err != nil {
		t.Fatal(err)
	}
	if err = svc.ProcessEventReceipt(context.Background(), receipt.ID); err != nil {
		t.Fatal(err)
	}
	m := inbox.rows["outbound:n"]
	if m == nil || m.DeliveryState != "read" {
		t.Fatalf("callback left history stale: %+v", m)
	}
}
