package services

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/payment"
	"go.uber.org/zap"
)

type checkoutWireTransport struct{ destination *url.URL }

func (t checkoutWireTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Scheme != "https" || r.URL.Host != "api.razorpay.com" {
		panic("unexpected billing provider destination")
	}
	request := r.Clone(r.Context())
	endpoint := *r.URL
	endpoint.Scheme, endpoint.Host = t.destination.Scheme, t.destination.Host
	request.URL = &endpoint
	return http.DefaultTransport.RoundTrip(request)
}

func TestBizGatewayRotatedCheckoutPublicKeyAndProof(t *testing.T) {
	checkout, merchants, gateway, lifecycle, _, es, scope := checkoutFixture(t)
	ctx := context.Background()
	ensureBillingIndex(t, es, bizbillingrepo.GatewayEventIndex, (&database.IndexTemplates{}).GetBizGatewayEventsTemplate())
	target := seedCheckoutInvoice(t, es, scope, 1000)
	old, err := checkout.Start(ctx, scope.AppID, scope.UserID, target.ID, bizbilling.GatewayTest, "target")
	require.NoError(t, err)
	// The ordinary test gateway uses UUIDs; this fixture exercises the real
	// provider adapter, whose IDs are alphanumeric.
	orderID := strings.ReplaceAll(old.OrderID, "-", "")
	_, err = checkout.effects.Update(ctx, old.OperationID, func(effect *bizbilling.GatewayEffect) error {
		effect.Order.ID = orderID
		effect.ProviderID = orderID
		return nil
	})
	require.NoError(t, err)
	key, secret, hook := "rzp_test_biz2", "rotated-api-secret", "rotated-hook-secret"
	_, err = merchants.Save(ctx, scope.AppID, bizbilling.GatewayTest, 1, bizbilling.MerchantPatch{KeyID: &key, KeySecret: &secret, WebhookSecret: &hook})
	require.NoError(t, err)
	proofInvoice := seedCheckoutInvoice(t, es, scope, 1000)
	proofOrder, err := checkout.Start(ctx, scope.AppID, scope.UserID, proofInvoice.ID, bizbilling.GatewayTest, "account-proof")
	require.NoError(t, err)
	gateway.ref.CredentialRevision = 2
	gateway.payment = bizbilling.CapturedPayment{ID: "pay_proof", OrderID: proofOrder.OrderID, Gateway: gateway.ref, AmountPaisa: 1000, Currency: scope.Currency, Captured: true, Status: "captured"}
	events := NewBizGatewayEventService(es, merchants, checkout, zap.NewNop())
	raw := gatewayRaw("payment.captured", "pay_proof", proofOrder.OrderID, gateway.ref.AccountID, scope.Currency, 1000)
	receipt, err := events.Accept(ctx, gateway.ref.ConnectionID, "account-proof", raw, gatewaySignature(raw, hook))
	require.NoError(t, err)
	_, err = events.Process(ctx, receipt.ID)
	require.NoError(t, err)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey, apiSecret, _ := r.BasicAuth()
		require.Equal(t, key, apiKey)
		require.Equal(t, secret, apiSecret)
		require.Equal(t, "/v1/payments/pay_target", r.URL.Path)
		json.NewEncoder(w).Encode(map[string]any{"entity": "payment", "id": "pay_target", "order_id": orderID, "amount": 1000, "currency": scope.Currency, "status": "captured", "captured": true})
	}))
	defer server.Close()
	endpoint, err := url.Parse(server.URL)
	require.NoError(t, err)
	factory := payment.NewBizRazorpayFactory(merchants.Resolve, &http.Client{Transport: checkoutWireTransport{endpoint}})
	factory.SetAPIResolver(merchants.ResolveAPI)
	checkout.factory = factory
	resumed, err := checkout.Start(ctx, scope.AppID, scope.UserID, target.ID, bizbilling.GatewayTest, "target")
	require.NoError(t, err)
	require.Equal(t, key, resumed.PublicKeyID, "a retired public key must not be returned to new checkout attempts")
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(orderID + "|pay_target"))
	proof := bizbilling.CheckoutProof{OrderID: orderID, PaymentID: "pay_target", Signature: hex.EncodeToString(mac.Sum(nil))}
	result, err := checkout.Verify(ctx, scope.AppID, scope.UserID, target.ID, proof)
	require.NoError(t, err)
	require.Equal(t, bizbilling.OperationCompleted, result.State)
	mac = hmac.New(sha256.New, []byte("owned-api-secret"))
	mac.Write([]byte(orderID + "|pay_target"))
	proof.Signature = hex.EncodeToString(mac.Sum(nil))
	replayed, err := checkout.Verify(ctx, scope.AppID, scope.UserID, target.ID, proof)
	require.NoError(t, err)
	require.Equal(t, result.OperationID, replayed.OperationID)
	saved, err := lifecycle.GetPayment(ctx, scope.AppID, operationResource(t, result, "payment"))
	require.NoError(t, err)
	require.Equal(t, int64(1), saved.GatewayBinding.CredentialRevision, "the original order binding remains immutable")
}
