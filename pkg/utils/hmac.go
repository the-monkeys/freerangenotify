package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// GenerateSubscriberHash creates an HMAC-SHA256 hash of the userID using the
// application's API key as the secret. This is used for SSE subscriber
// authentication to prevent unauthorized connections.
func GenerateSubscriberHash(userID, apiKey string) string {
	mac := hmac.New(sha256.New, []byte(apiKey))
	mac.Write([]byte(userID))
	return hex.EncodeToString(mac.Sum(nil))
}

// ValidateSubscriberHash verifies that the provided hash matches the expected
// HMAC-SHA256 hash of the userID using the API key.
func ValidateSubscriberHash(userID, apiKey, hash string) bool {
	expected := GenerateSubscriberHash(userID, apiKey)
	return hmac.Equal([]byte(expected), []byte(hash))
}
