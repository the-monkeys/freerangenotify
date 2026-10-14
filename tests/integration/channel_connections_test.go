package integration

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"sync"
	"sync/atomic"
	"testing"
)

func TestChannelConnectionsRealES(t *testing.T) {
	client := channelFixture(t)
	ctx := context.Background()
	repo := repository.NewChannelConnectionRepository(client)
	id := uuid.NewString()
	original := &channelconnection.Connection{ID: id, AppID: id, EnvironmentID: "default", Channel: notification.ChannelWhatsApp, Provider: "meta", AccountID: "account-" + id, SenderID: "sender-" + id, CredentialSource: "byoc", Revision: 1, Readiness: "pending"}
	if err := repo.Save(ctx, original, 0); err != nil {
		t.Fatal("initial connection persistence", err)
	}
	var wins atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := repo.Get(ctx, id)
			if err != nil {
				t.Error(err)
				return
			}
			c.Revision = 2
			c.SenderID = "updated"
			if err = repo.Save(ctx, c, 1); err == nil {
				wins.Add(1)
			} else if !errors.Is(err, channelconnection.ErrConflict) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("expected one CAS writer, got %d", wins.Load())
	}
	// Interrupted/restarted migration leaves the old immutable revision readable.
	restarted := repository.NewChannelConnectionRepository(client)
	old, err := restarted.GetRevision(ctx, id, 1)
	if err != nil || old.SenderID != original.SenderID {
		t.Fatalf("history was overwritten: %v", err)
	}
	current, err := restarted.Get(ctx, id)
	if err != nil || current.Revision != 2 {
		t.Fatalf("current revision unavailable: %v", err)
	}
}
