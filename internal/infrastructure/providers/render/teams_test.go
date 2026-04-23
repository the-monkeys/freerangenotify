package render

import "testing"

func TestDetectTeamsWebhookFlavor(t *testing.T) {
	cases := []struct {
		name   string
		url    string
		expect TeamsWebhookFlavor
	}{
		{name: "legacy office webhook", url: "https://outlook.webhook.office.com/webhookb2/abc", expect: TeamsWebhookFlavorLegacy},
		{name: "workflow logic app", url: "https://prod-12.westus.logic.azure.com:443/workflows/abc/triggers/manual", expect: TeamsWebhookFlavorWorkflow},
		{name: "unknown defaults to legacy", url: "https://example.com/hook", expect: TeamsWebhookFlavorLegacy},
		{name: "invalid defaults to legacy", url: "not-a-url", expect: TeamsWebhookFlavorLegacy},
	}

	for _, tc := range cases {
		if got := DetectTeamsWebhookFlavor(tc.url); got != tc.expect {
			t.Fatalf("%s: expected %q, got %q", tc.name, tc.expect, got)
		}
	}
}
