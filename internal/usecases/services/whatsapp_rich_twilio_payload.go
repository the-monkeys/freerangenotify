package services

import (
	"strings"

	"github.com/the-monkeys/freerangenotify/internal/domain/whatsapp"
)

// twilioContentPayload converts an FRN RichTemplate into the Twilio
// Content API authoring shape. Twilio uses `{{n}}` placeholders verbatim
// in body strings and exposes example values via the top-level `variables`
// map (string keys, indexed from 1). The provider-specific component
// shapes live under types.{twilio/carousel|twilio/call-to-action|...}.
//
// Twilio is deliberately permissive: unknown variables are echoed, missing
// values render as the literal placeholder. We supply placeholder examples
// matching the count of positional variables so approval reviewers see a
// realistic preview.
func twilioContentPayload(t *whatsapp.RichTemplate) map[string]interface{} {
	out := map[string]interface{}{
		"friendly_name": t.Name,
		"language":      twilioLanguage(t.Language),
	}

	// Twilio expects positional variables as a flat map even when only the
	// body references them — the SDK auto-prunes unused keys.
	variables := map[string]string{}
	for i, v := range exampleVarMatrix(t.Body) {
		variables[positionalKey(i+1)] = v
	}
	if len(variables) > 0 {
		out["variables"] = variables
	}

	switch t.Kind {
	case whatsapp.KindCarousel:
		out["types"] = map[string]interface{}{
			"twilio/carousel": twilioCarouselType(t),
		}
	case whatsapp.KindCouponCode:
		// Twilio represents coupon codes as a quick-reply with a single
		// COPY_CODE-style action. The closest first-class type today is
		// twilio/call-to-action with body + a single text action; FRN
		// emits this minimally so the binding round-trips. UI may evolve
		// to use Twilio's coupon component once GA.
		out["types"] = map[string]interface{}{
			"twilio/call-to-action": map[string]interface{}{
				"body": t.Body,
				"actions": []map[string]interface{}{
					{"type": "QUICK_REPLY", "title": "Copy " + t.CouponCode, "id": "copy_code"},
				},
			},
		}
	case whatsapp.KindCTAURL:
		out["types"] = map[string]interface{}{
			"twilio/call-to-action": map[string]interface{}{
				"body":    t.Body,
				"actions": twilioActionsFromButtons(t.Buttons),
			},
		}
	case whatsapp.KindQuickReply:
		out["types"] = map[string]interface{}{
			"twilio/quick-reply": map[string]interface{}{
				"body":    t.Body,
				"actions": twilioActionsFromButtons(t.Buttons),
			},
		}
	case whatsapp.KindList:
		items := []map[string]interface{}{}
		for _, sec := range t.ListSections {
			for _, r := range sec.Rows {
				items = append(items, map[string]interface{}{
					"item":        r.Title,
					"id":          r.ID,
					"description": r.Description,
				})
			}
		}
		out["types"] = map[string]interface{}{
			"twilio/list-picker": map[string]interface{}{
				"body":   t.Body,
				"button": firstNonEmpty(t.ListButtonText, "Menu"),
				"items":  items,
			},
		}
	}
	return out
}

// twilioCarouselType emits the twilio/carousel type body with one card per
// authored CarouselCard. Twilio's per-card schema:
//
//	{
//	  "title": "Polo {{1}} {{2}}",
//	  "media": ["https://cdn/.../1.jpg"],
//	  "actions": [{"type":"URL","title":"View","url":"https://shop/p/1"}]
//	}
//
// Twilio doesn't separate header from body the way Meta does — the card
// "title" carries the body text and the optional "media" array provides
// the header image/video URL.
func twilioCarouselType(t *whatsapp.RichTemplate) map[string]interface{} {
	cards := make([]map[string]interface{}, 0, len(t.Cards))
	for _, c := range t.Cards {
		card := map[string]interface{}{"title": c.Body}
		switch {
		case c.HeaderImageURL != "":
			card["media"] = []string{c.HeaderImageURL}
		case c.HeaderVideoURL != "":
			card["media"] = []string{c.HeaderVideoURL}
		}
		card["actions"] = twilioActionsFromButtons(c.Buttons)
		cards = append(cards, card)
	}
	return map[string]interface{}{
		"body":  t.Body,
		"cards": cards,
	}
}

// twilioActionsFromButtons converts FRN Button slice to Twilio's action
// schema. Twilio uses uppercase action types matching Meta's button
// taxonomy except for COPY_CODE (no first-class equivalent yet).
func twilioActionsFromButtons(buttons []whatsapp.Button) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(buttons))
	for _, b := range buttons {
		a := map[string]interface{}{
			"type":  string(b.Type),
			"title": b.Text,
		}
		switch b.Type {
		case whatsapp.ButtonURL:
			a["url"] = b.URL
		case whatsapp.ButtonPhone:
			a["phone"] = b.PhoneNumber
		case whatsapp.ButtonQuickReply:
			if b.Payload != "" {
				a["id"] = b.Payload
			}
		case whatsapp.ButtonCopyCode:
			// Twilio doesn't yet expose a first-class COPY_CODE action;
			// degrade to QUICK_REPLY with the code surfaced in the title.
			a["type"] = "QUICK_REPLY"
			a["id"] = "copy_code"
			a["title"] = "Copy " + b.CouponCode
		}
		out = append(out, a)
	}
	return out
}

// twilioLanguage maps Meta-style `en_US` to Twilio-style `en`. Twilio
// accepts the underscore form for some templates but the base code is
// safer and consistent with their docs' examples.
func twilioLanguage(meta string) string {
	if idx := strings.Index(meta, "_"); idx > 0 {
		return strings.ToLower(meta[:idx])
	}
	return strings.ToLower(meta)
}

// positionalKey formats a 1-based integer as the string key Twilio uses
// for its variables map.
func positionalKey(n int) string {
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	if digits == "" {
		return "0"
	}
	return digits
}

// firstNonEmpty returns the first non-empty string from its arguments.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
