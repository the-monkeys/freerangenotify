package services

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"go.uber.org/zap"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func (r *memRichRepo) UpdateDraftIfRevision(ctx context.Context, tpl *whatsapp.RichTemplate, expected string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	old := r.byID[tpl.ID]
	if old == nil || old.UpdatedAt.Format(time.RFC3339Nano) != expected {
		return fmt.Errorf("draft revision conflict")
	}
	raw, _ := json.Marshal(tpl)
	saved := new(whatsapp.RichTemplate)
	json.Unmarshal(raw, saved)
	r.byID[tpl.ID] = saved
	return nil
}
func TestChannelConnectionOnboardingSubmitOnce(t *testing.T) {
	for _, unknown := range []bool{false, true} {
		t.Run(fmt.Sprint(unknown), func(t *testing.T) {
			repo := newMemRichRepo()
			apps := &richStubAppRepo{app: &application.Application{AppID: "a", Settings: application.Settings{WhatsApp: &application.WhatsAppAppConfig{Provider: "meta", MetaWABAID: "account", MetaAccessToken: "fixture-token"}}}}
			sends := 0
			client := &http.Client{Transport: connectionValidationTransport(func(r *http.Request) (*http.Response, error) {
				sends++
				if unknown {
					return nil, fmt.Errorf("connection reset after submission")
				}
				return &http.Response{StatusCode: 201, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"id":"provider-template","status":"PENDING"}`))}, nil
			})}
			svc := NewWhatsAppRichTemplateService(repo, apps, client, "v23.0", "", "", zap.NewNop()).(ChannelTemplateDraftService)
			tpl, err := svc.CreateDraft(context.Background(), &whatsapp.RichTemplate{AppID: "a", Name: "submit_once", Kind: whatsapp.KindCouponCode, Category: "MARKETING", Language: "en_US", Body: "Coupon", CouponCode: "DEAL"})
			if err != nil {
				t.Fatal(err)
			}
			revision := tpl.UpdatedAt.Format(time.RFC3339Nano)
			_, err = svc.SubmitDraft(context.Background(), "a", tpl.ID, revision)
			if unknown && err == nil || !unknown && err != nil {
				t.Fatalf("submission outcome wrong: %v", err)
			}
			_, err = svc.SubmitDraft(context.Background(), "a", tpl.ID, revision)
			if err == nil || sends != 1 {
				t.Fatalf("submission replayed: %d %v", sends, err)
			}
		})
	}
}
func TestChannelConnectionOnboardingDraftSave(t *testing.T) {
	repo := newMemRichRepo()
	app := &application.Application{AppID: "a", Settings: application.Settings{WhatsApp: &application.WhatsAppAppConfig{Provider: "meta", MetaWABAID: "customer", MetaAccessToken: "fixture-token"}}}
	svc := NewWhatsAppRichTemplateService(repo, &richStubAppRepo{app: app}, nil, "v23.0", "", "", zap.NewNop()).(ChannelTemplateDraftService)
	tpl, err := svc.CreateDraft(context.Background(), &whatsapp.RichTemplate{AppID: "a", Name: "local_draft", Kind: whatsapp.KindCouponCode, Category: "MARKETING", Language: "en_US", Body: "Your coupon", CouponCode: "DEAL"})
	if err != nil {
		t.Fatal(err)
	}
	if tpl.ApprovalState != whatsapp.ApprovalDraft || tpl.Providers.Meta != nil || tpl.Providers.Twilio != nil {
		t.Fatal("local draft was submitted")
	}
	copy := *tpl
	copy.Body = "Updated coupon"
	updated, err := svc.UpdateDraft(context.Background(), "a", tpl.ID, tpl.UpdatedAt.Format(time.RFC3339Nano), &copy)
	if err != nil || updated.Body != "Updated coupon" {
		t.Fatal("draft edit failed", err)
	}
	if _, err = svc.UpdateDraft(context.Background(), "a", tpl.ID, tpl.UpdatedAt.Format(time.RFC3339Nano), &copy); err == nil {
		t.Fatal("stale draft update accepted")
	}
	if _, err = svc.SubmitDraft(context.Background(), "other", tpl.ID, updated.UpdatedAt.Format(time.RFC3339Nano)); err == nil {
		t.Fatal("foreign app submitted draft")
	}
}
