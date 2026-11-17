package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"go.uber.org/zap"
)

type reminderApps struct {
	application.Repository
	rows map[string]*application.Application
}

func (r reminderApps) GetByID(_ context.Context, id string) (*application.Application, error) {
	a := r.rows[id]
	if a == nil {
		return nil, fmt.Errorf("unknown application")
	}
	return a, nil
}

type legacyReminderConnections struct{ channelconnection.Repository }

func (legacyReminderConnections) Get(context.Context, string) (*channelconnection.Connection, error) {
	return nil, channelconnection.ErrNotFound
}

type reminderTransport struct{ target string }

func (r reminderTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	copy := req.Clone(req.Context())
	copy.URL.Scheme = "http"
	copy.URL.Host = strings.TrimPrefix(r.target, "http://")
	return http.DefaultTransport.RoundTrip(copy)
}

// Removing app scoping or borrowing platform credentials must fail this test.
// Only the vendor boundary is replaced; admission and provider payloads are real.
func TestBizWhatsAppBroadcastAndReminderScope(t *testing.T) {
	for _, provider := range []string{"meta", "twilio"} {
		t.Run(provider, func(t *testing.T) {
			apps := reminderApps{rows: map[string]*application.Application{}}
			for _, id := range []string{"business-a", "business-b"} {
				apps.rows[id] = &application.Application{AppID: id, Settings: application.Settings{WhatsApp: &application.WhatsAppAppConfig{Provider: provider, AccountSID: id, AuthToken: "fixture-" + id, FromNumber: "+15550000001", MetaWABAID: id, MetaPhoneNumberID: "phone-" + id, MetaAccessToken: "fixture-" + id}}}
			}
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if provider == "meta" {
					require.Equal(t, "Bearer fixture-business-a", r.Header.Get("Authorization"))
					require.Contains(t, r.URL.Path, "/phone-business-a/messages")
					var body map[string]interface{}
					require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
					raw, _ := json.Marshal(body)
					require.Contains(t, string(raw), "https://billing.example.invalid/invoice-a")
					fmt.Fprint(w, `{"messages":[{"id":"wamid.fixture"}]}`)
				} else {
					account, secret, _ := r.BasicAuth()
					require.Equal(t, "business-a", account)
					require.Equal(t, "fixture-business-a", secret)
					require.NoError(t, r.ParseForm())
					require.Equal(t, "HXapproved", r.Form.Get("ContentSid"))
					require.Contains(t, r.Form.Get("ContentVariables"), "https://billing.example.invalid/invoice-a")
					fmt.Fprint(w, `{"sid":"SMfixture","status":"queued"}`)
				}
			}))
			defer server.Close()
			connections := &services.ChannelConnectionService{Apps: apps, Repo: legacyReminderConnections{}}
			dispatch := &services.ChannelDispatchService{Connections: connections, Logger: zap.NewNop(), Client: &http.Client{Transport: reminderTransport{server.URL}}}
			for i := 0; i < 3; i++ {
				data := map[string]interface{}{"content_sid": "HXapproved", "content_variables": map[string]interface{}{"1": "https://billing.example.invalid/invoice-a"}}
				if provider == "meta" {
					data = map[string]interface{}{"whatsapp_template": map[string]interface{}{"name": "invoice_reminder", "language": "en_US", "components": []interface{}{map[string]interface{}{"type": "body", "parameters": []interface{}{map[string]interface{}{"type": "text", "text": "https://billing.example.invalid/invoice-a"}}}}}}
				}
				n := &notification.Notification{NotificationID: fmt.Sprint(i), AppID: "business-a", Channel: notification.ChannelWhatsApp, Content: notification.Content{Body: "Invoice INV-1 is unpaid", Data: data}}
				d, err := dispatch.PrepareChannelDelivery(context.Background(), n, &user.User{Phone: fmt.Sprintf("+1555000010%d", i)})
				require.NoError(t, err)
				require.Equal(t, "business-a", d.Snapshot.AppID)
				result, err := dispatch.DispatchPrepared(context.Background(), d)
				require.NoError(t, err)
				require.True(t, result.Success)
				// A persisted prepared identity cannot be replayed under another app.
				n.AppID = "business-b"
				_, err = dispatch.PrepareChannelDelivery(context.Background(), n, d.User)
				require.Error(t, err)
			}
			require.Equal(t, 3, calls)
			_, err := dispatch.PrepareChannelDelivery(context.Background(), &notification.Notification{AppID: "business-a", Channel: notification.ChannelWhatsApp}, &user.User{})
			require.Error(t, err)
			apps.rows["business-a"].Settings.WhatsApp.AuthToken = ""
			apps.rows["business-a"].Settings.WhatsApp.MetaAccessToken = ""
			_, err = dispatch.PrepareChannelDelivery(context.Background(), &notification.Notification{AppID: "business-a", Channel: notification.ChannelWhatsApp}, &user.User{Phone: "+15550000100"})
			require.Error(t, err)
			require.Equal(t, 3, calls, "invalid admissions must not contact a provider")
		})
	}
}
