package whatsapp

import (
	"context"
	"time"
)

type VerifiedEvent struct {
	ID       string               `json:"id"`
	Binding  InboundSenderBinding `json:"binding"`
	Kind     string               `json:"kind"`
	Message  *InboundMessage      `json:"message,omitempty"`
	Status   *DeliveryStatus      `json:"status,omitempty"`
	Template *TemplateStatusEvent `json:"template,omitempty"`
}
type TemplateStatusEvent struct {
	TemplateID string `json:"template_id"`
	Status     string `json:"status"`
	Reason     string `json:"reason,omitempty"`
}
type EventReceipt struct {
	ID          string            `json:"id"`
	Event       VerifiedEvent     `json:"event"`
	State       string            `json:"state"`
	Steps       map[string]string `json:"steps"`
	LeaseToken  string            `json:"lease_token,omitempty"`
	LeaseUntil  time.Time         `json:"lease_until"`
	Attempts    int               `json:"attempts"`
	NextAttempt time.Time         `json:"next_attempt"`
	LastError   string            `json:"last_error,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Sequence    int64             `json:"-"`
	PrimaryTerm int64             `json:"-"`
}
type EventRepository interface {
	AcceptEvent(context.Context, *EventReceipt) error
	ClaimEvent(context.Context, string, time.Time) (*EventReceipt, error)
	SaveEvent(context.Context, *EventReceipt) error
	PendingEvents(context.Context, time.Time, int) ([]string, error)
}
