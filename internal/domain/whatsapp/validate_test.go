package whatsapp

import (
	"strings"
	"testing"
)

// validCarousel is a minimum-viable carousel template used as the baseline
// for the negative-case tests. Each test mutates one field then asserts that
// Validate returns the expected error code, isolating the rule under test.
func validCarousel() *RichTemplate {
	return &RichTemplate{
		Name:     "snapdeal_styles",
		AppID:    "app-1",
		Language: "en_US",
		Category: "MARKETING",
		Kind:     KindCarousel,
		Body:     "Hi {{1}}, check these out:",
		Cards: []CarouselCard{
			{
				HeaderImageURL: "https://cdn/.../1.jpg",
				Body:           "Polo {{1}} {{2}}",
				Buttons:        []Button{{Type: ButtonURL, Text: "View", URL: "https://shop/p/1"}},
			},
			{
				HeaderImageURL: "https://cdn/.../2.jpg",
				Body:           "Tee {{1}}",
				Buttons:        []Button{{Type: ButtonURL, Text: "View", URL: "https://shop/p/2"}},
			},
		},
	}
}

// TestValidate_HappyPaths covers one fully-populated example per kind.
// Together they prove the validator does not reject well-formed templates.
func TestValidate_HappyPaths(t *testing.T) {
	cases := map[string]*RichTemplate{
		"carousel":     validCarousel(),
		"coupon_code":  {Name: "festive", AppID: "a", Language: "en_US", Category: "MARKETING", Kind: KindCouponCode, Body: "Use code", CouponCode: "DEAL50"},
		"cta_url":      {Name: "see_more", AppID: "a", Language: "en_US", Category: "MARKETING", Kind: KindCTAURL, Body: "Tap below", Buttons: []Button{{Type: ButtonURL, Text: "Visit", URL: "https://x"}}},
		"quick_reply":  {Name: "yes_no", AppID: "a", Language: "en_US", Category: "UTILITY", Kind: KindQuickReply, Body: "Confirm?", Buttons: []Button{{Type: ButtonQuickReply, Text: "Yes", Payload: "YES"}, {Type: ButtonQuickReply, Text: "No", Payload: "NO"}}},
		"list":         {Name: "menu", AppID: "a", Language: "en_US", Category: "UTILITY", Kind: KindList, Body: "Pick one", ListButtonText: "Menu", ListSections: []ListSection{{Title: "Sizes", Rows: []ListRow{{ID: "s", Title: "S"}, {ID: "m", Title: "M"}}}}},
	}
	for name, tpl := range cases {
		errs := Validate(tpl)
		if !errs.IsEmpty() {
			t.Errorf("%s: expected no errors, got %v", name, errs)
		}
	}
}

// TestValidate_NameRules exercises the snake_case + non-empty rules. Keeping
// these in one table-driven test so the surface is obvious in one place.
func TestValidate_NameRules(t *testing.T) {
	cases := []struct {
		name    string
		wantErr string
	}{
		{"", "name is required"},
		{"CamelCase", "lower snake_case"},
		{"with-hyphen", "lower snake_case"},
		{"1leading_digit", "lower snake_case"},
		{strings.Repeat("a", maxTemplateNameChars+1), "exceeds"},
	}
	for _, tc := range cases {
		tpl := validCarousel()
		tpl.Name = tc.name
		errs := Validate(tpl)
		if errs.IsEmpty() || !strings.Contains(errs.Error(), tc.wantErr) {
			t.Errorf("name %q: expected error containing %q, got %v", tc.name, tc.wantErr, errs)
		}
	}
}

// TestValidate_Carousel_MixedHeaders catches the most common Meta-submission
// rejection: image header on card 0, video header on card 1.
func TestValidate_Carousel_MixedHeaders(t *testing.T) {
	tpl := validCarousel()
	tpl.Cards[1].HeaderImageURL = ""
	tpl.Cards[1].HeaderVideoURL = "https://cdn/.../v2.mp4"

	errs := Validate(tpl)
	if !containsCode(errs, "CAROUSEL_MIXED_HEADERS") {
		t.Errorf("expected CAROUSEL_MIXED_HEADERS, got %v", errs)
	}
}

// TestValidate_Carousel_MixedButtonTypes catches the second most common
// submission rejection: card 0 is URL, card 1 is QUICK_REPLY.
func TestValidate_Carousel_MixedButtonTypes(t *testing.T) {
	tpl := validCarousel()
	tpl.Cards[1].Buttons = []Button{{Type: ButtonQuickReply, Text: "Yes", Payload: "YES"}}

	errs := Validate(tpl)
	if !containsCode(errs, "CAROUSEL_MIXED_BUTTONS") {
		t.Errorf("expected CAROUSEL_MIXED_BUTTONS, got %v", errs)
	}
}

// TestValidate_Carousel_CountBoundaries covers the 2..10 card limit.
func TestValidate_Carousel_CountBoundaries(t *testing.T) {
	tpl := validCarousel()
	tpl.Cards = tpl.Cards[:1] // 1 card

	errs := Validate(tpl)
	if !containsCode(errs, "CAROUSEL_CARD_COUNT") {
		t.Errorf("expected CAROUSEL_CARD_COUNT for 1 card, got %v", errs)
	}

	tpl = validCarousel()
	for len(tpl.Cards) < 11 {
		tpl.Cards = append(tpl.Cards, tpl.Cards[0])
	}
	errs = Validate(tpl)
	if !containsCode(errs, "CAROUSEL_CARD_COUNT") {
		t.Errorf("expected CAROUSEL_CARD_COUNT for 11 cards, got %v", errs)
	}
}

// TestValidate_Variables_Contiguous ensures we reject "Hello {{2}}" without {{1}}.
func TestValidate_Variables_Contiguous(t *testing.T) {
	tpl := validCarousel()
	tpl.Body = "Hello {{2}}" // gap: missing {{1}}

	errs := Validate(tpl)
	if !containsCode(errs, "VARIABLE_GAP") {
		t.Errorf("expected VARIABLE_GAP, got %v", errs)
	}
}

// TestValidate_URLButton_ExampleRequired enforces Meta's submission rule
// that URL buttons with variables must supply an example value.
func TestValidate_URLButton_ExampleRequired(t *testing.T) {
	tpl := &RichTemplate{
		Name: "promo", AppID: "a", Language: "en_US", Category: "MARKETING",
		Kind: KindCTAURL, Body: "See more",
		Buttons: []Button{{Type: ButtonURL, Text: "Visit", URL: "https://shop/p/{{1}}"}}, // no Example
	}
	errs := Validate(tpl)
	if !containsCode(errs, "URL_BUTTON_EXAMPLE_MISSING") {
		t.Errorf("expected URL_BUTTON_EXAMPLE_MISSING, got %v", errs)
	}
}

// containsCode is a tiny helper to assert that ValidationErrors carry at
// least one error with the given Code, keeping the table tests terse.
func containsCode(es ValidationErrors, code string) bool {
	for _, e := range es {
		if e.Code == code {
			return true
		}
	}
	return false
}
