package whatsapp

import "context"

// Repository defines data access for WhatsApp messages.
type Repository interface {
	StoreMessage(ctx context.Context, msg *InboundMessage) error
	GetByMetaMessageID(ctx context.Context, metaMessageID string) (*InboundMessage, error)
	List(ctx context.Context, filter *MessageFilter) ([]*InboundMessage, int64, error)
	ListConversations(ctx context.Context, appID string, limit, offset int) ([]*Conversation, int64, error)
}
