package filestore

import (
	"testing"
	"time"
)

func newTestSigner(t *testing.T, nowSec int64) *Signer {
	t.Helper()
	s, err := NewSigner(15*time.Minute, "primary-key")
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	s.now = func() time.Time { return time.Unix(nowSec, 0) }
	return s
}

func TestSigner_New_RequiresKey(t *testing.T) {
	if _, err := NewSigner(time.Minute); err != ErrSigningKeyMissing {
		t.Fatalf("want ErrSigningKeyMissing, got %v", err)
	}
	if _, err := NewSigner(time.Minute, "", ""); err != ErrSigningKeyMissing {
		t.Fatalf("empty keys should be filtered; want ErrSigningKeyMissing, got %v", err)
	}
}

func TestSigner_New_DefaultTTL(t *testing.T) {
	s, err := NewSigner(0, "k")
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	if s.ttl != 15*time.Minute {
		t.Errorf("default TTL = %v, want 15m", s.ttl)
	}
}

func TestSigner_SignAndVerify_Roundtrip(t *testing.T) {
	s := newTestSigner(t, 1_700_000_000)
	exp, sig := s.Sign("app_a", "file_1")
	if exp != 1_700_000_000+900 {
		t.Errorf("exp = %d, want %d", exp, 1_700_000_000+900)
	}
	if err := s.Verify("app_a", "file_1", exp, sig); err != nil {
		t.Errorf("verify same tuple should pass; got %v", err)
	}
}

func TestSigner_Verify_RejectsWrongTuple(t *testing.T) {
	s := newTestSigner(t, 1_700_000_000)
	exp, sig := s.Sign("app_a", "file_1")

	cases := []struct {
		name             string
		appID, fileID    string
		want             error
	}{
		{"wrong app", "app_b", "file_1", ErrSignatureMismatch},
		{"wrong file", "app_a", "file_2", ErrSignatureMismatch},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := s.Verify(tc.appID, tc.fileID, exp, sig); err != tc.want {
				t.Errorf("want %v, got %v", tc.want, err)
			}
		})
	}
}

func TestSigner_Verify_Expired(t *testing.T) {
	s := newTestSigner(t, 1_700_000_000)
	exp, sig := s.Sign("app_a", "file_1")
	// Move clock past exp.
	s.now = func() time.Time { return time.Unix(exp+1, 0) }
	if err := s.Verify("app_a", "file_1", exp, sig); err != ErrSignatureExpired {
		t.Errorf("want ErrSignatureExpired, got %v", err)
	}
}

func TestSigner_Verify_Malformed(t *testing.T) {
	s := newTestSigner(t, 1_700_000_000)
	exp, _ := s.Sign("app_a", "file_1")
	if err := s.Verify("app_a", "file_1", exp, ""); err != ErrSignatureMissing {
		t.Errorf("want ErrSignatureMissing, got %v", err)
	}
	if err := s.Verify("app_a", "file_1", exp, "%%not-base64%%"); err != ErrSignatureMalformed {
		t.Errorf("want ErrSignatureMalformed, got %v", err)
	}
	if err := s.Verify("app_a", "file_1", 0, "abc"); err != ErrSignatureMalformed {
		t.Errorf("want ErrSignatureMalformed for exp=0, got %v", err)
	}
}

func TestSigner_KeyRotation(t *testing.T) {
	// Old key signed it; current key is different; both must accept.
	oldS, _ := NewSigner(15*time.Minute, "old-key")
	oldS.now = func() time.Time { return time.Unix(1_700_000_000, 0) }
	exp, sig := oldS.Sign("app_a", "file_1")

	rotated, _ := NewSigner(15*time.Minute, "new-key", "old-key")
	rotated.now = func() time.Time { return time.Unix(1_700_000_100, 0) }

	if err := rotated.Verify("app_a", "file_1", exp, sig); err != nil {
		t.Errorf("rotated signer must still accept old-key signature; got %v", err)
	}

	// New signatures use the new key only.
	exp2, sig2 := rotated.Sign("app_a", "file_2")
	if err := oldS.Verify("app_a", "file_2", exp2, sig2); err != ErrSignatureMismatch {
		t.Errorf("old signer must reject signatures from the new key; got %v", err)
	}
}

func TestSigner_VerifyQuery(t *testing.T) {
	s := newTestSigner(t, 1_700_000_000)
	exp, sig := s.Sign("app_a", "file_1")
	if err := s.VerifyQuery("app_a", "file_1", "  1700000900  ", sig); err != nil {
		t.Errorf("VerifyQuery should trim and parse; got %v", err)
	}
	_ = exp
	if err := s.VerifyQuery("app_a", "file_1", "not-a-number", sig); err != ErrSignatureMalformed {
		t.Errorf("want ErrSignatureMalformed, got %v", err)
	}
}

// Sanity check: signatures of two distinct tuples must differ.
func TestSigner_SignaturesAreTupleSpecific(t *testing.T) {
	s := newTestSigner(t, 1_700_000_000)
	_, a := s.Sign("app", "file_1")
	_, b := s.Sign("app", "file_2")
	if a == b {
		t.Error("signatures for different file ids must differ")
	}
}
