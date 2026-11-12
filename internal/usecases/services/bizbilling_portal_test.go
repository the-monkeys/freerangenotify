package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"go.uber.org/zap"
	"testing"
	"time"
)

func TestBizPortalTokenStorageDigest(t *testing.T) {
	_, _, _, es, scope := financialFacadesFixture(t)
	ensureBillingIndex(t, es, bizbillingrepo.PortalTokenIndex, (&database.IndexTemplates{}).GetBizPortalTokensTemplate())
	stores := bizbillingrepo.NewStores(es, zap.NewNop())
	service := NewBizPortalService(stores, "https://billing.example.invalid/portal", zap.NewNop())
	token, err := service.CreateToken(context.Background(), scope.AppID, scope.UserID, time.Hour)
	require.NoError(t, err)
	sum := sha256.Sum256([]byte(token.Token))
	id := "v2_" + hex.EncodeToString(sum[:])
	saved, err := stores.PortalTokens.Get(context.Background(), id)
	require.NoError(t, err, "new portal credentials must use a digest document key")
	require.Empty(t, saved.Token, "bearer credential must not be stored in plaintext")
	_, err = stores.PortalTokens.Get(context.Background(), token.Token)
	require.Error(t, err, "new raw token cannot be a document key")
	resolved, err := service.Resolve(context.Background(), token.Token)
	require.NoError(t, err)
	require.Equal(t, scope.UserID, resolved.UserID)
}

func TestBizPortalLegacyExpiry(t *testing.T) {
	_, _, _, es, scope := financialFacadesFixture(t)
	ensureBillingIndex(t, es, bizbillingrepo.PortalTokenIndex, (&database.IndexTemplates{}).GetBizPortalTokensTemplate())
	stores := bizbillingrepo.NewStores(es, zap.NewNop())
	service := NewBizPortalService(stores, "", zap.NewNop())
	ctx := context.Background()
	raw := "legacy-fixture-" + scope.AppID
	legacy := &bizbilling.PortalToken{Token: raw, AppID: scope.AppID, UserID: scope.UserID, CreatedAt: time.Now().Add(-time.Hour), ExpiresAt: time.Now().Add(time.Hour)}
	require.NoError(t, stores.PortalTokens.Create(ctx, raw, legacy))
	before, err := service.Resolve(ctx, raw)
	require.NoError(t, err)
	require.Equal(t, legacy.ExpiresAt.Unix(), before.ExpiresAt.Unix())
	_, err = stores.PortalTokens.Update(ctx, raw, func(row *bizbilling.PortalToken) error { row.ExpiresAt = time.Now().Add(-time.Minute); return nil })
	require.NoError(t, err)
	_, err = service.Resolve(ctx, raw)
	require.Error(t, err)
}
