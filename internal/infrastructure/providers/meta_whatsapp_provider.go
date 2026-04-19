package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"go.uber.org/zap"
)

// MetaWhatsAppConfig holds configuration for the Meta WhatsApp Cloud API provider.
type MetaWhatsAppConfig struct {
	Config                    // Common: Timeout, MaxRetries, RetryDelay
	PhoneNumberID string     // Meta Phone Number ID (numeric)
	WABAID        string     // WhatsApp Business Account ID
	AccessToken   string     // Permanent System User token (starts with EAAG...)
	APIVersion    string     // Graph API version (default: v23.0)
}

// MetaWhatsAppProvider delivers notifications via Meta's WhatsApp Cloud API.
type MetaWhatsAppProvider struct {
	config     MetaWhatsAppConfig
	httpClient *http.Client
	logger     *zap.Logger
}

// NewMetaWhatsAppProvider creates a new MetaWhatsAppProvider.
func NewMetaWhatsAppProvider(config MetaWhatsAppConfig, logger *zap.Logger) (*MetaWhatsAppProvider, error) {
	if config.APIVersion == "" {
		config.APIVersion = "v23.0"
	}
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}

	return &MetaWhatsAppProvider{
		config:     config,
		httpClient: &http.Client{Timeout: timeout},
		logger:     logger,
	}, nil
}

// --- Meta Cloud API request/response types ---

type metaMessage struct {
	MessagingProduct string             `json:"messaging_product"`
	RecipientType    string             `json:"recipient_type"`
	To               string             `json:"to"`
	Type             string             `json:"type"`
	Text             *metaText          `json:"text,omitempty"`
	Template         *metaTemplate      `json:"template,omitempty"`
	Image            *metaMedia         `json:"image,omitempty"`
	Video            *metaMedia         `json:"video,omitempty"`
	Audio            *metaMedia         `json:"audio,omitempty"`
	Document         *metaDocumentMedia `json:"document,omitempty"`
	Sticker          *metaMedia         `json:"sticker,omitempty"`
	Location         *metaLocation      `json:"location,omitempty"`
	Contacts         []metaContactCard  `json:"contacts,omitempty"`
	Reaction         *metaReaction      `json:"reaction,omitempty"`
	Interactive      *metaInteractive   `json:"interactive,omitempty"`
	Context          *metaContext       `json:"context,omitempty"`
}

type metaText struct {
	PreviewURL bool   `json:"preview_url"`
	Body       string `json:"body"`
}

type metaTemplate struct {
	Name       string              `json:"name"`
	Language   metaLanguage        `json:"language"`
	Components []metaComponent     `json:"components,omitempty"`
}

type metaLanguage struct {
	Code string `json:"code"`
}

type metaComponent struct {
	Type       string          `json:"type"`
	Parameters []metaParameter `json:"parameters,omitempty"`
}

type metaParameter struct {
	Type  string `json:"type"`
	Text  string `json:"text,omitempty"`
	Image *metaMedia `json:"image,omitempty"`
}

type metaMedia struct {
	Link    string `json:"link,omitempty"`
	ID      string `json:"id,omitempty"`
	Caption string `json:"caption,omitempty"`
}

type metaDocumentMedia struct {
	Link     string `json:"link,omitempty"`
	ID       string `json:"id,omitempty"`
	Caption  string `json:"caption,omitempty"`
	Filename string `json:"filename,omitempty"`
}

type metaLocation struct {
	Longitude float64 `json:"longitude"`
	Latitude  float64 `json:"latitude"`
	Name      string  `json:"name,omitempty"`
	Address   string  `json:"address,omitempty"`
}

type metaContactCard struct {
	Name   metaContactName    `json:"name"`
	Phones []metaContactPhone `json:"phones,omitempty"`
	Emails []metaContactEmail `json:"emails,omitempty"`
}

type metaContactName struct {
	FormattedName string `json:"formatted_name"`
	FirstName     string `json:"first_name,omitempty"`
	LastName      string `json:"last_name,omitempty"`
}

type metaContactPhone struct {
	Phone string `json:"phone"`
	Type  string `json:"type,omitempty"`
}

type metaContactEmail struct {
	Email string `json:"email"`
	Type  string `json:"type,omitempty"`
}

type metaReaction struct {
	MessageID string `json:"message_id"`
	Emoji     string `json:"emoji"`
}

type metaInteractive struct {
	Type   string                 `json:"type"` // button, list, cta_url
	Header *metaInteractiveHeader `json:"header,omitempty"`
	Body   metaInteractiveBody    `json:"body"`
	Footer *metaInteractiveFooter `json:"footer,omitempty"`
	Action metaInteractiveAction  `json:"action"`
}

type metaInteractiveHeader struct {
	Type     string     `json:"type"` // text, image, video, document
	Text     string     `json:"text,omitempty"`
	Image    *metaMedia `json:"image,omitempty"`
	Video    *metaMedia `json:"video,omitempty"`
	Document *metaMedia `json:"document,omitempty"`
}

type metaInteractiveBody struct {
	Text string `json:"text"`
}

type metaInteractiveFooter struct {
	Text string `json:"text"`
}

type metaInteractiveAction struct {
	Buttons  []metaButton  `json:"buttons,omitempty"`
	Button   string        `json:"button,omitempty"`
	Sections []metaSection `json:"sections,omitempty"`
	Name     string        `json:"name,omitempty"`
	Parameters *metaCTAParams `json:"parameters,omitempty"`
}

type metaButton struct {
	Type  string          `json:"type"` // reply
	Reply metaButtonReply `json:"reply"`
}

type metaButtonReply struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type metaSection struct {
	Title string        `json:"title,omitempty"`
	Rows  []metaSectionRow `json:"rows"`
}

type metaSectionRow struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

type metaCTAParams struct {
	DisplayText string `json:"display_text"`
	URL         string `json:"url"`
}

type metaContext struct {
	MessageID string `json:"message_id"`
}

type metaResponse struct {
	MessagingProduct string `json:"messaging_product"`
	Contacts         []struct {
		Input string `json:"input"`
		WAID  string `json:"wa_id"`
	} `json:"contacts"`
	Messages []struct {
		ID string `json:"id"`
	} `json:"messages"`
}

type metaErrorResponse struct {
	Error struct {
		Message   string `json:"message"`
		Type      string `json:"type"`
		Code      int    `json:"code"`
		FBTraceID string `json:"fbtrace_id"`
	} `json:"error"`
}

// Send delivers a notification via Meta WhatsApp Cloud API.
func (p *MetaWhatsAppProvider) Send(ctx context.Context, notif *notification.Notification, usr *user.User) (*Result, error) {
	start := time.Now()

	if usr == nil || usr.Phone == "" {
		return NewErrorResult(
			fmt.Errorf("user has no phone number for WhatsApp delivery"),
			ErrorTypeInvalid,
		), nil
	}

	// Per-app credential override
	phoneNumberID := p.config.PhoneNumberID
	accessToken := p.config.AccessToken
	apiVersion := p.config.APIVersion
	credSource := CredSourceSystem

	if appCfg, ok := ctx.Value(WhatsAppConfigKey).(*application.WhatsAppAppConfig); ok && appCfg != nil {
		if appCfg.Provider == "meta" && appCfg.MetaPhoneNumberID != "" && appCfg.MetaAccessToken != "" {
			phoneNumberID = appCfg.MetaPhoneNumberID
			accessToken = appCfg.MetaAccessToken
			credSource = CredSourceBYOC
			p.logger.Debug("Using per-app Meta WhatsApp config",
				zap.String("notification_id", notif.NotificationID),
				zap.String("phone_number_id", phoneNumberID))
		}
	}

	if phoneNumberID == "" || accessToken == "" {
		return NewErrorResult(
			fmt.Errorf("Meta WhatsApp credentials not configured: set phone_number_id and access_token in app settings or global config"),
			ErrorTypeConfiguration,
		), nil
	}

	// Strip any non-digit chars from the recipient phone (Meta expects digits only, no +)
	toNumber := sanitizePhone(usr.Phone)

	// Build the API payload
	msg := p.buildMessage(notif, toNumber)

	payload, err := json.Marshal(msg)
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to marshal WhatsApp message: %w", err), ErrorTypeUnknown), nil
	}

	apiURL := fmt.Sprintf("https://graph.facebook.com/%s/%s/messages", apiVersion, phoneNumberID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(payload))
	if err != nil {
		return NewErrorResult(fmt.Errorf("failed to create Meta WhatsApp request: %w", err), ErrorTypeUnknown), nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return NewErrorResult(fmt.Errorf("Meta WhatsApp request failed: %w", err), ErrorTypeNetwork), nil
	}
	defer resp.Body.Close()

	bodyBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return NewErrorResult(fmt.Errorf("failed to read Meta response: %w", readErr), ErrorTypeNetwork), nil
	}

	if resp.StatusCode != http.StatusOK {
		return p.handleErrorResponse(resp.StatusCode, bodyBytes, notif.NotificationID)
	}

	var metaResp metaResponse
	if err := json.Unmarshal(bodyBytes, &metaResp); err != nil {
		p.logger.Warn("Failed to decode Meta WhatsApp response",
			zap.Error(err),
			zap.String("raw_body", string(bodyBytes)))
	}

	providerMsgID := ""
	if len(metaResp.Messages) > 0 {
		providerMsgID = metaResp.Messages[0].ID
	}
	if providerMsgID == "" {
		providerMsgID = "meta-wa-" + notif.NotificationID
	}

	p.logger.Info("Meta WhatsApp notification delivered",
		zap.String("notification_id", notif.NotificationID),
		zap.String("meta_message_id", providerMsgID),
		zap.String("to", toNumber),
		zap.Duration("delivery_time", time.Since(start)))

	result := NewResult(providerMsgID, time.Since(start))
	result.Metadata["credential_source"] = credSource
	result.Metadata["billing_channel"] = "whatsapp"
	result.Metadata["provider"] = "meta"
	return result, nil
}

// buildMessage constructs the Meta API message payload.
func (p *MetaWhatsAppProvider) buildMessage(notif *notification.Notification, toNumber string) metaMessage {
	if notif.Content.Data != nil {
		// WhatsApp template
		if tplData, ok := notif.Content.Data["whatsapp_template"]; ok {
			if tpl, ok := tplData.(map[string]interface{}); ok {
				return p.buildTemplateMessage(tpl, toNumber)
			}
		}

		// Interactive messages (buttons, lists, CTA)
		if interData, ok := notif.Content.Data["whatsapp_interactive"]; ok {
			if inter, ok := interData.(map[string]interface{}); ok {
				return p.buildInteractiveMessage(inter, notif, toNumber)
			}
		}

		// Reaction
		if reactData, ok := notif.Content.Data["whatsapp_reaction"]; ok {
			if react, ok := reactData.(map[string]interface{}); ok {
				msgID, _ := react["message_id"].(string)
				emoji, _ := react["emoji"].(string)
				return metaMessage{
					MessagingProduct: "whatsapp",
					RecipientType:    "individual",
					To:               toNumber,
					Type:             "reaction",
					Reaction:         &metaReaction{MessageID: msgID, Emoji: emoji},
				}
			}
		}

		// Location
		if locData, ok := notif.Content.Data["whatsapp_location"]; ok {
			if loc, ok := locData.(map[string]interface{}); ok {
				lat, _ := loc["latitude"].(float64)
				lng, _ := loc["longitude"].(float64)
				name, _ := loc["name"].(string)
				addr, _ := loc["address"].(string)
				return metaMessage{
					MessagingProduct: "whatsapp",
					RecipientType:    "individual",
					To:               toNumber,
					Type:             "location",
					Location:         &metaLocation{Latitude: lat, Longitude: lng, Name: name, Address: addr},
				}
			}
		}

		// Contacts
		if contactsData, ok := notif.Content.Data["whatsapp_contacts"]; ok {
			if contacts, ok := contactsData.([]interface{}); ok {
				return p.buildContactsMessage(contacts, toNumber)
			}
		}

		// Video
		if videoData, ok := notif.Content.Data["whatsapp_video"]; ok {
			if v, ok := videoData.(map[string]interface{}); ok {
				link, _ := v["link"].(string)
				caption, _ := v["caption"].(string)
				return metaMessage{
					MessagingProduct: "whatsapp",
					RecipientType:    "individual",
					To:               toNumber,
					Type:             "video",
					Video:            &metaMedia{Link: link, Caption: caption},
				}
			}
		}

		// Audio
		if audioData, ok := notif.Content.Data["whatsapp_audio"]; ok {
			if a, ok := audioData.(map[string]interface{}); ok {
				link, _ := a["link"].(string)
				return metaMessage{
					MessagingProduct: "whatsapp",
					RecipientType:    "individual",
					To:               toNumber,
					Type:             "audio",
					Audio:            &metaMedia{Link: link},
				}
			}
		}

		// Document
		if docData, ok := notif.Content.Data["whatsapp_document"]; ok {
			if d, ok := docData.(map[string]interface{}); ok {
				link, _ := d["link"].(string)
				caption, _ := d["caption"].(string)
				filename, _ := d["filename"].(string)
				return metaMessage{
					MessagingProduct: "whatsapp",
					RecipientType:    "individual",
					To:               toNumber,
					Type:             "document",
					Document:         &metaDocumentMedia{Link: link, Caption: caption, Filename: filename},
				}
			}
		}

		// Sticker
		if stickerData, ok := notif.Content.Data["whatsapp_sticker"]; ok {
			if s, ok := stickerData.(map[string]interface{}); ok {
				link, _ := s["link"].(string)
				return metaMessage{
					MessagingProduct: "whatsapp",
					RecipientType:    "individual",
					To:               toNumber,
					Type:             "sticker",
					Sticker:          &metaMedia{Link: link},
				}
			}
		}
	}

	// Default: send a text message
	body := notif.Content.Body
	if notif.Content.Title != "" {
		body = fmt.Sprintf("*%s*\n\n%s", notif.Content.Title, notif.Content.Body)
	}

	msg := metaMessage{
		MessagingProduct: "whatsapp",
		RecipientType:    "individual",
		To:               toNumber,
		Type:             "text",
		Text: &metaText{
			PreviewURL: false,
			Body:       body,
		},
	}

	// If media is attached, send as image
	if notif.Content.MediaURL != "" {
		msg.Type = "image"
		msg.Text = nil
		msg.Image = &metaMedia{Link: notif.Content.MediaURL}
	}

	return msg
}

// buildInteractiveMessage constructs a buttons/list/cta_url interactive message.
func (p *MetaWhatsAppProvider) buildInteractiveMessage(inter map[string]interface{}, notif *notification.Notification, toNumber string) metaMessage {
	interType, _ := inter["type"].(string) // "button", "list", "cta_url"

	interactive := &metaInteractive{
		Type: interType,
		Body: metaInteractiveBody{Text: notif.Content.Body},
	}

	if headerData, ok := inter["header"].(map[string]interface{}); ok {
		hType, _ := headerData["type"].(string)
		hText, _ := headerData["text"].(string)
		interactive.Header = &metaInteractiveHeader{Type: hType, Text: hText}
	}
	if footerData, ok := inter["footer"].(map[string]interface{}); ok {
		fText, _ := footerData["text"].(string)
		interactive.Footer = &metaInteractiveFooter{Text: fText}
	}

	if actionData, ok := inter["action"].(map[string]interface{}); ok {
		// Reply buttons
		if btns, ok := actionData["buttons"].([]interface{}); ok {
			for _, b := range btns {
				bm, ok := b.(map[string]interface{})
				if !ok {
					continue
				}
				reply, _ := bm["reply"].(map[string]interface{})
				id, _ := reply["id"].(string)
				title, _ := reply["title"].(string)
				interactive.Action.Buttons = append(interactive.Action.Buttons, metaButton{
					Type:  "reply",
					Reply: metaButtonReply{ID: id, Title: title},
				})
			}
		}
		// List sections
		if btnText, ok := actionData["button"].(string); ok {
			interactive.Action.Button = btnText
		}
		if sections, ok := actionData["sections"].([]interface{}); ok {
			for _, sec := range sections {
				sm, ok := sec.(map[string]interface{})
				if !ok {
					continue
				}
				section := metaSection{}
				section.Title, _ = sm["title"].(string)
				if rows, ok := sm["rows"].([]interface{}); ok {
					for _, r := range rows {
						rm, ok := r.(map[string]interface{})
						if !ok {
							continue
						}
						row := metaSectionRow{}
						row.ID, _ = rm["id"].(string)
						row.Title, _ = rm["title"].(string)
						row.Description, _ = rm["description"].(string)
						section.Rows = append(section.Rows, row)
					}
				}
				interactive.Action.Sections = append(interactive.Action.Sections, section)
			}
		}
		// CTA URL
		if interType == "cta_url" {
			interactive.Action.Name = "cta_url"
			if params, ok := actionData["parameters"].(map[string]interface{}); ok {
				displayText, _ := params["display_text"].(string)
				ctaURL, _ := params["url"].(string)
				interactive.Action.Parameters = &metaCTAParams{DisplayText: displayText, URL: ctaURL}
			}
		}
	}

	return metaMessage{
		MessagingProduct: "whatsapp",
		RecipientType:    "individual",
		To:               toNumber,
		Type:             "interactive",
		Interactive:      interactive,
	}
}

// buildContactsMessage constructs a contact card message.
func (p *MetaWhatsAppProvider) buildContactsMessage(contacts []interface{}, toNumber string) metaMessage {
	var cards []metaContactCard
	for _, c := range contacts {
		cm, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		card := metaContactCard{}
		if nameData, ok := cm["name"].(map[string]interface{}); ok {
			card.Name.FormattedName, _ = nameData["formatted_name"].(string)
			card.Name.FirstName, _ = nameData["first_name"].(string)
			card.Name.LastName, _ = nameData["last_name"].(string)
		}
		if phones, ok := cm["phones"].([]interface{}); ok {
			for _, ph := range phones {
				pm, ok := ph.(map[string]interface{})
				if !ok {
					continue
				}
				phone, _ := pm["phone"].(string)
				pType, _ := pm["type"].(string)
				card.Phones = append(card.Phones, metaContactPhone{Phone: phone, Type: pType})
			}
		}
		cards = append(cards, card)
	}

	return metaMessage{
		MessagingProduct: "whatsapp",
		RecipientType:    "individual",
		To:               toNumber,
		Type:             "contacts",
		Contacts:         cards,
	}
}

// buildTemplateMessage constructs a template message from content data.
func (p *MetaWhatsAppProvider) buildTemplateMessage(tpl map[string]interface{}, toNumber string) metaMessage {
	name, _ := tpl["name"].(string)
	lang, _ := tpl["language"].(string)
	if lang == "" {
		lang = "en_US"
	}

	msg := metaMessage{
		MessagingProduct: "whatsapp",
		RecipientType:    "individual",
		To:               toNumber,
		Type:             "template",
		Template: &metaTemplate{
			Name:     name,
			Language: metaLanguage{Code: lang},
		},
	}

	// Parse components if present
	if comps, ok := tpl["components"].([]interface{}); ok {
		for _, c := range comps {
			comp, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			mc := metaComponent{Type: fmt.Sprintf("%v", comp["type"])}
			if params, ok := comp["parameters"].([]interface{}); ok {
				for _, param := range params {
					pm, ok := param.(map[string]interface{})
					if !ok {
						continue
					}
					mp := metaParameter{Type: fmt.Sprintf("%v", pm["type"])}
					if text, ok := pm["text"].(string); ok {
						mp.Text = text
					}
					mc.Parameters = append(mc.Parameters, mp)
				}
			}
			msg.Template.Components = append(msg.Template.Components, mc)
		}
	}

	return msg
}

// handleErrorResponse parses a Meta API error and returns an appropriate Result.
func (p *MetaWhatsAppProvider) handleErrorResponse(statusCode int, body []byte, notifID string) (*Result, error) {
	var metaErr metaErrorResponse
	if err := json.Unmarshal(body, &metaErr); err != nil {
		p.logger.Error("Meta WhatsApp API error (unparseable)",
			zap.Int("http_status", statusCode),
			zap.String("raw_body", string(body)))
		return NewErrorResult(
			fmt.Errorf("Meta WhatsApp API returned status %d", statusCode),
			ErrorTypeProviderAPI,
		), nil
	}

	errType := ErrorTypeProviderAPI
	switch {
	case statusCode == http.StatusUnauthorized || metaErr.Error.Code == 190:
		errType = ErrorTypeAuth
	case statusCode == http.StatusTooManyRequests || metaErr.Error.Code == 80007:
		errType = ErrorTypeRateLimit
	case statusCode == http.StatusBadRequest:
		errType = ErrorTypeInvalid
	}

	errMsg := fmt.Sprintf("Meta WhatsApp error %d: %s (trace: %s)",
		metaErr.Error.Code, metaErr.Error.Message, metaErr.Error.FBTraceID)

	p.logger.Error("Meta WhatsApp API error",
		zap.Int("http_status", statusCode),
		zap.Int("meta_error_code", metaErr.Error.Code),
		zap.String("meta_error_type", metaErr.Error.Type),
		zap.String("meta_error_message", metaErr.Error.Message),
		zap.String("fbtrace_id", metaErr.Error.FBTraceID),
		zap.String("notification_id", notifID))

	return NewErrorResult(fmt.Errorf("%s", errMsg), errType), nil
}

// sanitizePhone strips non-digit characters from a phone number.
// Meta expects digits only (no +, spaces, or dashes).
func sanitizePhone(phone string) string {
	// Strip common prefixes used by the Twilio provider
	phone = stripPrefix(phone, "whatsapp:")
	phone = stripPrefix(phone, "+")

	result := make([]byte, 0, len(phone))
	for i := 0; i < len(phone); i++ {
		if phone[i] >= '0' && phone[i] <= '9' {
			result = append(result, phone[i])
		}
	}
	return string(result)
}

func stripPrefix(s, prefix string) string {
	if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):]
	}
	return s
}

// GetName returns the provider name.
func (p *MetaWhatsAppProvider) GetName() string { return "meta_whatsapp" }

// GetSupportedChannel returns the channel this provider supports.
func (p *MetaWhatsAppProvider) GetSupportedChannel() notification.Channel {
	return notification.ChannelWhatsApp
}

// IsHealthy checks if the provider is healthy.
func (p *MetaWhatsAppProvider) IsHealthy(_ context.Context) bool { return true }

// Close releases provider resources.
func (p *MetaWhatsAppProvider) Close() error { return nil }

func init() {
	RegisterFactory("meta_whatsapp", func(cfg map[string]interface{}, logger *zap.Logger) (Provider, error) {
		enabled, _ := cfg["enabled"].(bool)
		phoneNumberID, _ := cfg["phone_number_id"].(string)

		if !enabled && phoneNumberID == "" {
			return nil, fmt.Errorf("meta_whatsapp: provider disabled")
		}

		accessToken, _ := cfg["access_token"].(string)
		wabaID, _ := cfg["waba_id"].(string)
		apiVersion, _ := cfg["api_version"].(string)

		timeout := 15
		if t, ok := cfg["timeout"].(float64); ok && t > 0 {
			timeout = int(t)
		}
		maxRetries := 3
		if r, ok := cfg["max_retries"].(float64); ok {
			maxRetries = int(r)
		}

		return NewMetaWhatsAppProvider(MetaWhatsAppConfig{
			Config:        Config{Timeout: time.Duration(timeout) * time.Second, MaxRetries: maxRetries, RetryDelay: 2 * time.Second},
			PhoneNumberID: phoneNumberID,
			WABAID:        wabaID,
			AccessToken:   accessToken,
			APIVersion:    apiVersion,
		}, logger)
	})
}
