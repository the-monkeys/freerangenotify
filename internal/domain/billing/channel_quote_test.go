package billing

import (
	"strings"
	"testing"
)

func TestChannelQuoteRejectsUnverifiedTariff(t *testing.T) {
	c := &RateCard{Version: "detailed", PricingModel: "detailed-v1", DetailedRates: []ChannelRate{{Channel: "sms", Provider: "twilio", CredentialSource: "byoc", DestinationPrefix: "+91", Country: "IN", Category: "*", CreditsPerUnit: 2, VendorCurrency: "INR"}}}
	if err := c.ValidatePublication(); err == nil {
		t.Fatal("tariff without dated verified sources admitted")
	}
}

func TestChannelQuoteSMSSegments(t *testing.T) {
	for _, tc := range []struct {
		body, encoding string
		units          int64
	}{
		{strings.Repeat("a", 160), "GSM-7", 1}, {strings.Repeat("a", 161), "GSM-7", 2},
		{strings.Repeat("界", 70), "UCS-2", 1}, {strings.Repeat("界", 71), "UCS-2", 2},
		{strings.Repeat("^", 80), "GSM-7", 1}, {strings.Repeat("^", 81), "GSM-7", 2},
		{strings.Repeat("😀", 35), "UCS-2", 1}, {strings.Repeat("😀", 36), "UCS-2", 2},
		{strings.Repeat("a", 152) + "😀", "UCS-2", 3},
	} {
		enc, count, err := SMSUnits(tc.body)
		if err != nil || enc != tc.encoding || count != tc.units {
			t.Errorf("len=%d got %s/%d/%v want %s/%d", len(tc.body), enc, count, err, tc.encoding, tc.units)
		}
	}
}
