package providers

import (
	"testing"

	"github.com/the-monkeys/freerangenotify/internal/domain/attachment"
	"github.com/the-monkeys/freerangenotify/internal/domain/notification"
)

func TestAssignUniqueFilenames_NoDuplicates(t *testing.T) {
	got := assignUniqueFilenames([]*attachment.Resolved{
		{Filename: "a.png"},
		{Filename: "b.png"},
	})
	want := []string{"a.png", "b.png"}
	if got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestAssignUniqueFilenames_Duplicates(t *testing.T) {
	got := assignUniqueFilenames([]*attachment.Resolved{
		{Filename: "photo.jpg"},
		{Filename: "photo.jpg"},
		{Filename: "photo.jpg"},
	})
	want := []string{"photo.jpg", "photo-2.jpg", "photo-3.jpg"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestAssignUniqueFilenames_EmptyFallsBack(t *testing.T) {
	got := assignUniqueFilenames([]*attachment.Resolved{
		{Filename: ""},
		{Filename: "   "},
	})
	if got[0] != "attachment" || got[1] != "attachment-2" {
		t.Fatalf("got %v", got)
	}
}

func TestAssignUniqueFilenames_NoExtension(t *testing.T) {
	got := assignUniqueFilenames([]*attachment.Resolved{
		{Filename: "image"},
		{Filename: "image"},
	})
	if got[0] != "image" || got[1] != "image-2" {
		t.Fatalf("got %v", got)
	}
}

func TestBuildDiscordAttachmentRefs_MixedSources(t *testing.T) {
	specs := []notification.Attachment{
		{Type: "image", URL: "https://cdn/a.png"},      // 0: URL source
		{Type: "image", FileID: "file_b"},              // 1: file_id
		{Type: "image", ContentBase64: "iVBORw0KGgo="}, // 2: inline
		{Type: "image", URL: "https://cdn/d.png"},      // 3: URL source
	}
	resolved := []*attachment.Resolved{
		{Filename: "b.png"},
		{Filename: "c.png"},
	}
	got := buildDiscordAttachmentRefs(specs, resolved)
	if len(got) != 4 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].UploadedFilename != "" {
		t.Fatalf("idx 0 want empty, got %q", got[0].UploadedFilename)
	}
	if got[1].UploadedFilename != "b.png" {
		t.Fatalf("idx 1 = %q", got[1].UploadedFilename)
	}
	if got[2].UploadedFilename != "c.png" {
		t.Fatalf("idx 2 = %q", got[2].UploadedFilename)
	}
	if got[3].UploadedFilename != "" {
		t.Fatalf("idx 3 want empty, got %q", got[3].UploadedFilename)
	}
}
