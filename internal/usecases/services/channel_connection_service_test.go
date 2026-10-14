package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/environment"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/secrets"
	"strings"
	"sync"
	"testing"
)

type connectionMemory struct {
	mu   sync.Mutex
	rows map[string]map[int64]*channelconnection.Connection
}

func cloneConnection(c *channelconnection.Connection) *channelconnection.Connection {
	if c == nil {
		return nil
	}
	b, _ := json.Marshal(c)
	out := new(channelconnection.Connection)
	json.Unmarshal(b, out)
	return out
}
func (r *connectionMemory) Get(_ context.Context, id string) (*channelconnection.Connection, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out *channelconnection.Connection
	for _, c := range r.rows[id] {
		if out == nil || out.Revision < c.Revision {
			out = c
		}
	}
	if out == nil {
		return nil, channelconnection.ErrNotFound
	}
	return cloneConnection(out), nil
}
func (r *connectionMemory) GetRevision(_ context.Context, id string, revision int64) (*channelconnection.Connection, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.rows[id][revision]
	if out == nil {
		return nil, channelconnection.ErrNotFound
	}
	return cloneConnection(out), nil
}
func (r *connectionMemory) Save(_ context.Context, c *channelconnection.Connection, expected int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.rows == nil {
		r.rows = map[string]map[int64]*channelconnection.Connection{}
	}
	if int64(len(r.rows[c.ID])) != expected {
		return channelconnection.ErrConflict
	}
	if r.rows[c.ID] == nil {
		r.rows[c.ID] = map[int64]*channelconnection.Connection{}
	}
	r.rows[c.ID][c.Revision] = cloneConnection(c)
	return nil
}
func (r *connectionMemory) FindSender(context.Context, string, string, string) ([]*channelconnection.Connection, error) {
	return nil, nil
}
func (r *connectionMemory) FindAccount(context.Context, string, string) ([]*channelconnection.Connection, error) {
	return nil, nil
}

type connectionApps struct {
	application.Repository
	app *application.Application
	err error
}

func (r *connectionApps) GetByID(context.Context, string) (*application.Application, error) {
	return r.app, r.err
}
func connectionTestService() *ChannelConnectionService {
	codec, _ := secrets.NewConnectionCodec("fixture:" + base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{21}, 32)))
	return &ChannelConnectionService{Repo: &connectionMemory{}, Codec: codec, WritesEnabled: true, Apps: &connectionApps{app: &application.Application{AppID: "a", Settings: application.Settings{WhatsApp: &application.WhatsAppAppConfig{AccountSID: "legacy", AuthToken: "fixture-legacy", FromNumber: "+123"}}}}}
}
func textPointer(s string) *string { return &s }

type connectionEnvironments struct{ environment.Repository }

func (r *connectionEnvironments) GetByID(context.Context, string) (*environment.Environment, error) {
	return &environment.Environment{ID: "prod", AppID: "a"}, nil
}
func TestChannelConnectionEnvironmentAndMigration(t *testing.T) {
	ctx := context.Background()
	s := connectionTestService()
	s.Environments = &connectionEnvironments{}
	ch := notification.ChannelWhatsApp
	dry, err := s.MigrateLegacyConnection(ctx, "a", ch, false)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(dry)
	if strings.Contains(string(raw), "fixture-legacy") {
		t.Fatal("migration exposes secret")
	}
	if _, err = s.Repo.Get(ctx, channelconnection.ID("a", "default", ch)); err != channelconnection.ErrNotFound {
		t.Fatal("dry run wrote data")
	}
	first, err := s.MigrateLegacyConnection(ctx, "a", ch, true)
	if err != nil || first.Action != "migrated" {
		t.Fatal("migration failed", err)
	}
	again, err := s.MigrateLegacyConnection(ctx, "a", ch, true)
	if err != nil || again.Action != "already_explicit" {
		t.Fatal("migration not resumable", err)
	}
	_, err = s.UpdateConnection(ctx, channelconnection.ID("a", "prod", ch), 0, channelconnection.Patch{AppID: "a", EnvironmentID: "prod", Channel: ch, Provider: textPointer("twilio"), CredentialSource: textPointer("byoc"), AccountID: textPointer("production-account"), SenderID: textPointer("+456"), Secrets: map[string]string{"auth_token": "fixture-env"}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.ResolveDeliveryConnection(ctx, "a", "prod", ch)
	if err != nil || got.AccountID != "production-account" || got.EnvironmentID != "prod" {
		t.Fatal("environment override ignored", err)
	}
	s.Environments = nil
	if _, err = s.ResolveDeliveryConnection(ctx, "a", "prod", ch); err == nil {
		t.Fatal("unknown environment inherited parent")
	}
}
func TestChannelConnectionCompatibility(t *testing.T) {
	ctx := context.Background()
	s := connectionTestService()
	ch := notification.ChannelWhatsApp
	t.Run("legacy Twilio aliases without global credentials", func(t *testing.T) {
		got, err := s.ResolveDeliveryConnection(ctx, "a", "", ch)
		if err != nil {
			t.Fatal(err)
		}
		if got.Provider != "twilio" || got.CredentialSource != "byoc" || got.Credentials["auth_token"] != "fixture-legacy" {
			t.Fatalf("wrong legacy connection: %#v", got)
		}
	})
	t.Run("environment before parent and immutable queued revision", func(t *testing.T) {
		id := channelconnection.ID("a", "default", ch)
		parent, err := s.UpdateConnection(ctx, id, 0, channelconnection.Patch{AppID: "a", Channel: ch, Provider: textPointer("meta"), CredentialSource: textPointer("byoc"), AccountID: textPointer("waba"), SenderID: textPointer("phone"), Secrets: map[string]string{"access_token": "fixture-first"}})
		if err != nil {
			t.Fatal(err)
		}
		_, err = s.UpdateConnection(ctx, id, parent.Revision, channelconnection.Patch{SenderID: textPointer("new-phone"), Secrets: map[string]string{"access_token": "fixture-second"}})
		if err != nil {
			t.Fatal(err)
		}
		old, err := s.ResolveRevision(ctx, id, 1)
		if err != nil || old.SenderID != "phone" || old.Credentials["access_token"] != "fixture-first" {
			t.Fatal("queued revision changed", err)
		}
		_, err = s.UpdateConnection(ctx, id, 1, channelconnection.Patch{SenderID: textPointer("stale")})
		if err != channelconnection.ErrConflict {
			t.Fatalf("stale writer accepted: %v", err)
		}
		envID := channelconnection.ID("a", "default", notification.ChannelSMS)
		_, err = s.UpdateConnection(ctx, envID, 0, channelconnection.Patch{AppID: "a", Channel: notification.ChannelSMS, Provider: textPointer("twilio"), CredentialSource: textPointer("byoc"), AccountID: textPointer("independent"), SenderID: textPointer("+456"), Secrets: map[string]string{"auth_token": "fixture-sms"}})
		if err != nil {
			t.Fatal(err)
		}
		got, err := s.ResolveDeliveryConnection(ctx, "a", "", notification.ChannelSMS)
		if err != nil || got.AccountID != "independent" {
			t.Fatal("BYOC fell back", err)
		}
	})
	t.Run("disconnect blocks fallback", func(t *testing.T) {
		id := channelconnection.ID("a", "default", ch)
		current, loadErr := s.Repo.Get(ctx, id)
		if loadErr != nil {
			t.Fatal(loadErr)
		}
		_, err := s.UpdateConnection(ctx, id, current.Revision, channelconnection.Patch{Disconnect: true})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.ResolveDeliveryConnection(ctx, "a", "", ch); err == nil {
			t.Fatal("disconnected resolved legacy")
		}
	})
	t.Run("reader failures never fall back", func(t *testing.T) {
		s.Apps = &connectionApps{err: fmt.Errorf("unavailable")}
		if _, err := s.ResolveDeliveryConnection(ctx, "b", "", ch); err == nil {
			t.Fatal("app error ignored")
		}
	})
	t.Run("rollout disabled", func(t *testing.T) {
		s.WritesEnabled = false
		if _, err := s.UpdateConnection(ctx, "x", 0, channelconnection.Patch{}); err != channelconnection.ErrWritesDisabled {
			t.Fatal(err)
		}
	})
	raw, _ := json.Marshal(s.Repo.(*connectionMemory).rows)
	if strings.Contains(string(raw), "fixture-first") {
		t.Fatal("connection storage contains plaintext")
	}
}
