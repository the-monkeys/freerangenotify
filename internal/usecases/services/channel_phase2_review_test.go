package services

import (
	"context"
	"fmt"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"go.uber.org/zap"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type reviewMultipleApps struct {
	application.Repository
	apps []*application.Application
}

func (r *reviewMultipleApps) GetByID(_ context.Context, id string) (*application.Application, error) {
	for _, app := range r.apps {
		if app.AppID == id {
			return app, nil
		}
	}
	return nil, fmt.Errorf("app unavailable")
}
func (r *reviewMultipleApps) List(_ context.Context, filter application.ApplicationFilter) ([]*application.Application, error) {
	if filter.Offset > 0 {
		return nil, nil
	}
	return r.apps, nil
}
func TestChannelPhase2ReviewDualProviderOwnership(t *testing.T) {
	apps := &reviewMultipleApps{apps: []*application.Application{{AppID: "a", Settings: application.Settings{WhatsApp: &application.WhatsAppAppConfig{Provider: "meta", MetaWABAID: "exclusive-meta", MetaAccessToken: "fixture-meta", AccountSID: "ACshared", AuthToken: "fixture-twilio"}}}, {AppID: "b", Settings: application.Settings{WhatsApp: &application.WhatsAppAppConfig{AccountSID: "ACshared", AuthToken: "fixture-other"}}}}}
	twilioWrites := 0
	client := &http.Client{Transport: connectionValidationTransport(func(r *http.Request) (*http.Response, error) {
		body := `{"id":"meta-id","status":"PENDING"}`
		if r.URL.Host == "content.twilio.com" {
			twilioWrites++
			body = `{"sid":"HXfixture","status":"pending"}`
		}
		return &http.Response{StatusCode: 201, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	svc := NewWhatsAppRichTemplateService(newMemRichRepo(), apps, client, "v23.0", "", "", zap.NewNop())
	_, _ = svc.Create(context.Background(), &whatsapp.RichTemplate{AppID: "a", Name: "ownership_guard", Kind: whatsapp.KindCouponCode, Category: "MARKETING", Language: "en_US", Body: "Coupon", CouponCode: "DEAL"})
	if twilioWrites != 0 {
		t.Fatal("Meta selection bypassed shared Twilio ownership")
	}
}
func TestChannelPhase2ReviewSyncBoundConnection(t *testing.T) {
	ctx := channelconnection.WithEnvironment(context.Background(), "prod")
	connections := connectionTestService()
	connections.Environments = &connectionEnvironments{}
	row, err := connections.UpdateConnection(ctx, channelconnection.ID("a", "prod", "whatsapp"), 0, channelconnection.Patch{AppID: "a", EnvironmentID: "prod", Channel: "whatsapp", Provider: textPointer("meta"), AccountID: textPointer("account-b"), Secrets: map[string]string{"access_token": "fixture-b"}, SenderID: textPointer("phone-b")})
	if err != nil {
		t.Fatal(err)
	}
	tpl := &whatsapp.RichTemplate{ID: "sync-bound", AppID: "a", EnvironmentID: "prod", ConnectionID: row.ID, ConnectionRevision: row.Revision, ProviderAccountID: "account-b", ProviderType: "meta", Providers: whatsapp.ProviderBindings{Meta: &whatsapp.MetaBinding{TemplateName: "same_name", Status: "PENDING"}}, UpdatedAt: time.Now().UTC()}
	repo := newMemRichRepo()
	repo.Create(ctx, tpl)
	apps := &richStubAppRepo{app: &application.Application{AppID: "a", Settings: application.Settings{WhatsApp: &application.WhatsAppAppConfig{Provider: "meta", MetaWABAID: "account-a", MetaAccessToken: "fixture-a"}}}}
	queriedB := false
	client := &http.Client{Transport: connectionValidationTransport(func(r *http.Request) (*http.Response, error) {
		state := "REJECTED"
		if strings.Contains(r.URL.Path, "/account-b/") {
			queriedB = true
			state = "APPROVED"
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"data":[{"status":"` + state + `"}]}`))}, nil
	})}
	svc := NewWhatsAppRichTemplateService(repo, apps, client, "v23.0", "", "", zap.NewNop()).(*whatsappRichTemplateService)
	svc.SetChannelConnections(connections)
	out, err := svc.SyncApproval(ctx, "a", tpl.ID)
	if err != nil || !queriedB || out.Providers.Meta.Status != "APPROVED" {
		t.Fatal("Sync used legacy parent account instead of bound environment", err)
	}
}
