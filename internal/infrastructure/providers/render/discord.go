package render

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
)

type DiscordRenderOptions struct {
	NativePolls bool
}

// BuildDiscordPayload constructs a Discord webhook payload with embeds.
// It renders all rich content fields: Attachments, Actions, Fields, Poll, Style.
func BuildDiscordPayload(notif *notification.Notification) map[string]interface{} {
	return BuildDiscordPayloadWithOptions(notif, DiscordRenderOptions{})
}

func BuildDiscordPayloadWithOptions(notif *notification.Notification, opts DiscordRenderOptions) map[string]interface{} {
	c := notif.Content

	// Resolve embed color: Style.Color hex → int, Style.Severity → preset, default blue.
	color := resolveDiscordColor(c.Style)

	embed := map[string]interface{}{
		"title":       c.Title,
		"description": c.Body,
		"color":       color,
	}

	// Legacy: action_url becomes embed URL.
	if c.Data != nil {
		if actionURL, ok := c.Data["action_url"].(string); ok && actionURL != "" {
			embed["url"] = actionURL
		}
	}

	// --- Fields → Discord embed fields ---
	if len(c.Fields) > 0 {
		fields := make([]map[string]interface{}, 0, len(c.Fields))
		for _, f := range c.Fields {
			fields = append(fields, map[string]interface{}{
				"name":   f.Key,
				"value":  f.Value,
				"inline": f.Inline,
			})
		}
		embed["fields"] = fields
	}

	// --- First image attachment → embed image, additional → extra embeds ---
	embeds := []map[string]interface{}{embed}

	if len(c.Attachments) > 0 {
		first := true
		for _, a := range c.Attachments {
			switch a.Type {
			case "image":
				img := map[string]interface{}{"url": a.URL}
				if first {
					embed["image"] = img
					first = false
				} else {
					// Discord supports multiple embeds for image gallery
					embeds = append(embeds, map[string]interface{}{
						"image": img,
						"color": color,
					})
				}
			case "video":
				embed["video"] = map[string]interface{}{"url": a.URL}
			default:
				// file/audio: append as linked field
				label := a.Name
				if label == "" {
					label = a.Type
				}
				if existing, ok := embed["fields"].([]map[string]interface{}); ok {
					embed["fields"] = append(existing, map[string]interface{}{
						"name":   fmt.Sprintf("📎 %s", label),
						"value":  fmt.Sprintf("[Download](%s)", a.URL),
						"inline": false,
					})
				} else {
					embed["fields"] = []map[string]interface{}{{
						"name":   fmt.Sprintf("📎 %s", label),
						"value":  fmt.Sprintf("[Download](%s)", a.URL),
						"inline": false,
					}}
				}
			}
		}
	}

	// --- Actions → markdown link list rendered as a final embed field ---
	//
	// Discord interactive components (`type: 2` buttons in `components: [...]`)
	// are NOT honored on incoming-webhook deliveries. The webhook accepts the
	// payload (HTTP 204) but Discord silently strips `components` because
	// only application-owned webhooks with the `IS_COMPONENTS_V2` flag may
	// emit them. Emitting `components` therefore both wastes payload bytes
	// and produces the user-visible bug "actions disappeared in Discord".
	//
	// We instead render link actions as a clickable markdown link list in a
	// dedicated embed field. This is supported by every Discord webhook,
	// renders identically in desktop/mobile clients, and preserves the
	// click-through behavior customers expect.
	if len(c.Actions) > 0 {
		linkParts := make([]string, 0, len(c.Actions))
		for _, a := range c.Actions {
			if a.Type == "link" && a.URL != "" && a.Label != "" {
				linkParts = append(linkParts, fmt.Sprintf("[%s](%s)", a.Label, a.URL))
			}
		}
		if len(linkParts) > 0 {
			actionField := map[string]interface{}{
				"name":   "Actions",
				"value":  strings.Join(linkParts, " • "),
				"inline": false,
			}
			if existing, ok := embed["fields"].([]map[string]interface{}); ok {
				embed["fields"] = append(existing, actionField)
			} else {
				embed["fields"] = []map[string]interface{}{actionField}
			}
		}
	}

	// --- Poll ---
	// Default: embed field (incoming webhooks cannot emit native polls reliably).
	//
	// Discord exposes native poll objects only on application-owned webhooks
	// (those created and bound to a Discord application via the Bot API).
	// Plain channel webhooks created through "Server Settings → Integrations
	// → Webhooks" — which is what 99% of customers paste into FreeRangeNotify
	// — reject any payload containing a top-level `poll` field with HTTP 400
	// `{"proto_data": ["poll"]}`. There is no way to detect this at render
	// time without an out-of-band probe, so emitting the native poll object
	// is unsafe by default.
	//
	// We render the Poll as a numbered list inside a dedicated embed field.
	// This is supported on every Discord webhook, conveys all the poll
	// information (question, choices, optional emoji), and lets users react
	// with the listed emojis to vote. Customers who own an application-bound
	// webhook can opt back into native polls in a follow-up.
	if c.Poll != nil && c.Poll.Question != "" && len(c.Poll.Choices) > 0 && !opts.NativePolls {
		var lines []string
		for i, ch := range c.Poll.Choices {
			label := ch.Label
			if label == "" {
				label = fmt.Sprintf("Option %d", i+1)
			}
			if emoji := normalizeDiscordPollEmoji(ch.Emoji); emoji != "" {
				lines = append(lines, fmt.Sprintf("%s **%d.** %s", emoji, i+1, label))
			} else {
				lines = append(lines, fmt.Sprintf("**%d.** %s", i+1, label))
			}
		}
		pollField := map[string]interface{}{
			"name":   fmt.Sprintf("📊 %s", c.Poll.Question),
			"value":  strings.Join(lines, "\n"),
			"inline": false,
		}
		if existing, ok := embed["fields"].([]map[string]interface{}); ok {
			embed["fields"] = append(existing, pollField)
		} else {
			embed["fields"] = []map[string]interface{}{pollField}
		}
	}

	// --- Mentions → prepend to content text ---
	contentText := c.Title
	if len(c.Mentions) > 0 {
		var mentionParts []string
		for _, m := range c.Mentions {
			if m.Platform == "discord" && m.PlatformID != "" {
				mentionParts = append(mentionParts, fmt.Sprintf("<@%s>", m.PlatformID))
			}
		}
		if len(mentionParts) > 0 {
			contentText = strings.Join(mentionParts, " ") + " " + contentText
		}
	}

	payload := map[string]interface{}{
		"content": contentText,
		"embeds":  embeds,
	}

	// Native poll opt-in. This produces an interactive poll ONLY when the
	// destination webhook supports Discord's poll request object.
	if opts.NativePolls && c.Poll != nil && c.Poll.Question != "" && len(c.Poll.Choices) > 0 {
		payload["poll"] = buildDiscordNativePoll(c.Poll)
	}
	return payload
}

func buildDiscordNativePoll(p *notification.Poll) map[string]interface{} {
	// Discord accepts duration (hours) up to 32 days; default 24.
	d := 24
	if p.DurationHours > 0 {
		d = p.DurationHours
	}
	if d > 32*24 {
		d = 32 * 24
	}
	if d < 1 {
		d = 1
	}

	answers := make([]map[string]interface{}, 0, int(math.Min(float64(len(p.Choices)), 10)))
	for _, ch := range p.Choices {
		if len(answers) >= 10 {
			break
		}
		text := strings.TrimSpace(ch.Label)
		if text == "" {
			continue
		}
		media := map[string]interface{}{"text": text}
		if emoji := normalizeDiscordPollEmoji(ch.Emoji); emoji != "" {
			media["emoji"] = map[string]interface{}{"name": emoji}
		}
		answers = append(answers, map[string]interface{}{
			"poll_media": media,
		})
	}

	return map[string]interface{}{
		"question": map[string]interface{}{"text": p.Question},
		"answers":  answers,
		"duration": d,
		"allow_multiselect": p.MultiSelect,
		"layout_type": 1,
	}
}

// resolveDiscordColor returns the integer color for a Discord embed.
func resolveDiscordColor(style *notification.Style) int {
	if style == nil {
		return 3447003 // Discord blue (#3498DB)
	}
	if style.Color != "" {
		hex := strings.TrimPrefix(style.Color, "#")
		if v, err := strconv.ParseInt(hex, 16, 64); err == nil {
			return int(v)
		}
	}
	switch style.Severity {
	case "success":
		return 3066993 // Green (#2ECC71)
	case "warning":
		return 15105570 // Orange (#E67E22)
	case "danger":
		return 15158332 // Red (#E74C3C)
	case "info":
		return 3447003 // Blue (#3498DB)
	default:
		return 3447003
	}
}

// BuildCustomDiscordPayload builds the Discord payload used by the
// custom-provider path (Settings.CustomProviders[].Kind == "discord").
//
// Historically this returned a minimal {embeds:[{title,description}]} shape
// that silently dropped Attachments, Actions, Fields, Mentions, Poll and Style
// from notification.Content. That made the custom-provider channel an
// unintentional second-class citizen relative to the dedicated discord
// provider. The two now share a single renderer so a notification routed
// through either path produces the same rich payload.
func BuildCustomDiscordPayload(notif *notification.Notification) map[string]interface{} {
	return BuildDiscordPayload(notif)
}

func BuildCustomDiscordPayloadWithOptions(notif *notification.Notification, opts DiscordRenderOptions) map[string]interface{} {
	return BuildDiscordPayloadWithOptions(notif, opts)
}

// normalizeDiscordPollEmoji returns a string safe to send as
// `emoji.name` on a Discord poll answer, or "" when no emoji should be
// attached.
//
// Discord's poll API accepts a Partial Emoji where `name` must be a real
// unicode emoji codepoint sequence. Plain ASCII digits like "1" or "2" are
// NOT emojis — Discord rejects the entire `poll.answers` array with
// `{"poll": ["answers"]}` when any answer carries one. This caused the
// user-visible bug "Discord poll fails to deliver" when authors reused
// numbered list markers (1, 2, 3) as choice prefixes.
//
// Behavior:
//   - Single ASCII digits 0-9 are promoted to their keycap emoji form
//     ("1" → "1️⃣"), which Discord accepts.
//   - Other ASCII-only inputs are dropped (returns ""), so the answer is
//     submitted without an emoji rather than failing the whole poll.
//   - Inputs containing any non-ASCII rune are passed through unchanged
//     under the assumption they are real emoji.
func normalizeDiscordPollEmoji(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Pass through anything containing a non-ASCII rune (real emoji).
	for _, r := range s {
		if r > 127 {
			return s
		}
	}
	// Single ASCII digit → keycap emoji (digit + VS16 + COMBINING ENCLOSING KEYCAP).
	if len(s) == 1 && s[0] >= '0' && s[0] <= '9' {
		return s + "\ufe0f\u20e3"
	}
	// Anything else ASCII (letters, "1.", "a)", etc.) — drop, do not poison the poll.
	return ""
}
