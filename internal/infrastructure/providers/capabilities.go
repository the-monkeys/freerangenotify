package providers

// AttachmentMode describes how a provider accepts attachment bytes.
//
// The mode determines where the heavy lifting happens (resolver, provider,
// or both) and what the provider expects in the Notification it receives.
type AttachmentMode int

const (
	// AttachmentModeNone — the channel cannot deliver binary attachments at all
	// (e.g. SMS without MMS, plain push without a media URL).
	AttachmentModeNone AttachmentMode = iota

	// AttachmentModeInline — the provider expects bytes already attached to the
	// outbound payload (MIME parts for email, multipart blob for Slack, etc.).
	AttachmentModeInline

	// AttachmentModeMultipart — the provider performs its own multipart upload
	// to a 3rd-party API (Discord's multipart/form-data, Mailgun's form-data).
	AttachmentModeMultipart

	// AttachmentModePreUpload — the provider must upload bytes to the vendor
	// first and reference the returned media id in the payload (Twilio MMS,
	// WhatsApp Cloud media, APNs file-provider).
	AttachmentModePreUpload

	// AttachmentModeSignedURL — the provider only accepts a public/signed URL
	// pointing back to FRN's /files endpoint (webhook, custom in-app channels).
	AttachmentModeSignedURL
)

// String makes AttachmentMode log-friendly.
func (m AttachmentMode) String() string {
	switch m {
	case AttachmentModeNone:
		return "none"
	case AttachmentModeInline:
		return "inline"
	case AttachmentModeMultipart:
		return "multipart"
	case AttachmentModePreUpload:
		return "pre_upload"
	case AttachmentModeSignedURL:
		return "signed_url"
	default:
		return "unknown"
	}
}

// Capabilities advertises what a provider can do with attachments.
//
// Resolver code consults Capabilities before staging bytes so we never load a
// 25 MB PDF into memory only to discover the channel rejects it. Zero values
// mean "no limit known" except for AttachmentMode, whose zero value is
// AttachmentModeNone — i.e. safe-by-default.
type Capabilities struct {
	// AttachmentMode describes how the provider wants attachment bytes.
	AttachmentMode AttachmentMode

	// MaxAttachmentBytes is the per-attachment hard limit advertised by the
	// channel/vendor. 0 means unknown — caller should fall back to a global cap.
	MaxAttachmentBytes int64

	// MaxAttachmentCount is the maximum number of attachments per outbound
	// message. 0 means unknown / no per-message ceiling.
	MaxAttachmentCount int

	// AllowedMIMETypes is the whitelist enforced by the channel. nil/empty
	// means "no channel-side restriction" (the global allowlist still applies).
	AllowedMIMETypes []string

	// SupportsInlineCID indicates whether the channel can render
	// `cid:<content_id>` references inside an HTML body (email-style inline
	// images). Only relevant when AttachmentMode is Inline or Multipart.
	SupportsInlineCID bool
}

// DefaultCapabilities returns the conservative default: no attachments.
// Providers that haven't been audited keep their existing behavior.
func DefaultCapabilities() Capabilities {
	return Capabilities{
		AttachmentMode:     AttachmentModeNone,
		MaxAttachmentBytes: 0,
		MaxAttachmentCount: 0,
		AllowedMIMETypes:   nil,
		SupportsInlineCID:  false,
	}
}
