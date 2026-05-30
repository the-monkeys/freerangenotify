package file

import (
	"testing"
	"time"
)

func validFile() FileObject {
	return FileObject{
		FileID:    "file_01ABCD",
		AppID:     "app_test",
		Name:      "invoice.pdf",
		Size:      1024,
		MIMEType:  "application/pdf",
		SHA256:    "deadbeef",
		CreatedAt: time.Now(),
	}
}

func TestFileObject_Validate_OK(t *testing.T) {
	f := validFile()
	if err := f.Validate(); err != nil {
		t.Fatalf("valid file should pass; got %v", err)
	}
}

func TestFileObject_Validate_RejectsEmptyFields(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*FileObject)
		want error
	}{
		{"empty file id", func(f *FileObject) { f.FileID = "" }, ErrInvalidFileID},
		{"whitespace file id", func(f *FileObject) { f.FileID = "   " }, ErrInvalidFileID},
		{"empty app id", func(f *FileObject) { f.AppID = "" }, ErrInvalidAppID},
		{"empty name", func(f *FileObject) { f.Name = "" }, ErrInvalidFileName},
		{"empty mime", func(f *FileObject) { f.MIMEType = "" }, ErrInvalidMIMEType},
		{"zero size", func(f *FileObject) { f.Size = 0 }, ErrInvalidSize},
		{"negative size", func(f *FileObject) { f.Size = -1 }, ErrInvalidSize},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := validFile()
			tc.mut(&f)
			if err := f.Validate(); err != tc.want {
				t.Fatalf("want %v, got %v", tc.want, err)
			}
		})
	}
}

func TestFileObject_IsExpired(t *testing.T) {
	now := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	f := validFile()

	// No expiry set → never expired.
	if f.IsExpired(now) {
		t.Error("file without ExpiresAt should not be expired")
	}

	// In the future.
	f.ExpiresAt = now.Add(time.Hour)
	if f.IsExpired(now) {
		t.Error("file with future ExpiresAt should not be expired")
	}

	// In the past.
	f.ExpiresAt = now.Add(-time.Hour)
	if !f.IsExpired(now) {
		t.Error("file with past ExpiresAt should be expired")
	}
}
