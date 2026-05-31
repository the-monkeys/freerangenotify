package payment

import "testing"

func TestRazorpayReceiptIsShortAndUnique(t *testing.T) {
	first := razorpayReceipt("enterprise-plan-with-long-name")
	second := razorpayReceipt("enterprise-plan-with-long-name")

	if len(first) > 40 {
		t.Fatalf("receipt length = %d, want <= 40: %q", len(first), first)
	}
	if first == second {
		t.Fatalf("receipts should be unique, got %q twice", first)
	}
	if first[:4] != "frn_" {
		t.Fatalf("receipt prefix = %q, want frn_", first[:4])
	}
}
