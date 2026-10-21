package providers

import (
	"context"
	"fmt"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"go.uber.org/zap"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func exotelFixtureSnapshot() *channelconnection.Snapshot {
	return &channelconnection.Snapshot{AppID: "a", EnvironmentID: "default", ConnectionID: "c", Revision: 1, Provider: "exotel", Channel: notification.ChannelSMS, CredentialSource: "byoc", AccountID: "account", SenderID: "FRNAPP", Credentials: map[string]string{"api_key": "fixture-key", "api_token": "fixture-token"}, Exotel: &channelconnection.DomesticSMSConfig{Region: "mumbai", EntityID: "1234567890123456789", SMSType: "transactional", ApprovalReference: "https://example.test/approval", ApprovedAt: "2026-10-01T00:00:00Z", Templates: []channelconnection.DLTTemplate{{ID: "1234567890123456780", Body: "Hello {#var#} from FRN", VariableMaxLength: 30}}, Limits: channelconnection.DeliveryLimits{MessagesPerMinute: 100, UnitsPerMinute: 300, DailyUnits: 10000, DailySpendMinor: 100000, CostMinorPerUnit: 10, Currency: "INR", MaxSegments: 3, RateReference: "https://example.test/rate", RateVerifiedAt: "2026-10-01T00:00:00Z"}}}
}

type exotelFixtureTransport struct{ target string }

func (t exotelFixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	copy := r.Clone(r.Context())
	copy.URL.Scheme = "http"
	copy.URL.Host = strings.TrimPrefix(t.target, "http://")
	return http.DefaultTransport.RoundTrip(copy)
}

func TestExotelDelivery(t *testing.T) {
	for _, tc := range []struct {
		name              string
		http              int
		response          string
		success, rejected bool
	}{{"accepted", 200, `{"SMSMessage":{"Sid":"sms-1","AccountSid":"account","Status":"queued"}}`, true, false}, {"auth", 401, `{"message":"fixture-token"}`, false, true}, {"throttle", 429, `{}`, false, true}, {"unknown", 500, `{}`, false, false}, {"malformed", 200, `{}`, false, false}} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				key, token, _ := r.BasicAuth()
				r.ParseForm()
				if key != "fixture-key" || token != "fixture-token" || r.URL.Path != "/v1/Accounts/account/Sms/send.json" || r.Form.Get("To") != "+919876543210" || r.Form.Get("DltEntityId") != "1234567890123456789" || r.Form.Get("DltTemplateId") != "1234567890123456780" || r.Form.Get("Body") != "Hello Asha from FRN" {
					t.Error("wrong Exotel account, DLT binding or encoded payload")
				}
				w.WriteHeader(tc.http)
				fmt.Fprint(w, tc.response)
			}))
			defer server.Close()
			p, e := NewExotelProvider(ExotelConfig{Config: Config{HTTPClient: &http.Client{Transport: exotelFixtureTransport{server.URL}}}, Snapshot: exotelFixtureSnapshot(), StatusCallback: "https://callback.test/signed"}, zap.NewNop())
			if e != nil {
				t.Fatal(e)
			}
			result, e := p.Send(context.Background(), &notification.Notification{NotificationID: "n", Content: notification.Content{Body: "Hello Asha from FRN"}}, &user.User{Phone: "+919876543210"})
			if e != nil {
				t.Fatal(e)
			}
			if result.Success != tc.success || calls != 1 || result.Metadata["acceptance_rejected"] == true != tc.rejected {
				t.Fatalf("outcome %+v", result)
			}
			if result.Error != nil && strings.Contains(result.Error.Error(), "fixture-token") {
				t.Error("provider response leaked")
			}
			if tc.success && result.ProviderMessageID != "sms-1" {
				t.Fatal("lost real provider ID")
			}
		})
	}
}

func TestExotelDeliveryRejectsBeforeVendor(t *testing.T) {
	for _, tc := range []struct{ name, body, phone string }{{"mismatch", "Hello Asha from Other", "+919876543210"}, {"long variable", "Hello " + strings.Repeat("a", 31) + " from FRN", "+919876543210"}, {"international", "Hello Asha from FRN", "+14155551234"}} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
			defer server.Close()
			p, e := NewExotelProvider(ExotelConfig{Config: Config{HTTPClient: &http.Client{Transport: exotelFixtureTransport{server.URL}}}, Snapshot: exotelFixtureSnapshot()}, zap.NewNop())
			if e != nil {
				t.Fatal(e)
			}
			result, _ := p.Send(context.Background(), &notification.Notification{Content: notification.Content{Body: tc.body}}, &user.User{Phone: tc.phone})
			if result.Success || calls != 0 {
				t.Fatal("invalid DLT body/destination reached vendor")
			}
		})
	}
}

func TestExotelDeliveryNetworkUnknown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.(http.Hijacker)
		conn, _, e := h.Hijack()
		if e == nil {
			conn.Close()
		}
	}))
	defer server.Close()
	p, e := NewExotelProvider(ExotelConfig{Config: Config{HTTPClient: &http.Client{Transport: exotelFixtureTransport{server.URL}}}, Snapshot: exotelFixtureSnapshot()}, zap.NewNop())
	if e != nil {
		t.Fatal(e)
	}
	result, e := p.Send(context.Background(), &notification.Notification{Content: notification.Content{Body: "Hello Asha from FRN"}}, &user.User{Phone: "+919876543210"})
	if e != nil || result.Success || result.ErrorType != ErrorTypeNetwork || result.Metadata["acceptance_rejected"] == true {
		t.Fatal("network loss must remain unknown")
	}
}

func TestExotelUnsendableMessage(t *testing.T) {
	for _, which := range []string{"rolling units", "daily units", "daily spend"} {
		t.Run(which, func(t *testing.T) {
			snap := exotelFixtureSnapshot()
			snap.Exotel.Templates[0].VariableMaxLength = 200
			switch which {
			case "rolling units":
				snap.Exotel.Limits.UnitsPerMinute = 1
			case "daily units":
				snap.Exotel.Limits.DailyUnits = 1
			case "daily spend":
				snap.Exotel.Limits.DailySpendMinor = 15
			}
			_, _, e := ValidateExotelMessage(snap, &notification.Notification{Content: notification.Content{Body: "Hello " + strings.Repeat("a", 160) + " from FRN"}}, &user.User{Phone: "+919876543210"})
			if e == nil {
				t.Fatal("message that cannot ever fit sender policy was admitted")
			}
		})
	}
}

func TestExotelMultipartPackingSegmentCap(t *testing.T) {
	snap := exotelFixtureSnapshot()
	body := strings.Repeat("^", 153)
	snap.Exotel.Templates[0].Body = body
	snap.Exotel.Limits.MaxSegments = 2
	if _, _, e := ValidateExotelMessage(snap, &notification.Notification{Content: notification.Content{Body: body}}, &user.User{Phone: "+919876543210"}); e == nil {
		t.Fatal("three actual SMS segments passed a two-segment cap")
	}
}
