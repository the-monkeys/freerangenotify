package payment

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	razorpay "github.com/razorpay/razorpay-go"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"go.uber.org/zap"
)

func TestCapturedPaymentChecksGatewayAmountOrderAndStatus(t *testing.T) {
	for _, tc := range []struct {
		name, order, status, currency string
		amount                        int64
		wantError                     bool
	}{
		{"captured", "order-1", "captured", "INR", 49900, false},
		{"wrong-amount", "order-1", "captured", "INR", 1, true},
		{"wrong-order", "order-other", "captured", "INR", 49900, true},
		{"authorized-only", "order-1", "authorized", "INR", 49900, true},
		{"wrong-currency", "order-1", "captured", "USD", 49900, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/payments/pay-1" {
					t.Errorf("unexpected gateway path: %s", r.URL.Path)
				}
				fmt.Fprintf(w, `{"id":"pay-1","order_id":%q,"status":%q,"amount":%d,"currency":%q}`, tc.order, tc.status, tc.amount, tc.currency)
			}))
			defer server.Close()
			p := &RazorpayProvider{client: razorpay.NewClient("test-key", "test-secret"), logger: zap.NewNop()}
			p.client.BaseURL = server.URL
			verifier, ok := interface{}(p).(interface {
				VerifyCapturedPayment(context.Context, billing.PaymentVerification, int64, string) error
			})
			if !ok {
				t.Fatal("gateway capture verification is missing")
			}
			err := verifier.VerifyCapturedPayment(context.Background(), billing.PaymentVerification{OrderID: "order-1", PaymentID: "pay-1"}, 49900, "INR")
			if (err != nil) != tc.wantError {
				t.Fatalf("capture validation error=%v, wantError=%v", err, tc.wantError)
			}
		})
	}
}

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
