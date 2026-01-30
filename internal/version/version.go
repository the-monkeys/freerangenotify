package version

import (
	_ "embed"
	"strings"
)

//go:embed VERSION
var version string

// Get returns the current version
func Get() string {
	return strings.TrimSpace(version)
}
