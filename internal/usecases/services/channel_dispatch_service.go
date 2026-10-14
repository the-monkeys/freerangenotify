package services

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/providers"
	"go.uber.org/zap"
	"net/http"
	"strings"
	"time"
)

type ChannelDispatchService struct {
	Quotes        *ChannelQuoteService
	Connections   *ChannelConnectionService
	Redis         *redis.Client
	Client        *http.Client
	Logger        *zap.Logger
	GraphVersion  string
	RichTemplates WhatsAppRichTemplateService
	// System-gateway sends retain the existing verified-owner requirement.
	VerifySystemOwner func(context.Context, string) error
}

func (s *ChannelDispatchService) PrepareChannelDelivery(ctx context.Context, n *notification.Notification, u *user.User) (*channelconnection.PreparedDelivery, error) {
	if n == nil || u == nil || u.Phone == "" {
		return nil, fmt.Errorf("messaging recipient phone required")
	}
	if n.Channel != notification.ChannelWhatsApp && n.Channel != notification.ChannelSMS {
		return nil, fmt.Errorf("unsupported messaging channel")
	}
	var snap *channelconnection.Snapshot
	var err error
	if raw, exists := n.Metadata[channelconnection.DeliveryMetadataKey]; exists {
		data, e := json.Marshal(raw)
		if e != nil {
			return nil, fmt.Errorf("invalid admitted connection")
		}
		var safe channelconnection.Snapshot
		if json.Unmarshal(data, &safe) != nil || safe.AppID != n.AppID || safe.Channel != n.Channel {
			return nil, fmt.Errorf("invalid admitted connection")
		}
		if safe.Revision > 0 {
			snap, err = s.Connections.ResolveRevision(ctx, safe.ConnectionID, safe.Revision)
		} else {
			snap, err = s.Connections.LegacySnapshot(ctx, n.AppID, n.EnvironmentID, n.Channel)
		}
		if err != nil {
			return nil, err
		}
		if snap.ConnectionID != safe.ConnectionID || snap.AppID != safe.AppID || snap.EnvironmentID != safe.EnvironmentID || snap.Channel != safe.Channel || snap.Provider != safe.Provider || snap.AccountID != safe.AccountID || strings.TrimPrefix(snap.SenderID, "whatsapp:") != strings.TrimPrefix(safe.SenderID, "whatsapp:") || snap.CredentialSource != safe.CredentialSource {
			return nil, fmt.Errorf("admitted connection identity changed; reconciliation required")
		}
	} else {
		snap, err = s.Connections.ResolveDeliveryConnection(ctx, n.AppID, n.EnvironmentID, n.Channel)
		if err != nil {
			return nil, err
		}
	}
	if snap.Revision > 0 && snap.Readiness != "ready" {
		return nil, fmt.Errorf("messaging connection is not ready")
	}
	if snap.AccountID == "" || snap.SenderID == "" {
		return nil, fmt.Errorf("messaging account and sender required")
	}
	switch snap.Provider {
	case "twilio":
		if snap.Credentials["auth_token"] == "" {
			return nil, fmt.Errorf("messaging credentials unavailable")
		}
	case "meta":
		if n.Channel != notification.ChannelWhatsApp || snap.Credentials["access_token"] == "" {
			return nil, fmt.Errorf("Meta connection unavailable")
		}
	case "vonage":
		if n.Channel != notification.ChannelSMS || snap.Credentials["api_secret"] == "" {
			return nil, fmt.Errorf("Vonage connection unavailable")
		}
	default:
		return nil, fmt.Errorf("unsupported messaging provider")
	}
	if snap.CredentialSource == "system" && s.VerifySystemOwner != nil {
		if err := s.VerifySystemOwner(ctx, n.AppID); err != nil {
			return nil, err
		}
	}
	kind := "text"
	if n.Channel == notification.ChannelWhatsApp {
		if raw, ok := n.Content.Data["whatsapp_rich"].(map[string]interface{}); ok {
			if id, _ := raw["template_id"].(string); id != "" {
				if s.RichTemplates == nil {
					return nil, fmt.Errorf("rich template resolver unavailable")
				}
				ctx = channelconnection.WithEnvironment(ctx, n.EnvironmentID)
				tpl, e := s.RichTemplates.Get(ctx, n.AppID, id)
				if e != nil {
					return nil, e
				}
				if tpl.ProviderAccountID != "" && (tpl.ProviderAccountID != snap.AccountID || tpl.ProviderType != snap.Provider) {
					return nil, fmt.Errorf("template belongs to another provider account")
				}
				var payload whatsapp.SendPayload
				b, e := json.Marshal(raw)
				if e != nil || json.Unmarshal(b, &payload) != nil {
					return nil, fmt.Errorf("invalid rich template payload")
				}
				var resolved map[string]interface{}
				if snap.Provider == "meta" {
					resolved, e = s.RichTemplates.ResolveSendPayload(ctx, n.AppID, n.NotificationID, payload)
				} else {
					resolved, e = s.RichTemplates.ResolveTwilioSendPayload(ctx, n.AppID, n.NotificationID, payload)
				}
				if e != nil {
					return nil, e
				}
				if snap.Provider == "meta" {
					n.Content.Data["whatsapp_rich"] = resolved
				} else {
					delete(n.Content.Data, "whatsapp_rich")
					for key, value := range resolved {
						n.Content.Data[key] = value
					}
				}
			}
		}
		kind, err = providers.WhatsAppPayloadKind(n, snap.Provider)
		if err != nil {
			return nil, err
		}

		if kind == "text" {
			if s.Redis == nil {
				return nil, fmt.Errorf("WhatsApp window unavailable; approved template required")
			}
			key := channelconnection.WindowKey(n.AppID, n.EnvironmentID, snap.Provider, snap.AccountID, snap.SenderID, u.Phone)
			if s.Redis.Exists(ctx, key).Val() != 1 {
				return nil, fmt.Errorf("WhatsApp window closed; approved template required")
			}
		}
	}
	if n.Metadata == nil {
		n.Metadata = map[string]interface{}{}
	}
	// JSON conversion deliberately drops private credentials.
	safe := *snap
	safe.Credentials = nil
	n.Metadata[channelconnection.DeliveryMetadataKey] = safe
	d := &channelconnection.PreparedDelivery{Snapshot: snap, PayloadKind: kind, Notification: n, User: u}
	if s.Quotes != nil {
		if _, err := s.Quotes.QuoteChannelDelivery(ctx, d); err != nil {
			return nil, err
		}
	}
	return d, nil
}

func (s *ChannelDispatchService) DispatchPrepared(ctx context.Context, d *channelconnection.PreparedDelivery) (*providers.Result, error) {
	if d == nil || d.Snapshot == nil || d.Notification == nil || d.User == nil {
		return nil, fmt.Errorf("delivery not prepared")
	}
	snap := d.Snapshot
	cfg := providers.Config{Timeout: 15 * time.Second, HTTPClient: s.Client}
	logger := s.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	var p providers.Provider
	var err error
	switch snap.Provider {
	case "twilio":
		if snap.Channel == notification.ChannelSMS {
			p, err = providers.NewTwilioProvider(providers.TwilioConfig{Config: cfg, AccountSID: snap.AccountID, AuthToken: snap.Credentials["auth_token"], FromNumber: snap.SenderID}, logger)
		} else {
			p, err = providers.NewWhatsAppProvider(providers.WhatsAppConfig{Config: cfg, AccountSID: snap.AccountID, AuthToken: snap.Credentials["auth_token"], FromNumber: snap.SenderID}, logger)
		}
	case "meta":
		p, err = providers.NewMetaWhatsAppProvider(providers.MetaWhatsAppConfig{Config: cfg, PhoneNumberID: snap.SenderID, AccessToken: snap.Credentials["access_token"], APIVersion: s.GraphVersion}, logger)
	case "vonage":
		p, err = providers.NewVonageProvider(providers.VonageConfig{Config: cfg, APIKey: snap.AccountID, APISecret: snap.Credentials["api_secret"], FromNumber: snap.SenderID}, logger)
	default:
		err = fmt.Errorf("unsupported messaging provider")
	}
	if err != nil {
		return nil, &providers.DeliveryError{Provider: snap.Provider, Stage: "configuration", ErrorType: providers.ErrorTypeConfiguration, CredentialSource: snap.CredentialSource, Err: err}
	}
	defer p.Close()
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, &providers.DeliveryError{Provider: snap.Provider, Stage: "send", ErrorType: providers.ErrorTypeNetwork, Err: err}
	}
	result, err := p.Send(ctx, d.Notification, d.User)
	if err == nil && result != nil && !result.Success {
		err = result.Error
	}
	if err == nil && (result == nil || !result.Success || result.ProviderMessageID == "") {
		err = fmt.Errorf("provider acceptance unconfirmed")
	}
	if err != nil {
		category := providers.ErrorTypeUnknown
		if result != nil {
			category = result.ErrorType
		}
		stage := "send"
		if result != nil && result.Metadata["acceptance_rejected"] == true {
			stage = "rejected"
		}
		return result, &providers.DeliveryError{Provider: p.GetName(), Stage: stage, CredentialSource: snap.CredentialSource, ErrorType: category, Retryable: category == providers.ErrorTypeRateLimit, Err: err}
	}
	result.Metadata["credential_source"] = snap.CredentialSource
	result.Metadata["provider"] = snap.Provider
	return result, nil
}
