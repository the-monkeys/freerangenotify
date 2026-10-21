package sse

import (
	"context"
	"encoding/json"
	"go.uber.org/zap"
	"runtime"
	"sync"
	"testing"
	"time"
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
func TestChannelInboxDisconnectDuringBroadcast(t *testing.T) {
	b := NewBroadcaster(nil, zap.NewNop())
	var wg sync.WaitGroup
	wg.Add(1)
	stop := make(chan struct{})
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				b.Broadcast("owner", "fixture")
				runtime.Gosched()
			}
		}
	}()
	for i := 0; i < 10000; i++ {
		client := b.AddClient("owner")
		runtime.Gosched()
		b.RemoveClient("owner", client)
	}
	close(stop)
	wg.Wait()
}
func TestChannelInboxIsolationRejectsSpoofedSubscriber(t *testing.T) {
	b := NewBroadcaster(nil, zap.NewNop())
	b.SetInboxAuthorizer(func(context.Context, string, string) bool { return true })
	legacy := b.AddClient("owner")
	b.routeMessage(&SSEMessage{Type: "whatsapp_inbound", AppID: "a", Data: map[string]interface{}{"private": "fixture"}})
	select {
	case <-legacy.Chan:
		t.Fatal("unverified owner UUID received inbox data")
	default:
	}
	if registrar, ok := interface{}(b).(interface {
		AddDashboardClient(string, time.Time) *Client
	}); ok {
		dashboard := registrar.AddDashboardClient("owner", time.Now().Add(time.Minute))
		b.routeMessage(&SSEMessage{Type: "whatsapp_inbound", AppID: "a", Data: map[string]interface{}{"private": "fixture"}})
		select {
		case <-dashboard.Chan:
		default:
			t.Fatal("verified dashboard principal missed event")
		}
		select {
		case <-legacy.Chan:
			t.Fatal("subscriber sharing UUID received dashboard event")
		default:
		}
	}
}

func TestChannelInboxIsolationRevocation(t *testing.T) {
	b := NewBroadcaster(nil, zap.NewNop())
	member := b.AddDashboardClient("member", time.Now().Add(time.Minute))
	outsider := b.AddDashboardClient("other", time.Now().Add(time.Minute))
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
