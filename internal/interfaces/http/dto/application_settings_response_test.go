package dto

import (
	"encoding/json"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"strings"
	"testing"
)

func TestChannelSettingsSecrets(t *testing.T) {
	app := &application.Application{APIKey: "fixture-master-key", Settings: application.Settings{
		WhatsApp:        &application.WhatsAppAppConfig{AuthToken: "fixture-wa-token", MetaAccessToken: "fixture-meta-token"},
		SMS:             &application.SMSAppConfig{AuthToken: "fixture-sms-token"},
		EmailConfig:     &application.EmailConfig{SMTP: &application.SMTPConfig{Password: "fixture-smtp-password"}},
		CustomProviders: []application.CustomProviderConfig{{SigningKey: "fixture-signing", Headers: map[string]string{"Authorization": "fixture-header"}, WebhookURL: "https://example.com/fixture-url-token?q=fixture-query"}},
	}}
	out, _ := json.Marshal(ToApplicationResponse(app))
	for _, secret := range []string{"fixture-master-key", "fixture-wa-token", "fixture-meta-token", "fixture-sms-token", "fixture-smtp-password", "fixture-signing", "fixture-header", "fixture-url-token", "fixture-query"} {
		if strings.Contains(string(out), secret) {
			t.Errorf("ordinary response exposed %s", secret)
		}
	}
	if app.Settings.WhatsApp.AuthToken != "fixture-wa-token" || app.Settings.CustomProviders[0].Headers["Authorization"] != "fixture-header" {
		t.Fatal("response mutated stored credentials")
	}
}

func TestChannelSettingsSecretsProviderRoundTrip(t *testing.T) {
	old := application.Settings{CustomProviders: []application.CustomProviderConfig{{ProviderID: "p", Name: "fixture", SigningKey: "fixture-key", Headers: map[string]string{"Authorization": "fixture-header"}, WebhookURL: "https://example.test/private?key=value"}}}
	next := ToSafeApplicationSettings(old).Settings
	merged, err := PreserveSettingsSecrets(old, next, nil)
	if err != nil || merged.CustomProviders[0].SigningKey != "fixture-key" || merged.CustomProviders[0].Headers["Authorization"] != "fixture-header" || merged.CustomProviders[0].WebhookURL != old.CustomProviders[0].WebhookURL {
		t.Fatal("safe response round-trip erased provider credentials")
	}
}
