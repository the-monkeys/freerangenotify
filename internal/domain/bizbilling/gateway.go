package bizbilling

import (
	"errors"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
)

var (
	ErrMerchantAuthentication = errors.New("payment account authentication failed")
	ErrMerchantUnavailable    = errors.New("payment account verification unavailable")
	ErrMerchantNotReady       = errors.New("payment account is not ready for collection")
)

func (m GatewayMode) Valid() bool { return m == GatewayTest || m == GatewayLive }

// MerchantConnection is a retained credential revision. Account identity never
// changes in place; replacing an account allocates a new connection ID.
type MerchantConnection struct {
	ID               string                                       `json:"id"`
	AppID            string                                       `json:"app_id"`
	AccountID        string                                       `json:"account_id"`
	Mode             GatewayMode                                  `json:"mode"`
	Revision         int64                                        `json:"revision"`
	Readiness        string                                       `json:"readiness"`
	APIAuthenticated bool                                         `json:"api_authenticated"`
	AccountVerified  bool                                         `json:"account_verified"`
	AutomaticReady   bool                                         `json:"automatic_ready"`
	Secrets          map[string]channelconnection.EncryptedSecret `json:"secrets"`
	CreatedAt        time.Time                                    `json:"created_at"`
}

// MerchantPatch is write-only input. No response or log may serialize it.
type MerchantPatch struct {
	AccountID     string  `json:"account_id,omitempty"`
	KeyID         *string `json:"key_id,omitempty"`
	KeySecret     *string `json:"key_secret,omitempty"`
	WebhookSecret *string `json:"webhook_secret,omitempty"`
}

type MerchantSnapshot struct {
	Configured       bool        `json:"configured"`
	ConnectionID     string      `json:"connection_id,omitempty"`
	AppID            string      `json:"app_id"`
	AccountID        string      `json:"account_id,omitempty"`
	Mode             GatewayMode `json:"mode"`
	Revision         int64       `json:"revision"`
	Readiness        string      `json:"readiness"`
	APIAuthenticated bool        `json:"api_authenticated"`
	AccountVerified  bool        `json:"account_verified"`
	AutomaticReady   bool        `json:"automatic_ready"`
}

// Credentials exist only in memory. JSON and ordinary formatting redact them.
type MerchantCredentials struct {
	AppID         string     `json:"-"`
	Reference     GatewayRef `json:"-"`
	KeyID         string     `json:"-"`
	KeySecret     string     `json:"-"`
	WebhookSecret string     `json:"-"`
}

func (MerchantCredentials) String() string { return "[redacted merchant credentials]" }

type MerchantSwitchFence struct {
	Owner            string      `json:"owner"`
	Mode             GatewayMode `json:"mode"`
	ExpectedRevision int64       `json:"expected_revision"`
	PriorStatus      string      `json:"prior_status"`
	LeaseUntil       time.Time   `json:"lease_until"`
}
