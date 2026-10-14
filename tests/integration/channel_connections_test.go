package integration

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"go.uber.org/zap"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestChannelConnectionsRealES(t *testing.T) {
	client := channelFixture(t)
	ctx := context.Background()
	repo := repository.NewChannelConnectionRepository(client)
	templates := &database.IndexTemplates{}
	for index, mapping := range map[string]map[string]interface{}{"channel_connections": templates.GetChannelConnectionsTemplate(), "whatsapp_rich_templates": templates.GetWhatsAppRichTemplatesTemplate()} {
		raw, _ := json.Marshal(mapping)
		res, err := client.Indices.Create(index, client.Indices.Create.WithBody(strings.NewReader(string(raw))))
		if err != nil {
			t.Fatal(err)
		}
		if res.IsError() && res.StatusCode != 400 {
			t.Fatalf("fixture schema failed %d", res.StatusCode)
		}
		res.Body.Close()
	}
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
	t.Run("draft CAS preserves one writer and environment scope", func(t *testing.T) {
		draftRepo := repository.NewWhatsAppRichTemplateRepository(client, zap.NewNop())
		now := time.Now().UTC()
		draft := &whatsapp.RichTemplate{ID: uuid.NewString(), AppID: id, Name: "local_" + id, EnvironmentID: "prod", ApprovalState: whatsapp.ApprovalDraft, UpdatedAt: now, CreatedAt: now}
		if err := draftRepo.Create(ctx, draft); err != nil {
			t.Fatal(err)
		}
		atomicRepo := draftRepo.(interface {
			UpdateDraftIfRevision(context.Context, *whatsapp.RichTemplate, string) error
		})
		var won atomic.Int64
		var group sync.WaitGroup
		for i := 0; i < 2; i++ {
			group.Add(1)
			go func() {
				defer group.Done()
				copy := *draft
				copy.Body = uuid.NewString()
				copy.UpdatedAt = now.Add(time.Second)
				err := atomicRepo.UpdateDraftIfRevision(ctx, &copy, now.Format(time.RFC3339Nano))
				if err == nil {
					won.Add(1)
				} else if !errors.Is(err, channelconnection.ErrConflict) {
					t.Error(err)
				}
			}()
		}
		group.Wait()
		if won.Load() != 1 {
			t.Fatal("draft CAS accepted multiple writers")
		}
		other, _, err := draftRepo.List(ctx, whatsapp.RichTemplateFilter{AppID: id, EnvironmentID: "default"})
		if err != nil || len(other) != 0 {
			t.Fatal("environment draft exposed in default", err)
		}
		own, _, err := draftRepo.List(ctx, whatsapp.RichTemplateFilter{AppID: id, EnvironmentID: "prod"})
		if err != nil || len(own) != 1 {
			t.Fatal("environment draft unavailable", err)
		}
	})
}
