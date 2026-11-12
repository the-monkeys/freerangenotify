package bizbillingrepo

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
)

func TestBizLiveScopeIDsRemainBackwardCompatible(t *testing.T) {
	live := bizbilling.Scope{AppID: "app", UserID: "customer", Currency: "INR"}
	legacy := []byte(`{"app_id":"app","user_id":"customer","currency":"INR"}`)
	encoded, err := json.Marshal(live)
	require.NoError(t, err)
	require.Equal(t, string(legacy), string(encoded))
	sum := sha256.Sum256(legacy)
	require.Equal(t, hex.EncodeToString(sum[:]), ScopeID(live))
	test := live
	test.Mode = bizbilling.GatewayTest
	require.NotEqual(t, ScopeID(live), ScopeID(test))
	require.NoError(t, test.Validate())
}
