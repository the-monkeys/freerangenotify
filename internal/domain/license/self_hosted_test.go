package license

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelfHostedChecker_UnlicensedWithoutKey(t *testing.T) {
	checker, err := NewSelfHostedChecker(SelfHostedOptions{
		PublicKeyPEM: "dummy",
		CacheTTL:     5 * time.Minute,
	})
	require.NoError(t, err)

	decision, err := checker.Check(context.Background(), nil)
	require.NoError(t, err)
	assert.False(t, decision.Allowed)
	assert.Equal(t, StateUnlicensed, decision.State)
	assert.Equal(t, "license_required", decision.Reason)
}

func TestSelfHostedChecker_ValidSignedKey(t *testing.T) {
	privateKey, publicPEM := generateRSAKeyPair(t)
	licenseKey := signLicenseToken(t, privateKey, time.Now().UTC().Add(-time.Minute), time.Now().UTC().Add(2*time.Hour))

	checker, err := NewSelfHostedChecker(SelfHostedOptions{
		LicenseKey:   licenseKey,
		PublicKeyPEM: publicPEM,
		CacheTTL:     5 * time.Minute,
		FailMode:     FailModeClosed,
	})
	require.NoError(t, err)

	decision, err := checker.Check(context.Background(), nil)
	require.NoError(t, err)
	assert.True(t, decision.Allowed)
	assert.Equal(t, StateActive, decision.State)
	assert.Equal(t, "license_active", decision.Reason)
	assert.Equal(t, ModeSelfHosted, decision.Mode)
}

func TestSelfHostedChecker_ExpiredKey(t *testing.T) {
	privateKey, publicPEM := generateRSAKeyPair(t)
	licenseKey := signLicenseToken(t, privateKey, time.Now().UTC().Add(-2*time.Hour), time.Now().UTC().Add(-time.Hour))

	checker, err := NewSelfHostedChecker(SelfHostedOptions{
		LicenseKey:   licenseKey,
		PublicKeyPEM: publicPEM,
		CacheTTL:     5 * time.Minute,
		FailMode:     FailModeClosed,
	})
	require.NoError(t, err)

	decision, err := checker.Check(context.Background(), nil)
	require.NoError(t, err)
	assert.False(t, decision.Allowed)
	assert.Equal(t, StateExpired, decision.State)
	assert.Equal(t, "license_expired", decision.Reason)
}

func generateRSAKeyPair(t *testing.T) (*rsa.PrivateKey, string) {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	require.NoError(t, err)

	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})
	return priv, string(pubPEM)
}

func signLicenseToken(t *testing.T, priv *rsa.PrivateKey, notBefore, expiresAt time.Time) string {
	t.Helper()

	claims := jwt.MapClaims{
		"nbf":  notBefore.Unix(),
		"exp":  expiresAt.Unix(),
		"plan": "enterprise",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(priv)
	require.NoError(t, err)
	return signed
}
