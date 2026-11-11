package handlers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"go.uber.org/zap"
)

func TestBizInboxPersistBeforeAck(t *testing.T) {
	f := newPortalHTTPFixture(t)
	logger := zap.NewNop()
	response, err := f.es.Indices.Exists([]string{bizbillingrepo.GatewayEventIndex})
	require.NoError(t, err)
	exists := response.StatusCode == 200
	response.Body.Close()
	if !exists {
		body, err := json.Marshal((&database.IndexTemplates{}).GetBizGatewayEventsTemplate())
		require.NoError(t, err)
		response, err = f.es.Indices.Create(bizbillingrepo.GatewayEventIndex, f.es.Indices.Create.WithBody(bytes.NewReader(body)))
		require.NoError(t, err)
		require.False(t, response.IsError())
		response.Body.Close()
	}
	writes := 0
	failedStore := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writes++
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(503)
		io.WriteString(w, `{"error":{"type":"unavailable","reason":"fixture storage unavailable"},"status":503}`)
	}))
	defer failedStore.Close()
	badES, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{failedStore.URL}, DisableRetry: true})
	require.NoError(t, err)
	raw := []byte(`{"entity":"event","account_id":"acc_fixture","event":"payment.captured","payload":{"payment":{"entity":{"id":"pay_fixture","order_id":"order_fixture","amount":1000,"currency":"INR","status":"captured","captured":true}}}}`)
	mac := hmac.New(sha256.New, []byte("owned-hook-secret"))
	mac.Write(raw)
	signature := hex.EncodeToString(mac.Sum(nil))
	for _, tc := range []struct {
		name      string
		es        *elasticsearch.Client
		signature string
		status    int
	}{{"invalid-signature", badES, "invalid", 401}, {"not-persisted", badES, signature, 503}, {"persisted", f.es, signature, 202}} {
		t.Run(tc.name, func(t *testing.T) {
			service := services.NewBizGatewayEventService(tc.es, f.merchants, f.checkout, logger)
			handler := NewBizBillingGatewayCallbackHandler(service, logger)
			app := fiber.New()
			app.Post("/callback/:connection_id", handler.Handle)
			request := httptest.NewRequest("POST", "/callback/"+f.gateway.ref.ConnectionID, bytes.NewReader(raw))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Razorpay-Signature", tc.signature)
			request.Header.Set("X-Razorpay-Event-Id", "fixture-event")
			response, err := app.Test(request, -1)
			require.NoError(t, err)
			defer response.Body.Close()
			data, err := io.ReadAll(response.Body)
			require.NoError(t, err)
			require.Equal(t, tc.status, response.StatusCode, string(data))
			require.NotContains(t, string(data), "owned-hook-secret")
			if tc.name == "invalid-signature" {
				require.Zero(t, writes, "invalid HMAC must never reach the inbox")
			}
			if tc.status == 202 {
				var result struct {
					Data struct {
						ID string `json:"id"`
					} `json:"data"`
				}
				require.NoError(t, json.Unmarshal(data, &result))
				require.NotEmpty(t, result.Data.ID)
				stored, err := bizbillingrepo.NewGatewayEventRepo(f.es, logger).Get(context.Background(), result.Data.ID)
				require.NoError(t, err)
				require.Equal(t, "pending", stored.State)
			}
		})
	}
}

// Characterization of the original signed platform callback, not a new route
// for creating business orders with platform credentials.
func TestBizLegacyGatewayOriginalOrderCallback(t *testing.T) {
	f := newPortalHTTPFixture(t)
	ctx := context.Background()
	logger := zap.NewNop()
	require.NoError(t, bizbillingrepo.NewLifecycleStateRepo(f.es).Mutate(ctx, f.appID, func(state *bizbilling.LifecycleState) error {
		state.LifecycleVersion = 1
		state.Status = "legacy"
		state.PrimaryCurrency = ""
		return nil
	}))
	key, secret, hook := "rzp_test_replacement", "replacement-api-secret", "replacement-hook-secret"
	replacement, err := f.merchants.Save(ctx, f.appID, bizbilling.GatewayTest, 1, bizbilling.MerchantPatch{AccountID: "acc_replacement", KeyID: &key, KeySecret: &secret, WebhookSecret: &hook})
	require.NoError(t, err)
	invoice := &bizbilling.BizInvoice{ID: "legacy-" + f.appID, AppID: f.appID, UserID: f.userID, Currency: "INR", Status: bizbilling.InvoiceStatusOpen, InvoiceNumber: "LEGACY", TotalPaisa: 1000, SubtotalPaisa: 1000, AmountDuePaisa: 1000, GatewayOrderID: "order_platform_" + f.appID}
	require.NoError(t, bizbillingrepo.NewStore[bizbilling.BizInvoice](f.es, "frn_biz_invoices", logger).Create(ctx, invoice.ID, invoice))
	provider := &paymentTestProvider{event: billing.WebhookEvent{EventType: "payment.captured", OrderID: invoice.GatewayOrderID, PaymentID: "pay_legacy_" + f.appID}}
	handler := NewPaymentHandler(provider, nil, nil, billing.DefaultRates(), true, logger)
	handler.SetBizGatewayRecorder(f.lifecycle)
	app := fiber.New()
	app.Post("/legacy-webhook", handler.HandleWebhook)
	for i := 0; i < 2; i++ {
		request := httptest.NewRequest("POST", "/legacy-webhook", bytes.NewReader([]byte(`{}`)))
		request.Header.Set("X-Razorpay-Signature", "fixture-signed")
		response, err := app.Test(request, -1)
		require.NoError(t, err)
		response.Body.Close()
		require.Equal(t, 200, response.StatusCode)
	}
	saved, err := f.lifecycle.GetInvoice(ctx, f.appID, invoice.ID)
	require.NoError(t, err)
	require.Equal(t, bizbilling.InvoiceStatusPaid, saved.Status)
	require.Equal(t, int64(1000), saved.AmountPaidPaisa)
	stillCurrent, err := f.merchants.Snapshot(ctx, f.appID, bizbilling.GatewayTest)
	require.NoError(t, err)
	require.Equal(t, replacement.ConnectionID, stillCurrent.ConnectionID)
	require.Equal(t, "acc_replacement", stillCurrent.AccountID)
}
