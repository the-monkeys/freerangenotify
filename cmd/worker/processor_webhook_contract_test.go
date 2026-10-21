package main

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/providers"
	"go.uber.org/zap"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestTemplateFreeNamedWebhookDelivery(t *testing.T) {
	var selected, other atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/selected" {
			other.Add(1)
			w.WriteHeader(500)
			return
		}
		selected.Add(1)
		var body map[string]interface{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.Contains(t, body, "blocks")
		w.WriteHeader(200)
	}))
	defer server.Close()
	for _, prepared := range []bool{false, true} {
		t.Run(map[bool]string{false: "direct", true: "prepared"}[prepared], func(t *testing.T) {
			nrepo := newStubNotifRepo()
			n := &notification.Notification{NotificationID: "direct-hook", AppID: "app-1", Channel: notification.ChannelWebhook, Priority: notification.PriorityNormal, Status: notification.StatusQueued, Content: notification.Content{Title: "Plan", Body: "Choose", Data: map[string]interface{}{"webhook_target": "selected"}}, Metadata: map[string]interface{}{}}
			if prepared {
				n.Metadata["channel_content_prepared"] = true
			}
			nrepo.notifications[n.NotificationID] = n
			app := makeApp()
			app.Settings.CustomProviders = []application.CustomProviderConfig{{Name: "other", Channel: "webhook", Kind: "discord", Active: true, WebhookURL: server.URL + "/other"}, {Name: "selected", Channel: "webhook", Kind: "slack", Active: true, WebhookURL: server.URL + "/selected"}}
			proc := newProcessorForTest(nrepo, nil, nil, map[string]*application.Application{"app-1": app}, nil, nil)
			proc.providerManager = providers.NewManager(nil, nil, zap.NewNop())
			before := selected.Load()
			proc.processNotification(context.Background(), makeQueueItem(n.NotificationID), zap.NewNop())
			require.Equal(t, before+1, selected.Load())
			require.Zero(t, other.Load())
			require.Equal(t, "selected", n.Metadata["custom_provider_name"])
			require.Equal(t, notification.StatusSent, nrepo.statusUpdates[n.NotificationID])
		})
	}
}
