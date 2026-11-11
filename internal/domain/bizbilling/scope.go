package bizbilling

type Scope struct {
	AppID    string `json:"app_id"`
	UserID   string `json:"user_id"`
	Currency string `json:"currency"`
}
type GatewayMode string

const (
	GatewayTest GatewayMode = "test"
	GatewayLive GatewayMode = "live"
)

type GatewayRef struct {
	ConnectionID       string      `json:"connection_id"`
	AccountID          string      `json:"account_id"`
	Mode               GatewayMode `json:"mode"`
	CredentialRevision int64       `json:"credential_revision"`
}
