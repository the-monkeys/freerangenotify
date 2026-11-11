package container

import (
	"bytes"
	"context"
	"encoding/base64"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/viper"
	"github.com/the-monkeys/freerangenotify/internal/config"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"go.uber.org/zap"
	"os"
	"testing"
)

func TestChannelLegacyGatesRealES(t *testing.T) {
	if os.Getenv("FRN_CHANNEL_INTEGRATION") != "1" {
		t.Skip("designated disposable infrastructure required")
	}
	if os.Getenv("FRN_CHANNEL_ES_URL") != "http://127.0.0.1:19219" || os.Getenv("FRN_PHONE_OTP_REDIS_ADDR") != "127.0.0.1:16388" {
		t.Fatal("wrong fixture")
	}
	for _, outbound := range []bool{true, false} {
		t.Run(map[bool]string{true: "outbound only", false: "onboarding only"}[outbound], func(t *testing.T) {
			registry := prometheus.DefaultRegisterer
			prometheus.DefaultRegisterer = prometheus.NewRegistry()
			defer func() { prometheus.DefaultRegisterer = registry }()
			viper.Reset()
			defer viper.Reset()
			cfg, err := config.Load()
			if err != nil {
				t.Fatal(err)
			}
			cfg.Database.URLs = []string{"http://127.0.0.1:19219"}
			cfg.Redis.Host = "127.0.0.1"
			cfg.Redis.Port = 16388
			cfg.Connection.WritesEnabled = true
			cfg.Connection.EncryptionKeys = "fixture:" + base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{21}, 32))
			cfg.Providers.MetaWhatsApp.Enabled = outbound
			cfg.Features.WhatsAppMetaEnabled = !outbound
			c, err := NewContainer(cfg, zap.NewNop())
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			id := uuid.NewString()
			ctx := context.Background()
			a := &application.Application{AppID: id, AdminUserID: uuid.NewString(), Settings: application.Settings{WhatsApp: &application.WhatsAppAppConfig{Provider: "meta", MetaWABAID: "waba", MetaPhoneNumberID: "phone", MetaAccessToken: "fixture-token", MetaAppSecret: "fixture-secret"}}}
			if err = c.ChannelConnections.Apps.Create(ctx, a); err != nil {
				t.Fatal(err)
			}
			if outbound {
				provider := "meta"
				_, err = c.ChannelConnections.UpdateConnection(ctx, channelconnection.ID(id, "default", "whatsapp"), 0, channelconnection.Patch{AppID: id, Channel: "whatsapp", Provider: &provider})
				if err == nil {
					t.Fatal("legacy onboarding off admitted a new connection")
				}
			}
			if !outbound {
				n := &notification.Notification{AppID: id, NotificationID: uuid.NewString(), Channel: "whatsapp", Content: notification.Content{Body: "hello", Data: map[string]interface{}{"whatsapp_template": map[string]interface{}{"name": "hello", "language": "en_US"}}}}
				_, err = c.ChannelDispatch.PrepareChannelDelivery(ctx, n, &user.User{Phone: "+456"})
				if err == nil {
					t.Fatal("legacy delivery off admitted a send")
				}
			}
		})
	}
}
