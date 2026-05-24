package whatsapp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ClickPayload is the JSON struct embedded in a signed click-attribution
// URL. Field names are deliberately one-letter to keep the encoded URL
// short — these links are sent over WhatsApp where every character costs
// the user's attention.
type ClickPayload struct {
	NotificationID string `json:"n"`
	AppID          string `json:"a"`
	ButtonIndex    int    `json:"b"`
	TargetURL      string `json:"t"`
	Expiry         int64  `json:"x"` // unix seconds
}

// ClickSigner builds and verifies signed redirect URLs of the form
//
//	{public_url}/v1/r/{base64(payload)}.{hex(hmac(payload))}
//
// The HMAC binds the payload to the system signing key (rotated by
// operators), so an attacker cannot fabricate new redirect targets or
// extend expiry. The expiry is enforced at verify time.
type ClickSigner struct {
	key []byte
}

// NewClickSigner returns a signer using the provided key. A zero-length
// key is a hard error to avoid the silent "everything signs to the same
// HMAC" footgun.
func NewClickSigner(key string) (*ClickSigner, error) {
	if key == "" {
		return nil, errors.New("click signer requires a non-empty signing key")
	}
	return &ClickSigner{key: []byte(key)}, nil
}

// Sign serialises and signs the payload, returning the URL-safe path
// segment (no leading slash). Callers prepend `{public_url}/v1/r/`.
func (s *ClickSigner) Sign(p ClickPayload) (string, error) {
	if p.Expiry == 0 {
		// Default to 90 days. WhatsApp messages can stay on a phone
		// effectively forever, but Meta enforces 24h CSW for replies and
		// most attribution signal value decays inside a week. 90 days
		// strikes a balance.
		p.Expiry = time.Now().Add(90 * 24 * time.Hour).Unix()
	}
	body, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(body)
	mac := s.hmac(encoded)
	return encoded + "." + mac, nil
}

// Verify decodes the signed segment, asserts the HMAC and expiry, and
// returns the payload. Errors are returned as distinct sentinels so
// handlers can map them to different HTTP responses.
func (s *ClickSigner) Verify(signed string) (*ClickPayload, error) {
	dot := -1
	for i := 0; i < len(signed); i++ {
		if signed[i] == '.' {
			dot = i
			break
		}
	}
	if dot <= 0 || dot >= len(signed)-1 {
		return nil, ErrClickMalformed
	}
	encoded, mac := signed[:dot], signed[dot+1:]

	expected := s.hmac(encoded)
	if !hmac.Equal([]byte(expected), []byte(mac)) {
		return nil, ErrClickSignature
	}

	body, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, ErrClickMalformed
	}
	var p ClickPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, ErrClickMalformed
	}
	if p.Expiry > 0 && time.Now().Unix() > p.Expiry {
		return nil, ErrClickExpired
	}
	if p.TargetURL == "" {
		return nil, ErrClickMalformed
	}
	return &p, nil
}

// BuildRedirectURL is a convenience that joins the public base URL with
// the signed path segment. publicURL is expected to be set in
// cfg.Server.PublicURL (without a trailing slash).
func (s *ClickSigner) BuildRedirectURL(publicURL string, p ClickPayload) (string, error) {
	if publicURL == "" {
		return "", fmt.Errorf("publicURL is required to build click attribution links")
	}
	sig, err := s.Sign(p)
	if err != nil {
		return "", err
	}
	return publicURL + "/v1/r/" + sig, nil
}

// hmac computes the lowercase-hex HMAC-SHA256 over the encoded payload.
// Kept private so callers cannot mistakenly hash arbitrary inputs and
// accidentally generate a valid signature.
func (s *ClickSigner) hmac(encoded string) string {
	h := hmac.New(sha256.New, s.key)
	_, _ = h.Write([]byte(encoded))
	return hex.EncodeToString(h.Sum(nil))
}

// Sentinel errors returned by Verify so handlers can map them to HTTP
// status codes (400 vs 410 vs 403) and metrics labels.
var (
	ErrClickMalformed = errors.New("click: malformed signed payload")
	ErrClickSignature = errors.New("click: signature does not match")
	ErrClickExpired   = errors.New("click: signed payload expired")
)
