//go:build !selfhosted

package config

// LicensingOverrides returns nil for default builds, which keeps runtime
// licensing configuration sourced from config files and environment variables.
func LicensingOverrides() *LicensingConfig {
	return nil
}
