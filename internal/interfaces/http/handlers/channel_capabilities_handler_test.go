package handlers

import (
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/config"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

// Tests the HTTP boundary: runtime public IDs work without frontend build keys;
// neither secrets nor customer sender/account details may enter this response.
func TestChannelCapabilities(t *testing.T) {
	c := &config.Config{}
	c.Features.WhatsAppMetaEnabled = true
	c.Providers.MetaWhatsApp = config.MetaWhatsAppProviderConfig{MetaAppID: "public-app", MetaAppSecret: "fixture-oauth", AppSecret: "fixture-signing", AccessToken: "fixture-access", WABAID: "customer-waba", PhoneNumberID: "customer-phone", APIVersion: "v24.0"}
	c.Messaging.MetaSignupConfigID = "public-signup"
	app := fiber.New()
	app.Get("/v1/channel-capabilities", (&ChannelCapabilitiesHandler{Config: c}).Get)
	res, err := app.Test(httptest.NewRequest("GET", "/v1/channel-capabilities", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	for _, secret := range []string{"fixture-", "customer-", "auth_token", "access_token", "app_secret"} {
		if strings.Contains(string(b), secret) {
			t.Fatalf("public response leaked %s", secret)
		}
	}
	var body map[string]interface{}
	if json.Unmarshal(b, &body) != nil || res.StatusCode != 200 {
		t.Fatalf("bad response: %s", b)
	}
	data := body["data"].(map[string]interface{})
	meta := data["meta"].(map[string]interface{})
	if meta["app_id"] != "public-app" || meta["signup_config_id"] != "public-signup" || meta["graph_version"] != "v24.0" || meta["manual"] != true || meta["embedded"] != true {
		t.Fatalf("runtime capabilities: %s", b)
	}
	c.Features.WhatsAppMetaEnabled = false
	res, err = app.Test(httptest.NewRequest("GET", "/v1/channel-capabilities", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ = io.ReadAll(res.Body)
	_ = json.Unmarshal(b, &body)
	meta = body["data"].(map[string]interface{})["meta"].(map[string]interface{})
	if meta["manual"] != false || meta["embedded"] != false {
		t.Fatal("disabled onboarding advertised")
	}
}
