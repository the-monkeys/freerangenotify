package payment

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
)

func TestBizRefundWireContracts(t *testing.T) {
	ref := bizbilling.GatewayRef{ConnectionID: "fixture-business", AccountID: "acc_fixture", Mode: bizbilling.GatewayTest, CredentialRevision: 1}
	intent := bizbilling.RefundIntent{OperationID: "fixture-operation", ReservationID: "fixture-reservation", PaymentID: "pay_fixture", Scope: bizbilling.Scope{AppID: "fixture-app", UserID: "fixture-user", Currency: "INR"}, Gateway: ref, AmountPaisa: 1000, Currency: "INR", Receipt: "frn_refund_fixture"}
	status := "pending"
	amount := int64(1000)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		key, secret, ok := r.BasicAuth()
		require.True(t, ok)
		require.Equal(t, "rzp_test_biz1", key)
		require.Equal(t, "owned-api-secret", secret)
		refund := map[string]any{"id": "rfnd_fixture", "entity": "refund", "payment_id": "pay_fixture", "amount": amount, "currency": "INR", "receipt": intent.Receipt, "status": status, "notes": map[string]string{"frn_operation": intent.OperationID, "frn_reservation": intent.ReservationID, "frn_app": intent.Scope.AppID}}
		switch r.Method + " " + r.URL.Path {
		case "POST /v1/payments/pay_fixture/refund":
			var body struct {
				Amount  int64             `json:"amount"`
				Speed   string            `json:"speed"`
				Receipt string            `json:"receipt"`
				Notes   map[string]string `json:"notes"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.Equal(t, int64(1000), body.Amount)
			require.Equal(t, "normal", body.Speed)
			require.Equal(t, intent.Receipt, body.Receipt)
			require.Equal(t, intent.ReservationID, body.Notes["frn_reservation"])
			json.NewEncoder(w).Encode(refund)
		case "GET /v1/payments/pay_fixture/refunds":
			require.Equal(t, "100", r.URL.Query().Get("count"))
			require.Equal(t, "0", r.URL.Query().Get("skip"))
			json.NewEncoder(w).Encode(map[string]any{"entity": "collection", "count": 1, "items": []any{refund}})
		default:
			t.Errorf("unexpected refund request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	endpoint, err := url.Parse(server.URL)
	require.NoError(t, err)
	factory := NewBizRazorpayFactory(func(context.Context, bizbilling.GatewayRef) (bizbilling.MerchantCredentials, error) {
		return bizbilling.MerchantCredentials{AppID: "fixture-app", Reference: ref, KeyID: "rzp_test_biz1", KeySecret: "owned-api-secret"}, nil
	}, &http.Client{Transport: bizProbeTransport{endpoint}})
	gateway, err := factory.For(context.Background(), ref)
	require.NoError(t, err)
	refund, err := gateway.Refund(context.Background(), intent)
	require.NoError(t, err)
	require.Equal(t, "pending", refund.Status)
	require.Equal(t, intent, refund.Intent)
	status = "processed"
	found, err := gateway.FindRefund(context.Background(), intent)
	require.NoError(t, err)
	require.Equal(t, "processed", found.Status)
	require.Equal(t, intent, found.Intent)
	amount = 1001
	_, err = gateway.FindRefund(context.Background(), intent)
	require.Error(t, err, "a receipt cannot override mismatched refund money")
	require.Equal(t, 3, calls)
}
