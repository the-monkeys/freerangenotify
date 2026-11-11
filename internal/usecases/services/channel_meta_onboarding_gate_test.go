package services

import (
	"context"
	"fmt"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChannelEnvironmentMetaOnboardingGate(t *testing.T) {
	ctx := context.Background()
	enabled := false
	s := connectionTestService()
	s.MetaEnabled = &enabled
	patch := channelconnection.Patch{AppID: "a", EnvironmentID: "default", Channel: notification.ChannelWhatsApp, Provider: textPointer("meta"), AccountID: textPointer("waba"), SenderID: textPointer("phone"), Secrets: map[string]string{"access_token": "fixture-token"}}
	id := channelconnection.ID("a", "default", notification.ChannelWhatsApp)
	if _, err := s.UpdateConnection(ctx, id, 0, patch); err == nil {
		t.Fatal("disabled onboarding accepted a new Meta connection")
	}
	enabled = true
	c, err := s.UpdateConnection(ctx, id, 0, patch)
	if err != nil {
		t.Fatal(err)
	}
	enabled = false
	if _, err := s.UpdateConnection(ctx, id, c.Revision, patch); err == nil {
		t.Fatal("disabled onboarding accepted Meta edits")
	}
	if _, err := s.ResolveDeliveryConnection(ctx, "a", "default", notification.ChannelWhatsApp); err != nil {
		t.Fatalf("rollback disabled connection reader: %v", err)
	}
	if _, err := s.UpdateConnection(ctx, id, c.Revision, channelconnection.Patch{Disconnect: true}); err != nil {
		t.Fatalf("rollback prevented disconnect: %v", err)
	}
}
func TestChannelEnvironmentMetaValidationGate(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; fmt.Fprint(w, `{"data":[]}`) }))
	defer server.Close()
	enabled := false
	v := &ConnectionValidator{Client: &http.Client{Transport: dispatchTransport{server.URL}}}
	v.MetaEnabled = &enabled
	status, _, err := v.Validate(context.Background(), &channelconnection.Snapshot{Provider: "meta", AccountID: "waba", SenderID: "phone", Credentials: map[string]string{"access_token": "fixture-token"}})
	if status == "ready" || err == nil || calls != 0 {
		t.Fatalf("disabled validation contacted vendor: status=%s err=%v calls=%d", status, err, calls)
	}
}
