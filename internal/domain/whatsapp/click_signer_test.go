package whatsapp

import (
	"strings"
	"testing"
	"time"
)

// TestClickSigner_RoundTrip is the basic happy-path: signing and verifying
// a payload returns the same fields back.
func TestClickSigner_RoundTrip(t *testing.T) {
	s, err := NewClickSigner("test-key-A1")
	if err != nil {
		t.Fatalf("NewClickSigner: %v", err)
	}
	in := ClickPayload{
		NotificationID: "n-123",
		AppID:          "app-X",
		ButtonIndex:    2,
		TargetURL:      "https://shop.example.com/p/42",
	}
	signed, err := s.Sign(in)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if !strings.Contains(signed, ".") {
		t.Errorf("signed string should contain a dot separator: %q", signed)
	}
	out, err := s.Verify(signed)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if out.NotificationID != in.NotificationID || out.AppID != in.AppID || out.ButtonIndex != in.ButtonIndex || out.TargetURL != in.TargetURL {
		t.Errorf("round-trip mismatch: got %+v, want %+v", out, in)
	}
}

// TestClickSigner_TamperedSigRejected proves the HMAC actually protects
// the payload — flipping one byte of the signature makes Verify fail.
func TestClickSigner_TamperedSigRejected(t *testing.T) {
	s, _ := NewClickSigner("test-key-A1")
	signed, _ := s.Sign(ClickPayload{TargetURL: "https://x", NotificationID: "n"})

	tampered := signed[:len(signed)-1] + "0"
	if tampered == signed {
		tampered = signed[:len(signed)-1] + "1"
	}
	if _, err := s.Verify(tampered); err != ErrClickSignature {
		t.Errorf("expected ErrClickSignature, got %v", err)
	}
}

// TestClickSigner_TamperedPayloadRejected proves you can't swap the
// payload while keeping the original signature.
func TestClickSigner_TamperedPayloadRejected(t *testing.T) {
	s, _ := NewClickSigner("test-key-A1")
	signed, _ := s.Sign(ClickPayload{TargetURL: "https://x", NotificationID: "n"})

	// Replace the payload portion with a different base64 string but keep
	// the original signature — should fail HMAC check.
	dot := strings.Index(signed, ".")
	tampered := "ZmFrZQ" + signed[dot:]
	if _, err := s.Verify(tampered); err != ErrClickSignature {
		t.Errorf("expected ErrClickSignature, got %v", err)
	}
}

// TestClickSigner_ExpiredRejected proves the expiry check fires when
// the signed payload's expiry has passed.
func TestClickSigner_ExpiredRejected(t *testing.T) {
	s, _ := NewClickSigner("test-key-A1")
	signed, _ := s.Sign(ClickPayload{
		TargetURL:      "https://x",
		NotificationID: "n",
		Expiry:         time.Now().Add(-1 * time.Hour).Unix(),
	})
	if _, err := s.Verify(signed); err != ErrClickExpired {
		t.Errorf("expected ErrClickExpired, got %v", err)
	}
}

// TestClickSigner_EmptyKeyRejected blocks the silent footgun of an empty
// signing key (which would otherwise sign every payload identically).
func TestClickSigner_EmptyKeyRejected(t *testing.T) {
	if _, err := NewClickSigner(""); err == nil {
		t.Errorf("expected error for empty key")
	}
}

// TestClickSigner_DifferentKeysIndependent proves two signers with
// different keys cannot validate each other's signatures.
func TestClickSigner_DifferentKeysIndependent(t *testing.T) {
	a, _ := NewClickSigner("key-A")
	b, _ := NewClickSigner("key-B")
	signed, _ := a.Sign(ClickPayload{TargetURL: "https://x", NotificationID: "n"})
	if _, err := b.Verify(signed); err != ErrClickSignature {
		t.Errorf("expected ErrClickSignature, got %v", err)
	}
}
