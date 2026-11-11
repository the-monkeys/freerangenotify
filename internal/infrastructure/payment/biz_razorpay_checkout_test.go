package payment

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
)

func TestBizCheckoutWireContracts(t *testing.T) {
	ref := bizbilling.GatewayRef{ConnectionID: "fixture-business", AccountID: "acc_fixture", Mode: bizbilling.GatewayTest, CredentialRevision: 1}
	intent := bizbilling.OrderIntent{OperationID: "fixture-operation", InvoiceID: "fixture-invoice", Scope: bizbilling.Scope{AppID: "fixture-app", UserID: "fixture-user", Currency: "INR"}, Gateway: ref, AmountPaisa: 12345, Currency: "INR", Receipt: "frn_fixture_receipt"}
	calls := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key, secret, ok := r.BasicAuth()
		require.True(t, ok)
		require.Equal(t, "rzp_test_biz1", key)
		require.Equal(t, "owned-api-secret", secret)
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		order := map[string]any{"id": "order_fixture", "entity": "order", "amount": 12345, "currency": "INR", "receipt": intent.Receipt, "status": "created", "notes": map[string]string{"frn_operation": intent.OperationID, "frn_invoice": intent.InvoiceID, "frn_app": intent.Scope.AppID}}
		switch r.Method + " " + r.URL.Path {
		case "POST /v1/orders":
			var request struct {
				Amount   int64             `json:"amount"`
				Currency string            `json:"currency"`
				Receipt  string            `json:"receipt"`
				Notes    map[string]string `json:"notes"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
			require.Equal(t, int64(12345), request.Amount)
			require.Equal(t, "INR", request.Currency)
			require.Equal(t, intent.Receipt, request.Receipt)
			require.Equal(t, intent.OperationID, request.Notes["frn_operation"])
			json.NewEncoder(w).Encode(order)
		case "GET /v1/orders":
			require.Equal(t, intent.Receipt, r.URL.Query().Get("receipt"))
			json.NewEncoder(w).Encode(map[string]any{"entity": "collection", "count": 1, "items": []any{order}})
		case "GET /v1/payments/pay_fixture":
			json.NewEncoder(w).Encode(map[string]any{"id": "pay_fixture", "entity": "payment", "order_id": "order_fixture", "amount": 12345, "currency": "INR", "status": "captured", "captured": true})
		default:
			t.Errorf("unexpected wire request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	endpoint, err := url.Parse(server.URL)
	require.NoError(t, err)
	factory := NewBizRazorpayFactory(func(_ context.Context, r bizbilling.GatewayRef) (bizbilling.MerchantCredentials, error) {
		require.Equal(t, ref, r)
		return bizbilling.MerchantCredentials{AppID: "fixture-app", Reference: ref, KeyID: "rzp_test_biz1", KeySecret: "owned-api-secret", WebhookSecret: "owned-hook-secret"}, nil
	}, &http.Client{Transport: bizProbeTransport{endpoint}})
	gateway, err := factory.For(context.Background(), ref)
	require.NoError(t, err)
	order, err := gateway.CreateOrder(context.Background(), intent)
	require.NoError(t, err)
	require.Equal(t, "order_fixture", order.ID)
	require.Equal(t, intent, order.Intent)
	found, err := gateway.FindOrder(context.Background(), intent)
	require.NoError(t, err)
	require.Equal(t, order, *found)
	payment, err := gateway.FetchPayment(context.Background(), "pay_fixture")
	require.NoError(t, err)
	require.True(t, payment.Captured)
	require.Equal(t, ref, payment.Gateway)
	require.Equal(t, int64(12345), payment.AmountPaisa)
	mac := hmac.New(sha256.New, []byte("owned-api-secret"))
	mac.Write([]byte("order_fixture|pay_fixture"))
	signature := hex.EncodeToString(mac.Sum(nil))
	require.NoError(t, gateway.VerifyCheckout("order_fixture", "pay_fixture", signature))
	require.Error(t, gateway.VerifyCheckout("order_other", "pay_fixture", signature))
	require.Error(t, gateway.VerifyCheckout("order_fixture", "pay_fixture", "invalid"))
	require.Len(t, calls, 3)
}
