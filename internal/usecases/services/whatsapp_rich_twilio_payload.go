package services

import (
	"fmt"
	"strings"
	"unicode/utf8"

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
		// Twilio call-to-action supports a first-class copy-code button.
		out["types"] = map[string]interface{}{
			"twilio/call-to-action": map[string]interface{}{
				"body": t.Body,
				"actions": []map[string]interface{}{
					{"type": "COPY_CODE", "title": "Copy code", "code": t.CouponCode},
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

// twilioCarouselType keeps card body separate and emits one media URL string.
func twilioCarouselType(t *whatsapp.RichTemplate) map[string]interface{} {
	cards := make([]map[string]interface{}, 0, len(t.Cards))
	for _, c := range t.Cards {
		card := map[string]interface{}{"body": c.Body}
		switch {
		case c.HeaderImageURL != "":
			card["media"] = c.HeaderImageURL
		case c.HeaderVideoURL != "":
			card["media"] = c.HeaderVideoURL
		}
		card["actions"] = twilioActionsFromButtons(c.Buttons)
		cards = append(cards, card)
	}
	return map[string]interface{}{
		"body":  t.Body,
		"cards": cards,
	}
}

// twilioActionsFromButtons preserves supported provider action types.
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
			a["type"] = "COPY_CODE"
			a["code"] = b.CouponCode

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

// Validate only at explicit authoring; existing ContentSid bindings are untouched.
func validateTwilioRichAuthoring(t *whatsapp.RichTemplate) error {
	if t.Kind == whatsapp.KindCouponCode || t.Kind == whatsapp.KindCTAURL {
		if t.Body == "" || utf8.RuneCountInString(t.Body) > 640 {
			return fmt.Errorf("Twilio call-to-action requires a body of at most 640 characters")
		}
	}
	if t.Kind == whatsapp.KindCarousel {
		if len(t.Cards) < 2 || len(t.Cards) > 10 {
			return fmt.Errorf("Twilio carousel requires 2 to 10 cards")
		}
		for _, card := range t.Cards {
			if card.Body == "" || utf8.RuneCountInString(card.Body) > 160 {
				return fmt.Errorf("Twilio carousel card body must contain 1 to 160 characters")
			}
			if (card.HeaderImageURL == "") == (card.HeaderVideoURL == "") {
				return fmt.Errorf("Twilio carousel card requires one image or video URL")
			}
			if len(card.Buttons) < 1 || len(card.Buttons) > 2 {
				return fmt.Errorf("Twilio carousel card requires one or two actions")
			}
			for index, button := range card.Buttons {
				if index >= len(t.Cards[0].Buttons) || button.Type != t.Cards[0].Buttons[index].Type {
					return fmt.Errorf("Twilio carousel actions must use the same order on every card")
				}
				if button.Type != whatsapp.ButtonURL && button.Type != whatsapp.ButtonQuickReply && button.Type != whatsapp.ButtonPhone {
					return fmt.Errorf("unsupported Twilio carousel action")
				}
				if utf8.RuneCountInString(button.Text) > 25 {
					return fmt.Errorf("Twilio carousel action title exceeds 25 characters")
				}
			}
			if len(card.Buttons) != len(t.Cards[0].Buttons) {
				return fmt.Errorf("Twilio carousel actions must use the same count on every card")
			}
		}
	}
	return nil
}
