package handlers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"go.uber.org/zap"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBizPortalCrossCustomerAndApp(t *testing.T) {
	f := newPortalHTTPFixture(t)
	ctx := context.Background()
	log := zap.NewNop()
	// Additive fixture setup; no index deletion or historical document edits.
	response, err := f.es.Indices.Exists([]string{"frn_biz_subscriptions"})
	require.NoError(t, err)
	exists := response.StatusCode == 200
	response.Body.Close()
	if !exists {
		body, err := json.Marshal((&database.IndexTemplates{}).GetBizSubscriptionsTemplate())
		require.NoError(t, err)
		response, err = f.es.Indices.Create("frn_biz_subscriptions", f.es.Indices.Create.WithBody(bytes.NewReader(body)))
		require.NoError(t, err)
		require.False(t, response.IsError())
		response.Body.Close()
	}
	invoice := &bizbilling.BizInvoice{ID: uuid.NewString(), AppID: f.appID, UserID: f.userID, Currency: "INR", Status: bizbilling.InvoiceStatusOpen, TotalPaisa: 100, AmountDuePaisa: 100, IssuedSnapshot: &bizbilling.InvoiceSnapshot{InvoiceID: "owned", Currency: "INR", TotalPaisa: 100}}
	require.NoError(t, bizbillingrepo.NewStore[bizbilling.BizInvoice](f.es, "frn_biz_invoices", log).Create(ctx, invoice.ID, invoice))
	sub := &bizbilling.BizSubscription{ID: uuid.NewString(), AppID: f.appID, UserID: f.userID, Currency: "INR", Status: bizbilling.SubscriptionStatusActive}
	require.NoError(t, bizbillingrepo.NewStore[bizbilling.BizSubscription](f.es, "frn_biz_subscriptions", log).Create(ctx, sub.ID, sub))
	payment := &bizbilling.BizPayment{ID: uuid.NewString(), AppID: f.appID, UserID: f.userID, Currency: "INR", AmountPaisa: 100, InvoiceIDs: []string{invoice.ID}, Status: bizbilling.PaymentStatusSuccess}
	require.NoError(t, bizbillingrepo.NewStore[bizbilling.BizPayment](f.es, "frn_biz_payments", log).Create(ctx, payment.ID, payment))
	scope := bizbilling.Scope{AppID: f.appID, UserID: f.userID, Currency: "INR"}
	operation, _, err := bizbillingrepo.NewOperationRepo(f.es, log).Begin(ctx, bizbilling.Command{Kind: bizbilling.CommandOpeningBalance, Scope: scope, OpeningBalance: &bizbilling.OpeningBalance{AvailablePaisa: 1}})
	require.NoError(t, err)
	effect := &bizbilling.GatewayEffect{ID: uuid.NewString(), Scope: scope, State: "pending", Kind: "order"}
	require.NoError(t, bizbillingrepo.NewStore[bizbilling.GatewayEffect](f.es, bizbillingrepo.GatewayEffectIndex, log).Create(ctx, effect.ID, effect))
	h := NewBizPortalHandler(BizBillingServices{Portal: f.portal, Lifecycle: f.lifecycle}, log)
	h.SetCheckoutService(f.checkout)
	h.SetOperationService(f.operations)
	app := fiber.New()
	app.Get("/portal/:token/invoices/:id", h.GetInvoice)
	app.Get("/portal/:token/invoices/:id/document", h.GetInvoiceDocument)
	app.Get("/portal/:token/subscriptions/:id", h.GetSubscription)
	app.Get("/portal/:token/payments/:id/receipt", h.GetPaymentReceipt)
	app.Get("/portal/:token/operations/:id", h.GetOperation)
	paths := []string{"/invoices/" + invoice.ID, "/invoices/" + invoice.ID + "/document", "/subscriptions/" + sub.ID, "/payments/" + payment.ID + "/receipt", "/operations/" + operation.ID, "/operations/" + effect.ID}
	for _, owner := range []struct {
		app, user string
		status    int
	}{{f.appID, f.userID, 200}, {f.appID, uuid.NewString(), 404}, {uuid.NewString(), f.userID, 404}} {
		token, err := f.portal.CreateToken(ctx, owner.app, owner.user, time.Hour)
		require.NoError(t, err)
		for _, path := range paths {
			r, err := app.Test(httptest.NewRequest("GET", "/portal/"+token.Token+path, nil), 30000)
			require.NoError(t, err)
			var body map[string]interface{}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			r.Body.Close()
			require.Equal(t, owner.status, r.StatusCode, path, body)
		}
	}
	token, err := f.portal.CreateToken(ctx, f.appID, f.userID, time.Hour)
	require.NoError(t, err)
	require.Error(t, f.portal.Revoke(ctx, uuid.NewString(), token.ID))
	require.NoError(t, f.portal.Revoke(ctx, f.appID, token.ID))
	require.NoError(t, f.portal.Revoke(ctx, f.appID, token.ID))
	r, err := app.Test(httptest.NewRequest("GET", "/portal/"+token.Token+paths[0], nil), 30000)
	require.NoError(t, err)
	r.Body.Close()
	require.Equal(t, 401, r.StatusCode)
}

func TestBizPortalInvoicePagingCompatibility(t *testing.T) {
	f := newPortalHTTPFixture(t)
	ctx := context.Background()
	store := bizbillingrepo.NewStore[bizbilling.BizInvoice](f.es, "frn_biz_invoices", zap.NewNop(), repository.RefreshFalse)
	for i := 0; i < 25; i++ {
		row := &bizbilling.BizInvoice{ID: uuid.NewString(), AppID: f.appID, UserID: f.userID, Currency: "INR", Status: bizbilling.InvoiceStatusOpen, TotalPaisa: 100, AmountDuePaisa: 100, CreatedAt: time.Now().UTC()}
		require.NoError(t, store.Create(ctx, row.ID, row))
	}
	response, err := f.es.Indices.Refresh(f.es.Indices.Refresh.WithIndex("frn_biz_invoices"))
	require.NoError(t, err)
	response.Body.Close()
	token, err := f.portal.CreateToken(ctx, f.appID, f.userID, time.Hour)
	require.NoError(t, err)
	h := NewBizPortalHandler(BizBillingServices{Portal: f.portal, Lifecycle: f.lifecycle}, zap.NewNop())
	app := fiber.New()
	app.Get("/portal/:token/invoices", h.ListInvoices)
	for _, test := range []struct {
		query string
		count int
	}{{"", 25}, {"?limit=20", 20}, {"?limit=20&offset=20", 5}} {
		r, err := app.Test(httptest.NewRequest("GET", "/portal/"+token.Token+"/invoices"+test.query, nil), 30000)
		require.NoError(t, err)
		var body struct {
			Data []bizbilling.BizInvoice `json:"data"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		r.Body.Close()
		require.Equal(t, 200, r.StatusCode)
		require.Len(t, body.Data, test.count)
	}
}

func TestBizPortalExpiredAfterCheckoutCapture(t *testing.T) {
	f := newPortalHTTPFixture(t)
	ctx := context.Background()
	log := zap.NewNop()
	invoice := &bizbilling.BizInvoice{ID: uuid.NewString(), AppID: f.appID, UserID: f.userID, Currency: "INR", GatewayMode: bizbilling.GatewayTest, Status: bizbilling.InvoiceStatusOpen, TotalPaisa: 1000, AmountDuePaisa: 1000}
	require.NoError(t, bizbillingrepo.NewStore[bizbilling.BizInvoice](f.es, "frn_biz_invoices", log).Create(ctx, invoice.ID, invoice))
	token, err := f.portal.CreateToken(ctx, f.appID, f.userID, time.Hour)
	require.NoError(t, err)
	h := NewBizPortalHandler(BizBillingServices{Portal: f.portal, Lifecycle: f.lifecycle}, log)
	h.SetCheckoutService(f.checkout)
	app := fiber.New()
	app.Post("/portal/:token/invoices/:id/pay", h.PayInvoice)
	app.Get("/portal/:token/invoices/:id", h.GetInvoice)
	path := "/portal/" + token.Token + "/invoices/" + invoice.ID
	r, err := app.Test(httptest.NewRequest("POST", path+"/pay", nil), 30000)
	require.NoError(t, err)
	r.Body.Close()
	require.Equal(t, 200, r.StatusCode)
	store := bizbillingrepo.NewStore[bizbilling.PortalToken](f.es, bizbillingrepo.PortalTokenIndex, log)
	_, err = store.Update(ctx, token.ID, func(row *bizbilling.PortalToken) error { row.ExpiresAt = time.Now().Add(-time.Minute); return nil })
	require.NoError(t, err)
	r, err = app.Test(httptest.NewRequest("GET", path, nil), 30000)
	require.NoError(t, err)
	r.Body.Close()
	require.Equal(t, 401, r.StatusCode)
	// Real signed callback inbox and processor work independently of portal expiry.
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
	events := services.NewBizGatewayEventService(f.es, f.merchants, f.checkout, log)
	callback := NewBizBillingGatewayCallbackHandler(events, log)
	app.Post("/callback/:connection_id", callback.Handle)
	f.gateway.captured = true
	raw := `{"entity":"event","account_id":"acc_fixture","event":"payment.captured","payload":{"payment":{"entity":{"id":"pay_fixture","order_id":"order_fixture","amount":1000,"currency":"INR","status":"captured","captured":true}}}}`
	mac := hmac.New(sha256.New, []byte("owned-hook-secret"))
	mac.Write([]byte(raw))
	request := httptest.NewRequest("POST", "/callback/"+f.gateway.ref.ConnectionID, strings.NewReader(raw))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Razorpay-Signature", hex.EncodeToString(mac.Sum(nil)))
	r, err = app.Test(request, 30000)
	require.NoError(t, err)
	var receipt struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.NewDecoder(r.Body).Decode(&receipt))
	r.Body.Close()
	require.Equal(t, 202, r.StatusCode)
	result, err := events.Process(ctx, receipt.Data.ID)
	require.NoError(t, err)
	require.Equal(t, bizbilling.OperationCompleted, result.State)
	saved, err := f.lifecycle.GetInvoice(ctx, f.appID, invoice.ID)
	require.NoError(t, err)
	require.Equal(t, bizbilling.InvoiceStatusPaid, saved.Status)
	r, err = app.Test(httptest.NewRequest("GET", path, nil), 30000)
	require.NoError(t, err)
	r.Body.Close()
	require.Equal(t, 401, r.StatusCode, "capture cannot renew portal access")
}
