package services

import (
	"context"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"io"
	"net/http"
	"strings"
	"testing"
)

type connectionValidationTransport func(*http.Request) (*http.Response, error)

func (f connectionValidationTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
func TestChannelConnectionOnboardingReadiness(t *testing.T) {
	for _, account := range []string{"first-customer", "second-customer"} {
		t.Run(account, func(t *testing.T) {
			client := &http.Client{Transport: connectionValidationTransport(func(r *http.Request) (*http.Response, error) {
				if r.Method != "GET" {
					t.Fatal("validation sent a message or mutated provider")
				}
				body := `{"data":[]}`
				switch {
				case strings.HasSuffix(r.URL.Path, "/phone_numbers"):
					body = `{"data":[{"id":"owned","code_verification_status":"VERIFIED","platform_type":"CLOUD_API","status":"CONNECTED"}]}`
				case strings.HasSuffix(r.URL.Path, "/debug_token"):
					body = `{"data":{"is_valid":true,"app_id":"customer-app","scopes":["whatsapp_business_messaging","whatsapp_business_management"]}}`
				case strings.HasSuffix(r.URL.Path, "/subscribed_apps"):
					body = `{"data":[{"whatsapp_business_api_data":{"id":"customer-app"}}]}`
				case strings.HasSuffix(r.URL.Path, "/subscriptions"):
					body = `{"data":[{"object":"whatsapp_business_account","callback_url":"https://notify.example.test/v1/webhooks/meta/whatsapp","fields":[{"name":"messages"},{"name":"message_template_status_update"}]}]}`
				case strings.HasSuffix(r.URL.Path, "/owned"):
					body = `{"id":"owned","code_verification_status":"VERIFIED","platform_type":"CLOUD_API","status":"CONNECTED"}`
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
			})}
			v := &ConnectionValidator{Client: client, GraphVersion: "v23.0", PublicURL: "https://notify.example.test"}
			snap := &channelconnection.Snapshot{AppID: account, Provider: "meta", AccountID: account, SenderID: "owned", CredentialSource: "byoc", Credentials: map[string]string{"access_token": "fixture-" + account, "app_secret": "fixture-signing"}}
			state, facts, err := v.Validate(context.Background(), snap)
			if err != nil || state != "ready" || facts["sender_owned"] != "yes" {
				t.Fatalf("independent customer not ready: %s %#v %v", state, facts, err)
			}
			snap.SenderID = "foreign"
			state, _, err = v.Validate(context.Background(), snap)
			if err != nil || state != "invalid" {
				t.Fatal("foreign sender accepted", state, err)
			}
		})
	}
}
func TestChannelConnectionOnboardingWrongCallbackPending(t *testing.T) {
	client := &http.Client{Transport: connectionValidationTransport(func(r *http.Request) (*http.Response, error) {
		body := `{"data":[]}`
		switch {
		case strings.HasSuffix(r.URL.Path, "/phone_numbers"):
			body = `{"data":[{"id":"owned","code_verification_status":"VERIFIED","platform_type":"CLOUD_API","status":"CONNECTED"}]}`
		case strings.HasSuffix(r.URL.Path, "/debug_token"):
			body = `{"data":{"is_valid":true,"app_id":"customer-app","scopes":["whatsapp_business_messaging","whatsapp_business_management"]}}`
		case strings.HasSuffix(r.URL.Path, "/subscribed_apps"):
			body = `{"data":[{"whatsapp_business_api_data":{"id":"customer-app"}}]}`
		case strings.HasSuffix(r.URL.Path, "/subscriptions"):
			body = `{"data":[{"object":"whatsapp_business_account","callback_url":"https://other.example.test"}]}`
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	v := &ConnectionValidator{Client: client, PublicURL: "https://notify.example.test"}
	state, _, err := v.Validate(context.Background(), &channelconnection.Snapshot{Provider: "meta", AccountID: "waba", SenderID: "owned", Credentials: map[string]string{"access_token": "fixture-token", "app_secret": "fixture-signing"}})
	if err != nil || state != "pending" {
		t.Fatalf("wrong callback reported ready: %s %v", state, err)
	}
}
func TestChannelConnectionOnboardingPhoneChoices(t *testing.T) {
	client := &http.Client{Transport: connectionValidationTransport(func(r *http.Request) (*http.Response, error) {
		body := `{"data":[{"id":"first"}],"paging":{"next":"https://evil.example.test","cursors":{"after":"next-page"}}}`
		if r.URL.Query().Get("after") == "next-page" {
			body = `{"data":[{"id":"second"}]}`
		}
		if r.URL.Host != "graph.facebook.com" {
			t.Fatal("vendor next URL leaked credentials")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})}
	v := &ConnectionValidator{Client: client}
	state, facts, err := v.Validate(context.Background(), &channelconnection.Snapshot{Provider: "meta", AccountID: "waba", Credentials: map[string]string{"access_token": "fixture"}})
	if err != nil || state != "pending" || !strings.Contains(facts["available_phone_numbers"], "second") {
		t.Fatalf("complete phone selection missing: %s %#v %v", state, facts, err)
	}
}
func TestChannelConnectionOnboardingTwilioV2(t *testing.T) {
	client := &http.Client{Transport: connectionValidationTransport(func(r *http.Request) (*http.Response, error) {
		status := 200
		body := `{"sid":"ACfixture","status":"active","type":"Full"}`
		if r.URL.Host == "messaging.twilio.com" {
			if r.URL.Query().Get("Channel") != "whatsapp" {
				status = 400
			}
			body = `{"senders":[{"sender_id":"whatsapp:+123","status":"ONLINE","webhook":{"callback_url":"https://notify.example.test/v1/webhooks/twilio/whatsapp"}}],"meta":{"next_page_url":null}}`
		}
		return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	v := &ConnectionValidator{Client: client, PublicURL: "https://notify.example.test"}
	state, _, err := v.Validate(context.Background(), &channelconnection.Snapshot{Provider: "twilio", Channel: "whatsapp", AccountID: "ACfixture", SenderID: "+123", Credentials: map[string]string{"auth_token": "fixture"}})
	if err != nil || state != "ready" {
		t.Fatalf("Twilio v2 required channel filter missing: %s %v", state, err)
	}
}
