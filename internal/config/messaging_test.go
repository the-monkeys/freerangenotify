package config

import (
	"github.com/spf13/viper"
	"os"
	"strings"
	"testing"
)

// Catches env-only fields being ignored, canonical flags changing independent
// legacy gates, and shared credentials overriding explicit channel accounts.
func TestChannelEnvironmentCompatibility(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		env                  map[string]string
		delivery, onboarding bool
		sms, wa, version     string
		invalid              bool
	}{
		{name: "legacy outbound only", env: map[string]string{"PROVIDERS_META_WHATSAPP_ENABLED": "true"}, delivery: true, version: "v23.0"},
		{name: "legacy onboarding only", env: map[string]string{"FEATURES_WHATSAPP_META_ENABLED": "true"}, onboarding: true, version: "v23.0"},
		{name: "canonical on", env: map[string]string{"MESSAGING_META_ENABLED": "true", "MESSAGING_META_GRAPH_VERSION": "v24.0"}, delivery: true, onboarding: true, version: "v24.0"},
		{name: "explicit off with stale credentials", env: map[string]string{"MESSAGING_META_ENABLED": "false", "PROVIDERS_META_WHATSAPP_ENABLED": "true", "FEATURES_WHATSAPP_META_ENABLED": "true", "PROVIDERS_META_WHATSAPP_ACCESS_TOKEN": "fixture-token"}, version: "v23.0"},
		{name: "shared twilio", env: map[string]string{"MESSAGING_TWILIO_ACCOUNT_SID": "shared", "MESSAGING_TWILIO_AUTH_TOKEN": "fixture-shared"}, sms: "shared", wa: "shared", version: "v23.0"},
		{name: "separate accounts", env: map[string]string{"MESSAGING_TWILIO_ACCOUNT_SID": "shared", "MESSAGING_TWILIO_AUTH_TOKEN": "fixture-shared", "PROVIDERS_TWILIO_ACCOUNT_SID": "sms", "PROVIDERS_TWILIO_AUTH_TOKEN": "fixture-sms", "PROVIDERS_WHATSAPP_ACCOUNT_SID": "wa", "PROVIDERS_WHATSAPP_AUTH_TOKEN": "fixture-wa"}, sms: "sms", wa: "wa", version: "v23.0"},
		{name: "incomplete override never borrows foreign secret", env: map[string]string{"MESSAGING_TWILIO_ACCOUNT_SID": "shared", "MESSAGING_TWILIO_AUTH_TOKEN": "fixture-shared", "PROVIDERS_WHATSAPP_ACCOUNT_SID": "wa"}, sms: "shared", wa: "wa", version: "v23.0"},
		{name: "conflicting canonical secret", env: map[string]string{"MESSAGING_META_APP_SECRET": "fixture-new", "PROVIDERS_META_WHATSAPP_APP_SECRET": "fixture-old"}, invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			viper.Reset()
			t.Cleanup(viper.Reset)
			for _, e := range os.Environ() {
				k, _, _ := strings.Cut(e, "=")
				if strings.HasPrefix(k, "FREERANGE_") {
					t.Setenv(k, "")
				}
			}
			for k, v := range tc.env {
				t.Setenv("FREERANGE_"+k, v)
			}
			c, err := Load()
			if tc.invalid {
				if err == nil {
					t.Fatal("conflicting signing contexts accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if c.Providers.MetaWhatsApp.Enabled != tc.delivery || c.Features.WhatsAppMetaEnabled != tc.onboarding {
				t.Fatalf("delivery/onboarding = %v/%v", c.Providers.MetaWhatsApp.Enabled, c.Features.WhatsAppMetaEnabled)
			}
			if c.Providers.Twilio.AccountSID != tc.sms || c.Providers.WhatsApp.AccountSID != tc.wa || c.Providers.MetaWhatsApp.APIVersion != tc.version {
				t.Fatalf("account/version aliases not resolved")
			}
			if tc.name == "incomplete override never borrows foreign secret" && c.Providers.WhatsApp.AuthToken != "" {
				t.Fatal("borrowed secret from different account")
			}
		})
	}
	t.Run("different legacy signing contexts remain explicit", func(t *testing.T) {
		viper.Reset()
		defer viper.Reset()
		t.Setenv("FREERANGE_PROVIDERS_META_WHATSAPP_APP_SECRET", "fixture-signing")
		t.Setenv("FREERANGE_PROVIDERS_META_WHATSAPP_META_APP_SECRET", "fixture-oauth")
		c, err := Load()
		if err != nil {
			t.Fatal(err)
		}
		if c.Providers.MetaWhatsApp.AppSecret != "fixture-signing" || c.Providers.MetaWhatsApp.MetaAppSecret != "fixture-oauth" {
			t.Fatal("legacy contexts overwritten")
		}
	})
}
