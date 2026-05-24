package otp

import (
	"testing"
)

func TestChannel_Valid(t *testing.T) {
	cases := []struct {
		in   Channel
		want bool
	}{
		{ChannelSMS, true},
		{ChannelWhatsApp, true},
		{ChannelEmail, true},
		{Channel(""), false},
		{Channel("push"), false},
		{Channel("SMS"), false}, // case-sensitive on purpose
	}
	for _, tc := range cases {
		t.Run(string(tc.in), func(t *testing.T) {
			if got := tc.in.Valid(); got != tc.want {
				t.Errorf("Channel(%q).Valid() = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestDefaults_Sanity(t *testing.T) {
	if DefaultCodeLength < MinCodeLength || DefaultCodeLength > MaxCodeLength {
		t.Errorf("DefaultCodeLength %d outside [%d,%d]", DefaultCodeLength, MinCodeLength, MaxCodeLength)
	}
	if DefaultMaxAttempts < MinAttempts || DefaultMaxAttempts > MaxAttemptsCap {
		t.Errorf("DefaultMaxAttempts %d outside [%d,%d]", DefaultMaxAttempts, MinAttempts, MaxAttemptsCap)
	}
	if DefaultTTLSeconds <= 0 || DefaultTTLSeconds > MaxTTLSeconds {
		t.Errorf("DefaultTTLSeconds %d invalid", DefaultTTLSeconds)
	}
	if DefaultResendCooldownS <= 0 {
		t.Errorf("DefaultResendCooldownS must be positive")
	}
}
