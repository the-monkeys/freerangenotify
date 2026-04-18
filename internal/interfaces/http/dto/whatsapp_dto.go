package dto

// MetaWebhookPayload is the top-level webhook payload from Meta Cloud API.
type MetaWebhookPayload struct {
	Object string           `json:"object"`
	Entry  []MetaWebhookEntry `json:"entry"`
}

// MetaWebhookEntry represents a single entry in the webhook payload.
type MetaWebhookEntry struct {
	ID      string                `json:"id"` // WABA ID
	Changes []MetaWebhookChange   `json:"changes"`
}

// MetaWebhookChange represents a change within an entry.
type MetaWebhookChange struct {
	Value MetaWebhookValue `json:"value"`
	Field string           `json:"field"` // "messages"
}

// MetaWebhookValue contains the actual webhook event data.
type MetaWebhookValue struct {
	MessagingProduct string              `json:"messaging_product"`
	Metadata         MetaWebhookMetadata `json:"metadata"`
	Contacts         []MetaContact       `json:"contacts,omitempty"`
	Messages         []MetaInboundMsg    `json:"messages,omitempty"`
	Statuses         []MetaStatus        `json:"statuses,omitempty"`
	Errors           []MetaWebhookError  `json:"errors,omitempty"`
}

// MetaWebhookMetadata contains the phone number info from the webhook.
type MetaWebhookMetadata struct {
	DisplayPhoneNumber string `json:"display_phone_number"`
	PhoneNumberID      string `json:"phone_number_id"`
}

// MetaContact represents a sender contact in an inbound message.
type MetaContact struct {
	Profile MetaContactProfile `json:"profile"`
	WAID    string             `json:"wa_id"`
}

// MetaContactProfile holds profile info for a contact.
type MetaContactProfile struct {
	Name string `json:"name"`
}

// MetaInboundMsg represents a single inbound WhatsApp message.
type MetaInboundMsg struct {
	From      string               `json:"from"`
	ID        string               `json:"id"`
	Timestamp string               `json:"timestamp"`
	Type      string               `json:"type"` // text, image, video, audio, document, location, contacts, interactive, reaction, order
	Text      *MetaMsgText         `json:"text,omitempty"`
	Image     *MetaMsgMedia        `json:"image,omitempty"`
	Video     *MetaMsgMedia        `json:"video,omitempty"`
	Audio     *MetaMsgMedia        `json:"audio,omitempty"`
	Document  *MetaMsgMedia        `json:"document,omitempty"`
	Location  *MetaMsgLocation     `json:"location,omitempty"`
	Context   *MetaMsgContext      `json:"context,omitempty"`
	Referral  *MetaMsgReferral     `json:"referral,omitempty"`
}

// MetaMsgText is the text content of a message.
type MetaMsgText struct {
	Body string `json:"body"`
}

// MetaMsgMedia is a media attachment (image, video, audio, document).
type MetaMsgMedia struct {
	ID       string `json:"id"`
	MimeType string `json:"mime_type"`
	SHA256   string `json:"sha256"`
	Caption  string `json:"caption,omitempty"`
}

// MetaMsgLocation is a location message.
type MetaMsgLocation struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Name      string  `json:"name,omitempty"`
	Address   string  `json:"address,omitempty"`
}

// MetaMsgContext holds context (reply-to) info.
type MetaMsgContext struct {
	From    string `json:"from"`
	ID      string `json:"id"`
	Forwarded bool `json:"forwarded,omitempty"`
}

// MetaMsgReferral holds referral info (ad click, etc).
type MetaMsgReferral struct {
	SourceURL  string `json:"source_url"`
	SourceType string `json:"source_type"`
	SourceID   string `json:"source_id"`
	Headline   string `json:"headline"`
	Body       string `json:"body"`
}

// MetaStatus represents a message delivery status update.
type MetaStatus struct {
	ID           string                `json:"id"` // meta message ID
	Status       string                `json:"status"` // sent, delivered, read, failed
	Timestamp    string                `json:"timestamp"`
	RecipientID  string                `json:"recipient_id"`
	Conversation *MetaConversation     `json:"conversation,omitempty"`
	Pricing      *MetaPricing          `json:"pricing,omitempty"`
	Errors       []MetaWebhookError    `json:"errors,omitempty"`
}

// MetaConversation holds conversation info from a status webhook.
type MetaConversation struct {
	ID     string         `json:"id"`
	Origin MetaOrigin     `json:"origin"`
}

// MetaOrigin is the origination info for a conversation.
type MetaOrigin struct {
	Type string `json:"type"` // user_initiated, business_initiated, referral_conversion
}

// MetaPricing holds pricing info from a status webhook.
type MetaPricing struct {
	Billable        bool   `json:"billable"`
	PricingModel    string `json:"pricing_model"`
	Category        string `json:"category"` // service, authentication, marketing, utility
}

// MetaWebhookError represents an error in the webhook payload.
type MetaWebhookError struct {
	Code    int    `json:"code"`
	Title   string `json:"title"`
	Message string `json:"message"`
	Details string `json:"error_data,omitempty"`
}
