package providers

import "testing"

func TestAttachmentMode_String(t *testing.T) {
	cases := []struct {
		mode AttachmentMode
		want string
	}{
		{AttachmentModeNone, "none"},
		{AttachmentModeInline, "inline"},
		{AttachmentModeMultipart, "multipart"},
		{AttachmentModePreUpload, "pre_upload"},
		{AttachmentModeSignedURL, "signed_url"},
		{AttachmentMode(999), "unknown"},
	}
	for _, tc := range cases {
		if got := tc.mode.String(); got != tc.want {
			t.Errorf("AttachmentMode(%d).String() = %q, want %q", tc.mode, got, tc.want)
		}
	}
}

func TestDefaultCapabilities_IsSafe(t *testing.T) {
	c := DefaultCapabilities()
	if c.AttachmentMode != AttachmentModeNone {
		t.Errorf("default AttachmentMode = %v, want None", c.AttachmentMode)
	}
	if c.MaxAttachmentBytes != 0 || c.MaxAttachmentCount != 0 {
		t.Errorf("default limits should be 0; got bytes=%d count=%d", c.MaxAttachmentBytes, c.MaxAttachmentCount)
	}
	if len(c.AllowedMIMETypes) != 0 {
		t.Errorf("default AllowedMIMETypes should be empty; got %v", c.AllowedMIMETypes)
	}
	if c.SupportsInlineCID {
		t.Errorf("default SupportsInlineCID should be false")
	}
}
