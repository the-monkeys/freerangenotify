package main

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/the-monkeys/freerangenotify/internal/config"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/secrets"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"go.uber.org/zap"
)

// Explicit IDs form the checkpoint: an existing connection is never overwritten.
// No fleet-wide scan, deletion of legacy credentials, or paid provider requests.
func newChannelConnectionsCmd() *cobra.Command {
	var appIDs []string
	var apply, readersUpgraded bool
	cmd := &cobra.Command{Use: "channel-connections", Short: "Dry-run or migrate scoped legacy SMS/WhatsApp connections", RunE: func(cmd *cobra.Command, args []string) error {
		if len(appIDs) == 0 {
			return fmt.Errorf("at least one --app is required")
		}
		if apply && !readersUpgraded {
			return fmt.Errorf("apply requires --readers-upgraded after upgrading the whole fleet and pausing connection settings changes")
		}
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("configuration unavailable")
		}
		codec, err := secrets.NewConnectionCodec(cfg.Connection.EncryptionKeys)
		if err != nil {
			return err
		}
		client, err := database.NewElasticsearchClient(cfg, zap.NewNop())
		if err != nil {
			return fmt.Errorf("database unavailable")
		}
		defer client.Close()
		svc := &services.ChannelConnectionService{Apps: repository.NewApplicationRepository(client.GetClient(), zap.NewNop()), Repo: repository.NewChannelConnectionRepository(client.GetClient()), Codec: codec, WritesEnabled: apply && cfg.Connection.WritesEnabled}
		for _, appID := range appIDs {
			for _, channel := range []notification.Channel{notification.ChannelWhatsApp, notification.ChannelSMS} {
				result, err := svc.MigrateLegacyConnection(cmd.Context(), appID, channel, apply)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "application %s channel %s: migration unavailable; connection unchanged\n", appID, channel)
					return fmt.Errorf("migration stopped; completed connections are resumable")
				}
				if err = json.NewEncoder(cmd.OutOrStdout()).Encode(result); err != nil {
					return err
				}
			}
		}
		return nil
	}}
	cmd.Flags().StringSliceVar(&appIDs, "app", nil, "Application IDs to inspect (repeat or comma-separated)")
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply additive encrypted migration (default is dry-run)")
	cmd.Flags().BoolVar(&readersUpgraded, "readers-upgraded", false, "Confirm compatible readers and paused connection settings writes")
	return cmd
}
