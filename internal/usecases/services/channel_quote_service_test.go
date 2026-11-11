package services

import (
	"context"
	"fmt"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type quoteCard struct {
	billing.RateCardManager
	card *billing.RateCard
}

func TestChannelQuoteProviderCategoryBeatsClientFlags(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture-token" || !strings.Contains(r.URL.Path, "/account/message_templates") {
			t.Error("category used wrong account")
		}
		fmt.Fprint(w, `{"data":[{"name":"hello","language":"en_US","status":"APPROVED","category":"MARKETING"}]}`)
	}))
	defer server.Close()
	d := quoteFixture("meta", "byoc", "text")
	d.Snapshot.Channel = notification.ChannelWhatsApp
	d.Snapshot.Credentials = map[string]string{"access_token": "fixture-token"}
	d.Notification.Channel = notification.ChannelWhatsApp
	d.Notification.Content.Data["whatsapp_template"] = map[string]interface{}{"name": "hello", "language": "en_US", "category": "utility", "free": true}
	d.PayloadKind = "template"
	s := &ChannelQuoteService{Client: &http.Client{Transport: dispatchTransport{server.URL}}, Rates: &quoteCard{card: &billing.RateCard{Version: "fixture", PricingModel: "detailed-v1", DetailedRates: []billing.ChannelRate{{Channel: "whatsapp", Provider: "meta", CredentialSource: "byoc", DestinationPrefix: "+91", Country: "IN", Category: "marketing", CreditsPerUnit: 7, VendorCurrency: "INR"}, {Channel: "whatsapp", Provider: "meta", CredentialSource: "byoc", DestinationPrefix: "+91", Country: "IN", Category: "utility", CreditsPerUnit: 2, VendorCurrency: "INR"}}}}}
	q, err := s.QuoteChannelDelivery(context.Background(), d)
	if err != nil || q.Credits != 7 || q.Category != "marketing" {
		t.Fatalf("spoofed template category priced: %+v %v", q, err)
	}
}

func (r *quoteCard) GetActiveRateCard() *billing.RateCard { return r.card }
func quoteFixture(provider, source string, body string) *channelconnection.PreparedDelivery {
	return &channelconnection.PreparedDelivery{Snapshot: &channelconnection.Snapshot{AppID: "a", Channel: notification.ChannelSMS, ConnectionID: "c", EnvironmentID: "default", Revision: 3, Provider: provider, AccountID: "account", SenderID: "sender", CredentialSource: source}, Notification: &notification.Notification{AppID: "a", Channel: notification.ChannelSMS, NotificationID: "n", Content: notification.Content{Body: body, Data: map[string]interface{}{"credits_used": 0, "free": true, "category": "utility"}}}, User: &user.User{Phone: "+919876543210"}, PayloadKind: "text"}
}
func TestChannelQuoteFlatCompatibilityAndPin(t *testing.T) {
	card := &quoteCard{card: &billing.RateCard{Version: "old", ChannelCreditCost: map[string]int64{"sms": 80}}}
	svc := &ChannelQuoteService{Rates: card}
	d := quoteFixture("twilio", "byoc", strings.Repeat("a", 161))
	q, err := svc.QuoteChannelDelivery(context.Background(), d)
	if err != nil || q.Credits != 80 || q.Units != 2 || q.RateCardVersion != "old" {
		t.Fatalf("%+v %v", q, err)
	}
	card.card = &billing.RateCard{Version: "new", ChannelCreditCost: map[string]int64{"sms": 800}}
	again, err := svc.QuoteChannelDelivery(context.Background(), d)
	if err != nil || again.ID != q.ID || again.Credits != 80 {
		t.Fatal("quote drifted after refresh", again, err)
	}
	d.Snapshot.AccountID = "other"
	if _, err = svc.QuoteChannelDelivery(context.Background(), d); err == nil {
		t.Fatal("queued account quote reused on another account")
	}
}
func TestChannelQuoteDetailedDimensions(t *testing.T) {
	card := &billing.RateCard{Version: "fixture-detailed", PricingModel: "detailed-v1", DetailedRates: []billing.ChannelRate{
		{Channel: "sms", Provider: "twilio", CredentialSource: "byoc", DestinationPrefix: "+91", Country: "IN", Category: "*", CreditsPerUnit: 2, VendorCostMinorPerUnit: 10, VendorCurrency: "INR"},
		{Channel: "sms", Provider: "twilio", CredentialSource: "system", DestinationPrefix: "+91", Country: "IN", Category: "*", CreditsPerUnit: 8, VendorCostMinorPerUnit: 10, VendorCurrency: "INR"},
		{Channel: "sms", Provider: "twilio", CredentialSource: "byoc", DestinationPrefix: "+44", Country: "GB", Category: "*", CreditsPerUnit: 3, VendorCostMinorPerUnit: 20, VendorCurrency: "GBP"},
	}}
	svc := &ChannelQuoteService{Rates: &quoteCard{card: card}}
	for _, tc := range []struct {
		source, phone, country string
		credits                int64
	}{{"byoc", "+919876543210", "IN", 4}, {"system", "+919876543210", "IN", 16}, {"byoc", "+447700900123", "GB", 6}} {
		d := quoteFixture("twilio", tc.source, strings.Repeat("a", 161))
		d.User.Phone = tc.phone
		q, err := svc.QuoteChannelDelivery(context.Background(), d)
		if err != nil || q.Credits != tc.credits || q.Country != tc.country || q.EstimatedVendorCostMinor == 0 {
			t.Fatalf("%+v %v", q, err)
		}
	}
	d := quoteFixture("vonage", "byoc", "hello")
	if _, err := svc.QuoteChannelDelivery(context.Background(), d); err == nil {
		t.Fatal("unpriced provider silently used flat pricing")
	}
}
func TestChannelQuoteExactProviderBody(t *testing.T) {
	svc := &ChannelQuoteService{Rates: &quoteCard{card: &billing.RateCard{Version: "old", ChannelCreditCost: map[string]int64{"sms": 80}}}}
	d := quoteFixture("vonage", "system", strings.Repeat("a", 158))
	d.Notification.Content.Title = "x"
	q, err := svc.QuoteChannelDelivery(context.Background(), d)
	if err != nil || q.Units != 2 {
		t.Fatalf("must count title plus colon/space: %+v %v", q, err)
	}
	d.Notification.Content.Body = "changed"
	if _, err = svc.QuoteChannelDelivery(context.Background(), d); err == nil {
		t.Fatal("rendered payload drift accepted")
	}
}

func TestChannelQuoteLegacyPhoneCompatibility(t *testing.T) {
	svc := &ChannelQuoteService{Rates: &quoteCard{card: &billing.RateCard{Version: "old", ChannelCreditCost: map[string]int64{"sms": 80}}}}
	d := quoteFixture("twilio", "byoc", "hello")
	d.User.Phone = "919876543210"
	if _, err := svc.QuoteChannelDelivery(context.Background(), d); err != nil {
		t.Fatalf("flat legacy routing acquired a new country-pricing requirement: %v", err)
	}
}
