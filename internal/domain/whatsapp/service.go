package whatsapp

import "context"

// Service defines the business logic for WhatsApp inbound/status handling.
type Service interface {
	HandleInbound(ctx context.Context, appID string, msg *InboundMessage) error
	HandleStatus(ctx context.Context, status *DeliveryStatus) error
	ListMessages(ctx context.Context, filter *MessageFilter) ([]*InboundMessage, int64, error)
	IsCSWOpen(ctx context.Context, appID, contactWAID string) bool
	ListConversations(ctx context.Context, appID string, limit, offset int) ([]*Conversation, int64, error)
	Reply(ctx context.Context, appID, contactWAID, text, templateName string) error
	MarkRead(ctx context.Context, appID, contactWAID string) error
}

// EventProcessor is optional for legacy service implementations; production
// container wiring requires it before accepting provider callbacks.
type EventProcessor interface {
	AcceptVerifiedEvent(context.Context, *InboundSenderBinding, *VerifiedEvent) (*EventReceipt, error)
	ProcessEventReceipt(context.Context, string) error
	RecoverChannelEvents(context.Context) error
}
type TemplateEventConfigurer interface {
	SetTemplateStatusApplier(func(context.Context, *InboundSenderBinding, *TemplateStatusEvent) error)
}
