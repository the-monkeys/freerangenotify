package whatsapp

import (
	"context"
	"time"
)

// RichTemplateKind enumerates the rich-messaging template types supported by
// FreeRangeNotify across Meta Cloud API and Twilio Content API. Each kind has
// a matching renderer in the provider packages and a validation rule set in
// validate.go. Adding a new kind requires: (a) extending the validator,
// (b) wiring it in MetaWhatsAppProvider.buildRichMessage and the Twilio
// provider, and (c) (when persisted) extending the rich-template
// authoring service to translate the FRN-internal shape to the provider's
// upstream submission JSON.
type RichTemplateKind string

const (
	KindCarousel      RichTemplateKind = "carousel"
	KindCouponCode    RichTemplateKind = "coupon_code"
	KindCTAURL        RichTemplateKind = "cta_url"
	KindQuickReply    RichTemplateKind = "quick_reply"
	KindList          RichTemplateKind = "list"
	KindSingleProduct RichTemplateKind = "single_product"
	KindMultiProduct  RichTemplateKind = "multi_product"
	KindCatalog       RichTemplateKind = "catalog"
)

// ButtonType enumerates the WhatsApp template button types we expose.
// CALL is intentionally absent for now — Meta supports a PHONE_NUMBER button
// but our authoring API only ships the four below in the first cut.
type ButtonType string

const (
	ButtonQuickReply ButtonType = "QUICK_REPLY"
	ButtonURL        ButtonType = "URL"
	ButtonPhone      ButtonType = "PHONE_NUMBER"
	ButtonCopyCode   ButtonType = "COPY_CODE"
)

// ApprovalState reflects the *aggregate* approval status across Meta and
// Twilio. Per-provider statuses live in ProviderBindings; ApprovalState is
// the rolled-up view the UI surfaces on the template list.
type ApprovalState string

const (
	ApprovalDraft               ApprovalState = "draft"               // never submitted
	ApprovalPending             ApprovalState = "pending"             // submitted, awaiting at least one provider
	ApprovalPartiallySubmitted  ApprovalState = "partially_submitted" // submitted to one provider, the other failed
	ApprovalApproved            ApprovalState = "approved"            // approved by at least one configured provider
	ApprovalRejected            ApprovalState = "rejected"            // rejected by all configured providers
	ApprovalDisabled            ApprovalState = "disabled"            // approved then disabled by provider (rare)
)

// RichTemplate is the FRN-internal representation of a rich WhatsApp
// template. Persisted in Elasticsearch (index `whatsapp_rich_templates`) and
// keyed by `(tenant_id, app_id, name)` enforced at the service layer (Meta
// itself enforces uniqueness of name per WABA).
//
// The send-time payload (notification.Content.Data["whatsapp_rich"]) does
// NOT use this struct directly — it carries `template_id` plus runtime
// variables and the service resolves the binding. RichTemplate is the
// authoring shape; the runtime shape is intentionally narrower.
type RichTemplate struct {
	ID        string `json:"id" es:"id"`             // FRN-internal ID (frn_tpl_*)
	TenantID  string `json:"tenant_id" es:"tenant_id"`
	AppID     string `json:"app_id" es:"app_id"`
	Name      string `json:"name" es:"name"`         // snake_case, unique per app
	Kind      RichTemplateKind `json:"kind" es:"kind"`
	Language  string `json:"language" es:"language"` // BCP-47, e.g. en_US
	Category  string `json:"category" es:"category"` // MARKETING | UTILITY | AUTHENTICATION

	// Authoring payload (provider-agnostic). Optional per kind:
	//   carousel:     Cards (required, 2..10)
	//   coupon_code:  Body, CouponCode
	//   cta_url:      Body, Buttons (1 URL button)
	//   quick_reply:  Body, Buttons (1..3 QUICK_REPLY buttons)
	//   list:         Body, ListSections, ListButtonText
	Body            string           `json:"body,omitempty" es:"body"`
	Header          *Header          `json:"header,omitempty" es:"header"`
	Footer          string           `json:"footer,omitempty" es:"footer"`
	Cards           []CarouselCard   `json:"cards,omitempty" es:"cards"`
	Buttons         []Button         `json:"buttons,omitempty" es:"buttons"`
	CouponCode      string           `json:"coupon_code,omitempty" es:"coupon_code"`
	ListButtonText  string           `json:"list_button_text,omitempty" es:"list_button_text"`
	ListSections    []ListSection    `json:"list_sections,omitempty" es:"list_sections"`

	// Provider linkage (filled after submission).
	Providers     ProviderBindings `json:"providers" es:"providers"`
	ApprovalState ApprovalState    `json:"approval_state" es:"approval_state"`

	CreatedAt time.Time `json:"created_at" es:"created_at"`
	UpdatedAt time.Time `json:"updated_at" es:"updated_at"`
}

// Header is a template header. Exactly one of Text / ImageURL / VideoURL /
// DocumentURL is set; the validator rejects multiples.
type Header struct {
	Text        string `json:"text,omitempty" es:"text"`
	ImageURL    string `json:"image_url,omitempty" es:"image_url"`
	VideoURL    string `json:"video_url,omitempty" es:"video_url"`
	DocumentURL string `json:"document_url,omitempty" es:"document_url"`
}

// CarouselCard is one card in a carousel template. Cards must share the same
// header media type across the carousel (enforced by the validator).
type CarouselCard struct {
	HeaderImageURL string   `json:"header_image_url,omitempty" es:"header_image_url"`
	HeaderVideoURL string   `json:"header_video_url,omitempty" es:"header_video_url"`
	Body           string   `json:"body" es:"body"`             // ≤160 chars
	Variables      []string `json:"variables,omitempty" es:"variables"` // names matching {{1}}..{{n}}
	Buttons        []Button `json:"buttons" es:"buttons"`       // 1..2 per card
}

// Button is a single template button. The relevant fields depend on Type:
//   QUICK_REPLY: Text + Payload
//   URL:         Text + URL (with optional {{1}} suffix variable + Example)
//   PHONE_NUMBER: Text + PhoneNumber
//   COPY_CODE:   Text + CouponCode
//
// TrackClicks=true wraps the URL in `/v1/r/{sig}` at send-time for click
// attribution (Phase 3). Only meaningful for URL buttons.
type Button struct {
	Type        ButtonType `json:"type" es:"type"`
	Text        string     `json:"text" es:"text"`
	URL         string     `json:"url,omitempty" es:"url"`
	Payload     string     `json:"payload,omitempty" es:"payload"`
	PhoneNumber string     `json:"phone_number,omitempty" es:"phone_number"`
	CouponCode  string     `json:"coupon_code,omitempty" es:"coupon_code"`
	Example     string     `json:"example,omitempty" es:"example"` // required by Meta for URL with {{n}}
	TrackClicks bool       `json:"track_clicks,omitempty" es:"track_clicks"`
}

// ListSection is one section in a list-picker template.
type ListSection struct {
	Title string    `json:"title" es:"title"`
	Rows  []ListRow `json:"rows" es:"rows"` // 1..10 per section
}

// ListRow is one row inside a list section.
type ListRow struct {
	ID          string `json:"id" es:"id"`                     // returned as ReplyID on tap
	Title       string `json:"title" es:"title"`               // ≤24 chars
	Description string `json:"description,omitempty" es:"description"` // ≤72 chars
}

// ProviderBindings holds the per-provider IDs and approval status assigned
// by Meta / Twilio after submission. Either binding may be nil if the app
// is not configured for that provider.
type ProviderBindings struct {
	Meta   *MetaBinding   `json:"meta,omitempty" es:"meta"`
	Twilio *TwilioBinding `json:"twilio,omitempty" es:"twilio"`
}

// MetaBinding is the Meta side of a submitted template.
type MetaBinding struct {
	TemplateName string    `json:"template_name" es:"template_name"`
	TemplateID   string    `json:"template_id" es:"template_id"`
	Status       string    `json:"status" es:"status"`             // APPROVED | PENDING | REJECTED | DISABLED
	Reason       string    `json:"reason,omitempty" es:"reason"`
	SubmittedAt  time.Time `json:"submitted_at,omitempty" es:"submitted_at"`
	UpdatedAt    time.Time `json:"updated_at,omitempty" es:"updated_at"`
}

// TwilioBinding is the Twilio Content API side of a submitted template.
type TwilioBinding struct {
	ContentSid  string    `json:"content_sid" es:"content_sid"`
	ApprovalSid string    `json:"approval_sid,omitempty" es:"approval_sid"`
	Status      string    `json:"status" es:"status"`           // approved | pending | rejected | unsubmitted
	Reason      string    `json:"reason,omitempty" es:"reason"`
	SubmittedAt time.Time `json:"submitted_at,omitempty" es:"submitted_at"`
	UpdatedAt   time.Time `json:"updated_at,omitempty" es:"updated_at"`
}

// SendPayload is the runtime shape carried in
// notification.Content.Data["whatsapp_rich"] for typed rich sends. It is
// resolved server-side against a RichTemplate to produce provider-specific
// JSON.
//
// Fields are union-style: TemplateID is the FRN-internal ID; the worker
// loads the template and maps Variables / Cards onto its components.
type SendPayload struct {
	TemplateID string                 `json:"template_id"`
	Variables  map[string]string      `json:"variables,omitempty"`
	Cards      []SendPayloadCard      `json:"cards,omitempty"`
}

// SendPayloadCard is per-card runtime overrides for a carousel send.
type SendPayloadCard struct {
	HeaderImageURL string            `json:"header_image_url,omitempty"`
	HeaderVideoURL string            `json:"header_video_url,omitempty"`
	Variables      map[string]string `json:"variables,omitempty"`
	// ButtonValues fills the URL suffix or payload variable for each button
	// in card order. nil means "no runtime substitution required".
	ButtonValues []string `json:"button_values,omitempty"`
}

// RichTemplateFilter describes a query for listing rich templates.
type RichTemplateFilter struct {
	TenantID string `json:"tenant_id"`
	AppID    string `json:"app_id"`
	Kind     string `json:"kind,omitempty"`
	Status   string `json:"status,omitempty"` // aggregate ApprovalState
	NamePrefix string `json:"name_prefix,omitempty"`
	Limit  int `json:"limit,omitempty"`
	Offset int `json:"offset,omitempty"`
}

// RichTemplateRepository is the persistence interface for RichTemplate.
// Implemented by infrastructure/repository/whatsapp_rich_template_repo.go.
type RichTemplateRepository interface {
	Create(ctx context.Context, tpl *RichTemplate) error
	GetByID(ctx context.Context, id string) (*RichTemplate, error)
	GetByName(ctx context.Context, appID, name string) (*RichTemplate, error)
	List(ctx context.Context, filter RichTemplateFilter) ([]*RichTemplate, int64, error)
	Update(ctx context.Context, tpl *RichTemplate) error
	Delete(ctx context.Context, id string) error
}
