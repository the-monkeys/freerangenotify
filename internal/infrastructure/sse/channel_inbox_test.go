package sse

import (
	"context"
	"encoding/json"
	"go.uber.org/zap"
	"testing"
)

func TestChannelInboxIsolation(t *testing.T) {
	b := NewBroadcaster(nil, zap.NewNop())
	owner := b.AddClient("owner")
	other := b.AddClient("outsider")
	raw := []byte(`{"type":"whatsapp_inbound","data":{"app_id":"a","text_body":"private"}}`)
	// Exercise the same routing operation used by the Redis subscriber.
	var m SSEMessage
	json.Unmarshal(raw, &m)
	b.routeMessage(&m)
	select {
	case <-other.Chan:
		t.Fatal("private event delivered globally")
	default:
	}
	select {
	case <-owner.Chan:
		t.Fatal("unscoped private event delivered")
	default:
	}
}

func TestChannelInboxIsolationRevocation(t *testing.T) {
	b := NewBroadcaster(nil, zap.NewNop())
	member := b.AddClient("member")
	outsider := b.AddClient("other")
	allowed := true
	b.SetInboxAuthorizer(func(_ context.Context, appID, userID string) bool {
		return allowed && appID == "a" && userID == "member"
	})
	msg := &SSEMessage{Type: "whatsapp_inbound", AppID: "a", Data: map[string]interface{}{"text_body": "private"}}
	b.routeMessage(msg)
	select {
	case <-member.Chan:
	default:
		t.Fatal("authorized member did not receive event")
	}
	select {
	case <-outsider.Chan:
		t.Fatal("outsider received event")
	default:
	}
	allowed = false
	b.routeMessage(msg)
	select {
	case <-member.Chan:
		t.Fatal("revoked member received event")
	default:
	}
}
