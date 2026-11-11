package config

import (
	"github.com/stretchr/testify/require"
	"reflect"
	"testing"
)

func TestBizBillingNewCollectionFlag(t *testing.T) {
	t.Setenv("FREERANGE_BIZ_BILLING_NEW_COLLECTION_ENABLED", "true")
	cfg, err := Load()
	require.NoError(t, err)
	field := reflect.ValueOf(cfg.BizBilling).FieldByName("NewCollectionEnabled")
	require.True(t, field.IsValid(), "default-off collection capability is missing")
	require.True(t, field.Bool())
	t.Setenv("FREERANGE_BIZ_BILLING_NEW_COLLECTION_ENABLED", "")
	cfg, err = Load()
	require.NoError(t, err)
	require.False(t, reflect.ValueOf(cfg.BizBilling).FieldByName("NewCollectionEnabled").Bool())
}
