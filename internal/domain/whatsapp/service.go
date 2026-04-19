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
