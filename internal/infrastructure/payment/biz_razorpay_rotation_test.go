package payment

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
)

func TestBizGatewayPinnedRevisionAPIRecovery(t *testing.T) {
	ref := bizbilling.GatewayRef{ConnectionID: "fixture-business", AccountID: "acc_fixture", Mode: bizbilling.GatewayTest, CredentialRevision: 1}
	old := bizbilling.MerchantCredentials{AppID: "fixture-app", Reference: ref, KeyID: "rzp_test_old", KeySecret: "old-api-secret"}
	rotated := old
	rotated.Reference.CredentialRevision = 2
	rotated.KeyID = "rzp_test_new"
	rotated.KeySecret = "new-api-secret"
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		key, secret, _ := r.BasicAuth()
		if key != rotated.KeyID || secret != rotated.KeySecret {
			w.WriteHeader(401)
			io.WriteString(w, `{"error":{"description":"old-api-secret"}}`)
			return
		}
		require.Equal(t, "/v1/payments/pay_fixture", r.URL.Path)
		io.WriteString(w, `{"entity":"payment","id":"pay_fixture","order_id":"order_fixture","amount":1000,"currency":"INR","status":"captured","captured":true}`)
	}))
	defer server.Close()
	endpoint, err := url.Parse(server.URL)
	require.NoError(t, err)
	factory := NewBizRazorpayFactory(func(context.Context, bizbilling.GatewayRef) (bizbilling.MerchantCredentials, error) { return old, nil }, &http.Client{Transport: bizProbeTransport{endpoint}})
	factory.SetAPIResolver(func(context.Context, bizbilling.GatewayRef) (bizbilling.MerchantCredentials, error) {
		return rotated, nil
	})
	gateway, err := factory.For(context.Background(), ref)
	require.NoError(t, err)
	payment, err := gateway.FetchPayment(context.Background(), "pay_fixture")
	require.NoError(t, err)
	require.Equal(t, ref, payment.Gateway, "API rotation must not rewrite historical binding")
	mac := hmac.New(sha256.New, []byte(old.KeySecret))
	mac.Write([]byte("order_fixture|pay_fixture"))
	require.NoError(t, gateway.VerifyCheckout("order_fixture", "pay_fixture", hex.EncodeToString(mac.Sum(nil))), "checkout HMAC still uses original revision")
	exact, err := factory.ForExact(context.Background(), ref)
	require.NoError(t, err)
	_, err = exact.FetchPayment(context.Background(), "pay_fixture")
	require.Error(t, err)
	require.NotContains(t, err.Error(), old.KeySecret)
	rotated.Reference.AccountID = "acc_wrong"
	_, err = factory.For(context.Background(), ref)
	require.Error(t, err)
	require.Equal(t, 2, calls, "another account must never receive a request")
}
