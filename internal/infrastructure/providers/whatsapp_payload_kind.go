package providers

import (
	"fmt"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
)

// Admission uses the same renderer as dispatch, so a client label cannot turn
// an interactive/text send into a template for permission or fee purposes.
func WhatsAppPayloadKind(n *notification.Notification, provider string) (string, error) {
	if provider == "twilio" {
		if raw, exists := n.Content.Data["content_sid"]; exists {
			sid, ok := raw.(string)
			if !ok || sid == "" {
				return "", fmt.Errorf("invalid Content SID")
			}
			if n.Content.Data["whatsapp_template"] != nil || n.Content.Data["whatsapp_rich"] != nil {
				return "", fmt.Errorf("incompatible WhatsApp payload")
			}
			return "template", nil
		}
		if n.Content.Data["whatsapp_template"] != nil || n.Content.Data["whatsapp_rich"] != nil {
			return "", fmt.Errorf("Meta payload requires Meta account")
		}
		return "text", nil
	}
	if provider != "meta" || n.Content.Data["content_sid"] != nil {
		return "", fmt.Errorf("payload does not match selected provider")
	}
	p := &MetaWhatsAppProvider{}
	if raw, exists := n.Content.Data["whatsapp_rich"]; exists {
		rich, ok := raw.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("invalid rich payload")
		}
		msg, ok := p.buildRichMessage(rich, "fixture")
		if !ok {
			return "", fmt.Errorf("invalid rich payload")
		}
		if msg.Type == "template" {
			return "template", nil
		}
		return "text", nil
	}
	if raw, exists := n.Content.Data["whatsapp_template"]; exists {
		tpl, ok := raw.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("invalid Meta template")
		}
		name, ok := tpl["name"].(string)
		if !ok || name == "" {
			return "", fmt.Errorf("Meta template name required")
		}
		return "template", nil
	}
	return "text", nil
}
