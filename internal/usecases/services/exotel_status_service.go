package services

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/providers"
)

var ErrExotelCallbackAuth = errors.New("invalid SMS callback capability")

type ExotelStatusService struct {
	Connections   *ChannelConnectionService
	Notifications notification.Repository
	Events        whatsapp.EventRepository
	Client        *http.Client
}

// This is our per-delivery capability, not a vendor signing protocol. Status is
// always read again using the archived account credentials before projection.
func ExotelCallbackProof(s *channelconnection.Snapshot, n *notification.Notification, expires int64) string {
	mac := hmac.New(sha256.New, []byte(s.Credentials["api_token"]))
	fmt.Fprintf(mac, "frn-exotel-v1\x00%s\x00%s\x00%s\x00%s\x00%d\x00%s\x00%s\x00%d", n.NotificationID, n.AppID, s.EnvironmentID, s.ConnectionID, s.Revision, s.AccountID, s.SenderID, expires)
	return hex.EncodeToString(mac.Sum(nil))
}
func ExotelCallbackURL(public string, s *channelconnection.Snapshot, n *notification.Notification) (string, error) {
	u, e := url.Parse(public)
	if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("public HTTPS callback URL required")
	}
	expires := time.Now().Add(7 * 24 * time.Hour).Unix()
	u.Path = strings.TrimRight(u.Path, "/") + "/v1/webhooks/exotel/sms/" + url.PathEscape(n.NotificationID)
	q := url.Values{"expires": {strconv.FormatInt(expires, 10)}, "proof": {ExotelCallbackProof(s, n, expires)}}
	u.RawQuery = q.Encode()
	return u.String(), nil
}
func (s *ExotelStatusService) bound(ctx context.Context, id string) (*notification.Notification, *channelconnection.Snapshot, error) {
	n, e := s.Notifications.GetByID(ctx, id)
	if e != nil || n == nil || n.Channel != notification.ChannelSMS {
		return nil, nil, ErrExotelCallbackAuth
	}
	raw, e := json.Marshal(n.Metadata[channelconnection.DeliveryMetadataKey])
	var safe channelconnection.Snapshot
	if e != nil || json.Unmarshal(raw, &safe) != nil || safe.Provider != "exotel" || safe.Channel != notification.ChannelSMS || safe.AppID != n.AppID || safe.Revision < 1 {
		return nil, nil, ErrExotelCallbackAuth
	}
	snap, e := s.Connections.ResolveRevision(ctx, safe.ConnectionID, safe.Revision)
	if e != nil {
		return nil, nil, fmt.Errorf("SMS connection history unavailable")
	}
	if snap.Provider != "exotel" || snap.Channel != safe.Channel || snap.AppID != safe.AppID || snap.EnvironmentID != safe.EnvironmentID || snap.AccountID != safe.AccountID || snap.SenderID != safe.SenderID || snap.Credentials["api_token"] == "" {
		return nil, nil, ErrExotelCallbackAuth
	}
	return n, snap, nil
}
func (s *ExotelStatusService) AcceptCallback(ctx context.Context, id, sid, state string, expires int64, proof string) (string, error) {
	n, snap, e := s.bound(ctx, id)
	if e != nil {
		return "", e
	}
	now := time.Now().UTC()
	if expires < now.Unix() || expires > now.Add(8*24*time.Hour).Unix() || !hmac.Equal([]byte(proof), []byte(ExotelCallbackProof(snap, n, expires))) {
		return "", ErrExotelCallbackAuth
	}
	original, _ := n.Metadata["provider_message_id"].(string)
	if original == "" {
		return "", fmt.Errorf("provider acceptance not yet persisted")
	}
	if sid != original {
		return "", ErrExotelCallbackAuth
	}
	switch state {
	case "queued", "sending", "submitted", "sent", "failed", "failed-dnd":
	default:
		return "", ErrExotelCallbackAuth
	}
	if s.Events == nil {
		return "", fmt.Errorf("SMS callback store unavailable")
	}
	receiptID := fmt.Sprintf("%x", sha256.Sum256([]byte("exotel\x00"+id+"\x00"+sid+"\x00"+state)))
	binding := whatsapp.InboundSenderBinding{ID: snap.ConnectionID, AppID: n.AppID, EnvironmentID: snap.EnvironmentID, Provider: "exotel", AccountID: snap.AccountID, SenderID: snap.SenderID}
	r := &whatsapp.EventReceipt{ID: receiptID, State: "pending", Steps: map[string]string{}, CreatedAt: now, UpdatedAt: now, NextAttempt: now, LeaseUntil: now, Event: whatsapp.VerifiedEvent{ID: receiptID, Kind: "exotel_status", Binding: binding, Status: &whatsapp.DeliveryStatus{NotificationID: id, MetaMessageID: sid, Status: state, Timestamp: now}}}
	if e = s.Events.AcceptEvent(ctx, r); e != nil {
		return "", e
	}
	return receiptID, nil
}
func (s *ExotelStatusService) Process(ctx context.Context, id string) error {
	r, e := s.Events.ClaimEvent(ctx, id, time.Now().UTC())
	if e != nil || r == nil {
		return e
	}
	if r.Event.Kind != "exotel_status" || r.Event.Status == nil {
		e = fmt.Errorf("invalid SMS receipt")
	} else {
		e = s.project(ctx, r)
	}
	if e != nil {
		r.LastError = "authenticated SMS status recovery unavailable"
		r.State = "pending"
		r.NextAttempt = time.Now().UTC().Add(time.Duration(r.Attempts) * 10 * time.Second)
		if r.Attempts >= 5 {
			r.State = "dead_letter"
		}
	} else {
		r.State = "done"
		r.LastError = ""
	}
	r.LeaseUntil = time.Time{}
	if save := s.Events.SaveEvent(ctx, r); save != nil {
		return save
	}
	return e
}
func exotelSenderMatches(from, sender string) bool {
	parts := strings.Split(from, "/")
	return parts[len(parts)-1] == sender
}
func exotelDomesticPhone(phone string) string {
	if len(phone) == 10 {
		return "+91" + phone
	}
	return phone
}
func (s *ExotelStatusService) project(ctx context.Context, r *whatsapp.EventReceipt) error {
	status := r.Event.Status
	n, snap, e := s.bound(ctx, status.NotificationID)
	if e != nil {
		return e
	}
	if n.Metadata["provider_message_id"] != status.MetaMessageID || r.Event.Binding.ID != snap.ConnectionID || r.Event.Binding.AppID != snap.AppID || r.Event.Binding.EnvironmentID != snap.EnvironmentID || r.Event.Binding.AccountID != snap.AccountID || r.Event.Binding.SenderID != snap.SenderID {
		return fmt.Errorf("SMS status binding mismatch")
	}
	msg, e := providers.ReadExotelSMS(ctx, s.Client, snap, status.MetaMessageID)
	if e != nil {
		return e
	}
	destination, _ := n.Metadata["channel_destination"].(string)
	if destination == "" || exotelDomesticPhone(msg.To) != destination || !exotelSenderMatches(msg.From, snap.SenderID) || msg.Body != n.Content.Body {
		return fmt.Errorf("SMS status message mismatch")
	}
	var state notification.Status
	switch msg.Status {
	case "sent":
		state = notification.StatusDelivered
	case "failed", "failed-dnd":
		state = notification.StatusFailed
	default:
		return fmt.Errorf("SMS delivery still pending")
	}
	projection, ok := s.Notifications.(interface {
		ApplyProviderDeliveryStatus(context.Context, string, string, string, notification.Status, time.Time) (bool, error)
	})
	if !ok {
		return fmt.Errorf("SMS status projection unavailable")
	}
	// Vendor local timestamps have no documented zone. Preserve the raw value;
	// receipt UTC is the observation time, not a fabricated provider sent time.
	status.VendorTimestamp = msg.DateUpdated
	_, e = projection.ApplyProviderDeliveryStatus(ctx, n.NotificationID, n.AppID, msg.Sid, state, r.CreatedAt)
	return e
}
func (s *ExotelStatusService) Recover(ctx context.Context) error {
	repo, ok := s.Events.(interface {
		PendingEventsForKind(context.Context, time.Time, int, string) ([]string, error)
	})
	if !ok {
		return fmt.Errorf("SMS recovery store unavailable")
	}
	ids, e := repo.PendingEventsForKind(ctx, time.Now().UTC(), 100, "exotel_status")
	if e != nil {
		return e
	}
	for _, id := range ids {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		_ = s.Process(ctx, id)
	}
	return nil
}
