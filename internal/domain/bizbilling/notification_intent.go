package bizbilling

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
)

// NotificationIntent is a durable promise to attempt notification admission,
// not proof that a provider delivered the message.
type NotificationIntent struct {
	ID         string            `json:"id"`
	AppID      string            `json:"app_id"`
	UserID     string            `json:"user_id"`
	TemplateID string            `json:"template_id"`
	EventID    string            `json:"event_id"`
	InvoiceID  string            `json:"invoice_id,omitempty"`
	StepID     string            `json:"step_id,omitempty"`
	Channel    string            `json:"channel"`
	Payload    map[string]string `json:"payload"`
}

type NotificationDelivery struct {
	ID                string                             `json:"id"`
	AppID             string                             `json:"app_id"`
	Scope             Scope                              `json:"scope"`
	Sequence          int64                              `json:"billing_sequence"`
	Intent            NotificationIntent                 `json:"intent"`
	NotificationID    string                             `json:"notification_id"`
	Status            string                             `json:"status"`
	FailureCode       string                             `json:"failure_code,omitempty"`
	RetryAfterAttempt int                                `json:"retry_after_attempt"`
	Attempts          int                                `json:"attempts"`
	NextAttemptAt     time.Time                          `json:"next_attempt_at"`
	LeaseOwner        string                             `json:"lease_owner,omitempty"`
	LeaseUntil        time.Time                          `json:"lease_until,omitempty"`
	MayHaveDispatched bool                               `json:"may_have_dispatched"`
	SensitivePayload  *channelconnection.EncryptedSecret `json:"sensitive_payload,omitempty"`
	CreatedAt         time.Time                          `json:"created_at"`
	UpdatedAt         time.Time                          `json:"updated_at"`
}

func NotificationIntentID(intent NotificationIntent) string {
	data, _ := json.Marshal([]string{intent.AppID, intent.EventID, intent.TemplateID, intent.StepID, intent.Channel})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
