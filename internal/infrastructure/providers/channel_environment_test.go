package providers

import (
	"go.uber.org/zap"
	"testing"
)

func TestChannelEnvironmentDisabledMetaFactory(t *testing.T) {
	p, err := GetFactory("meta_whatsapp")(map[string]interface{}{"enabled": false, "phone_number_id": "stale-phone", "access_token": "fixture-stale"}, zap.NewNop())
	if err == nil || p != nil {
		t.Fatal("disabled provider bootstrapped from stale credentials")
	}
}
