//go:build !selfhosted

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLicensingOverrides_DefaultBuildReturnsNil(t *testing.T) {
	assert.Nil(t, LicensingOverrides())
}
