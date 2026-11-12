package services

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"go.uber.org/zap"
)

type billingPreferencesRepo struct {
	config *bizbilling.BizConfig
	writes int
}

func (r *billingPreferencesRepo) Get(context.Context, string, bizbilling.ConfigType) (*bizbilling.BizConfig, error) {
	return r.config, nil
}
func (r *billingPreferencesRepo) Upsert(_ context.Context, c *bizbilling.BizConfig) error {
	r.config = c
	r.writes++
	return nil
}

func TestBizPreferencesDefaultAndValidation(t *testing.T) {
	repo := &billingPreferencesRepo{}
	s := NewBizConfigService(repo, zap.NewNop())
	ctx := context.Background()
	initial, err := s.GetConfig(ctx, "app", bizbilling.ConfigTypePreferences)
	require.NoError(t, err)
	require.Equal(t, "INR", initial.Data["default_currency"])
	require.Zero(t, repo.writes)
	_, err = s.UpdateConfig(ctx, "app", bizbilling.ConfigTypePreferences, map[string]interface{}{"default_currency": "USD"})
	require.NoError(t, err)
	stored, err := s.GetConfig(ctx, "app", bizbilling.ConfigTypePreferences)
	require.NoError(t, err)
	require.Equal(t, "USD", stored.Data["default_currency"])
	for _, bad := range []interface{}{"", "XYZ", "usd", 100} {
		_, err = s.UpdateConfig(ctx, "app", bizbilling.ConfigTypePreferences, map[string]interface{}{"default_currency": bad})
		require.Error(t, err)
	}
	require.Equal(t, 1, repo.writes, "invalid defaults must not replace the saved choice")
}
