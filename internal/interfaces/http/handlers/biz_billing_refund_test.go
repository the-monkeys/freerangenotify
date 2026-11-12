package handlers

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"go.uber.org/zap"
)

type refundHTTPGateway struct {
	*portalCheckoutGateway
	refund bizbilling.GatewayRefund
	status string
	calls  int
}

func (g *refundHTTPGateway) For(_ context.Context, ref bizbilling.GatewayRef) (bizbilling.BizGateway, error) {
	if ref != g.ref {
		return nil, bizbilling.ErrMerchantUnavailable
	}
	return g, nil
}
func (g *refundHTTPGateway) Refund(_ context.Context, intent bizbilling.RefundIntent) (bizbilling.GatewayRefund, error) {
	g.calls++
	g.refund = bizbilling.GatewayRefund{ID: "rfnd_http_fixture", Intent: intent, Status: g.status}
	return g.refund, nil
}
func (g *refundHTTPGateway) FindRefund(_ context.Context, intent bizbilling.RefundIntent) (*bizbilling.GatewayRefund, error) {
	if g.refund.Intent != intent {
		return nil, nil
	}
	result := g.refund
	result.Status = g.status
	return &result, nil
}

func TestBizRefundHTTPPendingStatusAndConfirmedReplay(t *testing.T) {
	f := newPortalHTTPFixture(t)
	ctx := context.Background()
	invoice := &bizbilling.BizInvoice{ID: uuid.NewString(), AppID: f.appID, UserID: f.userID, Currency: "INR", GatewayMode: bizbilling.GatewayTest, InvoiceNumber: "TEST-HTTP", Status: bizbilling.InvoiceStatusOpen, TotalPaisa: 1000, AmountDuePaisa: 1000}
	require.NoError(t, bizbillingrepo.NewStore[bizbilling.BizInvoice](f.es, "frn_biz_invoices", zap.NewNop()).Create(ctx, invoice.ID, invoice))
	order, err := f.checkout.Start(ctx, f.appID, f.userID, invoice.ID, bizbilling.GatewayTest, "http-checkout")
	require.NoError(t, err)
	f.gateway.captured = true
	result, err := f.checkout.Verify(ctx, f.appID, f.userID, invoice.ID, bizbilling.CheckoutProof{OrderID: order.OrderID, PaymentID: "pay_fixture", Signature: "fixture-valid"})
	require.NoError(t, err)
	paymentID := ""
	for _, resource := range result.Resources {
		if resource.Kind == "payment" {
			paymentID = resource.ID
		}
	}
	require.NotEmpty(t, paymentID)
	gateway := &refundHTTPGateway{portalCheckoutGateway: f.gateway, status: "pending"}
	f.lifecycle.SetRefundService(services.NewBizRefundService(f.es, f.lifecycle, f.merchants, gateway, zap.NewNop()))
	handler := NewBizBillingHandler(BizBillingServices{Lifecycle: f.lifecycle}, zap.NewNop())
	handler.SetCheckoutService(f.checkout)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error { c.Locals("app_id", f.appID); return c.Next() })
	app.Use(handler.GuardMutations)
	app.Post("/payments/:id/refund", handler.RefundPayment)
	app.Get("/effects/:id", handler.GetGatewayEffect)
	request := func(method, path, key string, status int) map[string]any {
		req := httptest.NewRequest(method, path, strings.NewReader(`{"amount_paisa":500,"reason":"customer return"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", key)
		response, err := app.Test(req, 30000)
		require.NoError(t, err)
		defer response.Body.Close()
		var body map[string]any
		require.NoError(t, json.NewDecoder(response.Body).Decode(&body))
		require.Equal(t, status, response.StatusCode, body)
		if status == 503 {
			require.Equal(t, body["operation_id"], response.Header.Get("Operation-Id"))
		}
		return body
	}
	path := "/payments/" + paymentID + "/refund"
	request("POST", path, "", 400)
	pending := request("POST", path, "http-refund", 503)
	require.Equal(t, "BILLING_GATEWAY_PENDING", pending["code"])
	operationID := pending["operation_id"].(string)
	require.NotEmpty(t, operationID)
	stored := request("GET", "/effects/"+operationID, "", 200)["data"].(map[string]any)
	require.Equal(t, "provider_pending", stored["state"])
	payment, err := f.lifecycle.GetPayment(ctx, f.appID, paymentID)
	require.NoError(t, err)
	require.Zero(t, payment.RefundedPaisa)
	gateway.status = "processed"
	for i := 0; i < 2; i++ {
		paid := request("POST", path, "http-refund", 200)["data"].(map[string]any)
		require.Equal(t, float64(500), paid["refunded_paisa"])
	}
	require.Equal(t, 1, gateway.calls)
}
