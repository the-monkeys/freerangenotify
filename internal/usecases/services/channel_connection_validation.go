package services

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/providers"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type ConnectionValidator struct {
	ExotelEnabled                      bool
	Client                             *http.Client
	GraphVersion, MetaAppID, PublicURL string
}
type MetaPhone struct {
	ID                     string `json:"id"`
	DisplayPhoneNumber     string `json:"display_phone_number"`
	CodeVerificationStatus string `json:"code_verification_status"`
	PlatformType           string `json:"platform_type"`
	Status                 string `json:"status"`
	QualityRating          string `json:"quality_rating"`
}

func (v *ConnectionValidator) get(ctx context.Context, endpoint, token, account string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return fmt.Errorf("invalid provider request")
	}
	if account != "" {
		req.SetBasicAuth(account, token)
	} else {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := v.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	res, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("provider validation unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("provider validation returned %d", res.StatusCode)
	}
	if err = json.NewDecoder(io.LimitReader(res.Body, 2<<20)).Decode(out); err != nil {
		return fmt.Errorf("invalid provider validation response")
	}
	return nil
}
func (v *ConnectionValidator) version() string {
	if v.GraphVersion == "" {
		return "v23.0"
	}
	return v.GraphVersion
}

// Pagination uses only the cursor on the same trusted Graph endpoint; next URLs
// returned by a vendor are never followed with customer credentials.
func (v *ConnectionValidator) MetaPhones(ctx context.Context, waba, token string) ([]MetaPhone, error) {
	var phones []MetaPhone
	after := ""
	for page := 0; page < 100; page++ {
		endpoint := "https://graph.facebook.com/" + v.version() + "/" + url.PathEscape(waba) + "/phone_numbers?limit=100&fields=id,display_phone_number,quality_rating,code_verification_status,platform_type,status"
		if after != "" {
			endpoint += "&after=" + url.QueryEscape(after)
		}
		var out struct {
			Data   []MetaPhone `json:"data"`
			Paging struct {
				Next    string `json:"next"`
				Cursors struct {
					After string `json:"after"`
				} `json:"cursors"`
			} `json:"paging"`
		}
		if err := v.get(ctx, endpoint, token, "", &out); err != nil {
			return nil, err
		}
		phones = append(phones, out.Data...)
		if out.Paging.Next == "" {
			return phones, nil
		}
		if out.Paging.Cursors.After == "" || out.Paging.Cursors.After == after {
			return nil, fmt.Errorf("invalid provider pagination")
		}
		after = out.Paging.Cursors.After
	}
	return nil, fmt.Errorf("provider phone list exceeds safe limit")
}
func (v *ConnectionValidator) Validate(ctx context.Context, snap *channelconnection.Snapshot) (string, map[string]string, error) {
	facts := map[string]string{"account_access": "unknown", "sender_owned": "unknown", "sender_registered": "unknown", "callback_subscription": "unknown", "billing_status": "not_exposed"}
	if snap.AccountID == "" || snap.Provider != "meta" && snap.SenderID == "" {
		facts["reason"] = "Account and sender are required"
		return "invalid", facts, nil
	}
	if snap.Provider == "meta" {
		token := snap.Credentials["access_token"]
		if token == "" {
			facts["reason"] = "Access token required"
			return "invalid", facts, nil
		}
		phones, err := v.MetaPhones(ctx, snap.AccountID, token)
		if err != nil {
			return "invalid", facts, err
		}
		facts["account_access"] = "yes"
		choices, _ := json.Marshal(phones)
		facts["available_phone_numbers"] = string(choices)
		if snap.SenderID == "" {
			facts["reason"] = "Select a phone number from this WABA"
			return "pending", facts, nil
		}
		var selected *MetaPhone
		for i := range phones {
			if phones[i].ID == snap.SenderID {
				selected = &phones[i]
				break
			}
		}
		if selected == nil {
			facts["sender_owned"] = "no"
			return "invalid", facts, nil
		}
		facts["sender_owned"] = "yes"
		facts["display_phone"] = selected.DisplayPhoneNumber
		facts["sender_registered"] = "no"
		if selected.CodeVerificationStatus == "VERIFIED" && selected.PlatformType == "CLOUD_API" && selected.Status == "CONNECTED" {
			facts["sender_registered"] = "yes"
		}
		var debug struct {
			Data struct {
				IsValid bool     `json:"is_valid"`
				AppID   string   `json:"app_id"`
				Scopes  []string `json:"scopes"`
			} `json:"data"`
		}
		if err = v.get(ctx, "https://graph.facebook.com/"+v.version()+"/debug_token?input_token="+url.QueryEscape(token), token, "", &debug); err != nil {
			return "pending", facts, err
		}
		facts["permissions"] = "no"
		messaging, management := false, false
		for _, scope := range debug.Data.Scopes {
			messaging = messaging || scope == "whatsapp_business_messaging"
			management = management || scope == "whatsapp_business_management"
		}
		if debug.Data.IsValid && messaging && management {
			facts["permissions"] = "yes"
		}
		facts["provider_app_id"] = debug.Data.AppID
		var subscriptions struct {
			Data []struct {
				App struct {
					ID string `json:"id"`
				} `json:"whatsapp_business_api_data"`
			} `json:"data"`
		}
		if err = v.get(ctx, "https://graph.facebook.com/"+v.version()+"/"+url.PathEscape(snap.AccountID)+"/subscribed_apps", token, "", &subscriptions); err != nil {
			return "pending", facts, err
		}
		facts["callback_subscription"] = "no"
		for _, sub := range subscriptions.Data {
			if sub.App.ID == debug.Data.AppID && debug.Data.AppID != "" {
				facts["callback_subscription"] = "yes"
			}
		}
		facts["callback_signing"] = "missing"
		if snap.Credentials["app_secret"] != "" {
			facts["callback_signing"] = "configured"
		}
		facts["callback_url"] = "unknown"
		if debug.Data.AppID != "" && snap.Credentials["app_secret"] != "" {
			var appSubscriptions struct {
				Data []struct {
					Object      string `json:"object"`
					CallbackURL string `json:"callback_url"`
					Fields      []struct {
						Name string `json:"name"`
					} `json:"fields"`
				} `json:"data"`
			}
			if e := v.get(ctx, "https://graph.facebook.com/"+v.version()+"/"+url.PathEscape(debug.Data.AppID)+"/subscriptions", debug.Data.AppID+"|"+snap.Credentials["app_secret"], "", &appSubscriptions); e == nil {
				facts["callback_url"] = "no"
				for _, sub := range appSubscriptions.Data {
					messages, templates := false, false
					for _, field := range sub.Fields {
						messages = messages || field.Name == "messages"
						templates = templates || field.Name == "message_template_status_update"
					}
					if sub.Object == "whatsapp_business_account" && sub.CallbackURL == strings.TrimRight(v.PublicURL, "/")+"/v1/webhooks/meta/whatsapp" && messages && templates {
						facts["callback_url"] = "yes"
					}
				}
			}
		}
		// Registration and subscription are necessary, but payment setup and actual
		// receipt of callbacks remain provider/operator checks, never inferred from a token.
		if facts["sender_registered"] == "yes" && facts["permissions"] == "yes" && facts["callback_subscription"] == "yes" && facts["callback_signing"] == "configured" && facts["callback_url"] == "yes" && v.PublicURL != "" {
			return "ready", facts, nil
		}
		return "pending", facts, nil
	}
	if snap.Provider == "exotel" {
		if !v.ExotelEnabled {
			facts["reason"] = "Domestic SMS is disabled for this deployment"
			return "pending", facts, nil
		}
		if snap.Exotel == nil || snap.Exotel.Validate() != nil || snap.CredentialSource != "byoc" || snap.Credentials["api_key"] == "" || snap.Credentials["api_token"] == "" {
			facts["reason"] = "Complete credentials, DLT approval and verified budgets required"
			return "invalid", facts, nil
		}
		if _, e := ExotelCallbackURL(v.PublicURL, snap, &notification.Notification{NotificationID: "readiness", AppID: snap.AppID}); e != nil {
			facts["reason"] = "Public HTTPS callback URL required"
			return "pending", facts, nil
		}
		if snap.Exotel.VerificationMessageID == "" {
			facts["reason"] = "Previously delivered SMS ID required for a read-only account and sender check"
			return "pending", facts, nil
		}
		msg, e := providers.ReadExotelSMS(ctx, v.Client, snap, snap.Exotel.VerificationMessageID)
		if e != nil {
			return "pending", facts, e
		}
		matched := false
		for _, t := range snap.Exotel.Templates {
			if _, e := snap.Exotel.Match(snap.SenderID, exotelDomesticPhone(msg.To), msg.Body, t.ID); e == nil {
				matched = true
				break
			}
		}
		if !matched || !exotelSenderMatches(msg.From, snap.SenderID) || msg.Status != "sent" {
			facts["reason"] = "Historical SMS must match this sender and an approved DLT body"
			return "invalid", facts, nil
		}
		facts["account_access"] = "yes"
		facts["sender_owned"] = "yes"
		facts["sender_registered"] = "historical_delivery_verified"
		facts["dlt_approval"] = "owner_confirmed"
		facts["provider_rate"] = "owner_verified"
		facts["callback_auth"] = "per_delivery_capability_and_authenticated_status_read"
		facts["reason"] = "Account and historical sender verified; DLT approval and current prices remain the account owner confirmation"
		return "ready", facts, nil
	}
	if snap.Provider != "twilio" {
		return "invalid", facts, fmt.Errorf("unsupported validation provider")
	}
	token := snap.Credentials["auth_token"]
	if token == "" {
		return "invalid", facts, nil
	}
	var account struct {
		SID    string `json:"sid"`
		Status string `json:"status"`
		Type   string `json:"type"`
	}
	if err := v.get(ctx, "https://api.twilio.com/2010-04-01/Accounts/"+url.PathEscape(snap.AccountID)+".json", token, snap.AccountID, &account); err != nil {
		return "invalid", facts, err
	}
	if account.SID != snap.AccountID || account.Status != "active" {
		return "invalid", facts, nil
	}
	facts["account_access"] = "yes"
	facts["account_type"] = account.Type
	if snap.Channel == "sms" {
		var numbers struct {
			Numbers []struct {
				PhoneNumber  string `json:"phone_number"`
				Capabilities struct {
					SMS bool `json:"sms"`
				} `json:"capabilities"`
			} `json:"incoming_phone_numbers"`
		}
		if err := v.get(ctx, "https://api.twilio.com/2010-04-01/Accounts/"+url.PathEscape(snap.AccountID)+"/IncomingPhoneNumbers.json?PhoneNumber="+url.QueryEscape(snap.SenderID), token, snap.AccountID, &numbers); err != nil {
			return "pending", facts, err
		}
		for _, n := range numbers.Numbers {
			if n.PhoneNumber == snap.SenderID {
				facts["sender_owned"] = "yes"
				if n.Capabilities.SMS {
					facts["sender_registered"] = "yes"
					return "ready", facts, nil
				}
			}
		}
		return "invalid", facts, nil
	}
	var senders struct {
		Meta struct {
			NextPageURL string `json:"next_page_url"`
		} `json:"meta"`
		Senders []struct {
			SenderID string `json:"sender_id"`
			Status   string `json:"status"`
			Webhook  struct {
				CallbackURL string `json:"callback_url"`
			} `json:"webhook"`
		} `json:"senders"`
	}
	endpoint := "https://messaging.twilio.com/v2/Channels/Senders?Channel=whatsapp&PageSize=1000"
	for page := 0; page < 100; page++ {
		if err := v.get(ctx, endpoint, token, snap.AccountID, &senders); err != nil {
			return "pending", facts, err
		}
		for _, sender := range senders.Senders {
			if strings.TrimPrefix(sender.SenderID, "whatsapp:") == strings.TrimPrefix(snap.SenderID, "whatsapp:") {
				senders.Meta.NextPageURL = ""
				break
			}
		}
		if senders.Meta.NextPageURL == "" {
			break
		}
		next, err := url.Parse(senders.Meta.NextPageURL)
		if err != nil || next.Scheme != "https" || next.Host != "messaging.twilio.com" || next.Path != "/v2/Channels/Senders" || next.User != nil {
			return "pending", facts, fmt.Errorf("invalid Twilio pagination")
		}
		if page == 99 {
			return "pending", facts, fmt.Errorf("Twilio sender list exceeds safe limit")
		}
		endpoint = next.String()
		senders.Senders = nil
		senders.Meta.NextPageURL = ""
	}
	for _, sender := range senders.Senders {
		if strings.TrimPrefix(sender.SenderID, "whatsapp:") != strings.TrimPrefix(snap.SenderID, "whatsapp:") {
			continue
		}
		facts["sender_owned"] = "yes"
		if sender.Status == "ONLINE" {
			facts["sender_registered"] = "yes"
		}
		if v.PublicURL != "" && sender.Webhook.CallbackURL == strings.TrimRight(v.PublicURL, "/")+"/v1/webhooks/twilio/whatsapp" {
			facts["callback_subscription"] = "yes"
		}
		if facts["sender_registered"] == "yes" && facts["callback_subscription"] == "yes" {
			return "ready", facts, nil
		}
		return "pending", facts, nil
	}
	return "invalid", facts, nil
}
