package billing

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"net/url"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"
)

const ChannelQuoteMetadataKey = "channel_quote"

type ChannelRate struct {
	Channel                string `json:"channel"`
	Provider               string `json:"provider"`
	CredentialSource       string `json:"credential_source"`
	DestinationPrefix      string `json:"destination_prefix"`
	Country                string `json:"country"`
	Category               string `json:"category"`
	CreditsPerUnit         int64  `json:"credits_per_unit"`
	VendorCostMinorPerUnit int64  `json:"vendor_cost_minor_per_unit"`
	VendorCurrency         string `json:"vendor_currency"`
}

type ChannelQuote struct {
	ID                       string                     `json:"id"`
	RateCardVersion          string                     `json:"rate_card_version"`
	PricingModel             string                     `json:"pricing_model"`
	Connection               channelconnection.Snapshot `json:"connection"`
	DeliveryEnvironment      string                     `json:"delivery_environment"`
	PayloadHash              string                     `json:"payload_hash"`
	Channel                  string                     `json:"channel"`
	Units                    int64                      `json:"units"`
	Encoding                 string                     `json:"encoding,omitempty"`
	Credits                  int64                      `json:"credits"`
	Category                 string                     `json:"category"`
	Country                  string                     `json:"country"`
	EstimatedVendorCostMinor int64                      `json:"estimated_vendor_cost_minor"`
	VendorCurrency           string                     `json:"vendor_currency,omitempty"`
	VendorCostStatus         string                     `json:"vendor_cost_status"`
	Attempt                  int                        `json:"attempt,omitempty"`
}

func (q *ChannelQuote) Fingerprint() string {
	copy := *q
	copy.ID = ""
	copy.Attempt = 0
	copy.Connection.Credentials = nil
	b, _ := json.Marshal(copy)
	return fmt.Sprintf("%x", sha256.Sum256(b))
}
func (q *ChannelQuote) Validate() error {
	if q == nil || q.Credits <= 0 || q.Units <= 0 || q.RateCardVersion == "" || q.ID != q.Fingerprint() || q.Attempt < 0 {
		return fmt.Errorf("invalid immutable channel quote")
	}
	return nil
}

// Counts wire units, including escaped GSM characters and UTF-16 surrogate pairs.
func SMSUnits(body string) (string, int64, error) {
	if body == "" || !utf8.ValidString(body) {
		return "", 0, fmt.Errorf("SMS body must be valid nonempty text")
	}
	basic := "@£$¥èéùìòÇ\nØø\rÅåΔ_ΦΓΛΩΠΨΣΘΞÆæßÉ !\"#¤%&'()*+,-./0123456789:;<=>?¡ABCDEFGHIJKLMNOPQRSTUVWXYZÄÖÑÜ§¿abcdefghijklmnopqrstuvwxyzäöñüà"
	ext := "\f^{}\\[~]|€"
	units := int64(0)
	gsm := true
	for _, r := range body {
		if strings.ContainsRune(basic, r) {
			units++
		} else if strings.ContainsRune(ext, r) {
			units += 2
		} else {
			gsm = false
			break
		}
	}
	if gsm {
		if units <= 160 {
			return "GSM-7", 1, nil
		}
		return "GSM-7", (units + 152) / 153, nil
	}
	units = int64(len(utf16.Encode([]rune(body))))
	if units <= 70 {
		return "UCS-2", 1, nil
	}
	return "UCS-2", (units + 66) / 67, nil
}

// Publication is an operator decision, backed by dated provider/account rates.
func (c *RateCard) ValidatePublication() error {
	if c == nil || c.Version == "" {
		return fmt.Errorf("rate version required")
	}
	if c.PricingModel == "" || c.PricingModel == "flat" {
		return nil
	}
	if c.PricingModel != "detailed-v1" || c.RatesVerifiedAt.IsZero() || c.RatesVerifiedAt.After(time.Now().Add(time.Minute)) || len(c.RateReferences) == 0 || len(c.DetailedRates) == 0 {
		return fmt.Errorf("detailed rates require verified dated provider references")
	}
	for _, ref := range c.RateReferences {
		u, e := url.Parse(ref)
		if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
			return fmt.Errorf("invalid provider rate reference")
		}
	}
	seen := map[string]bool{}
	for _, r := range c.DetailedRates {
		key := strings.Join([]string{r.Channel, r.Provider, r.CredentialSource, r.DestinationPrefix, r.Category}, "\x00")
		if seen[key] || r.CreditsPerUnit <= 0 || r.VendorCostMinorPerUnit < 0 || len(r.VendorCurrency) != 3 || r.Country == "" || r.Channel != "sms" && r.Channel != "whatsapp" || r.CredentialSource != "system" && r.CredentialSource != "byoc" {
			return fmt.Errorf("invalid detailed rate dimensions")
		}
		seen[key] = true
	}
	return nil
}
