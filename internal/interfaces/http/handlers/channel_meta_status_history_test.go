package handlers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/secrets"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"go.uber.org/zap"
	"net/http/httptest"
	"strings"
	"testing"
)

type phase5StatusNotifications struct {
	notification.Repository
	row *notification.Notification
}

func (r *phase5StatusNotifications) List(_ context.Context, f *notification.NotificationFilter) ([]*notification.Notification, error) {
	if f.ProviderMessageID == "wamid.old" {
		return []*notification.Notification{r.row}, nil
	}
	return nil, nil
}

type phase5ConnectionHistory struct {
	channelconnection.Repository
	old, current *channelconnection.Connection
}

func (r *phase5ConnectionHistory) Get(context.Context, string) (*channelconnection.Connection, error) {
	return r.current, nil
}
func (r *phase5ConnectionHistory) GetRevision(_ context.Context, id string, v int64) (*channelconnection.Connection, error) {
	if id == r.old.ID && v == 1 {
		return r.old, nil
	}
	return nil, channelconnection.ErrNotFound
}
func (r *phase5ConnectionHistory) FindSender(context.Context, string, string, string) ([]*channelconnection.Connection, error) {
	return []*channelconnection.Connection{r.current}, nil
}

type phase5StatusCapture struct {
	captureService
	status *whatsapp.DeliveryStatus
	fail   bool
}

func (s *phase5StatusCapture) HandleStatus(_ context.Context, st *whatsapp.DeliveryStatus) error {
	if s.fail {
		return fmt.Errorf("durable store unavailable")
	}
	s.status = st
	return nil
}
func TestChannelMetaStatusAfterDisconnect(t *testing.T) {
	codec, _ := secrets.NewConnectionCodec("fixture:" + base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{23}, 32)))
	id := channelconnection.ID("a", "stage", "whatsapp")
	old := &channelconnection.Connection{ID: id, AppID: "a", EnvironmentID: "stage", Channel: "whatsapp", Provider: "meta", CredentialSource: "byoc", AccountID: "waba", SenderID: "phone", Revision: 1, Readiness: "ready", Secrets: map[string]channelconnection.EncryptedSecret{}}
	for field, value := range map[string]string{"app_secret": "fixture-original-signing", "access_token": "fixture-original-access"} {
		old.Secrets[field], _ = codec.Seal([]byte(value), secrets.SecretBinding{ConnectionID: id, AppID: "a", EnvironmentID: "stage", Provider: "meta", Field: field})
	}
	current := *old
	current.Revision = 2
	current.Readiness = "disconnected"
	current.Secrets = nil
	apps := &channelAppRepo{apps: []*application.Application{{AppID: "a", Settings: application.Settings{WhatsApp: &application.WhatsAppAppConfig{Provider: "meta", ConnectionStatus: "disconnected"}}}}}
	resolver := services.NewChannelSenderResolver(apps, nil)
	resolver.Connections = &services.ChannelConnectionService{Repo: &phase5ConnectionHistory{old: old, current: &current}, Apps: apps, Codec: codec}
	notifications := &phase5StatusNotifications{row: &notification.Notification{AppID: "a", EnvironmentID: "stage", NotificationID: "n", Channel: "whatsapp", Metadata: map[string]interface{}{channelconnection.DeliveryMetadataKey: channelconnection.Snapshot{ConnectionID: id, AppID: "a", EnvironmentID: "stage", Channel: "whatsapp", Provider: "meta", CredentialSource: "byoc", AccountID: "waba", SenderID: "phone", Revision: 1}}}}
	resolver.Notifications = notifications
	for _, tc := range []struct {
		name, account, phone, key string
		fail, ok                  bool
	}{{"late valid", "waba", "phone", "fixture-original-signing", false, true}, {"forged", "waba", "phone", "fixture-forged", false, false}, {"wrong account", "foreign", "phone", "fixture-original-signing", false, false}, {"wrong sender", "waba", "foreign", "fixture-original-signing", false, false}, {"storage outage", "waba", "phone", "fixture-original-signing", true, false}} {
		t.Run(tc.name, func(t *testing.T) {
			svc := &phase5StatusCapture{fail: tc.fail}
			h := NewMetaWebhookHandler(svc, apps, "fixture-unrelated-global", "verify", zap.NewNop())
			h.SetSenderResolver(resolver)
			app := fiber.New()
			app.Post("/", h.HandleWebhook)
			body := fmt.Sprintf(`{"object":"whatsapp_business_account","entry":[{"id":%q,"changes":[{"field":"messages","value":{"metadata":{"phone_number_id":%q},"statuses":[{"id":"wamid.old","status":"delivered","timestamp":"1792000000"}]}}]}]}`, tc.account, tc.phone)
			mac := hmac.New(sha256.New, []byte(tc.key))
			mac.Write([]byte(body))
			req := httptest.NewRequest("POST", "/", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
			res, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			res.Body.Close()
			if tc.ok {
				if res.StatusCode != 200 || svc.status == nil || svc.status.EnvironmentID != "stage" || svc.status.ConnectionID != id {
					t.Fatalf("late status lost original connection: status=%d", res.StatusCode)
				}
			} else if res.StatusCode == 200 || svc.status != nil {
				t.Fatal("unauthenticated/unpersisted status acknowledged")
			}
		})
	}
}

func (r *phase5StatusNotifications) FindAdmittedChannelStatus(context.Context, string, string, string, string) (*notification.Notification, error) {
	return r.row, nil
}
