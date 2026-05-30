package render

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
)

// These tests lock in the four routing scenarios for Discord attachments:
//
//   1. URL image only          → embed.image.url == url
//   2. file_id (uploaded) only → embed.image.url == attachment://<filename>
//   3. Two uploaded images     → first in embed.image, second in a 2nd embed
//   4. Mixed URL + uploaded    → URL in embed.image, uploaded in 2nd embed
//
// The pre-refactor renderer either dropped all non-URL attachments
// (causing the embed/multipart disagreement that broke 2,3,4) or — with
// the rejected quick fix — silently produced an empty image URL that
// Discord rejected with HTTP 400 `{"embeds":["N"]}`.

func newDiscordNotifWithAttachments(atts []notification.Attachment) *notification.Notification {
	return &notification.Notification{
		Content: notification.Content{
			Title:       "hi",
			Body:        "body",
			Attachments: atts,
		},
	}
}

func extractEmbeds(t *testing.T, payload map[string]interface{}) []map[string]interface{} {
	t.Helper()
	raw, ok := payload["embeds"].([]map[string]interface{})
	if !ok {
		// json round-trip path: when payload is marshalled back the type is []interface{}.
		anyEmbeds, ok2 := payload["embeds"].([]interface{})
		if !ok2 {
			t.Fatalf("embeds missing or wrong type: %T", payload["embeds"])
		}
		out := make([]map[string]interface{}, 0, len(anyEmbeds))
		for _, e := range anyEmbeds {
			out = append(out, e.(map[string]interface{}))
		}
		return out
	}
	return raw
}

func TestDiscord_Attachment_URLImageOnly(t *testing.T) {
	n := newDiscordNotifWithAttachments([]notification.Attachment{
		{Type: "image", URL: "https://cdn/example.png"},
	})
	got := BuildDiscordPayloadWithOptions(n, DiscordRenderOptions{})
	embeds := extractEmbeds(t, got)
	if len(embeds) != 1 {
		t.Fatalf("want 1 embed, got %d", len(embeds))
	}
	img := embeds[0]["image"].(map[string]interface{})
	if img["url"] != "https://cdn/example.png" {
		t.Fatalf("want URL image, got %v", img["url"])
	}
}

func TestDiscord_Attachment_UploadedImageOnly(t *testing.T) {
	n := newDiscordNotifWithAttachments([]notification.Attachment{
		{Type: "image", FileID: "file_abc"}, // URL deliberately empty
	})
	got := BuildDiscordPayloadWithOptions(n, DiscordRenderOptions{
		AttachmentRefs: []DiscordAttachmentRef{{UploadedFilename: "photo.png"}},
	})
	embeds := extractEmbeds(t, got)
	if len(embeds) != 1 {
		t.Fatalf("want 1 embed, got %d", len(embeds))
	}
	img := embeds[0]["image"].(map[string]interface{})
	if img["url"] != "attachment://photo.png" {
		t.Fatalf("want attachment:// ref, got %v", img["url"])
	}
}

func TestDiscord_Attachment_TwoUploadedImages(t *testing.T) {
	n := newDiscordNotifWithAttachments([]notification.Attachment{
		{Type: "image", FileID: "file_a"},
		{Type: "image", FileID: "file_b"},
	})
	got := BuildDiscordPayloadWithOptions(n, DiscordRenderOptions{
		AttachmentRefs: []DiscordAttachmentRef{
			{UploadedFilename: "a.png"},
			{UploadedFilename: "b.png"},
		},
	})
	embeds := extractEmbeds(t, got)
	if len(embeds) != 2 {
		t.Fatalf("want 2 embeds, got %d", len(embeds))
	}
	if u := embeds[0]["image"].(map[string]interface{})["url"]; u != "attachment://a.png" {
		t.Fatalf("first embed image url = %v", u)
	}
	if u := embeds[1]["image"].(map[string]interface{})["url"]; u != "attachment://b.png" {
		t.Fatalf("second embed image url = %v", u)
	}
}

func TestDiscord_Attachment_MixedURLAndUploaded(t *testing.T) {
	n := newDiscordNotifWithAttachments([]notification.Attachment{
		{Type: "image", URL: "https://cdn/one.png"},
		{Type: "image", FileID: "file_two"},
	})
	got := BuildDiscordPayloadWithOptions(n, DiscordRenderOptions{
		AttachmentRefs: []DiscordAttachmentRef{
			{}, // index 0 is URL-source — no ref
			{UploadedFilename: "two.png"},
		},
	})
	embeds := extractEmbeds(t, got)
	if len(embeds) != 2 {
		t.Fatalf("want 2 embeds, got %d", len(embeds))
	}
	if u := embeds[0]["image"].(map[string]interface{})["url"]; u != "https://cdn/one.png" {
		t.Fatalf("first embed url = %v", u)
	}
	if u := embeds[1]["image"].(map[string]interface{})["url"]; u != "attachment://two.png" {
		t.Fatalf("second embed url = %v", u)
	}
}

// Regression guard: a non-image attachment uploaded via multipart must not
// produce an embed field. Discord renders the file inline below the embed.
func TestDiscord_Attachment_UploadedFileOmittedFromEmbedFields(t *testing.T) {
	n := newDiscordNotifWithAttachments([]notification.Attachment{
		{Type: "file", FileID: "file_doc", Name: "report.pdf"},
	})
	got := BuildDiscordPayloadWithOptions(n, DiscordRenderOptions{
		AttachmentRefs: []DiscordAttachmentRef{{UploadedFilename: "report.pdf"}},
	})
	embeds := extractEmbeds(t, got)
	if fields, ok := embeds[0]["fields"]; ok {
		t.Fatalf("expected no fields for uploaded file, got %v", fields)
	}
}

// Regression guard: a URL non-image still surfaces as a Download field.
func TestDiscord_Attachment_URLFileDownloadField(t *testing.T) {
	n := newDiscordNotifWithAttachments([]notification.Attachment{
		{Type: "file", URL: "https://cdn/r.pdf", Name: "r.pdf"},
	})
	raw, _ := json.Marshal(BuildDiscordPayloadWithOptions(n, DiscordRenderOptions{}))
	if !strings.Contains(string(raw), "[Download](https://cdn/r.pdf)") {
		t.Fatalf("expected download field, got %s", raw)
	}
}
