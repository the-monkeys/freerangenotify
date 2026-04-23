package handlers

import "testing"

func TestInferProviderKind(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"https://discord.com/api/webhooks/123/abc", "discord"},
		{"https://discordapp.com/api/webhooks/123/abc", "discord"},
		{"https://hooks.slack.com/services/T/B/X", "slack"},
		{"https://outlook.webhook.office.com/webhookb2/abc", "teams"},
		{"https://prod-12.westus.logic.azure.com:443/workflows/abc/triggers/manual", "teams"},
		{"https://example.com/receive", "generic"},
		{"not a url", "generic"},
		{"", "generic"},
	}
	for _, tc := range cases {
		if got := inferProviderKind(tc.url); got != tc.want {
			t.Errorf("inferProviderKind(%q) = %q; want %q", tc.url, got, tc.want)
		}
	}
}

func TestValidateProviderURLForKind(t *testing.T) {
	// Matches — no error
	if err := validateProviderURLForKind("slack", "https://hooks.slack.com/services/T/B/X"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := validateProviderURLForKind("generic", "https://anywhere.example.com/x"); err != nil {
		t.Fatalf("generic should never fail: %v", err)
	}
	if err := validateProviderURLForKind("", "https://anywhere.example.com/x"); err != nil {
		t.Fatalf("empty kind should never fail: %v", err)
	}

	// Mismatch — errors
	if err := validateProviderURLForKind("slack", "https://discord.com/api/webhooks/1/x"); err == nil {
		t.Fatalf("expected error for slack/discord mismatch")
	}
	if err := validateProviderURLForKind("discord", "https://hooks.slack.com/services/T/B/X"); err == nil {
		t.Fatalf("expected error for discord/slack mismatch")
	}
	if err := validateProviderURLForKind("teams", "https://example.com/x"); err == nil {
		t.Fatalf("expected error for teams/generic mismatch")
	}
}

func TestValidateProviderURLSecurity(t *testing.T) {
	allowed := []string{
		"https://hooks.slack.com/services/T/B/X",
		"https://discord.com/api/webhooks/123/abc",
		"http://localhost:8080/hook",
		"http://127.0.0.1:8080/hook",
		"http://[::1]:8080/hook",
		"http://host.docker.internal:8080/v1/playground/abc123",
	}
	for _, u := range allowed {
		if err := validateProviderURLSecurity(u); err != nil {
			t.Fatalf("expected allowed URL %q, got err: %v", u, err)
		}
	}

	rejected := []string{
		"http://example.com/hook",
		"ftp://example.com/hook",
		"not a url",
		"",
	}
	for _, u := range rejected {
		if err := validateProviderURLSecurity(u); err == nil {
			t.Fatalf("expected rejected URL %q", u)
		}
	}
}
