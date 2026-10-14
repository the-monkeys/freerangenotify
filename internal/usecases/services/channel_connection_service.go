package services

import (
	"context"
	"errors"
	"fmt"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/environment"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/secrets"
	"strings"
	"time"
)

type ChannelConnectionService struct {
	Repo          channelconnection.Repository
	Apps          application.Repository
	Environments  environment.Repository
	Codec         *secrets.ConnectionCodec
	WritesEnabled bool
	System        map[notification.Channel]*channelconnection.Snapshot
}

func (s *ChannelConnectionService) checkScope(ctx context.Context, appID, envID string) error {
	app, err := s.Apps.GetByID(ctx, appID)
	if err != nil || app == nil {
		return fmt.Errorf("application unavailable")
	}
	if envID != "" && envID != "default" {
		if s.Environments == nil {
			return fmt.Errorf("environment unavailable")
		}
		env, err := s.Environments.GetByID(ctx, envID)
		if err != nil || env == nil || env.AppID != appID {
			return fmt.Errorf("environment does not belong to application")
		}
	}
	return nil
}
func (s *ChannelConnectionService) ResolveDeliveryConnection(ctx context.Context, appID, envID string, ch notification.Channel) (*channelconnection.Snapshot, error) {
	if err := s.checkScope(ctx, appID, envID); err != nil {
		return nil, err
	}
	if envID == "" {
		envID = "default"
	}
	scopes := []string{envID}
	if envID != "default" {
		scopes = append(scopes, "default")
	}
	for _, scope := range scopes {
		c, err := s.Repo.Get(ctx, channelconnection.ID(appID, scope, ch))
		if errors.Is(err, channelconnection.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if c.Readiness == "disconnected" {
			return nil, fmt.Errorf("channel connection disconnected")
		}
		return s.snapshot(c)
	}
	return s.LegacySnapshot(ctx, appID, envID, ch)
}

// LegacySnapshot is read-only. Customer credentials never borrow missing fields
// from the global account, which could accidentally bill or sign as another owner.
func (s *ChannelConnectionService) LegacySnapshot(ctx context.Context, appID, envID string, ch notification.Channel) (*channelconnection.Snapshot, error) {
	app, err := s.Apps.GetByID(ctx, appID)
	if err != nil || app == nil {
		return nil, fmt.Errorf("application unavailable")
	}
	snap := &channelconnection.Snapshot{ConnectionID: channelconnection.ID(appID, "default", ch), AppID: appID, EnvironmentID: "default", Channel: ch, Provider: "twilio", CredentialSource: "system", Readiness: "pending", Credentials: map[string]string{}}
	switch ch {
	case notification.ChannelWhatsApp:
		wa := app.Settings.WhatsApp
		if wa != nil {
			if wa.ConnectionStatus == "disconnected" {
				return nil, fmt.Errorf("channel connection disconnected")
			}
			if wa.Provider == "meta" {
				snap.Provider = "meta"
				snap.AccountID = wa.MetaWABAID
				snap.SenderID = wa.MetaPhoneNumberID
				snap.Credentials = map[string]string{"access_token": wa.MetaAccessToken, "app_secret": wa.MetaAppSecret}
				snap.CredentialSource = "byoc"
			} else {
				snap.AccountID = wa.AccountSID
				snap.SenderID = wa.FromNumber
				snap.Credentials["auth_token"] = wa.AuthToken
				if wa.AccountSID != "" || wa.AuthToken != "" {
					snap.CredentialSource = "byoc"
				}
			}
		}
	case notification.ChannelSMS:
		sms := app.Settings.SMS
		if sms != nil {
			snap.AccountID = sms.AccountSID
			snap.SenderID = sms.FromNumber
			snap.Credentials["auth_token"] = sms.AuthToken
			if sms.AccountSID != "" || sms.AuthToken != "" {
				snap.CredentialSource = "byoc"
			}
		}
	default:
		return nil, fmt.Errorf("unsupported channel")
	}
	if snap.CredentialSource == "system" {
		system := s.System[ch]
		if system == nil {
			return nil, fmt.Errorf("system channel not configured")
		}
		copy := *system
		snap.Provider = copy.Provider
		snap.AccountID = copy.AccountID
		if snap.SenderID == "" {
			snap.SenderID = copy.SenderID
		}
		snap.Credentials = map[string]string{}
		for field, value := range copy.Credentials {
			snap.Credentials[field] = value
		}
	}
	return snap, nil
}
func (s *ChannelConnectionService) binding(c *channelconnection.Connection, field string) secrets.SecretBinding {
	return secrets.SecretBinding{ConnectionID: c.ID, AppID: c.AppID, EnvironmentID: c.EnvironmentID, Provider: c.Provider, Field: field}
}
func (s *ChannelConnectionService) snapshot(c *channelconnection.Connection) (*channelconnection.Snapshot, error) {
	snap := &channelconnection.Snapshot{ConnectionID: c.ID, AppID: c.AppID, EnvironmentID: c.EnvironmentID, Channel: c.Channel, Revision: c.Revision, Provider: c.Provider, CredentialSource: c.CredentialSource, AccountID: c.AccountID, SenderID: c.SenderID, Readiness: c.Readiness, Credentials: map[string]string{}}
	if c.CredentialSource == "system" {
		system := s.System[c.Channel]
		if system == nil || system.Provider != c.Provider || system.AccountID != c.AccountID {
			return nil, fmt.Errorf("system account no longer matches connection")
		}
		for k, v := range system.Credentials {
			snap.Credentials[k] = v
		}
	} else {
		for field, value := range c.Secrets {
			plain, err := s.Codec.Open(value, s.binding(c, field))
			if err != nil {
				return nil, err
			}
			snap.Credentials[field] = string(plain)
		}
	}
	return snap, nil
}
func (s *ChannelConnectionService) UpdateConnection(ctx context.Context, id string, revision int64, patch channelconnection.Patch) (*channelconnection.Connection, error) {
	if !s.WritesEnabled {
		return nil, channelconnection.ErrWritesDisabled
	}
	c, err := s.Repo.Get(ctx, id)
	if errors.Is(err, channelconnection.ErrNotFound) {
		if revision != 0 {
			return nil, channelconnection.ErrConflict
		}
		env := patch.EnvironmentID
		if env == "" {
			env = "default"
		}
		c = &channelconnection.Connection{ID: id, AppID: patch.AppID, EnvironmentID: env, Channel: patch.Channel, CredentialSource: "byoc", CreatedAt: time.Now().UTC(), Secrets: map[string]channelconnection.EncryptedSecret{}}
		if id != channelconnection.ID(c.AppID, env, c.Channel) {
			return nil, fmt.Errorf("invalid connection identity")
		}
	} else if err != nil {
		return nil, err
	}
	if c.Revision != revision {
		return nil, channelconnection.ErrConflict
	}
	if err := s.checkScope(ctx, c.AppID, c.EnvironmentID); err != nil {
		return nil, err
	}
	if c.Channel != notification.ChannelWhatsApp && c.Channel != notification.ChannelSMS {
		return nil, fmt.Errorf("unsupported channel")
	}
	old := *c
	credentials := map[string]string{}
	for field, value := range c.Secrets {
		plain, err := s.Codec.Open(value, s.binding(&old, field))
		if err != nil {
			return nil, err
		}
		credentials[field] = string(plain)
	}
	if patch.Provider != nil {
		c.Provider = *patch.Provider
	}
	if patch.CredentialSource != nil {
		c.CredentialSource = *patch.CredentialSource
	}
	if patch.AccountID != nil {
		c.AccountID = *patch.AccountID
	}
	if patch.SenderID != nil {
		c.SenderID = *patch.SenderID
	}
	if c.Provider != "meta" && c.Provider != "twilio" {
		return nil, fmt.Errorf("unsupported provider")
	}
	if c.Provider == "meta" && c.Channel != notification.ChannelWhatsApp {
		return nil, fmt.Errorf("Meta is only available for WhatsApp")
	}
	if c.CredentialSource != "byoc" && c.CredentialSource != "system" {
		return nil, fmt.Errorf("invalid credential source")
	}
	if old.Provider != "" && old.Provider != c.Provider {
		credentials = map[string]string{}
	}
	for _, field := range patch.ClearSecrets {
		delete(credentials, field)
	}
	for field, value := range patch.Secrets {
		if field != "access_token" && field != "auth_token" && field != "app_secret" {
			return nil, fmt.Errorf("unsupported secret field")
		}
		if strings.TrimSpace(value) != "" && !strings.HasPrefix(value, "****") {
			credentials[field] = value
		}
	}
	c.Secrets = map[string]channelconnection.EncryptedSecret{}
	if c.CredentialSource == "byoc" {
		for field, value := range credentials {
			sealed, err := s.Codec.Seal([]byte(value), s.binding(c, field))
			if err != nil {
				return nil, err
			}
			c.Secrets[field] = sealed
		}
	}
	c.Readiness = "pending"
	c.Facts = nil
	if patch.Disconnect {
		c.Readiness = "disconnected"
		c.Secrets = nil
	}
	c.Revision++
	c.UpdatedAt = time.Now().UTC()
	if err = s.Repo.Save(ctx, c, revision); err != nil {
		return nil, err
	}
	return c, nil
}
func (s *ChannelConnectionService) ResolveRevision(ctx context.Context, id string, revision int64) (*channelconnection.Snapshot, error) {
	c, err := s.Repo.GetRevision(ctx, id, revision)
	if err != nil {
		return nil, err
	}
	return s.snapshot(c)
}
