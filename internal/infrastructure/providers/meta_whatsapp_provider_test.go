package providers

import (
	"encoding/json"
	"testing"

	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
	"go.uber.org/zap"
)

// newTestMetaProvider constructs a MetaWhatsAppProvider with a no-op logger
// for unit tests that exercise build* helpers in isolation.
func newTestMetaProvider(t *testing.T) *MetaWhatsAppProvider {
	t.Helper()
	p, err := NewMetaWhatsAppProvider(MetaWhatsAppConfig{
		PhoneNumberID: "stub",
		AccessToken:   "stub",
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("provider init: %v", err)
	}
	return p
}

// TestBuildTemplateMessage_AllParameterTypes verifies that every Meta
// template parameter type round-trips through the build path. Regression
// guard: prior to this change, only `text` parameters were forwarded, which
// caused approved templates with media headers or URL buttons to render
// blank on the device. The test asserts a) the typed structs are populated
// correctly, and b) the JSON Meta sees over the wire matches the spec.
func TestBuildTemplateMessage_AllParameterTypes(t *testing.T) {
	p := newTestMetaProvider(t)
	tpl := map[string]interface{}{
		"name":     "order_shipped",
		"language": "en_US",
		"components": []interface{}{
			map[string]interface{}{
				"type": "header",
				"parameters": []interface{}{
					map[string]interface{}{
						"type":  "image",
						"image": map[string]interface{}{"link": "https://cdn.example/h.jpg"},
					},
				},
			},
			map[string]interface{}{
				"type": "body",
				"parameters": []interface{}{
					map[string]interface{}{"type": "text", "text": "Asha"},
					map[string]interface{}{
						"type": "currency",
						"currency": map[string]interface{}{
							"fallback_value": "$12.34",
							"code":           "USD",
							"amount_1000":    float64(12340),
						},
					},
					map[string]interface{}{
						"type":      "date_time",
						"date_time": map[string]interface{}{"fallback_value": "Jan 1, 2026"},
					},
				},
			},
			map[string]interface{}{
				"type":     "button",
				"sub_type": "URL",
				"index":    "0",
				"parameters": []interface{}{
					map[string]interface{}{"type": "text", "text": "track/ORD-42"},
				},
			},
			map[string]interface{}{
				"type":     "button",
				"sub_type": "QUICK_REPLY",
				"index":    float64(1), // also accept JSON number form
				"parameters": []interface{}{
					map[string]interface{}{"type": "payload", "payload": "REORDER_42"},
				},
			},
		},
	}

	msg := p.buildTemplateMessage(tpl, "15551234567")

	if msg.Type != "template" || msg.Template == nil {
		t.Fatalf("expected template message, got %+v", msg)
	}
	if msg.Template.Name != "order_shipped" {
		t.Errorf("template name: got %q, want order_shipped", msg.Template.Name)
	}
	if got, want := len(msg.Template.Components), 4; got != want {
		t.Fatalf("components: got %d, want %d", got, want)
	}

	header := msg.Template.Components[0]
	if header.Type != "header" || len(header.Parameters) != 1 {
		t.Fatalf("header component shape: %+v", header)
	}
	if header.Parameters[0].Image == nil || header.Parameters[0].Image.Link != "https://cdn.example/h.jpg" {
		t.Errorf("image param dropped: %+v", header.Parameters[0])
	}

	body := msg.Template.Components[1]
	if len(body.Parameters) != 3 {
		t.Fatalf("body params: got %d, want 3", len(body.Parameters))
	}
	if body.Parameters[1].Currency == nil || body.Parameters[1].Currency.Code != "USD" || body.Parameters[1].Currency.Amount1000 != 12340 {
		t.Errorf("currency param dropped: %+v", body.Parameters[1])
	}
	if body.Parameters[2].DateTime == nil || body.Parameters[2].DateTime.FallbackValue != "Jan 1, 2026" {
		t.Errorf("date_time param dropped: %+v", body.Parameters[2])
	}

	urlBtn := msg.Template.Components[2]
	if urlBtn.SubType != "URL" || urlBtn.Index != "0" {
		t.Errorf("URL button missing sub_type/index: %+v", urlBtn)
	}

	qrBtn := msg.Template.Components[3]
	if qrBtn.SubType != "QUICK_REPLY" || qrBtn.Index != "1" {
		t.Errorf("QUICK_REPLY button missing sub_type/index: %+v", qrBtn)
	}
	if qrBtn.Parameters[0].Type != "payload" || qrBtn.Parameters[0].Payload != "REORDER_42" {
		t.Errorf("QUICK_REPLY payload dropped: %+v", qrBtn.Parameters[0])
	}

	// Wire-shape: the JSON Meta receives must include sub_type, index, and
	// the parameter sub-objects. Spot-check fields that broke before.
	raw, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	wire := string(raw)
	for _, needle := range []string{
		`"sub_type":"URL"`,
		`"sub_type":"QUICK_REPLY"`,
		`"index":"0"`,
		`"index":"1"`,
		`"payload":"REORDER_42"`,
		`"amount_1000":12340`,
		`"fallback_value":"Jan 1, 2026"`,
		`"link":"https://cdn.example/h.jpg"`,
	} {
		if !contains(wire, needle) {
			t.Errorf("expected JSON to contain %q\nwire: %s", needle, wire)
		}
	}
}

// TestBuildRichMessage_Carousel renders a two-card Snapdeal-style carousel
// via the typed whatsapp_rich payload and asserts that the wire JSON Meta
// receives matches the Cloud API carousel template spec: a top-level BODY
// component plus a `carousel` component whose cards each carry an image
// HEADER, a BODY with positional text parameters, and a card-scoped URL
// button with sub_type/index/parameters set correctly.
func TestBuildRichMessage_Carousel(t *testing.T) {
	p := newTestMetaProvider(t)
	rich := map[string]interface{}{
		"kind":           "carousel",
		"template_name":  "trendy_styles_carousel",
		"language":       "en_US",
		"body_variables": []interface{}{"Asha"},
		"cards": []interface{}{
			map[string]interface{}{
				"header_image_url": "https://cdn.example/p1.jpg",
				"body_variables":   []interface{}{"Trendy Polo", "₹229.00"},
				"buttons": []interface{}{
					map[string]interface{}{"sub_type": "URL", "text": "p/12345"},
				},
			},
			map[string]interface{}{
				"header_image_url": "https://cdn.example/p2.jpg",
				"body_variables":   []interface{}{"Athleisure Polo", "₹260.00"},
				"buttons": []interface{}{
					map[string]interface{}{"sub_type": "URL", "text": "p/67890"},
					map[string]interface{}{"sub_type": "QUICK_REPLY", "payload": "SHOP_NOW"},
				},
			},
		},
	}

	notif := &notification.Notification{
		NotificationID: "n-2",
		Content:        notification.Content{Data: map[string]interface{}{"whatsapp_rich": rich}},
	}

	msg := p.buildMessage(notif, "15551234567")
	if msg.Type != "template" || msg.Template == nil {
		t.Fatalf("expected template, got %+v", msg)
	}
	if msg.Template.Name != "trendy_styles_carousel" {
		t.Errorf("template name: got %q", msg.Template.Name)
	}
	if got, want := len(msg.Template.Components), 2; got != want {
		t.Fatalf("top-level components: got %d, want %d (body + carousel)", got, want)
	}
	if msg.Template.Components[0].Type != "body" {
		t.Errorf("first component should be body, got %s", msg.Template.Components[0].Type)
	}

	car := msg.Template.Components[1]
	if car.Type != "carousel" || len(car.Cards) != 2 {
		t.Fatalf("carousel shape wrong: %+v", car)
	}

	if car.Cards[0].CardIndex != 0 || car.Cards[1].CardIndex != 1 {
		t.Errorf("card_index not 0/1: %d, %d", car.Cards[0].CardIndex, car.Cards[1].CardIndex)
	}

	// Card 1 has two buttons → button indexes must be card-scoped 0 and 1
	c1 := car.Cards[1]
	var btnIdxs []string
	for _, comp := range c1.Components {
		if comp.Type == "button" {
			btnIdxs = append(btnIdxs, comp.Index)
		}
	}
	if len(btnIdxs) != 2 || btnIdxs[0] != "0" || btnIdxs[1] != "1" {
		t.Errorf("card-scoped button indexes wrong: %v", btnIdxs)
	}

	// Wire-shape spot-checks against Meta's documented carousel JSON.
	raw, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	wire := string(raw)
	for _, needle := range []string{
		`"type":"carousel"`,
		`"card_index":0`,
		`"card_index":1`,
		`"link":"https://cdn.example/p1.jpg"`,
		`"link":"https://cdn.example/p2.jpg"`,
		`"sub_type":"URL"`,
		`"sub_type":"QUICK_REPLY"`,
		`"payload":"SHOP_NOW"`,
	} {
		if !contains(wire, needle) {
			t.Errorf("expected JSON to contain %q\nwire: %s", needle, wire)
		}
	}
}

// TestBuildRichMessage_CouponCode renders a template with a COPY_CODE button
// (the WhatsApp coupon-share pattern). Verifies the button parameter uses
// type=coupon_code with the literal code, which is what Meta expects to
// surface the "Copy" CTA on the recipient device.
func TestBuildRichMessage_CouponCode(t *testing.T) {
	p := newTestMetaProvider(t)
	rich := map[string]interface{}{
		"kind":           "coupon_code",
		"template_name":  "festive_discount",
		"language":       "en_US",
		"body_variables": []interface{}{"Diwali", "50"},
		"coupon_code":    "DEAL50",
	}
	notif := &notification.Notification{
		Content: notification.Content{Data: map[string]interface{}{"whatsapp_rich": rich}},
	}
	msg := p.buildMessage(notif, "15551234567")
	if msg.Type != "template" {
		t.Fatalf("expected template, got %s", msg.Type)
	}
	wire := mustJSON(t, msg)
	for _, needle := range []string{
		`"sub_type":"COPY_CODE"`,
		`"index":"0"`,
		`"type":"coupon_code"`,
		`"coupon_code":"DEAL50"`,
		`"text":"Diwali"`,
	} {
		if !contains(wire, needle) {
			t.Errorf("expected JSON to contain %q\nwire: %s", needle, wire)
		}
	}
}

// TestBuildRichMessage_FallsThroughOnMalformed asserts that a malformed
// whatsapp_rich blob (missing template_name, no cards for a carousel, unknown
// kind) does NOT crash and does NOT short-circuit the renderer — the caller
// gets the plain-text fallback so something still ships.
func TestBuildRichMessage_FallsThroughOnMalformed(t *testing.T) {
	p := newTestMetaProvider(t)
	cases := []map[string]interface{}{
		{"kind": "carousel"},                            // no template_name
		{"kind": "carousel", "template_name": "x"},      // no cards
		{"kind": "unknown_kind", "template_name": "x"},  // unknown kind
		{"kind": "coupon_code", "template_name": "x"},   // no coupon_code
	}
	for i, rich := range cases {
		notif := &notification.Notification{
			Content: notification.Content{
				Body: "fallback body",
				Data: map[string]interface{}{"whatsapp_rich": rich},
			},
		}
		msg := p.buildMessage(notif, "15551234567")
		if msg.Type != "text" {
			t.Errorf("case %d: expected text fallback, got %s (rich=%+v)", i, msg.Type, rich)
		}
	}
}

func mustJSON(t *testing.T, v interface{}) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

// TestBuildMessage_DefaultsToText guards against regressions where adding a
// new whatsapp_* data key inadvertently breaks the fall-through to a plain
// text message.
func TestBuildMessage_DefaultsToText(t *testing.T) {
	p := newTestMetaProvider(t)
	notif := &notification.Notification{
		NotificationID: "n-1",
		Content: notification.Content{
			Title: "Hello",
			Body:  "world",
		},
	}
	msg := p.buildMessage(notif, "15551234567")
	if msg.Type != "text" || msg.Text == nil {
		t.Fatalf("expected text fallback, got %+v", msg)
	}
	if msg.Text.Body == "" {
		t.Errorf("text body empty")
	}
}

// TestBuildRichMessage_SingleProduct asserts a kind=product payload renders
// as interactive.type=product with catalog_id + product_retailer_id on the
// action root, not under template components. Validates the Phase 4
// commerce path.
func TestBuildRichMessage_SingleProduct(t *testing.T) {
	p := newTestMetaProvider(t)
	notif := &notification.Notification{
		Content: notification.Content{
			Data: map[string]interface{}{
				"whatsapp_rich": map[string]interface{}{
					"kind":                "product",
					"body":                "Check it out",
					"footer":              "Tap to view",
					"catalog_id":          "1234567890",
					"product_retailer_id": "sku-001",
				},
			},
		},
	}
	msg := p.buildMessage(notif, "15551234567")
	if msg.Type != "interactive" || msg.Interactive == nil {
		t.Fatalf("expected interactive message, got %s / %+v", msg.Type, msg.Interactive)
	}
	if msg.Interactive.Type != "product" {
		t.Errorf("interactive.type: got %q want product", msg.Interactive.Type)
	}
	wire := mustJSON(t, msg)
	for _, needle := range []string{
		`"type":"product"`,
		`"catalog_id":"1234567890"`,
		`"product_retailer_id":"sku-001"`,
		`"body":{"text":"Check it out"}`,
		`"footer":{"text":"Tap to view"}`,
	} {
		if !contains(wire, needle) {
			t.Errorf("missing %q in wire JSON\nwire: %s", needle, wire)
		}
	}
}

// TestBuildRichMessage_MultiProduct asserts a kind=multi_product payload
// renders as interactive.type=product_list with the section.product_items
// shape Meta requires. The MarshalJSON override is exercised here.
func TestBuildRichMessage_MultiProduct(t *testing.T) {
	p := newTestMetaProvider(t)
	notif := &notification.Notification{
		Content: notification.Content{
			Data: map[string]interface{}{
				"whatsapp_rich": map[string]interface{}{
					"kind":       "multi_product",
					"header":     "Best Sellers",
					"body":       "Pick one",
					"footer":     "Free shipping today",
					"catalog_id": "999",
					"sections": []interface{}{
						map[string]interface{}{
							"title":                "Tops",
							"product_retailer_ids": []interface{}{"sku-1", "sku-2"},
						},
						map[string]interface{}{
							"title":                "Bottoms",
							"product_retailer_ids": []interface{}{"sku-3"},
						},
					},
				},
			},
		},
	}
	msg := p.buildMessage(notif, "15551234567")
	if msg.Type != "interactive" || msg.Interactive == nil || msg.Interactive.Type != "product_list" {
		t.Fatalf("expected interactive.product_list, got %+v", msg.Interactive)
	}
	wire := mustJSON(t, msg)
	for _, needle := range []string{
		`"type":"product_list"`,
		`"catalog_id":"999"`,
		`"sections":[`,
		`"title":"Tops"`,
		`"product_items":[{"product_retailer_id":"sku-1"},{"product_retailer_id":"sku-2"}]`,
		`"header":{"type":"text","text":"Best Sellers"}`,
		`"footer":{"text":"Free shipping today"}`,
	} {
		if !contains(wire, needle) {
			t.Errorf("missing %q in wire JSON\nwire: %s", needle, wire)
		}
	}
}

// TestBuildRichMessage_Catalog asserts a kind=catalog payload renders as
// interactive.type=catalog_message with the thumbnail key emitted under
// action.parameters — the bespoke field Meta requires for the catalog
// launcher's hero product.
func TestBuildRichMessage_Catalog(t *testing.T) {
	p := newTestMetaProvider(t)
	notif := &notification.Notification{
		Content: notification.Content{
			Data: map[string]interface{}{
				"whatsapp_rich": map[string]interface{}{
					"kind":                          "catalog",
					"body":                          "Browse our catalog",
					"footer":                        "Tap below",
					"thumbnail_product_retailer_id": "sku-hero",
				},
			},
		},
	}
	msg := p.buildMessage(notif, "15551234567")
	if msg.Type != "interactive" || msg.Interactive == nil || msg.Interactive.Type != "catalog_message" {
		t.Fatalf("expected interactive.catalog_message, got %+v", msg.Interactive)
	}
	wire := mustJSON(t, msg)
	for _, needle := range []string{
		`"type":"catalog_message"`,
		`"name":"catalog_message"`,
		`"parameters":{"thumbnail_product_retailer_id":"sku-hero"}`,
		`"body":{"text":"Browse our catalog"}`,
	} {
		if !contains(wire, needle) {
			t.Errorf("missing %q in wire JSON\nwire: %s", needle, wire)
		}
	}
}

// contains is a tiny helper to keep the table-driven assertions above terse.
func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
