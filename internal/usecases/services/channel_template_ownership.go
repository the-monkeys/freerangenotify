package services

import (
	"context"
	"errors"
	"fmt"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
)

// Vendor mutations on a shared account need a durable per-template owner map.
// Until such a grant exists, deny mutations rather than modifying a neighbour's asset.
func EnsureExclusiveTemplateAccount(ctx context.Context, apps application.Repository, connections *ChannelConnectionService, appID, envID, provider, account, globalAccount string) error {
	if account == "" {
		return fmt.Errorf("provider account required")
	}
	if envID == "" {
		envID = "default"
	}
	if connections != nil {
		rows, err := connections.Repo.FindAccount(ctx, provider, account)
		if err != nil {
			return err
		}
		for _, row := range rows {
			if row.Channel == notification.ChannelWhatsApp && row.Readiness != "disconnected" && (row.AppID != appID || row.EnvironmentID != envID) {
				return fmt.Errorf("shared provider account does not grant template mutation ownership")
			}
		}
	}
	for offset := 0; ; offset += 500 {
		rows, err := apps.List(ctx, application.ApplicationFilter{Limit: 500, Offset: offset})
		if err != nil {
			return fmt.Errorf("template account ownership unavailable")
		}
		for _, app := range rows {
			if app.AppID == appID {
				continue
			}
			if connections != nil {
				_, err := connections.Repo.Get(ctx, channelconnection.ID(app.AppID, "default", notification.ChannelWhatsApp))
				if err == nil {
					continue
				}
				if !errors.Is(err, channelconnection.ErrNotFound) {
					return err
				}
			}
			wa := app.Settings.WhatsApp
			other := ""
			if wa != nil && wa.ConnectionStatus == "disconnected" {
				continue
			}
			if provider == "meta" {
				if wa != nil && wa.Provider == "meta" {
					other = wa.MetaWABAID
				} else if wa == nil || wa.Provider == "" && wa.MetaWABAID == "" && wa.MetaAccessToken == "" {
					other = globalAccount
				}
			} else {
				if wa != nil {
					other = wa.AccountSID
				}
				if other == "" {
					other = globalAccount
				}
			}
			if other == account {
				return fmt.Errorf("shared provider account does not grant template mutation ownership")
			}
		}
		if len(rows) < 500 {
			return nil
		}
	}
}
func (s *ChannelConnectionService) WhatsAppConfig(ctx context.Context, appID, envID string) (*application.WhatsAppAppConfig, *channelconnection.Snapshot, error) {
	snap, err := s.ResolveDeliveryConnection(ctx, appID, envID, notification.ChannelWhatsApp)
	if err != nil {
		return nil, nil, err
	}
	return &application.WhatsAppAppConfig{Provider: snap.Provider, AccountSID: snap.AccountID, AuthToken: snap.Credentials["auth_token"], FromNumber: snap.SenderID, MetaWABAID: snap.AccountID, MetaPhoneNumberID: snap.SenderID, MetaAccessToken: snap.Credentials["access_token"], MetaAppSecret: snap.Credentials["app_secret"]}, snap, nil
}

func (s *ChannelConnectionService) ExplicitWhatsAppConfig(ctx context.Context, appID, envID string) (*application.WhatsAppAppConfig, *channelconnection.Snapshot, bool, error) {
	if err := s.checkScope(ctx, appID, envID); err != nil {
		return nil, nil, false, err
	}
	if envID == "" {
		envID = "default"
	}
	scopes := []string{envID}
	if envID != "default" {
		scopes = append(scopes, "default")
	}
	for _, env := range scopes {
		row, err := s.Repo.Get(ctx, channelconnection.ID(appID, env, notification.ChannelWhatsApp))
		if errors.Is(err, channelconnection.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, nil, false, err
		}
		if row.Readiness == "disconnected" {
			return nil, nil, true, fmt.Errorf("connection disconnected")
		}
		wa, snap, err := s.WhatsAppConfig(ctx, appID, envID)
		return wa, snap, true, err
	}
	return nil, nil, false, nil
}
