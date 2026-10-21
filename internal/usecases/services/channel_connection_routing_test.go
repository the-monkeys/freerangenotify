package services

import (
	"context"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"testing"
)

type connectionRoutingApps struct {
	application.Repository
	app *application.Application
}

func (r *connectionRoutingApps) GetByID(context.Context, string) (*application.Application, error) {
	return r.app, nil
}
func (r *connectionRoutingApps) List(context.Context, application.ApplicationFilter) ([]*application.Application, error) {
	return []*application.Application{r.app}, nil
}

type connectionSenderRows struct {
	*connectionMemory
	rows []*channelconnection.Connection
}

func (r *connectionSenderRows) FindAccount(context.Context, string, string) ([]*channelconnection.Connection, error) {
	return r.rows, nil
}
func (r *connectionSenderRows) FindSender(context.Context, string, string, string) ([]*channelconnection.Connection, error) {
	return r.rows, nil
}
func TestChannelConnectionOnboardingEncryptedCallbackScope(t *testing.T) {
	ctx := context.Background()
	s := connectionTestService()
	apps := &connectionRoutingApps{app: &application.Application{AppID: "a"}}
	s.Apps = apps
	row, err := s.UpdateConnection(ctx, channelconnection.ID("a", "default", notification.ChannelWhatsApp), 0, channelconnection.Patch{AppID: "a", Channel: notification.ChannelWhatsApp, Provider: textPointer("meta"), AccountID: textPointer("waba"), SenderID: textPointer("phone"), Secrets: map[string]string{"access_token": "fixture-token", "app_secret": "fixture-signing"}})
	if err != nil {
		t.Fatal(err)
	}
	row.Readiness = "ready"
	row.Revision++
	if err = s.Repo.Save(ctx, row, 1); err != nil {
		t.Fatal(err)
	}
	s.Repo = &connectionSenderRows{connectionMemory: s.Repo.(*connectionMemory), rows: []*channelconnection.Connection{row}}
	resolver := NewChannelSenderResolver(apps, nil)
	resolver.Connections = s
	bindings, err := resolver.ResolveMetaTemplateSenders(ctx, "waba")
	if err != nil || len(bindings) != 1 || bindings[0].SigningSecret != "fixture-signing" || bindings[0].AppID != "a" {
		t.Fatal("encrypted template callback scope unavailable", err)
	}
	row.Readiness = "pending"
	if _, err = resolver.ResolveInboundSender(ctx, "meta", "waba", "phone"); err == nil {
		t.Fatal("unvalidated sender accepted inbound")
	}
}
