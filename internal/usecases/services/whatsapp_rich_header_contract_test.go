package services

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"go.uber.org/zap"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStoredMetaApprovedHeaderWire(t *testing.T) {
	for _, kind := range []whatsapp.RichTemplateKind{whatsapp.KindCTAURL, whatsapp.KindQuickReply} {
		for _, tc := range []struct {
			name      string
			header    *whatsapp.Header
			parameter map[string]interface{}
		}{
			{"text", &whatsapp.Header{Text: "Hello {{1}}"}, map[string]interface{}{"type": "text", "text": "Alice"}},
			{"image", &whatsapp.Header{ImageURL: "https://cdn.example.test/header.png"}, map[string]interface{}{"type": "image", "image": map[string]interface{}{"link": "https://cdn.example.test/header.png"}}},
			{"video", &whatsapp.Header{VideoURL: "https://cdn.example.test/header.mp4"}, map[string]interface{}{"type": "video", "video": map[string]interface{}{"link": "https://cdn.example.test/header.mp4"}}},
			{"document", &whatsapp.Header{DocumentURL: "https://cdn.example.test/header.pdf"}, map[string]interface{}{"type": "document", "document": map[string]interface{}{"link": "https://cdn.example.test/header.pdf"}}},
		} {
			t.Run(string(kind)+"/"+tc.name, func(t *testing.T) {
				var wire map[string]interface{}
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					require.NoError(t, json.NewDecoder(r.Body).Decode(&wire))
					fmt.Fprint(w, `{"messages":[{"id":"wamid.fixture"}]}`)
				}))
				defer server.Close()
				apps := &richStubAppRepo{app: &application.Application{AppID: "app-1", Settings: application.Settings{WhatsApp: &application.WhatsAppAppConfig{Provider: "meta", MetaWABAID: "waba", MetaPhoneNumberID: "phone", MetaAccessToken: "fixture-token"}}}}
				repo := newMemRichRepo()
				client := &http.Client{Transport: dispatchTransport{server.URL}}
				rich := NewWhatsAppRichTemplateService(repo, apps, client, "v23.0", "fixture", "fixture", zap.NewNop())
				tpl := &whatsapp.RichTemplate{ID: "approved", AppID: "app-1", Kind: kind, Name: "approved", Language: "en_US", Body: "Body {{1}}", Header: tc.header, Providers: whatsapp.ProviderBindings{Meta: &whatsapp.MetaBinding{TemplateName: "approved", Status: "APPROVED"}}}
				if kind == whatsapp.KindCTAURL {
					tpl.Buttons = []whatsapp.Button{{Type: whatsapp.ButtonURL, Text: "Open", URL: "https://example.test/open"}}
				} else {
					tpl.Buttons = []whatsapp.Button{{Type: whatsapp.ButtonQuickReply, Text: "Yes", Payload: "yes"}}
				}
				require.NoError(t, repo.Create(context.Background(), tpl))
				connections := connectionTestService()
				connections.Apps = apps
				dispatch := &ChannelDispatchService{Connections: connections, Client: client, RichTemplates: rich, Logger: zap.NewNop(), GraphVersion: "v23.0"}
				n := &notification.Notification{NotificationID: "n", AppID: "app-1", Channel: notification.ChannelWhatsApp, Content: notification.Content{Data: map[string]interface{}{"whatsapp_rich": map[string]interface{}{"template_id": "approved", "variables": map[string]interface{}{"1": "Alice", "2": "World"}}}}}
				prepared, e := dispatch.PrepareChannelDelivery(context.Background(), n, &user.User{Phone: "+14155551234"})
				require.NoError(t, e)
				result, e := dispatch.DispatchPrepared(context.Background(), prepared)
				require.NoError(t, e)
				require.True(t, result.Success)
				components := wire["template"].(map[string]interface{})["components"].([]interface{})
				var header map[string]interface{}
				for _, component := range components {
					c := component.(map[string]interface{})
					if c["type"] == "header" {
						header = c
					}
				}
				require.NotNil(t, header, "approved header was dropped")
				require.Equal(t, []interface{}{tc.parameter}, header["parameters"])
			})
		}
	}
}
