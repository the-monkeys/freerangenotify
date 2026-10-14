package services

import (
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"strings"
)

type InboundSenderBinding = whatsapp.InboundSenderBinding
type SenderBindingStore = whatsapp.SenderBindingStore
type ChannelSenderResolver struct {
	Connections   *ChannelConnectionService
	apps          application.Repository
	bindings      SenderBindingStore
	MetaSecret    string
	TwilioAccount string
	TwilioSecret  string
}

func NewChannelSenderResolver(apps application.Repository, bindings SenderBindingStore) *ChannelSenderResolver {
	return &ChannelSenderResolver{apps: apps, bindings: bindings}
}

// Templates belong to a WABA. Each local app binding is processed separately;
// no single phone or first-page application is treated as the WABA owner.
func (r *ChannelSenderResolver) ResolveMetaTemplateSenders(ctx context.Context, account string) ([]*InboundSenderBinding, error) {
	if r.apps == nil || account == "" {
		return nil, fmt.Errorf("template account required")
	}
	var out []*InboundSenderBinding
	if r.Connections != nil {
		rows, err := r.Connections.Repo.FindAccount(ctx, "meta", account)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if row.Channel != "whatsapp" || row.Readiness != "ready" {
				continue
			}
			snap, err := r.Connections.ResolveRevision(ctx, row.ID, row.Revision)
			if err != nil {
				return nil, err
			}
			out = append(out, &InboundSenderBinding{ID: "meta-templates:" + account + ":" + row.ID, AppID: row.AppID, EnvironmentID: row.EnvironmentID, Provider: "meta", AccountID: account, SigningSecret: snap.Credentials["app_secret"]})
		}
	}
	for offset := 0; ; offset += 500 {
		apps, err := r.apps.List(ctx, application.ApplicationFilter{Limit: 500, Offset: offset})
		if err != nil {
			return nil, err
		}
		for _, app := range apps {
			if r.Connections != nil {
				_, err := r.Connections.Repo.Get(ctx, channelconnection.ID(app.AppID, "default", "whatsapp"))
				if err == nil {
					continue
				}
				if err != channelconnection.ErrNotFound {
					return nil, err
				}
			}
			cfg := app.Settings.WhatsApp
			if cfg == nil || cfg.Provider != "meta" || cfg.MetaWABAID != account || cfg.ConnectionStatus == "disconnected" {
				continue
			}
			secret := cfg.MetaAppSecret
			if secret == "" {
				secret = r.MetaSecret
			}
			out = append(out, &InboundSenderBinding{ID: "meta-templates:" + account + ":" + app.AppID, AppID: app.AppID, EnvironmentID: "default", Provider: "meta", AccountID: account, SigningSecret: secret})
		}
		if len(apps) < 500 {
			break
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("template account is not assigned")
	}
	return out, nil
}
func (r *ChannelSenderResolver) ResolveInboundSender(ctx context.Context, provider, accountID, senderID string) (*InboundSenderBinding, error) {
	if accountID == "" || senderID == "" {
		return nil, fmt.Errorf("inbound account and sender required")
	}
	var explicit *InboundSenderBinding
	if r.Connections != nil {
		rows, err := r.Connections.Repo.FindSender(ctx, provider, accountID, strings.TrimPrefix(senderID, "whatsapp:"))
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if row.Channel != "whatsapp" || row.Readiness != "ready" {
				continue
			}
			if explicit != nil {
				return nil, fmt.Errorf("ambiguous inbound sender assignment")
			}
			snap, err := r.Connections.ResolveRevision(ctx, row.ID, row.Revision)
			if err != nil {
				return nil, err
			}
			secret := snap.Credentials["app_secret"]
			if provider == "twilio" {
				secret = snap.Credentials["auth_token"]
			}
			canonicalSender := strings.TrimPrefix(senderID, "whatsapp:")
			id := fmt.Sprintf("%x", sha256.Sum256([]byte(provider+"\x00"+accountID+"\x00"+canonicalSender)))
			explicit = &InboundSenderBinding{ID: id, AppID: row.AppID, EnvironmentID: row.EnvironmentID, Provider: provider, AccountID: accountID, SenderID: canonicalSender, SigningSecret: secret}
		}
	}
	var candidates []*application.Application
	var err error
	if finder, ok := r.apps.(interface {
		FindWhatsAppSenderForAccount(context.Context, string, string, string, string) ([]*application.Application, error)
	}); ok {
		candidates, err = finder.FindWhatsAppSenderForAccount(ctx, provider, accountID, senderID, r.TwilioAccount)
		if err != nil {
			return nil, err
		}
	} else {
		for offset := 0; ; offset += 500 {
			page, e := r.apps.List(ctx, application.ApplicationFilter{Limit: 500, Offset: offset})
			if e != nil {
				return nil, e
			}
			candidates = append(candidates, page...)
			if len(page) < 500 {
				break
			}
		}
	}
	var binding *InboundSenderBinding
	for _, app := range candidates {
		if r.Connections != nil {
			_, err := r.Connections.Repo.Get(ctx, channelconnection.ID(app.AppID, "default", "whatsapp"))
			if err == nil {
				continue
			}
			if err != channelconnection.ErrNotFound {
				return nil, err
			}
		}
		cfg := app.Settings.WhatsApp
		if cfg == nil {
			continue
		}
		match := false
		secret := ""
		if provider == "meta" {
			match = cfg.Provider == "meta" && cfg.MetaWABAID == accountID && cfg.MetaPhoneNumberID == senderID && cfg.ConnectionStatus != "disconnected"
			secret = cfg.MetaAppSecret
			if secret == "" {
				secret = r.MetaSecret
			}
		}
		if provider == "twilio" {
			acct := cfg.AccountSID
			secret = cfg.AuthToken
			if acct == "" {
				acct = r.TwilioAccount
				secret = r.TwilioSecret
			}
			match = (cfg.Provider == "" || cfg.Provider == "twilio") && acct == accountID && strings.TrimPrefix(cfg.FromNumber, "whatsapp:") == strings.TrimPrefix(senderID, "whatsapp:")
		}
		if !match {
			continue
		}
		if binding != nil {
			return nil, fmt.Errorf("ambiguous inbound sender assignment")
		}
		id := fmt.Sprintf("%x", sha256.Sum256([]byte(provider+"\x00"+accountID+"\x00"+senderID)))
		binding = &InboundSenderBinding{ID: id, AppID: app.AppID, EnvironmentID: "default", Provider: provider, AccountID: accountID, SenderID: senderID, SigningSecret: secret}
	}
	if binding == nil {
		binding = explicit
	} else if explicit != nil {
		return nil, fmt.Errorf("ambiguous inbound sender assignment")
	}
	if binding == nil {
		return nil, fmt.Errorf("inbound sender is not assigned")
	}
	if r.bindings != nil {
		if err := r.bindings.ClaimSender(ctx, binding); err != nil {
			return nil, err
		}
	}
	return binding, nil
}
