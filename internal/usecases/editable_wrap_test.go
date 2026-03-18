package usecases

import (
	"testing"

	templateDomain "github.com/the-monkeys/freerangenotify/internal/domain/template"
)

func TestWrapEditableVariables_TextContent(t *testing.T) {
	body := `<h1>{{.headline}}</h1><p>{{.body_text}}</p>`
	result := wrapEditableVariables(body)

	expected := `<h1><span contenteditable="true" data-frn-var="headline" class="frn-editable">{{.headline}}</span></h1>` +
		`<p><span contenteditable="true" data-frn-var="body_text" class="frn-editable">{{.body_text}}</span></p>`

	if result != expected {
		t.Errorf("text content wrapping failed.\nGot:      %s\nExpected: %s", result, expected)
	}
}

func TestWrapEditableVariables_AttributesUntouched(t *testing.T) {
	body := `<a href="{{.url}}">Click</a><img src="{{.image}}" />`
	result := wrapEditableVariables(body)

	// Variables inside attributes should NOT be wrapped
	if result != body {
		t.Errorf("attribute variables should not be wrapped.\nGot:      %s\nExpected: %s", result, body)
	}
}

func TestWrapEditableVariables_MixedContent(t *testing.T) {
	body := `<a href="{{.url}}">{{.link_text}}</a>`
	result := wrapEditableVariables(body)

	expected := `<a href="{{.url}}"><span contenteditable="true" data-frn-var="link_text" class="frn-editable">{{.link_text}}</span></a>`
	if result != expected {
		t.Errorf("mixed content wrapping failed.\nGot:      %s\nExpected: %s", result, expected)
	}
}

func TestWrapEditableVariables_NoVariables(t *testing.T) {
	body := `<p>Hello World</p>`
	result := wrapEditableVariables(body)
	if result != body {
		t.Errorf("no-variable body should remain unchanged.\nGot: %s", result)
	}
}

func TestWrapEditableVariables_Keywords(t *testing.T) {
	body := `{{if .show}}<p>{{.name}}</p>{{end}}`
	result := wrapEditableVariables(body)

	// "if" and "end" are keywords, should not be wrapped
	// ".show" and ".name" are variables, but "show" is inside an {{if ...}} that isn't in an HTML tag
	// Actually, {{if .show}} has two tokens so our simple \w+ regex won't match it
	// Let's just check that "if" and "end" are NOT wrapped
	if result == body {
		// "name" is a variable and should be wrapped, so result should differ
		// But wait — {{if .show}} won't match our regex because the regex expects {{.var}}
		// and "if .show" has a space after "if". So only {{.name}} and potentially
		// {{end}} need consideration. "end" is a keyword.
	}

	// The regex \{\{\s*\.?(\w+)\s*\}\} should match:
	// {{.name}} -> captures "name"
	// {{end}} -> captures "end" (keyword, skip)
	// {{if .show}} -> won't match (two tokens)
	expected := `{{if .show}}<p><span contenteditable="true" data-frn-var="name" class="frn-editable">{{.name}}</span></p>{{end}}`
	if result != expected {
		t.Errorf("keyword handling failed.\nGot:      %s\nExpected: %s", result, expected)
	}
}

func TestIsInsideHTMLTag(t *testing.T) {
	tests := []struct {
		body string
		pos  int
		want bool
	}{
		{`<a href="X">text</a>`, 9, true},   // inside <a href="X">
		{`<a href="X">text</a>`, 13, false}, // "text" is outside tag
		{`<p>{{.name}}</p>`, 3, false},      // {{.name}} is text content
		{`<img src="{{.url}}"/>`, 10, true}, // inside <img src="...">
	}
	for _, tt := range tests {
		got := isInsideHTMLTag(tt.body, tt.pos)
		if got != tt.want {
			t.Errorf("isInsideHTMLTag(%q, %d) = %v, want %v", tt.body, tt.pos, got, tt.want)
		}
	}
}

func TestClassifyAttributeVariables(t *testing.T) {
	body := `<img src="{{.logo_url}}" alt="Logo"><a href="{{.cta_link}}">Click {{.cta_text}}</a><p style="color:{{.text_color}}">Hello {{.username}}</p>`
	got := classifyAttributeVariables(body)

	// Expected: logo_url (image), cta_link (url), text_color (attribute)
	// NOT expected: cta_text (text content), username (text content)
	want := []templateDomain.AttributeVar{
		{Name: "logo_url", Type: "image"},
		{Name: "cta_link", Type: "url"},
		{Name: "text_color", Type: "attribute"},
	}

	if len(got) != len(want) {
		t.Fatalf("classifyAttributeVariables: got %d vars, want %d\ngot: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i].Name != w.Name || got[i].Type != w.Type {
			t.Errorf("var[%d]: got {%s, %s}, want {%s, %s}", i, got[i].Name, got[i].Type, w.Name, w.Type)
		}
	}
}

func TestClassifyAttributeVariables_NoAttributes(t *testing.T) {
	body := `<h1>{{.headline}}</h1><p>{{.body}}</p>`
	got := classifyAttributeVariables(body)
	if len(got) != 0 {
		t.Errorf("expected no attribute vars, got %+v", got)
	}
}

func TestClassifyAttributeVariables_BackgroundImage(t *testing.T) {
	body := `<div style="background-image: url({{.bg_image}})"><p>{{.text}}</p></div>`
	got := classifyAttributeVariables(body)
	if len(got) != 1 || got[0].Name != "bg_image" || got[0].Type != "image" {
		t.Errorf("expected [{bg_image image}], got %+v", got)
	}
}
