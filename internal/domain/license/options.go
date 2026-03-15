package license

import "time"

// Fail mode values.
const (
	FailModeClosed = "fail_closed"
	FailModeOpen   = "fail_open"
)

// HostedOptions configures hosted checker behavior.
type HostedOptions struct {
	CacheTTL    time.Duration
	GraceWindow time.Duration
	FailMode    string
}

// SelfHostedOptions configures self-hosted checker behavior.
type SelfHostedOptions struct {
	CacheTTL     time.Duration
	GraceWindow  time.Duration
	FailMode     string
	LicenseKey   string
	PublicKeyPEM string
}
