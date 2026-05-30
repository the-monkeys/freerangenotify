package notification

import (
	"encoding/json"
	"testing"
)

// TestContent_JSON_BackCompat_LegacyShape asserts that a Content value with
// only the legacy fields marshals to exactly the pre-Phase-7 byte sequence.
// Any change to this golden output is a breaking change for existing
// webhook receivers and must be reviewed deliberately.
func TestContent_JSON_BackCompat_LegacyShape(t *testing.T) {
	cases := []struct {
		name    string
		content Content
		want    string
	}{
		{
			name:    "title and body only",
			content: Content{Title: "Hello", Body: "World"},
			want:    `{"title":"Hello","body":"World"}`,
		},
		{
			name:    "with media url",
			content: Content{Title: "Hi", Body: "There", MediaURL: "https://example.com/x.png"},
			want:    `{"title":"Hi","body":"There","media_url":"https://example.com/x.png"}`,
		},
		{
			name: "with data map",
			content: Content{
				Title: "T", Body: "B",
				Data: map[string]interface{}{"action_url": "https://a.test"},
			},
			want: `{"title":"T","body":"B","data":{"action_url":"https://a.test"}}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(tc.content)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(got) != tc.want {
				t.Fatalf("back-compat JSON drift\n got: %s\nwant: %s", got, tc.want)
			}
		})
	}
}

func TestContent_JSON_Rich_OmitsUnsetFields(t *testing.T) {
	c := Content{
		Title: "T", Body: "B",
		Attachments: []Attachment{{Type: "image", URL: "https://x/y.png"}},
	}
	got, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"title":"T","body":"B","attachments":[{"type":"image","url":"https://x/y.png"}]}`
	if string(got) != want {
		t.Fatalf("rich JSON drift\n got: %s\nwant: %s", got, want)
	}
}

func TestContent_Validate_LegacyPasses(t *testing.T) {
	c := Content{Title: "T", Body: "B"}
	if err := c.Validate(); err != nil {
		t.Fatalf("legacy content should validate; got %v", err)
	}
}

func TestContent_Validate_AttachmentLimits(t *testing.T) {
	c := Content{}
	for i := 0; i < 11; i++ {
		c.Attachments = append(c.Attachments, Attachment{Type: "image", URL: "https://x/y"})
	}
	if err := c.Validate(); err != ErrTooManyAttachments {
		t.Fatalf("want ErrTooManyAttachments, got %v", err)
	}

	// Missing Type still fails with the generic invalid sentinel.
	c = Content{Attachments: []Attachment{{URL: "https://x/y"}}}
	if err := c.Validate(); err != ErrInvalidAttachment {
		t.Fatalf("want ErrInvalidAttachment, got %v", err)
	}

	// No source at all → ErrAttachmentMissingSource.
	c = Content{Attachments: []Attachment{{Type: "image"}}}
	if err := c.Validate(); err != ErrAttachmentMissingSource {
		t.Fatalf("want ErrAttachmentMissingSource, got %v", err)
	}
}

func TestContent_Validate_Attachment_FileIDOnly(t *testing.T) {
	c := Content{Attachments: []Attachment{{Type: "file", FileID: "file_01ABCD"}}}
	if err := c.Validate(); err != nil {
		t.Fatalf("file_id-only attachment should validate; got %v", err)
	}
}

func TestContent_Validate_Attachment_FileIDFormat(t *testing.T) {
	// Each entry must produce ErrInvalidFileID. The motivating bug was a
	// caller pasting a URL into the file_id slot; the resolver then asked
	// ES for `/files/_doc/<url>` and the notification dead-lettered after
	// pointless retries. Validation must reject the obvious misuse
	// patterns before the doc ever enters the queue.
	bad := []string{
		"https://example.com/photo.jpg", // full URL
		"http://example.com/x.png",      // http URL
		"/etc/passwd",                   // absolute path
		"file_with/slash",               // path separator
		"file_with:colon",               // scheme separator
		"file_with?q=1",                 // query
		"file_with#frag",                // fragment
		"file_with space",               // whitespace
		"FILE_uppercase",                // wrong prefix case
		"random_id",                     // missing prefix
		"file_",                         // empty tail
	}
	for _, id := range bad {
		c := Content{Attachments: []Attachment{{Type: "file", FileID: id}}}
		if err := c.Validate(); err != ErrInvalidFileID {
			t.Errorf("file_id %q: want ErrInvalidFileID, got %v", id, err)
		}
	}

	good := []string{
		"file_abc",
		"file_1e7570ccb062489bb18c8ab432108aa5",
		"file_test_1", // test-suite shape
		"file_with-dash_and.underscore",
	}
	for _, id := range good {
		c := Content{Attachments: []Attachment{{Type: "file", FileID: id}}}
		if err := c.Validate(); err != nil {
			t.Errorf("file_id %q should validate; got %v", id, err)
		}
	}
}

func TestContent_Validate_Attachment_InlineBase64Only(t *testing.T) {
	c := Content{Attachments: []Attachment{{Type: "file", ContentBase64: "JVBERi0xLjQK"}}}
	if err := c.Validate(); err != nil {
		t.Fatalf("base64-only attachment should validate; got %v", err)
	}
}

func TestContent_Validate_Attachment_AmbiguousSource(t *testing.T) {
	cases := []Attachment{
		{Type: "file", URL: "https://x/y", FileID: "file_1"},
		{Type: "file", URL: "https://x/y", ContentBase64: "AAA"},
		{Type: "file", FileID: "file_1", ContentBase64: "AAA"},
		{Type: "file", URL: "https://x/y", FileID: "file_1", ContentBase64: "AAA"},
	}
	for i, a := range cases {
		c := Content{Attachments: []Attachment{a}}
		if err := c.Validate(); err != ErrAmbiguousAttachmentSource {
			t.Fatalf("case %d: want ErrAmbiguousAttachmentSource, got %v", i, err)
		}
	}
}

func TestContent_Validate_Attachment_InlineTooLarge(t *testing.T) {
	// 14 MB + 1 of base64 characters
	big := make([]byte, 14*1024*1024+1)
	for i := range big {
		big[i] = 'A'
	}
	c := Content{Attachments: []Attachment{{Type: "file", ContentBase64: string(big)}}}
	if err := c.Validate(); err != ErrAttachmentTooLarge {
		t.Fatalf("want ErrAttachmentTooLarge, got %v", err)
	}
}

func TestContent_Validate_Attachment_InvalidDisposition(t *testing.T) {
	c := Content{Attachments: []Attachment{{Type: "file", URL: "https://x/y", Disposition: "weird"}}}
	if err := c.Validate(); err != ErrInvalidAttachment {
		t.Fatalf("want ErrInvalidAttachment, got %v", err)
	}
}

func TestContent_Validate_ActionLimits(t *testing.T) {
	c := Content{}
	for i := 0; i < 6; i++ {
		c.Actions = append(c.Actions, Action{Type: "link", Label: "x", URL: "https://x"})
	}
	if err := c.Validate(); err != ErrTooManyActions {
		t.Fatalf("want ErrTooManyActions, got %v", err)
	}

	c = Content{Actions: []Action{{Type: "link", Label: "go"}}} // link without URL
	if err := c.Validate(); err != ErrInvalidAction {
		t.Fatalf("want ErrInvalidAction, got %v", err)
	}
}

func TestContent_Validate_FieldLimits(t *testing.T) {
	c := Content{}
	for i := 0; i < 26; i++ {
		c.Fields = append(c.Fields, Field{Key: "k", Value: "v"})
	}
	if err := c.Validate(); err != ErrTooManyFields {
		t.Fatalf("want ErrTooManyFields, got %v", err)
	}

	c = Content{Fields: []Field{{Key: "k"}}} // missing value
	if err := c.Validate(); err != ErrInvalidField {
		t.Fatalf("want ErrInvalidField, got %v", err)
	}
}

func TestContent_Validate_PollLimits(t *testing.T) {
	c := Content{Poll: &Poll{Question: "?", Choices: []PollChoice{{Label: "a"}}}}
	if err := c.Validate(); err != ErrInvalidPoll {
		t.Fatalf("want ErrInvalidPoll for single choice, got %v", err)
	}

	c = Content{Poll: &Poll{Question: "", Choices: []PollChoice{{Label: "a"}, {Label: "b"}}}}
	if err := c.Validate(); err != ErrInvalidPoll {
		t.Fatalf("want ErrInvalidPoll for missing question, got %v", err)
	}

	// Over max
	tooMany := &Poll{Question: "?"}
	for i := 0; i < 11; i++ {
		tooMany.Choices = append(tooMany.Choices, PollChoice{Label: "x"})
	}
	c = Content{Poll: tooMany}
	if err := c.Validate(); err != ErrInvalidPoll {
		t.Fatalf("want ErrInvalidPoll for 11 choices, got %v", err)
	}

	// Valid
	c = Content{Poll: &Poll{Question: "Pick?", Choices: []PollChoice{{Label: "a"}, {Label: "b"}}}}
	if err := c.Validate(); err != nil {
		t.Fatalf("valid poll should pass, got %v", err)
	}
}
