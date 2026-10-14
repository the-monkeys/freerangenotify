package services

import (
	"context"
	"errors"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
)

type ConnectionMigrationResult struct {
	AppID    string               `json:"app_id"`
	Channel  notification.Channel `json:"channel"`
	Action   string               `json:"action"`
	Revision int64                `json:"revision"`
}

// Migration is additive: legacy plaintext is retained until the entire reader
// fleet supports encrypted connections. Re-running never overwrites explicit edits.
func (s *ChannelConnectionService) MigrateLegacyConnection(ctx context.Context, appID string, ch notification.Channel, apply bool) (*ConnectionMigrationResult, error) {
	result := &ConnectionMigrationResult{AppID: appID, Channel: ch, Action: "would_migrate"}
	id := channelconnection.ID(appID, "default", ch)
	existing, err := s.Repo.Get(ctx, id)
	if err == nil {
		result.Action = "already_explicit"
		result.Revision = existing.Revision
		return result, nil
	}
	if !errors.Is(err, channelconnection.ErrNotFound) {
		return nil, err
	}
	snap, err := s.LegacySnapshot(ctx, appID, "default", ch)
	if err != nil {
		return nil, err
	}
	if !apply {
		return result, nil
	}
	current, err := s.UpdateConnection(ctx, id, 0, channelconnection.Patch{AppID: appID, Channel: ch, Provider: &snap.Provider, CredentialSource: &snap.CredentialSource, AccountID: &snap.AccountID, SenderID: &snap.SenderID, Secrets: snap.Credentials})
	if errors.Is(err, channelconnection.ErrConflict) {
		result.Action = "already_explicit"
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	result.Action = "migrated"
	result.Revision = current.Revision
	return result, nil
}
