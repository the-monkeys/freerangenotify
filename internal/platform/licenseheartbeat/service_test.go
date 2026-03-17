package licenseheartbeat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/the-monkeys/freerangenotify/internal/config"
	"go.uber.org/zap"
)

func TestNewReturnsNilOutsideSelfHosted(t *testing.T) {
	svc := New(&config.Config{
		Licensing: config.LicensingConfig{Enabled: true, DeploymentMode: "hosted"},
	}, zap.NewNop())
	assert.Nil(t, svc)
}

func TestSendIncludesSignedHeaders(t *testing.T) {
	var gotHeader http.Header
	var gotBody payload

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Clone()
		defer r.Body.Close()
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	svc := &Service{
		endpoint:   ts.URL,
		licenseKey: "abc123",
		version:    "0.2.3",
		mode:       "self_hosted",
		instanceID: "instance-a",
		client:     &http.Client{},
		logger:     zap.NewNop(),
	}

	svc.send(context.Background())

	assert.NotEmpty(t, gotBody.Timestamp)
	assert.Equal(t, "instance-a", gotBody.InstanceID)
	assert.Equal(t, "self_hosted", gotBody.DeploymentMode)
	assert.NotEmpty(t, gotHeader.Get("X-License-Fingerprint"))
	assert.NotEmpty(t, gotHeader.Get("X-Heartbeat-Signature"))
}
