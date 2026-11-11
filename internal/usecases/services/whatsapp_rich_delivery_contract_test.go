package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"go.uber.org/zap"
)

func TestStoredWhatsAppRichDeliveryContract(t *testing.T) {
	address := os.Getenv("FRN_OTP_REDIS_ADDR")
	if address == "" {
		t.Skip("requires isolated local Redis fixture")
	}
	require.Equal(t, "127.0.0.1:16390", address)
	client := redis.NewClient(&redis.Options{Addr: address})
	defer client.Close()
	for _, kind := range []whatsapp.RichTemplateKind{whatsapp.KindList, whatsapp.KindCTAURL, whatsapp.KindQuickReply} {
		t.Run(string(kind), func(t *testing.T) {
			ctx := context.Background()
			var wire map[string]interface{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v23.0/phone/messages" {
					require.NoError(t, json.NewDecoder(r.Body).Decode(&wire))
					fmt.Fprint(w, `{"messages":[{"id":"wamid.fixture"}]}`)
					return
				}
				fmt.Fprint(w, `{"id":"template-fixture","status":"APPROVED"}`)
			}))
			defer server.Close()
			wa := &application.WhatsAppAppConfig{Provider: "meta", MetaWABAID: "waba", MetaPhoneNumberID: "phone", MetaAccessToken: "fixture-token"}
			apps := &richStubAppRepo{app: &application.Application{AppID: "app-1", Settings: application.Settings{WhatsApp: wa}}}
			repo := newMemRichRepo()
			httpClient := &http.Client{Transport: dispatchTransport{server.URL}}
			rich := NewWhatsAppRichTemplateService(repo, apps, httpClient, "v23.0", "fixture-app", "fixture-secret", zap.NewNop())
			tpl := &whatsapp.RichTemplate{AppID: "app-1", Name: "contract_" + string(kind), Kind: kind, Language: "en_US", Category: "UTILITY", Body: "Choose {{1}}"}
			switch kind {
			case whatsapp.KindList:
				tpl.ListButtonText = "Menu"
				tpl.ListSections = []whatsapp.ListSection{{Title: "Options", Rows: []whatsapp.ListRow{{ID: "row-one", Title: "One"}}}}
			case whatsapp.KindCTAURL:
				tpl.Buttons = []whatsapp.Button{{Type: whatsapp.ButtonURL, Text: "Open", URL: "https://example.test/open"}}
			case whatsapp.KindQuickReply:
				tpl.Buttons = []whatsapp.Button{{Type: whatsapp.ButtonQuickReply, Text: "Confirm", Payload: "confirm"}}
			}
			stored, err := rich.Create(ctx, tpl)
			require.NoError(t, err)
			loaded, err := rich.Get(ctx, "app-1", stored.ID)
			require.NoError(t, err)
			connections := connectionTestService()
			connections.Apps = apps
			dispatch := &ChannelDispatchService{Connections: connections, Redis: client, Client: httpClient, RichTemplates: rich, Logger: zap.NewNop(), GraphVersion: "v23.0", Quotes: &ChannelQuoteService{Rates: &quoteCard{card: &billing.RateCard{Version: "contract", ChannelCreditCost: map[string]int64{"whatsapp": 1}}}}}
			makeNotification := func() *notification.Notification {
				return &notification.Notification{NotificationID: "contract-notification", AppID: "app-1", EnvironmentID: "default", Channel: notification.ChannelWhatsApp, Content: notification.Content{Body: "Choose Alice", Data: map[string]interface{}{"whatsapp_rich": map[string]interface{}{"template_id": stored.ID, "variables": map[string]interface{}{"1": "Alice"}}}}}
			}
			recipient := &user.User{Phone: "+14155551234"}
			key := channelconnection.WindowKey("app-1", "default", "meta", "waba", "phone", recipient.Phone)
			defer client.Del(ctx, key)
			require.NoError(t, client.Del(ctx, key).Err())
			if kind == whatsapp.KindList {
				_, err = dispatch.PrepareChannelDelivery(ctx, makeNotification(), recipient)
				require.Error(t, err)
				require.NoError(t, client.Set(ctx, key, "1", time.Minute).Err())
			}
			n := makeNotification()
			prepared, err := dispatch.PrepareChannelDelivery(ctx, n, recipient)
			require.NoError(t, err)
			expected := "template"
			if kind == whatsapp.KindList {
				expected = "text"
			}
			require.Equal(t, expected, prepared.PayloadKind)
			result, err := dispatch.DispatchPrepared(ctx, prepared)
			require.NoError(t, err)
			require.True(t, result.Success)
			if kind == whatsapp.KindList {
				require.Equal(t, "interactive", wire["type"])
				interactive := wire["interactive"].(map[string]interface{})
				require.Equal(t, "list", interactive["type"])
			} else {
				require.Equal(t, "template", wire["type"])
				meta := wire["template"].(map[string]interface{})
				require.Equal(t, loaded.Providers.Meta.TemplateName, meta["name"])
				components := meta["components"].([]interface{})
				require.NotEmpty(t, components)
				if kind == whatsapp.KindQuickReply {
					require.Len(t, components, 2)
					require.Equal(t, "quick_reply", components[1].(map[string]interface{})["sub_type"])
				}
			}
			// Equivalent translated raw payload follows the same route/classification.
			raw := makeNotification()
			raw.Content.Data = map[string]interface{}{}
			for k, v := range n.Content.Data {
				if k != "whatsapp_rich" {
					raw.Content.Data[k] = v
				}
			}
			rawPrepared, err := dispatch.PrepareChannelDelivery(ctx, raw, recipient)
			require.NoError(t, err)
			require.Equal(t, prepared.PayloadKind, rawPrepared.PayloadKind)
			require.Equal(t, prepared.Snapshot.AccountID, rawPrepared.Snapshot.AccountID)
			require.Equal(t, prepared.Snapshot.SenderID, rawPrepared.Snapshot.SenderID)
			q := n.Metadata[billing.ChannelQuoteMetadataKey].(*billing.ChannelQuote)
			rawQuote := raw.Metadata[billing.ChannelQuoteMetadataKey].(*billing.ChannelQuote)
			require.Equal(t, q.Credits, rawQuote.Credits)
			require.Equal(t, q.PayloadHash, rawQuote.PayloadHash)
			require.Equal(t, q.ID, rawQuote.ID)
			_, err = rich.ResolveSendPayload(ctx, "other-app", "n", whatsapp.SendPayload{TemplateID: stored.ID})
			require.Error(t, err)
			_, err = rich.ResolveSendPayload(channelconnection.WithEnvironment(ctx, "staging"), "app-1", "n", whatsapp.SendPayload{TemplateID: stored.ID})
			require.Error(t, err)
			if kind == whatsapp.KindList {
				oldBinding := loaded.Providers.Meta
				loaded.ApprovalState = whatsapp.ApprovalDraft
				_, err = rich.ResolveSendPayload(ctx, "app-1", "n", whatsapp.SendPayload{TemplateID: stored.ID})
				require.NoError(t, err, "legacy runtime-only list remains usable")
				loaded.Providers.Meta = nil
				_, err = rich.ResolveSendPayload(ctx, "app-1", "n", whatsapp.SendPayload{TemplateID: stored.ID})
				require.Error(t, err, "new unsubmitted draft must not dispatch")
				loaded.Providers.Meta = oldBinding
				loaded.ApprovalState = whatsapp.ApprovalApproved
			}
			if kind != whatsapp.KindList {
				for _, status := range []string{"PENDING", "REJECTED", "DISABLED"} {
					loaded.Providers.Meta.Status = status
					_, err = rich.ResolveSendPayload(ctx, "app-1", "n", whatsapp.SendPayload{TemplateID: stored.ID})
					require.Error(t, err)
				}
			}
		})
	}
}
