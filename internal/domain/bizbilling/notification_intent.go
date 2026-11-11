package bizbilling

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
