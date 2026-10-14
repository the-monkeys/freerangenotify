package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"strings"
)

type SecretBinding struct{ ConnectionID, AppID, EnvironmentID, Provider, Field string }
type EncryptedSecret = channelconnection.EncryptedSecret
type ConnectionCodec struct {
	active string
	keys   map[string][]byte
}

// Keyring is an ordered comma-separated list of id:base64-encoded-32-byte-key.
// The first key encrypts; retained keys only decrypt older revisions.
func NewConnectionCodec(keyring string) (*ConnectionCodec, error) {
	c := &ConnectionCodec{keys: map[string][]byte{}}
	if strings.TrimSpace(keyring) == "" {
		return c, nil
	}
	for _, entry := range strings.Split(keyring, ",") {
		parts := strings.SplitN(strings.TrimSpace(entry), ":", 2)
		if len(parts) != 2 || parts[0] == "" {
			return nil, fmt.Errorf("invalid connection encryption keyring")
		}
		key, err := base64.StdEncoding.DecodeString(parts[1])
		if err != nil || len(key) != 32 {
			return nil, fmt.Errorf("connection encryption keys must contain 32 bytes")
		}
		if _, ok := c.keys[parts[0]]; ok {
			return nil, fmt.Errorf("duplicate connection encryption key ID")
		}
		c.keys[parts[0]] = key
		if c.active == "" {
			c.active = parts[0]
		}
	}
	return c, nil
}
func (c *ConnectionCodec) aead(id string) (cipher.AEAD, error) {
	if c == nil || len(c.keys[id]) != 32 {
		return nil, fmt.Errorf("connection encryption key unavailable")
	}
	block, err := aes.NewCipher(c.keys[id])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
func (c *ConnectionCodec) Seal(plaintext []byte, binding SecretBinding) (EncryptedSecret, error) {
	aead, err := c.aead(c.active)
	if err != nil {
		return EncryptedSecret{}, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return EncryptedSecret{}, fmt.Errorf("connection encryption randomness unavailable")
	}
	aad, _ := json.Marshal(binding)
	return EncryptedSecret{Version: 1, KeyID: c.active, Nonce: base64.StdEncoding.EncodeToString(nonce), Ciphertext: base64.StdEncoding.EncodeToString(aead.Seal(nil, nonce, plaintext, aad))}, nil
}
func (c *ConnectionCodec) Open(secret EncryptedSecret, binding SecretBinding) ([]byte, error) {
	if secret.Version != 1 {
		return nil, fmt.Errorf("unsupported connection secret version")
	}
	aead, err := c.aead(secret.KeyID)
	if err != nil {
		return nil, err
	}
	nonce, err := base64.StdEncoding.DecodeString(secret.Nonce)
	if err != nil || len(nonce) != aead.NonceSize() {
		return nil, fmt.Errorf("invalid connection secret")
	}
	data, err := base64.StdEncoding.DecodeString(secret.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("invalid connection secret")
	}
	aad, _ := json.Marshal(binding)
	plain, err := aead.Open(nil, nonce, data, aad)
	if err != nil {
		return nil, fmt.Errorf("connection secret authentication failed")
	}
	return plain, nil
}
