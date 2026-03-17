//go:build selfhosted

package config

// LicensingOverrides forces strict licensing behavior for self-hosted builds.
func LicensingOverrides() *LicensingConfig {
	return &LicensingConfig{
		Enabled:        true,
		DeploymentMode: "self_hosted",
		FailMode:       "fail_closed",
	}
}
