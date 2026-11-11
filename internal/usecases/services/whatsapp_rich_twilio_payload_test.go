package services

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
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"go.uber.org/zap"
)

func TestTwilioRichAuthoringSchema(t *testing.T) {
	for _, kind := range []whatsapp.RichTemplateKind{whatsapp.KindCarousel, whatsapp.KindCouponCode} {
		t.Run(string(kind), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				account, token, _ := r.BasicAuth()
				if account != "fixture-account" || token != "fixture-token" {
					http.Error(w, "wrong account", 401)
					return
				}
				if strings.HasSuffix(r.URL.Path, "/ApprovalRequests/whatsapp") {
					fmt.Fprint(w, `{"sid":"HAfixture","status":"pending"}`)
					return
				}
				var payload map[string]interface{}
				if json.NewDecoder(r.Body).Decode(&payload) != nil {
					http.Error(w, "invalid JSON", 400)
					return
				}
				types := payload["types"].(map[string]interface{})
				if kind == whatsapp.KindCarousel {
					carousel := types["twilio/carousel"].(map[string]interface{})
					cards := carousel["cards"].([]interface{})
					for _, raw := range cards {
						card := raw.(map[string]interface{})
						body, _ := card["body"].(string)
						media, _ := card["media"].(string)
						if body == "" || media == "" {
							http.Error(w, "card body and string media required", 400)
							return
						}
					}
				} else {
					action := types["twilio/call-to-action"].(map[string]interface{})["actions"].([]interface{})[0].(map[string]interface{})
					if action["type"] != "COPY_CODE" || action["code"] != "DEAL50" {
						http.Error(w, "COPY_CODE with code required", 400)
						return
					}
				}
				fmt.Fprint(w, `{"sid":"HXfixture"}`)
			}))
			defer server.Close()
			service := NewWhatsAppRichTemplateService(newMemRichRepo(), nil, &http.Client{Transport: dispatchTransport{server.URL}}, "v23.0", "", "", zap.NewNop()).(*whatsappRichTemplateService)
			tpl := &whatsapp.RichTemplate{Name: "contract", Kind: kind, Body: "Choose an offer", Language: "en_US", Category: "MARKETING", CouponCode: "DEAL50"}
			if kind == whatsapp.KindCarousel {
				for i := 0; i < 2; i++ {
					tpl.Cards = append(tpl.Cards, whatsapp.CarouselCard{Body: "Offer", HeaderImageURL: "https://cdn.example/image.png", Buttons: []whatsapp.Button{{Type: whatsapp.ButtonURL, Text: "Open", URL: "https://example.test/open"}}})
				}
			}
			before, err := json.Marshal(tpl)
			require.NoError(t, err)
			binding, err := service.submitToTwilio(context.Background(), tpl, &application.WhatsAppAppConfig{AccountSID: "fixture-account", AuthToken: "fixture-token"})
			require.NoError(t, err)
			require.Equal(t, "HXfixture", binding.ContentSid)
			require.Equal(t, 2, calls)
			after, err := json.Marshal(tpl)
			require.NoError(t, err)
			require.JSONEq(t, string(before), string(after), "authoring model must remain unchanged")
		})
	}
}
