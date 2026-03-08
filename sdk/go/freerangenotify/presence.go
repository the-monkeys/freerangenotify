package freerangenotify

import "context"

// PresenceClient handles user presence operations.
type PresenceClient struct {
	client *Client
}

// CheckIn registers a user's presence, enabling smart delivery routing.
// If WebhookURL is provided, it overrides the user's static webhook URL.
func (p *PresenceClient) CheckIn(ctx context.Context, params CheckInParams) error {
	return p.client.do(ctx, "POST", "/presence/check-in", params, nil)
}
