package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// Fetch only from the signed callback's account, checking both endpoints.
func (s *ChannelDispatchService) ResolveTwilioInboundTimestamp(ctx context.Context, b *InboundSenderBinding, messageID, contact string) (time.Time, error) {
	safe := regexp.MustCompile(`^[A-Za-z0-9]+$`)
	if b == nil || b.Provider != "twilio" || b.SigningSecret == "" || !safe.MatchString(b.AccountID) || !safe.MatchString(messageID) {
		return time.Time{}, fmt.Errorf("invalid Twilio message identity")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages/%s.json", b.AccountID, messageID), nil)
	if err != nil {
		return time.Time{}, fmt.Errorf("timestamp request invalid")
	}
	req.SetBasicAuth(b.AccountID, b.SigningSecret)
	client := s.Client
	if client == nil {
		client = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	resp, err := client.Do(req)
	if err != nil {
		return time.Time{}, fmt.Errorf("provider timestamp unavailable")
	}
	defer resp.Body.Close()
	var row struct {
		Account string `json:"account_sid"`
		SID     string `json:"sid"`
		To      string `json:"to"`
		From    string `json:"from"`
		Created string `json:"date_created"`
	}
	canonical := func(v string) string { return strings.TrimPrefix(strings.TrimPrefix(v, "whatsapp:"), "+") }
	if resp.StatusCode != 200 || json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&row) != nil || row.Account != b.AccountID || row.SID != messageID || canonical(row.To) != canonical(b.SenderID) || canonical(row.From) != canonical(contact) {
		return time.Time{}, fmt.Errorf("provider message ownership unconfirmed")
	}
	for _, layout := range []string{time.RFC1123Z, time.RFC1123, time.RFC3339} {
		if stamp, e := time.Parse(layout, row.Created); e == nil && !stamp.IsZero() && !stamp.After(time.Now().Add(time.Minute)) {
			return stamp, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid provider timestamp")
}
