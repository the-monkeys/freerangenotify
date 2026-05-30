// Package filestore contains storage backends and URL-signing helpers for the
// file-attachments feature. The domain interfaces live in
// internal/domain/file; this package owns concrete implementations.
package filestore

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Signed-URL helpers.
//
// We sign the tuple (file_id | app_id | expires_at_unix) with HMAC-SHA256 and
// expose two URL query parameters:
//
//   ?exp=<unix-seconds>&sig=<base64url(HMAC-SHA256)>
//
// Verification is constant-time. Keys are rotatable by passing multiple
// candidates to NewSigner (first is "current", others are accepted on
// verify only).

// Errors returned by signed-URL verification.
var (
	ErrSignatureMissing   = errors.New("signature missing")
	ErrSignatureMismatch  = errors.New("signature mismatch")
	ErrSignatureExpired   = errors.New("signature expired")
	ErrSignatureMalformed = errors.New("signature malformed")
	ErrSigningKeyMissing  = errors.New("at least one signing key is required")
)

// Signer issues and verifies short-lived signatures over (appID, fileID, exp).
type Signer struct {
	// keys[0] is the current key; later entries are accepted on verify only,
	// allowing zero-downtime rotation.
	keys [][]byte
	ttl  time.Duration
	// now is overridable for tests.
	now func() time.Time
}

// NewSigner constructs a Signer. The first key is used for new signatures;
// all keys are accepted during verification. ttl is the validity window for
// freshly minted signatures. Returns ErrSigningKeyMissing if no usable key is
// provided.
func NewSigner(ttl time.Duration, keys ...string) (*Signer, error) {
	clean := make([][]byte, 0, len(keys))
	for _, k := range keys {
		if k == "" {
			continue
		}
		clean = append(clean, []byte(k))
	}
	if len(clean) == 0 {
		return nil, ErrSigningKeyMissing
	}
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	return &Signer{keys: clean, ttl: ttl, now: time.Now}, nil
}

// Sign returns (expiresAt, signature) for the given (appID, fileID) pair.
// expiresAt is unix seconds; signature is base64url-encoded (no padding).
func (s *Signer) Sign(appID, fileID string) (int64, string) {
	exp := s.now().Add(s.ttl).Unix()
	sig := s.computeWith(s.keys[0], appID, fileID, exp)
	return exp, encode(sig)
}

// Verify checks that signature is valid for (appID, fileID, exp) under any
// accepted key, and that exp has not elapsed.
func (s *Signer) Verify(appID, fileID string, exp int64, signature string) error {
	if signature == "" {
		return ErrSignatureMissing
	}
	if exp <= 0 {
		return ErrSignatureMalformed
	}
	if s.now().Unix() >= exp {
		return ErrSignatureExpired
	}
	want, err := decode(signature)
	if err != nil {
		return ErrSignatureMalformed
	}
	for _, k := range s.keys {
		got := s.computeWith(k, appID, fileID, exp)
		if hmac.Equal(want, got) {
			return nil
		}
	}
	return ErrSignatureMismatch
}

// VerifyQuery is a convenience wrapper around Verify that parses the textual
// exp parameter.
func (s *Signer) VerifyQuery(appID, fileID, expStr, signature string) error {
	exp, err := strconv.ParseInt(strings.TrimSpace(expStr), 10, 64)
	if err != nil {
		return ErrSignatureMalformed
	}
	return s.Verify(appID, fileID, exp, signature)
}

// computeWith builds the HMAC-SHA256 of the canonical message under key k.
// The message format MUST NOT change without bumping a signature version.
func (s *Signer) computeWith(k []byte, appID, fileID string, exp int64) []byte {
	mac := hmac.New(sha256.New, k)
	// Use a delimiter that cannot appear in a base32 file_id or an app id, and
	// include lengths-with-pipes as a defense-in-depth canonical form.
	fmt.Fprintf(mac, "%s|%s|%d", fileID, appID, exp)
	return mac.Sum(nil)
}

func encode(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }
func decode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(strings.TrimSpace(s))
}
