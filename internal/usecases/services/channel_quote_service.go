package services

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"math"
	"net/http"
	"regexp"
	"strings"
)

type ChannelQuoteService struct {
	Client          *http.Client
	GraphVersion    string
	Rates           billing.RateCardManager
	ResolveCategory func(context.Context, *channelconnection.PreparedDelivery) (string, error)
}

func ChannelPayloadHash(d *channelconnection.PreparedDelivery) string {
	var b []byte
	if d.Snapshot.Channel == notification.ChannelSMS {
		text := d.Notification.Content.Body
		if d.Snapshot.Provider == "vonage" && d.Notification.Content.Title != "" {
			text = d.Notification.Content.Title + ": " + text
		}
		b = []byte(text)
	} else {
		b, _ = json.Marshal(d.Notification.Content)
	}
	// Destination is also immutable: a profile edit cannot redirect an admitted send.
	return fmt.Sprintf("%x", sha256.Sum256(append(b, []byte("\x00"+d.User.Phone)...)))
}
func (s *ChannelQuoteService) QuoteChannelDelivery(ctx context.Context, d *channelconnection.PreparedDelivery) (*billing.ChannelQuote, error) {
	if d == nil || d.Snapshot == nil || d.Notification == nil || d.User == nil {
		return nil, fmt.Errorf("delivery preparation required")
	}
	n, snap := d.Notification, d.Snapshot
	if raw, ok := n.Metadata[billing.ChannelQuoteMetadataKey]; ok {
		b, err := json.Marshal(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid quote")
		}
		q := new(billing.ChannelQuote)
		if json.Unmarshal(b, q) != nil || q.Validate() != nil {
			return nil, fmt.Errorf("invalid quote")
		}
		c := q.Connection
		if c.ConnectionID != snap.ConnectionID || c.Revision != snap.Revision || c.Provider != snap.Provider || c.AccountID != snap.AccountID || c.SenderID != snap.SenderID || c.CredentialSource != snap.CredentialSource || c.AppID != n.AppID || q.DeliveryEnvironment != n.EnvironmentID || q.PayloadHash != ChannelPayloadHash(d) {
			return nil, fmt.Errorf("admitted quote or destination changed")
		}
		return q, nil
	}
	safe := *snap
	safe.Credentials = nil
	q := &billing.ChannelQuote{Connection: safe, DeliveryEnvironment: n.EnvironmentID, PayloadHash: ChannelPayloadHash(d), Channel: string(n.Channel), Units: 1, Category: "unknown", Country: "unknown", PricingModel: "flat", VendorCostStatus: "not_quoted"}
	if n.Channel == notification.ChannelSMS {
		body := n.Content.Body
		if snap.Provider == "vonage" && n.Content.Title != "" {
			body = n.Content.Title + ": " + body
		}
		enc, units, err := billing.SMSUnits(body)
		if err != nil {
			return nil, err
		}
		q.Encoding, q.Units = enc, units
		q.Category = "sms"
	}
	var card *billing.RateCard
	if s.Rates != nil {
		card = s.Rates.GetActiveRateCard()
	}
	if card == nil {
		q.RateCardVersion = "default"
		if n.Channel == notification.ChannelSMS {
			q.Credits = 80
		} else {
			q.Credits = 108
		}
	} else {
		q.RateCardVersion = card.Version
		switch card.PricingModel {
		case "", "flat":
			q.Credits = card.ChannelCreditCost[q.Channel]
			if q.Credits <= 0 {
				return nil, fmt.Errorf("channel price unavailable")
			}
		case "detailed-v1":
			if !regexp.MustCompile(`^\+[1-9][0-9]{6,14}$`).MatchString(nPhone(d.User.Phone)) {
				return nil, fmt.Errorf("E.164 destination required for detailed pricing")
			}
			q.PricingModel = card.PricingModel
			if n.Channel == notification.ChannelWhatsApp {
				q.Category = "service"
				if d.PayloadKind == "template" {
					resolve := s.ResolveCategory
					if resolve == nil {
						resolve = s.lookupProviderCategory
					}
					category, err := resolve(ctx, d)
					if err != nil {
						return nil, err
					}
					q.Category = strings.ToLower(category)
					if q.Category != "marketing" && q.Category != "utility" && q.Category != "authentication" {
						return nil, fmt.Errorf("invalid provider template category")
					}
				}
			}
			var selected *billing.ChannelRate
			score := -1
			for i := range card.DetailedRates {
				r := &card.DetailedRates[i]
				if r.Channel != q.Channel || r.Provider != snap.Provider || r.CredentialSource != snap.CredentialSource || !strings.HasPrefix(nPhone(d.User.Phone), r.DestinationPrefix) || r.Category != "*" && r.Category != q.Category {
					continue
				}
				rank := len(r.DestinationPrefix) * 2
				if r.Category == q.Category {
					rank++
				}
				if rank == score {
					return nil, fmt.Errorf("ambiguous detailed pricing")
				}
				if rank > score {
					selected, score = r, rank
				}
			}
			if selected == nil || selected.CreditsPerUnit <= 0 || selected.CreditsPerUnit > math.MaxInt64/q.Units || selected.VendorCostMinorPerUnit < 0 || selected.VendorCostMinorPerUnit > math.MaxInt64/q.Units || selected.VendorCurrency == "" {
				return nil, fmt.Errorf("verified detailed price unavailable")
			}
			q.Country = selected.Country
			q.Credits = selected.CreditsPerUnit * q.Units
			q.EstimatedVendorCostMinor = selected.VendorCostMinorPerUnit * q.Units
			q.VendorCurrency = selected.VendorCurrency
			q.VendorCostStatus = "estimated"
		default:
			return nil, fmt.Errorf("unsupported pricing model")
		}
	}
	if snap.Provider == "exotel" {
		if snap.Exotel == nil || snap.Exotel.Validate() != nil {
			return nil, fmt.Errorf("verified domestic SMS cost unavailable")
		}
		q.Country = "IN"
		q.EstimatedVendorCostMinor = snap.Exotel.Limits.CostMinorPerUnit * q.Units
		q.VendorCurrency = snap.Exotel.Limits.Currency
		q.VendorCostStatus = "estimated"
	}

	q.ID = q.Fingerprint()
	if err := q.Validate(); err != nil {
		return nil, err
	}
	if n.Metadata == nil {
		n.Metadata = map[string]interface{}{}
	}
	n.Metadata[billing.ChannelQuoteMetadataKey] = q
	return q, nil
}
func nPhone(s string) string { return strings.TrimPrefix(s, "whatsapp:") }

// Older holds predate quote storage. Preserve their admitted amount/version;
// current tariffs cannot retroactively replace that financial admission.
func (s *ChannelQuoteService) QuoteExistingReservation(d *channelconnection.PreparedDelivery, r *billing.CreditReservation) (*billing.ChannelQuote, error) {
	if r.CreditsReserved <= 0 || r.RateCardVersion == "" {
		return nil, fmt.Errorf("legacy admission price unavailable")
	}
	safe := *d.Snapshot
	safe.Credentials = nil
	q := &billing.ChannelQuote{Connection: safe, DeliveryEnvironment: d.Notification.EnvironmentID, PayloadHash: ChannelPayloadHash(d), Channel: string(d.Notification.Channel), Units: 1, Credits: r.CreditsReserved, RateCardVersion: r.RateCardVersion, PricingModel: "flat", Country: "unknown", Category: "unknown", VendorCostStatus: "not_quoted"}
	if q.Channel == "sms" {
		body := d.Notification.Content.Body
		if safe.Provider == "vonage" && d.Notification.Content.Title != "" {
			body = d.Notification.Content.Title + ": " + body
		}
		var e error
		q.Encoding, q.Units, e = billing.SMSUnits(body)
		if e != nil {
			return nil, e
		}
	}
	q.ID = q.Fingerprint()
	if e := q.Validate(); e != nil {
		return nil, e
	}
	d.Notification.Metadata[billing.ChannelQuoteMetadataKey] = q
	return q, nil
}
