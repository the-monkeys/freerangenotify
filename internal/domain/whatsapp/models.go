package whatsapp

import "time"

// MessageDirection indicates whether a message was sent or received.
type MessageDirection string

const (
	DirectionInbound  MessageDirection = "inbound"
	DirectionOutbound MessageDirection = "outbound"
)

// InboundMessage represents a WhatsApp message stored in Elasticsearch.
type InboundMessage struct {
	ID            string           `json:"id" es:"id"`
	AppID         string           `json:"app_id" es:"app_id"`
	WABAID        string           `json:"waba_id" es:"waba_id"`
	PhoneNumberID string           `json:"phone_number_id" es:"phone_number_id"`
	ContactWAID   string           `json:"contact_wa_id" es:"contact_wa_id"`
	ContactName   string           `json:"contact_name" es:"contact_name"`
	UserID        string           `json:"user_id,omitempty" es:"user_id"`
	Direction     MessageDirection `json:"direction" es:"direction"`
	MessageType   string           `json:"message_type" es:"message_type"` // text, image, video, audio, document, location, contacts, interactive, reaction, order
	MetaMessageID string           `json:"meta_message_id" es:"meta_message_id"`
	Timestamp     time.Time        `json:"timestamp" es:"timestamp"`

	TextBody      string                 `json:"text_body,omitempty" es:"text_body"`
	MediaURL      string                 `json:"media_url,omitempty" es:"media_url"`
	MediaMimeType string                 `json:"media_mime_type,omitempty" es:"media_mime_type"`
	Latitude      float64                `json:"latitude,omitempty" es:"latitude"`
	Longitude     float64                `json:"longitude,omitempty" es:"longitude"`
	RawPayload    map[string]interface{} `json:"raw_payload,omitempty" es:"raw_payload"`

	ContextMessageID string `json:"context_message_id,omitempty" es:"context_message_id"`
	IsForwarded      bool   `json:"is_forwarded,omitempty" es:"is_forwarded"`

	CreatedAt time.Time `json:"created_at" es:"created_at"`
}

// DeliveryStatus represents a status webhook event from Meta.
type DeliveryStatus struct {
	MetaMessageID      string    `json:"meta_message_id"`
	NotificationID     string    `json:"notification_id,omitempty"`
	Status             string    `json:"status"` // sent, delivered, read, failed
	Timestamp          time.Time `json:"timestamp"`
	RecipientID        string    `json:"recipient_id"`
	ConversationID     string    `json:"conversation_id,omitempty"`
	ConversationOrigin string    `json:"conversation_origin,omitempty"` // user_initiated, business_initiated, referral_conversion
	Billable           bool      `json:"billable,omitempty"`
	PricingCategory    string    `json:"pricing_category,omitempty"` // service, authentication, marketing, utility
	ErrorCode          int       `json:"error_code,omitempty"`
	ErrorMessage       string    `json:"error_message,omitempty"`
}

// InboundRouteAction determines how inbound messages are handled.
type InboundRouteAction string

const (
	RouteAutoReply      InboundRouteAction = "auto_reply"
	RouteWorkflow       InboundRouteAction = "workflow_trigger"
	RouteWebhookForward InboundRouteAction = "webhook_forward"
	RouteInbox          InboundRouteAction = "inbox"
)

// InboundConfig is per-app configuration for inbound WhatsApp message handling.
// Stored in application.Settings.WhatsAppInbound.
type InboundConfig struct {
	Enabled             bool               `json:"enabled" es:"enabled"`
	RouteAction         InboundRouteAction `json:"route_action" es:"route_action"`
	AutoReplyText       string             `json:"auto_reply_text,omitempty" es:"auto_reply_text"`
	AutoReplyTemplateID string             `json:"auto_reply_template_id,omitempty" es:"auto_reply_template_id"`
	WorkflowTriggerID   string             `json:"workflow_trigger_id,omitempty" es:"workflow_trigger_id"`
	WebhookForwardURL   string             `json:"webhook_forward_url,omitempty" es:"webhook_forward_url"`
}

// Conversation is an aggregated view grouping messages by contact.
type Conversation struct {
	ContactWAID    string    `json:"contact_wa_id"`
	ContactName    string    `json:"contact_name"`
	LastMessage    string    `json:"last_message"`
	LastMessageAt  time.Time `json:"last_message_at"`
	LastDirection  string    `json:"last_direction"`
	UnreadCount    int64     `json:"unread_count"`
	CSWOpen        bool      `json:"csw_open"`
}

// MessageFilter is used to query stored WhatsApp messages.
type MessageFilter struct {
	AppID       string `json:"app_id"`
	ContactWAID string `json:"contact_wa_id,omitempty"`
	Direction   string `json:"direction,omitempty"`
	Limit       int    `json:"limit,omitempty"`
	Offset      int    `json:"offset,omitempty"`
}
