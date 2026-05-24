package services

import (
	"regexp"
	"strings"

	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
)

// metaAuthoringPayload converts an FRN RichTemplate into the JSON shape
// Meta's POST /{waba_id}/message_templates expects. This is intentionally
// the *authoring* shape — different from the runtime template message JSON
// the Meta provider builds for send-time (see meta_whatsapp_provider.go).
//
// Key differences vs. send-time:
//   * Component types are UPPERCASE ("BODY" not "body").
//   * BODY/HEADER text components carry an `example` field so Meta can
//     review with real-looking content; we generate placeholders if the
//     authored template has variables but no example.
//   * Carousel header components use `format` ("IMAGE"/"VIDEO") rather than
//     inline media; the sample URL goes in example.header_handle.
//   * Buttons live inside a single "BUTTONS" component, not as separate
//     components with sub_type/index.
func metaAuthoringPayload(t *whatsapp.RichTemplate) map[string]interface{} {
	out := map[string]interface{}{
		"name":     t.Name,
		"language": t.Language,
		"category": t.Category,
	}
	components := []map[string]interface{}{}

	if t.Header != nil {
		if h := metaAuthHeaderComponent(t.Header); h != nil {
			components = append(components, h)
		}
	}
	if t.Body != "" {
		components = append(components, metaAuthBodyComponent(t.Body))
	}
	if t.Footer != "" {
		components = append(components, map[string]interface{}{"type": "FOOTER", "text": t.Footer})
	}
	buttons := t.Buttons
	// KindCouponCode stores the code on the template (not as a Button), since
	// the user authors "use code X" once and never wires a button matrix.
	// Synthesise the COPY_CODE button here so Meta sees the expected
	// BUTTONS component shape.
	if t.Kind == whatsapp.KindCouponCode && t.CouponCode != "" {
		buttons = append(buttons, whatsapp.Button{
			Type:       whatsapp.ButtonCopyCode,
			Text:       "Copy code",
			CouponCode: t.CouponCode,
		})
	}
	if len(buttons) > 0 {
		components = append(components, metaAuthButtonsComponent(buttons))
	}
	if t.Kind == whatsapp.KindCarousel && len(t.Cards) > 0 {
		components = append(components, metaAuthCarouselComponent(t.Cards))
	}

	out["components"] = components
	return out
}

// metaAuthHeaderComponent renders a HEADER for the top-level template. For
// media headers Meta wants `format` + an example handle (the public URL
// works for the initial submission; for production media you'd upload via
// the Resumable Upload API and use its handle — out of scope here).
func metaAuthHeaderComponent(h *whatsapp.Header) map[string]interface{} {
	switch {
	case h.Text != "":
		comp := map[string]interface{}{"type": "HEADER", "format": "TEXT", "text": h.Text}
		if hasVariablePlaceholders(h.Text) {
			comp["example"] = map[string]interface{}{"header_text": []string{"Example"}}
		}
		return comp
	case h.ImageURL != "":
		return map[string]interface{}{
			"type":   "HEADER",
			"format": "IMAGE",
			"example": map[string]interface{}{
				"header_handle": []string{h.ImageURL},
			},
		}
	case h.VideoURL != "":
		return map[string]interface{}{
			"type":   "HEADER",
			"format": "VIDEO",
			"example": map[string]interface{}{
				"header_handle": []string{h.VideoURL},
			},
		}
	case h.DocumentURL != "":
		return map[string]interface{}{
			"type":   "HEADER",
			"format": "DOCUMENT",
			"example": map[string]interface{}{
				"header_handle": []string{h.DocumentURL},
			},
		}
	}
	return nil
}

// metaAuthBodyComponent emits a BODY component with placeholder examples
// when the body contains {{n}} variables. Meta's reviewer requires example
// values for every positional variable so an empty example matrix is a
// rejection trigger.
func metaAuthBodyComponent(body string) map[string]interface{} {
	comp := map[string]interface{}{"type": "BODY", "text": body}
	if matrix := exampleVarMatrix(body); len(matrix) > 0 {
		comp["example"] = map[string]interface{}{"body_text": [][]string{matrix}}
	}
	return comp
}

// metaAuthButtonsComponent groups all buttons into a single BUTTONS
// component per Meta's authoring schema. Each button's per-type fields are
// pulled in line.
func metaAuthButtonsComponent(buttons []whatsapp.Button) map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(buttons))
	for _, b := range buttons {
		out = append(out, metaAuthOneButton(b))
	}
	return map[string]interface{}{"type": "BUTTONS", "buttons": out}
}

// metaAuthOneButton emits a single button entry. Caller decides where it
// lives (top-level BUTTONS or a carousel card BUTTONS).
func metaAuthOneButton(b whatsapp.Button) map[string]interface{} {
	out := map[string]interface{}{
		"type": string(b.Type),
		"text": b.Text,
	}
	switch b.Type {
	case whatsapp.ButtonURL:
		out["url"] = b.URL
		if hasVariablePlaceholders(b.URL) {
			example := b.Example
			if example == "" {
				// Meta requires an example URL for variable-suffixed URLs.
				// Substitute a sample so submission does not reject.
				example = strings.ReplaceAll(b.URL, "{{1}}", "example")
			}
			out["example"] = []string{example}
		}
	case whatsapp.ButtonPhone:
		out["phone_number"] = b.PhoneNumber
	case whatsapp.ButtonCopyCode:
		out["example"] = b.CouponCode
	case whatsapp.ButtonQuickReply:
		// QUICK_REPLY only needs text; payload is bound at send-time.
	}
	return out
}

// metaAuthCarouselComponent emits the carousel component for a carousel
// template. Each card becomes its own components array.
func metaAuthCarouselComponent(cards []whatsapp.CarouselCard) map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(cards))
	for _, card := range cards {
		comps := []map[string]interface{}{}
		switch {
		case card.HeaderImageURL != "":
			comps = append(comps, map[string]interface{}{
				"type":    "HEADER",
				"format":  "IMAGE",
				"example": map[string]interface{}{"header_handle": []string{card.HeaderImageURL}},
			})
		case card.HeaderVideoURL != "":
			comps = append(comps, map[string]interface{}{
				"type":    "HEADER",
				"format":  "VIDEO",
				"example": map[string]interface{}{"header_handle": []string{card.HeaderVideoURL}},
			})
		}
		if card.Body != "" {
			body := map[string]interface{}{"type": "BODY", "text": card.Body}
			matrix := card.Variables
			if matrix == nil {
				matrix = exampleVarMatrix(card.Body)
			}
			if len(matrix) > 0 {
				body["example"] = map[string]interface{}{"body_text": [][]string{matrix}}
			}
			comps = append(comps, body)
		}
		if len(card.Buttons) > 0 {
			comps = append(comps, metaAuthButtonsComponent(card.Buttons))
		}
		out = append(out, map[string]interface{}{"components": comps})
	}
	return map[string]interface{}{"type": "CAROUSEL", "cards": out}
}

// varRE here is the same shape as the validator's, duplicated to keep the
// services package independent of internal validator helpers. If we add a
// third copy, lift to a shared helper.
var varRE = regexp.MustCompile(`\{\{\s*(\d+)\s*\}\}`)

// hasVariablePlaceholders is a fast check used to decide whether to emit an
// `example` field on body/header/url-button components.
func hasVariablePlaceholders(s string) bool { return varRE.MatchString(s) }

// exampleVarMatrix returns ["Sample1","Sample2",…] sized to the maximum
// variable index in s. Meta wants one example per positional variable so
// this is the minimum shape that satisfies submission review.
func exampleVarMatrix(s string) []string {
	matches := varRE.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return nil
	}
	max := 0
	for _, m := range matches {
		n := 0
		for _, ch := range m[1] {
			n = n*10 + int(ch-'0')
		}
		if n > max {
			max = n
		}
	}
	out := make([]string, max)
	for i := range out {
		out[i] = "sample"
	}
	return out
}
