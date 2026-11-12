package payment

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
)

func TestBizProviderPeriodWireReadOnly(t *testing.T) {
	ref := bizbilling.GatewayRef{ConnectionID: "fixture", AccountID: "acc_fixture", Mode: bizbilling.GatewayTest, CredentialRevision: 1}
	wrong := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method, "period sync must never debit a customer")
		invoice := map[string]any{"id": "inv_fixture", "entity": "invoice", "subscription_id": "sub_fixture", "order_id": "order_fixture", "payment_id": "pay_fixture", "status": "paid", "amount": 1180, "amount_paid": 1180, "amount_due": 0, "currency": "INR", "billing_start": 1900000000, "billing_end": 1902678400}
		if wrong {
			invoice["billing_end"] = 1
		}
		switch r.URL.Path {
		case "/v1/invoices/inv_fixture":
			json.NewEncoder(w).Encode(invoice)
		case "/v1/invoices":
			require.Equal(t, "sub_fixture", r.URL.Query().Get("subscription_id"))
			require.Equal(t, "100", r.URL.Query().Get("count"))
			require.Equal(t, "100", r.URL.Query().Get("skip"))
			json.NewEncoder(w).Encode(map[string]any{"entity": "collection", "count": 1, "items": []any{invoice}})
		default:
			t.Errorf("unexpected provider read %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	endpoint, err := url.Parse(server.URL)
	require.NoError(t, err)
	factory := NewBizRazorpayFactory(func(context.Context, bizbilling.GatewayRef) (bizbilling.MerchantCredentials, error) {
		return bizbilling.MerchantCredentials{AppID: "fixture-app", Reference: ref, KeyID: "rzp_test_biz1", KeySecret: "fixture-secret"}, nil
	}, &http.Client{Transport: bizProbeTransport{endpoint}})
	base, err := factory.For(context.Background(), ref)
	require.NoError(t, err)
	gateway := base.(*bizRazorpayGateway)
	invoice, err := gateway.FetchRecurringInvoice(context.Background(), "inv_fixture")
	require.NoError(t, err)
	require.Equal(t, "sub_fixture", invoice.SubscriptionID)
	require.Equal(t, int64(1900000000), invoice.Start.Unix())
	rows, err := gateway.ListRecurringInvoices(context.Background(), "sub_fixture", 100, 100)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	wrong = true
	_, err = gateway.FetchRecurringInvoice(context.Background(), "inv_fixture")
	require.ErrorIs(t, err, bizbilling.ErrInvalidGatewayProof)
}

func TestBizInitialAutomaticIntentDoesNotDeferFirstCharge(t *testing.T) {
	ref := bizbilling.GatewayRef{ConnectionID: "fixture", AccountID: "acc_fixture", Mode: bizbilling.GatewayTest, CredentialRevision: 1}
	gateway := &bizRazorpayGateway{credentials: bizbilling.MerchantCredentials{AppID: "fixture-app", Reference: ref}}
	intent := bizbilling.RecurringIntent{OperationID: "fixture-initial", SubscriptionID: "fixture-subscription", Scope: bizbilling.Scope{AppID: "fixture-app", UserID: "fixture-user", Currency: "INR", Mode: bizbilling.GatewayTest}, Gateway: ref, Name: "Service", AmountPaisa: 1180, Currency: "INR", Period: "monthly", Interval: 1, TotalCount: 12, StartOnAuthorization: true, ExpireBy: time.Now().UTC().Add(time.Hour)}
	require.True(t, gateway.validRecurringIntent(intent), "a new subscription starts on authorization, without waiting a billing cycle")
	intent.StartOnAuthorization = false
	require.False(t, gateway.validRecurringIntent(intent), "existing subscriptions still require an explicit future start")
}
