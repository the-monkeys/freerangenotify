package services

import (
	"context"
	"fmt"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"net/url"
	"regexp"
	"strings"
)

func (s *ChannelQuoteService) lookupProviderCategory(ctx context.Context, d *channelconnection.PreparedDelivery) (string, error) {
	snap := d.Snapshot
	validator := &ConnectionValidator{Client: s.Client, GraphVersion: s.GraphVersion}
	if snap.Provider == "twilio" {
		sid, _ := d.Notification.Content.Data["content_sid"].(string)
		if !regexp.MustCompile(`^HX[A-Za-z0-9]+$`).MatchString(sid) {
			return "", fmt.Errorf("Content SID required for template price")
		}
		var out struct {
			Account  string `json:"account_sid"`
			WhatsApp struct {
				Status   string `json:"status"`
				Category string `json:"category"`
			} `json:"whatsapp"`
		}
		if err := validator.get(ctx, "https://content.twilio.com/v1/Content/"+sid+"/ApprovalRequests", snap.Credentials["auth_token"], snap.AccountID, &out); err != nil {
			return "", err
		}
		if out.Account != "" && out.Account != snap.AccountID || !strings.EqualFold(out.WhatsApp.Status, "approved") {
			return "", fmt.Errorf("template approval unconfirmed")
		}
		return out.WhatsApp.Category, nil
	}
	if snap.Provider != "meta" {
		return "", fmt.Errorf("template category unavailable")
	}
	name, language := "", "en_US"
	if raw, ok := d.Notification.Content.Data["whatsapp_template"].(map[string]interface{}); ok {
		name, _ = raw["name"].(string)
		if lang, ok := raw["language"].(string); ok && lang != "" {
			language = lang
		}
	}
	if raw, ok := d.Notification.Content.Data["whatsapp_rich"].(map[string]interface{}); ok {
		name, _ = raw["template_name"].(string)
		if lang, ok := raw["language"].(string); ok && lang != "" {
			language = lang
		}
	}
	if name == "" {
		return "", fmt.Errorf("template name required for price")
	}
	version := s.GraphVersion
	if version == "" {
		version = "v23.0"
	}
	endpoint := "https://graph.facebook.com/" + version + "/" + url.PathEscape(snap.AccountID) + "/message_templates?" + url.Values{"name": {name}, "fields": {"name,language,category,status"}}.Encode()
	var out struct {
		Data []struct {
			Name     string `json:"name"`
			Language string `json:"language"`
			Status   string `json:"status"`
			Category string `json:"category"`
		} `json:"data"`
	}
	if err := validator.get(ctx, endpoint, snap.Credentials["access_token"], "", &out); err != nil {
		return "", err
	}
	found := ""
	for _, row := range out.Data {
		if row.Name == name && row.Language == language && strings.EqualFold(row.Status, "approved") {
			if found != "" {
				return "", fmt.Errorf("ambiguous template category")
			}
			found = row.Category
		}
	}
	if found == "" {
		return "", fmt.Errorf("template approval unconfirmed")
	}
	return found, nil
}
