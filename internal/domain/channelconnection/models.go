package channelconnection

import (
	"crypto/sha256"
	"fmt"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"time"
)

type EncryptedSecret struct {
	Version    int    `json:"version"`
	KeyID      string `json:"key_id"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}
type Connection struct {
	ID               string                     `json:"id"`
	AppID            string                     `json:"app_id"`
	EnvironmentID    string                     `json:"environment_id"`
	Channel          notification.Channel       `json:"channel"`
	Provider         string                     `json:"provider"`
	CredentialSource string                     `json:"credential_source"`
	AccountID        string                     `json:"account_id"`
	SenderID         string                     `json:"sender_id"`
	Readiness        string                     `json:"readiness"`
	Facts            map[string]string          `json:"facts,omitempty"`
	Revision         int64                      `json:"revision"`
	Secrets          map[string]EncryptedSecret `json:"secrets,omitempty"`
	CreatedAt        time.Time                  `json:"created_at"`
	UpdatedAt        time.Time                  `json:"updated_at"`
}
type Patch struct {
	AppID            string               `json:"-"`
	EnvironmentID    string               `json:"-"`
	Channel          notification.Channel `json:"-"`
	Provider         *string              `json:"provider,omitempty"`
	CredentialSource *string              `json:"credential_source,omitempty"`
	AccountID        *string              `json:"account_id,omitempty"`
	SenderID         *string              `json:"sender_id,omitempty"`
	Secrets          map[string]string    `json:"secrets,omitempty"`
	ClearSecrets     []string             `json:"clear_secrets,omitempty"`
	Disconnect       bool                 `json:"disconnect,omitempty"`
}
type Snapshot struct {
	ConnectionID     string               `json:"connection_id"`
	AppID            string               `json:"app_id"`
	EnvironmentID    string               `json:"environment_id"`
	Channel          notification.Channel `json:"channel"`
	Revision         int64                `json:"revision"`
	Provider         string               `json:"provider"`
	CredentialSource string               `json:"credential_source"`
	AccountID        string               `json:"account_id"`
	SenderID         string               `json:"sender_id"`
	Readiness        string               `json:"readiness"`
	Credentials      map[string]string    `json:"-"`
}

func ID(appID, environmentID string, channel notification.Channel) string {
	if environmentID == "" {
		environmentID = "default"
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(appID+"\x00"+environmentID+"\x00"+string(channel))))
}
