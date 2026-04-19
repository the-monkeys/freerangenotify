package dto

// ── Twilio Content Template Request DTOs ──

// CreateTwilioTemplateRequest creates a new Twilio Content Template.
type CreateTwilioTemplateRequest struct {
	FriendlyName string                 `json:"friendly_name" validate:"required,min=3,max=100"`
	Language     string                 `json:"language" validate:"required,min=2,max=5"`
	Variables    map[string]string      `json:"variables,omitempty"`
	Types        map[string]interface{} `json:"types" validate:"required"`
}

// SubmitTwilioApprovalRequest submits a template for WhatsApp approval.
type SubmitTwilioApprovalRequest struct {
	Name     string `json:"name" validate:"required,min=1,max=512"`
	Category string `json:"category" validate:"required,oneof=UTILITY MARKETING AUTHENTICATION"`
}

// PreviewTwilioTemplateRequest renders a template with sample data.
type PreviewTwilioTemplateRequest struct {
	Variables map[string]string `json:"variables" validate:"required"`
}
