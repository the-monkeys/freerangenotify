package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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
	Type    string `json:"type"`               // "header" | "body" | "footer" | "button" | "carousel"
	SubType string `json:"sub_type,omitempty"` // "URL" | "QUICK_REPLY" | "COPY_CODE" (only for button)
	// Meta sends Index as a string in the JSON ("0", "1"...) — that is the
	// position of the button inside the template definition.
	Index      string          `json:"index,omitempty"`
	Parameters []metaParameter `json:"parameters,omitempty"`

	// Carousel-only: each carousel component holds an array of cards.
	Cards []metaCard `json:"cards,omitempty"`
}

// metaCard is a single carousel card. The Meta Cloud API expects an explicit
// card_index (0-based) plus a nested components array mirroring the standard
// header/body/button structure.
type metaCard struct {
	CardIndex  int             `json:"card_index"`
	Components []metaComponent `json:"components"`
}

type metaParameter struct {
	Type       string             `json:"type"`
	Text       string             `json:"text,omitempty"`
	Payload    string             `json:"payload,omitempty"`     // QUICK_REPLY button payload
	CouponCode string             `json:"coupon_code,omitempty"` // COPY_CODE button code
	Image      *metaMedia         `json:"image,omitempty"`
	Video      *metaMedia         `json:"video,omitempty"`
	Document   *metaDocumentMedia `json:"document,omitempty"`
	Location   *metaLocation      `json:"location,omitempty"`
	Currency   *metaCurrency      `json:"currency,omitempty"`
	DateTime   *metaDateTime      `json:"date_time,omitempty"`
}

type metaCurrency struct {
	FallbackValue string `json:"fallback_value"`
	Code          string `json:"code"`
	Amount1000    int64  `json:"amount_1000"`
}

type metaDateTime struct {
	FallbackValue string `json:"fallback_value"`
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
	Buttons    []metaButton   `json:"buttons,omitempty"`
	Button     string         `json:"button,omitempty"`
	Sections   []metaSection  `json:"sections,omitempty"`
	Name       string         `json:"name,omitempty"`
	Parameters *metaCTAParams `json:"-"` // see MarshalJSON
	// Catalog / commerce fields (Phase 4 of WHATSAPP_RICH_INTERACTIVE_PLAN.md).
	// Single-product (interactive.type=product) populates CatalogID +
	// ProductRetailerID. Multi-product (interactive.type=product_list)
	// populates CatalogID + ProductSections instead of Sections (text rows).
	// CatalogMessage uses CatalogParameters with thumbnail_product_retailer_id.
	CatalogID         string               `json:"catalog_id,omitempty"`
	ProductRetailerID string               `json:"product_retailer_id,omitempty"`
	ProductSections   []metaProductSection `json:"-"`
	CatalogParameters *metaCatalogParams   `json:"-"`
}

// MarshalJSON for metaInteractiveAction encodes the polymorphic action
// shape Meta requires. Three discriminated cases share one type to keep
// the rest of the codebase free of action-variant unions:
//
//   * cta_url            → emits `parameters: {display_text, url}`
//   * catalog_message    → emits `parameters: {thumbnail_product_retailer_id}`
//   * product_list       → emits `sections: [{title, product_items[]}, ...]`
//
// `sections` is mutually exclusive between text-list and product-list;
// per Meta only one of them ships on the wire.
func (a metaInteractiveAction) MarshalJSON() ([]byte, error) {
	type alias metaInteractiveAction
	out := struct {
		alias
		Sections   interface{} `json:"sections,omitempty"`
		Parameters interface{} `json:"parameters,omitempty"`
	}{alias: alias(a)}

	switch {
	case len(a.ProductSections) > 0:
		out.Sections = a.ProductSections
	case len(a.Sections) > 0:
		out.Sections = a.Sections
	}
	switch {
	case a.CatalogParameters != nil:
		out.Parameters = a.CatalogParameters
	case a.Parameters != nil:
		out.Parameters = a.Parameters
	}

	// Clear the embedded copies so they don't double-emit (alias retains
	// the un-tagged shape; we route them through the explicit fields).
	out.alias.Sections = nil
	out.alias.ProductSections = nil
	out.alias.Parameters = nil
	out.alias.CatalogParameters = nil
	return json.Marshal(out)
}

// metaProductSection is one section inside a product_list interactive
// message. Each product_item references a catalog SKU by retailer ID.
type metaProductSection struct {
	Title        string            `json:"title,omitempty"`
	ProductItems []metaProductItem `json:"product_items"`
}

type metaProductItem struct {
	ProductRetailerID string `json:"product_retailer_id"`
}

// metaCatalogParams is the `action.parameters` payload for
// interactive.type=catalog_message. The optional thumbnail picks a hero
// product for the catalog launcher card.
type metaCatalogParams struct {
	ThumbnailProductRetailerID string `json:"thumbnail_product_retailer_id,omitempty"`
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
		// Rich typed payloads (carousel / coupon / etc.) take precedence over
		// the lower-level whatsapp_template path so callers can opt into the
		// validated, kind-aware DSL without losing back-compat for raw
		// templates already in use.
		if richData, ok := notif.Content.Data["whatsapp_rich"]; ok {
			if rich, ok := richData.(map[string]interface{}); ok {
				if msg, ok := p.buildRichMessage(rich, toNumber); ok {
					return msg
				}
			}
		}

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
//
// Supports the full Meta parameter taxonomy required by approved templates:
// text, image/video/document/location headers, currency, date_time, and
// BUTTON components with sub_type (URL / QUICK_REPLY / COPY_CODE) + index.
// Dropping any of these silently is the most common cause of "template
// rendered blank header" or "URL button missing" on the recipient device.
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

	comps, ok := tpl["components"].([]interface{})
	if !ok {
		return msg
	}
	for _, c := range comps {
		comp, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		mc := metaComponent{Type: stringField(comp, "type")}
		if v := stringField(comp, "sub_type"); v != "" {
			mc.SubType = v
		}
		// Index arrives either as JSON string ("0") or number (0) depending
		// on the caller — normalise to string per Meta's spec.
		if idx, ok := comp["index"]; ok {
			mc.Index = fmt.Sprintf("%v", idx)
		}
		if params, ok := comp["parameters"].([]interface{}); ok {
			for _, param := range params {
				pm, ok := param.(map[string]interface{})
				if !ok {
					continue
				}
				mc.Parameters = append(mc.Parameters, parseTemplateParam(pm))
			}
		}
		msg.Template.Components = append(msg.Template.Components, mc)
	}

	return msg
}

// parseTemplateParam converts a single Meta template parameter from the
// untyped map[string]interface{} input shape to a typed metaParameter.
// Unknown parameter types pass through with Text populated when present so
// the worker never silently drops data that may still be valid for newer
// Meta features.
func parseTemplateParam(pm map[string]interface{}) metaParameter {
	mp := metaParameter{Type: stringField(pm, "type")}
	switch mp.Type {
	case "text":
		mp.Text = stringField(pm, "text")
	case "payload":
		mp.Payload = stringField(pm, "payload")
	case "coupon_code":
		mp.CouponCode = stringField(pm, "coupon_code")
	case "image":
		if media, ok := pm["image"].(map[string]interface{}); ok {
			mp.Image = &metaMedia{Link: stringField(media, "link"), ID: stringField(media, "id"), Caption: stringField(media, "caption")}
		}
	case "video":
		if media, ok := pm["video"].(map[string]interface{}); ok {
			mp.Video = &metaMedia{Link: stringField(media, "link"), ID: stringField(media, "id"), Caption: stringField(media, "caption")}
		}
	case "document":
		if media, ok := pm["document"].(map[string]interface{}); ok {
			mp.Document = &metaDocumentMedia{
				Link:     stringField(media, "link"),
				ID:       stringField(media, "id"),
				Caption:  stringField(media, "caption"),
				Filename: stringField(media, "filename"),
			}
		}
	case "location":
		if loc, ok := pm["location"].(map[string]interface{}); ok {
			lat, _ := loc["latitude"].(float64)
			lng, _ := loc["longitude"].(float64)
			mp.Location = &metaLocation{
				Latitude:  lat,
				Longitude: lng,
				Name:      stringField(loc, "name"),
				Address:   stringField(loc, "address"),
			}
		}
	case "currency":
		if cur, ok := pm["currency"].(map[string]interface{}); ok {
			amount, _ := cur["amount_1000"].(float64)
			mp.Currency = &metaCurrency{
				FallbackValue: stringField(cur, "fallback_value"),
				Code:          stringField(cur, "code"),
				Amount1000:    int64(amount),
			}
		}
	case "date_time":
		if dt, ok := pm["date_time"].(map[string]interface{}); ok {
			mp.DateTime = &metaDateTime{FallbackValue: stringField(dt, "fallback_value")}
		}
	default:
		// Forward-compatible: keep `text` if a caller used a new parameter
		// type Meta added since this code was written.
		mp.Text = stringField(pm, "text")
	}
	return mp
}

// stringField is a small helper that returns the string value at key, or ""
// if the key is missing or the value is not a string. Avoids the repetitive
// `v, _ := m[k].(string)` pattern in parsing code.
func stringField(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
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

// buildRichMessage routes the typed whatsapp_rich payload to the appropriate
// renderer based on its "kind". Returns (msg, true) on a recognized,
// well-formed kind; (zero, false) when the payload is malformed or the kind
// is unknown so the caller can fall through to legacy renderers.
//
// Caller-friendly shape accepted today:
//
//	{
//	  "kind": "carousel" | "coupon_code",
//	  "template_name": "trendy_styles_carousel",
//	  "language": "en_US",
//	  "body_variables": ["Asha"],          // BODY {{1}}, {{2}}, ...
//	  "cards": [                            // carousel only
//	    {
//	      "header_image_url": "https://cdn/.../1.jpg",
//	      "header_video_url": "https://...",   // alt to image
//	      "body_variables":   ["Polo", "₹229"],
//	      "buttons": [
//	        { "sub_type": "URL",         "text": "p/12345" },
//	        { "sub_type": "QUICK_REPLY", "payload": "REORDER_42" }
//	      ]
//	    }
//	  ],
//	  "coupon_code": "DEAL50"              // coupon_code only
//	}
func (p *MetaWhatsAppProvider) buildRichMessage(rich map[string]interface{}, toNumber string) (metaMessage, bool) {
	kind := stringField(rich, "kind")

	// Product / multi-product / catalog kinds are NOT templates — they
	// build interactive messages and don't require a template_name.
	switch kind {
	case "product":
		return p.buildProductMessage(rich, toNumber)
	case "multi_product":
		return p.buildMultiProductMessage(rich, toNumber)
	case "catalog":
		return p.buildCatalogMessage(rich, toNumber)
	}

	templateName := stringField(rich, "template_name")
	if templateName == "" {
		return metaMessage{}, false
	}
	language := stringField(rich, "language")
	if language == "" {
		language = "en_US"
	}

	msg := metaMessage{
		MessagingProduct: "whatsapp",
		RecipientType:    "individual",
		To:               toNumber,
		Type:             "template",
		Template: &metaTemplate{
			Name:     templateName,
			Language: metaLanguage{Code: language},
		},
	}

	// Top-level BODY parameters apply to all kinds: they fill {{1}}..{{n}} in
	// the template body outside the cards array.
	if body := buildBodyComponent(rich["body_variables"]); body != nil {
		msg.Template.Components = append(msg.Template.Components, *body)
	}

	switch kind {
	case "carousel":
		cards, ok := buildCarouselCards(rich["cards"])
		if !ok {
			return metaMessage{}, false
		}
		msg.Template.Components = append(msg.Template.Components, metaComponent{
			Type:  "carousel",
			Cards: cards,
		})
		return msg, true

	case "coupon_code":
		code := stringField(rich, "coupon_code")
		if code == "" {
			return metaMessage{}, false
		}
		msg.Template.Components = append(msg.Template.Components, metaComponent{
			Type:    "button",
			SubType: "COPY_CODE",
			Index:   "0",
			Parameters: []metaParameter{
				{Type: "coupon_code", CouponCode: code},
			},
		})
		return msg, true

	default:
		// Unknown kind — caller can fall through to legacy whatsapp_template.
		return metaMessage{}, false
	}
}

// buildBodyComponent turns a []interface{} of strings into a BODY component
// of typed text parameters. Returns nil when the input is missing or empty
// so callers can omit the component entirely (Meta rejects empty components).
func buildBodyComponent(raw interface{}) *metaComponent {
	vars, ok := raw.([]interface{})
	if !ok || len(vars) == 0 {
		return nil
	}
	params := make([]metaParameter, 0, len(vars))
	for _, v := range vars {
		params = append(params, metaParameter{Type: "text", Text: fmt.Sprintf("%v", v)})
	}
	return &metaComponent{Type: "body", Parameters: params}
}

// buildCarouselCards converts the caller's "cards" array into Meta's
// card_index + components shape. Returns (cards, false) when the array is
// missing or empty so the caller can refuse to send a malformed carousel.
func buildCarouselCards(raw interface{}) ([]metaCard, bool) {
	arr, ok := raw.([]interface{})
	if !ok || len(arr) == 0 {
		return nil, false
	}
	cards := make([]metaCard, 0, len(arr))
	for i, c := range arr {
		cm, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		card := metaCard{CardIndex: i}

		if header := buildCardHeader(cm); header != nil {
			card.Components = append(card.Components, *header)
		}
		if body := buildBodyComponent(cm["body_variables"]); body != nil {
			card.Components = append(card.Components, *body)
		}
		for btnIdx, b := range cardButtons(cm["buttons"]) {
			b.Index = fmt.Sprintf("%d", btnIdx)
			card.Components = append(card.Components, b)
		}
		cards = append(cards, card)
	}
	if len(cards) == 0 {
		return nil, false
	}
	return cards, true
}

// buildCardHeader picks the first present header media (image > video >
// document) for a card. Carousel cards must have uniform header type across
// all cards per Meta's spec; that is enforced at authoring time, not here.
func buildCardHeader(cm map[string]interface{}) *metaComponent {
	if link := stringField(cm, "header_image_url"); link != "" {
		return &metaComponent{Type: "header", Parameters: []metaParameter{{Type: "image", Image: &metaMedia{Link: link}}}}
	}
	if link := stringField(cm, "header_video_url"); link != "" {
		return &metaComponent{Type: "header", Parameters: []metaParameter{{Type: "video", Video: &metaMedia{Link: link}}}}
	}
	if doc, ok := cm["header_document"].(map[string]interface{}); ok {
		return &metaComponent{Type: "header", Parameters: []metaParameter{{Type: "document", Document: &metaDocumentMedia{
			Link:     stringField(doc, "link"),
			Filename: stringField(doc, "filename"),
		}}}}
	}
	return nil
}

// cardButtons normalises the per-card buttons array into typed button
// components. Each button needs sub_type (URL | QUICK_REPLY | COPY_CODE) and
// the appropriate parameter. The card-relative index is assigned by the
// caller — see buildCarouselCards.
func cardButtons(raw interface{}) []metaComponent {
	arr, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	out := make([]metaComponent, 0, len(arr))
	for _, b := range arr {
		bm, ok := b.(map[string]interface{})
		if !ok {
			continue
		}
		subType := strings.ToUpper(stringField(bm, "sub_type"))
		if subType == "" {
			continue
		}
		comp := metaComponent{Type: "button", SubType: subType}
		switch subType {
		case "URL":
			if text := stringField(bm, "text"); text != "" {
				comp.Parameters = []metaParameter{{Type: "text", Text: text}}
			}
		case "QUICK_REPLY":
			if payload := stringField(bm, "payload"); payload != "" {
				comp.Parameters = []metaParameter{{Type: "payload", Payload: payload}}
			}
		case "COPY_CODE":
			if code := stringField(bm, "coupon_code"); code != "" {
				comp.Parameters = []metaParameter{{Type: "coupon_code", CouponCode: code}}
			}
		default:
			// Unknown sub_type — skip rather than emit malformed JSON.
			continue
		}
		out = append(out, comp)
	}
	return out
}

// buildProductMessage emits an interactive.type=product payload (Meta
// commerce single-product card). Expected `rich` shape:
//
//	{
//	  "kind": "product",
//	  "body": "Check out our hero product",
//	  "footer": "Tap to view",
//	  "catalog_id": "1234567890",
//	  "product_retailer_id": "sku-001"
//	}
func (p *MetaWhatsAppProvider) buildProductMessage(rich map[string]interface{}, toNumber string) (metaMessage, bool) {
	catalogID := stringField(rich, "catalog_id")
	productID := stringField(rich, "product_retailer_id")
	if catalogID == "" || productID == "" {
		return metaMessage{}, false
	}
	body := stringField(rich, "body")
	msg := metaMessage{
		MessagingProduct: "whatsapp",
		RecipientType:    "individual",
		To:               toNumber,
		Type:             "interactive",
		Interactive: &metaInteractive{
			Type: "product",
			Body: metaInteractiveBody{Text: body},
			Action: metaInteractiveAction{
				CatalogID:         catalogID,
				ProductRetailerID: productID,
			},
		},
	}
	if footer := stringField(rich, "footer"); footer != "" {
		msg.Interactive.Footer = &metaInteractiveFooter{Text: footer}
	}
	return msg, true
}

// buildMultiProductMessage emits an interactive.type=product_list payload
// (Meta commerce multi-product picker). Expected `rich` shape:
//
//	{
//	  "kind": "multi_product",
//	  "header": "Best Sellers",
//	  "body":   "Pick your favourite",
//	  "footer": "Free shipping today",
//	  "catalog_id": "1234567890",
//	  "sections": [
//	    {
//	      "title": "Tops",
//	      "product_retailer_ids": ["sku-1", "sku-2"]
//	    },
//	    ...
//	  ]
//	}
func (p *MetaWhatsAppProvider) buildMultiProductMessage(rich map[string]interface{}, toNumber string) (metaMessage, bool) {
	catalogID := stringField(rich, "catalog_id")
	if catalogID == "" {
		return metaMessage{}, false
	}
	sectionsRaw, _ := rich["sections"].([]interface{})
	if len(sectionsRaw) == 0 {
		return metaMessage{}, false
	}

	sections := make([]metaProductSection, 0, len(sectionsRaw))
	for _, s := range sectionsRaw {
		sm, ok := s.(map[string]interface{})
		if !ok {
			continue
		}
		idsRaw, _ := sm["product_retailer_ids"].([]interface{})
		items := make([]metaProductItem, 0, len(idsRaw))
		for _, id := range idsRaw {
			if str, ok := id.(string); ok && str != "" {
				items = append(items, metaProductItem{ProductRetailerID: str})
			}
		}
		if len(items) == 0 {
			continue
		}
		sections = append(sections, metaProductSection{
			Title:        stringField(sm, "title"),
			ProductItems: items,
		})
	}
	if len(sections) == 0 {
		return metaMessage{}, false
	}

	msg := metaMessage{
		MessagingProduct: "whatsapp",
		RecipientType:    "individual",
		To:               toNumber,
		Type:             "interactive",
		Interactive: &metaInteractive{
			Type: "product_list",
			Body: metaInteractiveBody{Text: stringField(rich, "body")},
			Action: metaInteractiveAction{
				CatalogID:       catalogID,
				ProductSections: sections,
			},
		},
	}
	if header := stringField(rich, "header"); header != "" {
		msg.Interactive.Header = &metaInteractiveHeader{Type: "text", Text: header}
	}
	if footer := stringField(rich, "footer"); footer != "" {
		msg.Interactive.Footer = &metaInteractiveFooter{Text: footer}
	}
	return msg, true
}

// buildCatalogMessage emits an interactive.type=catalog_message payload
// (full catalog browser launch). Expected `rich` shape:
//
//	{
//	  "kind": "catalog",
//	  "body": "Browse our catalog",
//	  "footer": "Tap below",
//	  "thumbnail_product_retailer_id": "sku-hero"
//	}
//
// catalog_message uses the app's bound catalog automatically — there is
// no catalog_id field on the wire (Meta resolves it from the WABA's
// commerce settings).
func (p *MetaWhatsAppProvider) buildCatalogMessage(rich map[string]interface{}, toNumber string) (metaMessage, bool) {
	body := stringField(rich, "body")
	if body == "" {
		return metaMessage{}, false
	}
	thumb := stringField(rich, "thumbnail_product_retailer_id")
	msg := metaMessage{
		MessagingProduct: "whatsapp",
		RecipientType:    "individual",
		To:               toNumber,
		Type:             "interactive",
		Interactive: &metaInteractive{
			Type: "catalog_message",
			Body: metaInteractiveBody{Text: body},
			Action: metaInteractiveAction{
				Name: "catalog_message",
			},
		},
	}
	if thumb != "" {
		msg.Interactive.Action.CatalogParameters = &metaCatalogParams{ThumbnailProductRetailerID: thumb}
	}
	if footer := stringField(rich, "footer"); footer != "" {
		msg.Interactive.Footer = &metaInteractiveFooter{Text: footer}
	}
	return msg, true
}

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
