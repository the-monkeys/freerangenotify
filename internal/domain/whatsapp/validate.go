package whatsapp

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Hard limits derived from Meta's WhatsApp template policy (April 2026) and
// Twilio Content API constraints. Centralised here so we can swap in tighter
// per-provider limits later without touching call sites.
const (
	maxCarouselCards     = 10
	minCarouselCards     = 2
	maxCardBodyChars     = 160
	maxButtonsPerCard    = 2
	minButtonsPerCard    = 1
	maxQuickReplyButtons = 3
	maxListSections      = 10
	maxRowsPerSection    = 10
	maxRowTitleChars     = 24
	maxRowDescChars      = 72
	maxTemplateNameChars = 512
	maxBodyChars         = 1024
	maxHeaderTextChars   = 60
	maxFooterChars       = 60
)

// Lower-case-snake_case names only. Meta and Twilio both reject upper-case
// or hyphenated template names at submission time; failing fast here keeps
// the user out of a long round-trip.
var templateNameRE = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// variableRE finds `{{1}}`, `{{2}}` … in a body string. Whitespace tolerant
// to match human-typed templates.
var variableRE = regexp.MustCompile(`\{\{\s*(\d+)\s*\}\}`)

// ValidationError aggregates all problems found in a single Validate call so
// the UI can surface them all at once instead of one-at-a-time.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

func (e ValidationError) Error() string {
	if e.Field == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationErrors is the multi-error wrapper Validate returns. Implements
// the error interface and is JSON-marshalable as a flat list.
type ValidationErrors []ValidationError

func (es ValidationErrors) Error() string {
	if len(es) == 0 {
		return ""
	}
	parts := make([]string, 0, len(es))
	for _, e := range es {
		parts = append(parts, e.Error())
	}
	return strings.Join(parts, "; ")
}

// IsEmpty reports whether there are no errors so callers can `if errs := Validate(t); errs.IsEmpty() {...}`.
func (es ValidationErrors) IsEmpty() bool { return len(es) == 0 }

// Validate runs all enforceable rules against a RichTemplate. The intent is
// to mirror Meta + Twilio's submit-time rejection rules so callers get
// immediate feedback rather than a delayed approval rejection.
//
// Validate is intentionally provider-agnostic; provider-specific downgrades
// (e.g. Twilio's lack of video carousel headers) are handled separately at
// submission time so the same template can target both providers.
func Validate(t *RichTemplate) ValidationErrors {
	var errs ValidationErrors

	if t == nil {
		return append(errs, ValidationError{Field: "template", Message: "template is nil"})
	}

	errs = append(errs, validateName(t.Name)...)
	errs = append(errs, validateCommon(t)...)

	switch t.Kind {
	case KindCarousel:
		errs = append(errs, validateCarousel(t)...)
	case KindCouponCode:
		errs = append(errs, validateCouponCode(t)...)
	case KindCTAURL:
		errs = append(errs, validateCTAURL(t)...)
	case KindQuickReply:
		errs = append(errs, validateQuickReply(t)...)
	case KindList:
		errs = append(errs, validateList(t)...)
	case KindSingleProduct, KindMultiProduct, KindCatalog:
		// Product/catalog kinds are Phase 4. They submit a minimal template
		// referencing a catalog_id at send-time, not an authored RichTemplate.
		errs = append(errs, ValidationError{
			Field:   "kind",
			Message: fmt.Sprintf("kind %q is not yet supported in the authoring API", t.Kind),
			Code:    "UNSUPPORTED_KIND",
		})
	default:
		errs = append(errs, ValidationError{
			Field:   "kind",
			Message: fmt.Sprintf("unknown kind %q", t.Kind),
			Code:    "UNKNOWN_KIND",
		})
	}

	return errs
}

func validateName(name string) ValidationErrors {
	var errs ValidationErrors
	switch {
	case name == "":
		errs = append(errs, ValidationError{Field: "name", Message: "name is required"})
	case utf8.RuneCountInString(name) > maxTemplateNameChars:
		errs = append(errs, ValidationError{Field: "name", Message: fmt.Sprintf("name exceeds %d chars", maxTemplateNameChars)})
	case !templateNameRE.MatchString(name):
		errs = append(errs, ValidationError{
			Field:   "name",
			Message: "name must match ^[a-z][a-z0-9_]*$ (lower snake_case)",
			Code:    "INVALID_NAME",
		})
	}
	return errs
}

func validateCommon(t *RichTemplate) ValidationErrors {
	var errs ValidationErrors
	if t.AppID == "" {
		errs = append(errs, ValidationError{Field: "app_id", Message: "app_id is required"})
	}
	if t.Language == "" {
		errs = append(errs, ValidationError{Field: "language", Message: "language is required (e.g. en_US)"})
	}
	switch t.Category {
	case "MARKETING", "UTILITY", "AUTHENTICATION":
	case "":
		errs = append(errs, ValidationError{Field: "category", Message: "category is required"})
	default:
		errs = append(errs, ValidationError{Field: "category", Message: "category must be MARKETING, UTILITY or AUTHENTICATION"})
	}
	if t.Header != nil {
		errs = append(errs, validateHeader(t.Header)...)
	}
	if utf8.RuneCountInString(t.Body) > maxBodyChars {
		errs = append(errs, ValidationError{Field: "body", Message: fmt.Sprintf("body exceeds %d chars", maxBodyChars)})
	}
	if utf8.RuneCountInString(t.Footer) > maxFooterChars {
		errs = append(errs, ValidationError{Field: "footer", Message: fmt.Sprintf("footer exceeds %d chars", maxFooterChars)})
	}
	// Body variables must be contiguous starting at 1.
	if !variableIndicesContiguous(t.Body) {
		errs = append(errs, ValidationError{
			Field:   "body",
			Message: "body variables {{n}} must start at 1 and be contiguous (no gaps)",
			Code:    "VARIABLE_GAP",
		})
	}
	return errs
}

func validateHeader(h *Header) ValidationErrors {
	var errs ValidationErrors
	set := 0
	if h.Text != "" {
		set++
		if utf8.RuneCountInString(h.Text) > maxHeaderTextChars {
			errs = append(errs, ValidationError{Field: "header.text", Message: fmt.Sprintf("header text exceeds %d chars", maxHeaderTextChars)})
		}
	}
	if h.ImageURL != "" {
		set++
	}
	if h.VideoURL != "" {
		set++
	}
	if h.DocumentURL != "" {
		set++
	}
	if set > 1 {
		errs = append(errs, ValidationError{
			Field:   "header",
			Message: "exactly one of header.text / image_url / video_url / document_url may be set",
			Code:    "HEADER_MULTIPLE_TYPES",
		})
	}
	return errs
}

func validateCarousel(t *RichTemplate) ValidationErrors {
	var errs ValidationErrors
	n := len(t.Cards)
	if n < minCarouselCards || n > maxCarouselCards {
		errs = append(errs, ValidationError{
			Field:   "cards",
			Message: fmt.Sprintf("carousel requires %d..%d cards (got %d)", minCarouselCards, maxCarouselCards, n),
			Code:    "CAROUSEL_CARD_COUNT",
		})
		// Don't run per-card rules on an empty/oversized carousel; the count
		// rule is more actionable on its own.
		if n == 0 {
			return errs
		}
	}

	// All cards must share the same header media type (image or video). Meta
	// rejects mixed types at submission.
	headerType := cardHeaderType(t.Cards[0])
	if headerType == "" {
		errs = append(errs, ValidationError{Field: "cards[0]", Message: "first card must have either header_image_url or header_video_url"})
	}
	// All cards must use the same button type set in the same positions.
	wantButtonShape := cardButtonShape(t.Cards[0])

	for i, card := range t.Cards {
		field := fmt.Sprintf("cards[%d]", i)
		if utf8.RuneCountInString(card.Body) > maxCardBodyChars {
			errs = append(errs, ValidationError{Field: field + ".body", Message: fmt.Sprintf("card body exceeds %d chars", maxCardBodyChars)})
		}
		if !variableIndicesContiguous(card.Body) {
			errs = append(errs, ValidationError{Field: field + ".body", Message: "card body variables must be contiguous starting at 1"})
		}
		if ht := cardHeaderType(card); ht != headerType {
			errs = append(errs, ValidationError{
				Field:   field,
				Message: fmt.Sprintf("card header type %q differs from first card %q — all cards must use the same media type", ht, headerType),
				Code:    "CAROUSEL_MIXED_HEADERS",
			})
		}
		if bc := len(card.Buttons); bc < minButtonsPerCard || bc > maxButtonsPerCard {
			errs = append(errs, ValidationError{
				Field:   field + ".buttons",
				Message: fmt.Sprintf("each card needs %d..%d buttons (got %d)", minButtonsPerCard, maxButtonsPerCard, bc),
				Code:    "CAROUSEL_BUTTON_COUNT",
			})
		}
		if shape := cardButtonShape(card); shape != wantButtonShape {
			errs = append(errs, ValidationError{
				Field:   field + ".buttons",
				Message: "all cards must use the same button types in the same order",
				Code:    "CAROUSEL_MIXED_BUTTONS",
			})
		}
		for j, b := range card.Buttons {
			errs = append(errs, validateButton(b, fmt.Sprintf("%s.buttons[%d]", field, j))...)
		}
	}
	return errs
}

func validateCouponCode(t *RichTemplate) ValidationErrors {
	var errs ValidationErrors
	if t.Body == "" {
		errs = append(errs, ValidationError{Field: "body", Message: "body is required for coupon_code"})
	}
	if t.CouponCode == "" {
		errs = append(errs, ValidationError{Field: "coupon_code", Message: "coupon_code is required"})
	}
	return errs
}

func validateCTAURL(t *RichTemplate) ValidationErrors {
	var errs ValidationErrors
	if t.Body == "" {
		errs = append(errs, ValidationError{Field: "body", Message: "body is required for cta_url"})
	}
	if len(t.Buttons) != 1 || t.Buttons[0].Type != ButtonURL {
		errs = append(errs, ValidationError{
			Field:   "buttons",
			Message: "cta_url requires exactly one URL button",
			Code:    "CTA_URL_SHAPE",
		})
		return errs
	}
	errs = append(errs, validateButton(t.Buttons[0], "buttons[0]")...)
	return errs
}

func validateQuickReply(t *RichTemplate) ValidationErrors {
	var errs ValidationErrors
	if t.Body == "" {
		errs = append(errs, ValidationError{Field: "body", Message: "body is required for quick_reply"})
	}
	if n := len(t.Buttons); n < 1 || n > maxQuickReplyButtons {
		errs = append(errs, ValidationError{
			Field:   "buttons",
			Message: fmt.Sprintf("quick_reply requires 1..%d buttons (got %d)", maxQuickReplyButtons, n),
			Code:    "QUICK_REPLY_COUNT",
		})
	}
	for i, b := range t.Buttons {
		if b.Type != ButtonQuickReply {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("buttons[%d].type", i),
				Message: "quick_reply template buttons must all be QUICK_REPLY",
			})
		}
		errs = append(errs, validateButton(b, fmt.Sprintf("buttons[%d]", i))...)
	}
	return errs
}

func validateList(t *RichTemplate) ValidationErrors {
	var errs ValidationErrors
	if t.ListButtonText == "" {
		errs = append(errs, ValidationError{Field: "list_button_text", Message: "list_button_text is required (the launcher label)"})
	}
	if n := len(t.ListSections); n < 1 || n > maxListSections {
		errs = append(errs, ValidationError{
			Field:   "list_sections",
			Message: fmt.Sprintf("list requires 1..%d sections (got %d)", maxListSections, n),
		})
	}
	for i, sec := range t.ListSections {
		field := fmt.Sprintf("list_sections[%d]", i)
		if sec.Title == "" {
			errs = append(errs, ValidationError{Field: field + ".title", Message: "section title is required"})
		}
		if n := len(sec.Rows); n < 1 || n > maxRowsPerSection {
			errs = append(errs, ValidationError{
				Field:   field + ".rows",
				Message: fmt.Sprintf("each section needs 1..%d rows (got %d)", maxRowsPerSection, n),
			})
		}
		for j, r := range sec.Rows {
			rf := fmt.Sprintf("%s.rows[%d]", field, j)
			if r.ID == "" {
				errs = append(errs, ValidationError{Field: rf + ".id", Message: "row id is required"})
			}
			if r.Title == "" {
				errs = append(errs, ValidationError{Field: rf + ".title", Message: "row title is required"})
			}
			if utf8.RuneCountInString(r.Title) > maxRowTitleChars {
				errs = append(errs, ValidationError{Field: rf + ".title", Message: fmt.Sprintf("row title exceeds %d chars", maxRowTitleChars)})
			}
			if utf8.RuneCountInString(r.Description) > maxRowDescChars {
				errs = append(errs, ValidationError{Field: rf + ".description", Message: fmt.Sprintf("row description exceeds %d chars", maxRowDescChars)})
			}
		}
	}
	return errs
}

// validateButton runs the per-button checks for any kind that holds a Button
// slice. The field prefix is the path the caller wants in the error.
func validateButton(b Button, field string) ValidationErrors {
	var errs ValidationErrors
	if b.Text == "" {
		errs = append(errs, ValidationError{Field: field + ".text", Message: "button text is required"})
	}
	switch b.Type {
	case ButtonURL:
		if b.URL == "" {
			errs = append(errs, ValidationError{Field: field + ".url", Message: "URL is required for URL button"})
		}
		if hasVariable(b.URL) && b.Example == "" {
			errs = append(errs, ValidationError{
				Field:   field + ".example",
				Message: "URL buttons with {{n}} variables require an example value (Meta submission requirement)",
				Code:    "URL_BUTTON_EXAMPLE_MISSING",
			})
		}
	case ButtonQuickReply:
		if b.Payload == "" {
			errs = append(errs, ValidationError{Field: field + ".payload", Message: "payload is required for QUICK_REPLY button"})
		}
	case ButtonPhone:
		if b.PhoneNumber == "" {
			errs = append(errs, ValidationError{Field: field + ".phone_number", Message: "phone_number is required for PHONE_NUMBER button"})
		}
	case ButtonCopyCode:
		if b.CouponCode == "" {
			errs = append(errs, ValidationError{Field: field + ".coupon_code", Message: "coupon_code is required for COPY_CODE button"})
		}
	case "":
		errs = append(errs, ValidationError{Field: field + ".type", Message: "button type is required"})
	default:
		errs = append(errs, ValidationError{Field: field + ".type", Message: fmt.Sprintf("unknown button type %q", b.Type)})
	}
	return errs
}

// cardHeaderType returns "image", "video", or "" depending on which header
// URL is set on a carousel card. Used to enforce uniform-header rule.
func cardHeaderType(c CarouselCard) string {
	switch {
	case c.HeaderImageURL != "":
		return "image"
	case c.HeaderVideoURL != "":
		return "video"
	}
	return ""
}

// cardButtonShape returns a stable string fingerprint of a card's button
// types in order. Used to enforce "all cards use the same button types in
// the same order" without false positives when the labels/URLs differ.
func cardButtonShape(c CarouselCard) string {
	parts := make([]string, len(c.Buttons))
	for i, b := range c.Buttons {
		parts[i] = string(b.Type)
	}
	return strings.Join(parts, "|")
}

// variableIndicesContiguous reports whether the {{n}} placeholders in s
// form a contiguous sequence starting at 1. "Hello {{2}}" without {{1}} is
// rejected by Meta at submission.
func variableIndicesContiguous(s string) bool {
	matches := variableRE.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return true
	}
	seen := make(map[int]struct{}, len(matches))
	max := 0
	for _, m := range matches {
		n := 0
		// Manual atoi to avoid pulling strconv just for one parse — but
		// strconv is fine. Using fmt.Sscanf would be silly. Just use strconv.
		_, _ = fmt.Sscanf(m[1], "%d", &n)
		if n < 1 {
			return false
		}
		seen[n] = struct{}{}
		if n > max {
			max = n
		}
	}
	for i := 1; i <= max; i++ {
		if _, ok := seen[i]; !ok {
			return false
		}
	}
	return true
}

// hasVariable reports whether s contains at least one {{n}} placeholder.
func hasVariable(s string) bool {
	return variableRE.MatchString(s)
}
