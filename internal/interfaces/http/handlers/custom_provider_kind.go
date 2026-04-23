package handlers

import (
	"fmt"
	"net/url"
	"strings"
)

// inferProviderKind returns the rendering adapter kind from a webhook URL's host.
// The returned kind is one of: discord | slack | teams | generic.
func inferProviderKind(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return "generic"
	}
	host := strings.ToLower(u.Host)
	path := strings.ToLower(u.Path)
	switch {
	case strings.Contains(host, "discord.com") && strings.HasPrefix(path, "/api/webhooks"):
		return "discord"
	case strings.Contains(host, "discordapp.com") && strings.HasPrefix(path, "/api/webhooks"):
		return "discord"
	case strings.Contains(host, "hooks.slack.com"):
		return "slack"
	case strings.HasSuffix(host, "webhook.office.com"):
		return "teams"
	case strings.Contains(host, "logic.azure.com") && strings.Contains(path, "/workflows/"):
		return "teams"
	default:
		return "generic"
	}
}

// validateProviderURLForKind ensures the URL host matches the declared kind.
// Returns a user-friendly error when mismatched; nil for kind=generic.
func validateProviderURLForKind(kind, rawURL string) error {
	if kind == "" || kind == "generic" {
		return nil
	}
	detected := inferProviderKind(rawURL)
	if detected == kind {
		return nil
	}
	return fmt.Errorf("webhook URL does not match kind %q (detected %q)", kind, detected)
}

// validateProviderURLSecurity enforces HTTPS for provider URLs.
// Plain HTTP is allowed only for localhost during local development.
func validateProviderURLSecurity(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return fmt.Errorf("invalid webhook URL")
	}

	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "https":
		return nil
	case "http":
		if isLocalhostHost(u.Hostname()) {
			return nil
		}
		return fmt.Errorf("webhook URL must use https (http allowed only for localhost)")
	default:
		return fmt.Errorf("webhook URL must use https")
	}
}

func isLocalhostHost(host string) bool {
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "localhost", "127.0.0.1", "::1", "host.docker.internal":
		return true
	default:
		return false
	}
}
