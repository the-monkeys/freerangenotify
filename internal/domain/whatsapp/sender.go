package whatsapp

import "context"

type callbackContextKey string

const CallbackAppKey callbackContextKey = "whatsapp_callback_app"

type InboundSenderBinding struct {
	ID            string `json:"id"`
	AppID         string `json:"app_id"`
	EnvironmentID string `json:"environment_id"`
	Provider      string `json:"provider"`
	AccountID     string `json:"account_id"`
	SenderID      string `json:"sender_id"`
	SigningSecret string `json:"-"`
}
type SenderBindingStore interface {
	ClaimSender(context.Context, *InboundSenderBinding) error
}
