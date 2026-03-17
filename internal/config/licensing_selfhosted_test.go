//go:build selfhosted

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLicensingOverrides_SelfHostedBuildForcesStrictMode(t *testing.T) {
	o := LicensingOverrides()
	if assert.NotNil(t, o) {
		assert.True(t, o.Enabled)
		assert.Equal(t, "self_hosted", o.DeploymentMode)
		assert.Equal(t, "fail_closed", o.FailMode)
	}
}
