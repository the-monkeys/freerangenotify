package services

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"go.uber.org/zap"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type dispatchTransport struct{ target string }

func (r dispatchTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	copy := req.Clone(req.Context())
	copy.URL.Scheme = "http"
	copy.URL.Host = strings.TrimPrefix(r.target, "http://")
	return http.DefaultTransport.RoundTrip(copy)
}

func TestChannelDispatchRouting(t *testing.T) {
	for _, provider := range []string{"twilio", "meta"} {
		t.Run(provider, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if provider == "twilio" {
					a, b, _ := r.BasicAuth()
					if a != "customer" || b != "fixture-token" {
						t.Error("wrong customer credentials")
					}
					r.ParseForm()
					if r.Form.Get("From") != "whatsapp:+123" || r.Form.Get("ContentSid") != "HXfixture" {
						t.Errorf("wrong payload %v", r.Form)
					}
					fmt.Fprint(w, `{"sid":"SMfixture","status":"queued"}`)
				} else {
					if r.Header.Get("Authorization") != "Bearer fixture-token" || !strings.Contains(r.URL.Path, "/phone/messages") {
						t.Error("wrong Meta account")
					}
					fmt.Fprint(w, `{"messages":[{"id":"wamid.fixture"}]}`)
				}
			}))
			defer server.Close()
			connections := connectionTestService()
			wa := &application.WhatsAppAppConfig{Provider: provider, AccountSID: "customer", AuthToken: "fixture-token", FromNumber: "+123", MetaWABAID: "waba", MetaPhoneNumberID: "phone", MetaAccessToken: "fixture-token"}
			connections.Apps.(*connectionApps).app.Settings.WhatsApp = wa
			service := &ChannelDispatchService{Connections: connections, Client: &http.Client{Transport: dispatchTransport{server.URL}}, Logger: zap.NewNop()}
			n := &notification.Notification{AppID: "a", NotificationID: "n", Channel: notification.ChannelWhatsApp, Content: notification.Content{Body: "hello", Data: map[string]interface{}{}}}
			if provider == "twilio" {
				n.Content.Data["content_sid"] = "HXfixture"
			} else {
				n.Content.Data["whatsapp_template"] = map[string]interface{}{"name": "hello", "language": "en_US"}
			}
			d, err := service.PrepareChannelDelivery(context.Background(), n, &user.User{Phone: "+456"})
			if err != nil {
				t.Fatal(err)
			}
			wire, _ := json.Marshal(d.Snapshot)
			if strings.Contains(string(wire), "fixture-token") {
				t.Fatal("credentials serialized")
			}
			result, err := service.DispatchPrepared(context.Background(), d)
			if err != nil || !result.Success || calls != 1 {
				t.Fatalf("result %v err %v calls %d", result, err, calls)
			}
			wa.AccountSID = "other"
			wa.MetaWABAID = "other" // queued identity must fail closed
			if _, err = service.PrepareChannelDelivery(context.Background(), n, &user.User{Phone: "+456"}); err == nil {
				t.Fatal("queued account silently changed")
			}
		})
	}
}

func TestChannelDispatchRejectsMissingSenderAndCSW(t *testing.T) {
	s := connectionTestService()
	s.Apps.(*connectionApps).app.Settings.WhatsApp.FromNumber = ""
	service := &ChannelDispatchService{Connections: s, Logger: zap.NewNop()}
	n := &notification.Notification{AppID: "a", Channel: notification.ChannelWhatsApp, Content: notification.Content{Body: "text"}}
	if _, err := service.PrepareChannelDelivery(context.Background(), n, &user.User{Phone: "+456"}); err == nil {
		t.Fatal("missing sender accepted")
	}
	s.Apps.(*connectionApps).app.Settings.WhatsApp.FromNumber = "+123"
	if _, err := service.PrepareChannelDelivery(context.Background(), n, &user.User{Phone: "+456"}); err == nil {
		t.Fatal("closed window accepted free form")
	}
	s.Apps.(*connectionApps).app.Settings.WhatsApp.ConnectionStatus = "disconnected"
	n.Content.Data = map[string]interface{}{"content_sid": "HXfixture"}
	if _, err := service.PrepareChannelDelivery(context.Background(), n, &user.User{Phone: "+456"}); err == nil {
		t.Fatal("disconnected accepted")
	}
}

func TestChannelDispatchLostResponseNeverFallsBack(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; fmt.Fprint(w, `{}`) }))
	defer server.Close()
	s := &ChannelDispatchService{Connections: connectionTestService(), Client: &http.Client{Transport: dispatchTransport{server.URL}}, Logger: zap.NewNop()}
	n := &notification.Notification{AppID: "a", Channel: notification.ChannelWhatsApp, Content: notification.Content{Data: map[string]interface{}{"content_sid": "HXfixture"}}}
	d, err := s.PrepareChannelDelivery(context.Background(), n, &user.User{Phone: "+456"})
	if err != nil {
		t.Fatal(err)
	}
	if result, err := s.DispatchPrepared(context.Background(), d); err == nil || (result != nil && result.Success) {
		t.Fatal("malformed acceptance reported success")
	}
	if calls != 1 {
		t.Fatal("fallback sent again")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond)
	if _, err = s.DispatchPrepared(ctx, d); err == nil {
		t.Fatal("cancelled send accepted")
	}
	if calls != 1 {
		t.Fatal("cancelled send crossed vendor boundary")
	}
}

func TestChannelDispatchProviderTimestamp(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a, _, _ := r.BasicAuth()
		if a != "ACfixture" || r.URL.Path != "/2010-04-01/Accounts/ACfixture/Messages/SMfixture.json" {
			t.Error("foreign timestamp query")
		}
		fmt.Fprintf(w, `{"account_sid":"ACfixture","sid":"SMfixture","to":"whatsapp:+123","from":"whatsapp:+456","date_created":%q}`, time.Now().UTC().Format(time.RFC1123Z))
	}))
	defer server.Close()
	s := &ChannelDispatchService{Client: &http.Client{Transport: dispatchTransport{server.URL}}}
	binding := &InboundSenderBinding{Provider: "twilio", AccountID: "ACfixture", SenderID: "+123", SigningSecret: "fixture-token"}
	if _, err := s.ResolveTwilioInboundTimestamp(context.Background(), binding, "SMfixture", "+456"); err != nil {
		t.Fatal(err)
	}
	binding.SenderID = "+789"
	if _, err := s.ResolveTwilioInboundTimestamp(context.Background(), binding, "SMfixture", "+456"); err == nil {
		t.Fatal("wrong sender opened conversation window")
	}
}

func TestChannelDispatchSMSProviderSelection(t *testing.T) {
	for _, provider := range []string{"twilio", "vonage"} {
		t.Run(provider, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if provider == "twilio" {
					r.ParseForm()
					if r.Form.Get("To") != "+456" || r.Form.Get("Body") != "exact text" {
						t.Error("SMS body or plus-phone changed")
					}
					fmt.Fprint(w, `{"sid":"SMfixture"}`)
				} else {
					var body map[string]string
					json.NewDecoder(r.Body).Decode(&body)
					if body["text"] != "exact text" || body["api_key"] != "fixture-account" {
						t.Error("wrong Vonage payload")
					}
					fmt.Fprint(w, `{"messages":[{"status":"0","message-id":"fixture"}]}`)
				}
			}))
			defer server.Close()
			cs := connectionTestService()
			cs.System = map[notification.Channel]*channelconnection.Snapshot{notification.ChannelSMS: {Provider: provider, AccountID: "fixture-account", SenderID: "+123", Credentials: map[string]string{"auth_token": "fixture-token", "api_secret": "fixture-secret"}}}
			s := &ChannelDispatchService{Connections: cs, Client: &http.Client{Transport: dispatchTransport{server.URL}}}
			n := &notification.Notification{AppID: "a", Channel: notification.ChannelSMS, Content: notification.Content{Body: "exact text"}}
			d, err := s.PrepareChannelDelivery(context.Background(), n, &user.User{Phone: "+456"})
			if err != nil {
				t.Fatal(err)
			}
			r, err := s.DispatchPrepared(context.Background(), d)
			if err != nil || !r.Success {
				t.Fatalf("%v %v", r, err)
			}
		})
	}
}
