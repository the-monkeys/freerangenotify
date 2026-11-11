package handlers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/secrets"
	"github.com/the-monkeys/freerangenotify/internal/testutil"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"go.uber.org/zap"
)

type portalCheckoutGateway struct {
	ref      bizbilling.GatewayRef
	order    bizbilling.GatewayOrder
	creates  int
	captured bool
}

func (g *portalCheckoutGateway) For(_ context.Context, ref bizbilling.GatewayRef) (bizbilling.BizGateway, error) {
	if ref != g.ref {
		return nil, errors.New("wrong fixture account")
	}
	return g, nil
}
func (g *portalCheckoutGateway) CreateOrder(_ context.Context, intent bizbilling.OrderIntent) (bizbilling.GatewayOrder, error) {
	g.creates++
	g.order = bizbilling.GatewayOrder{ID: "order_fixture", Intent: intent}
	return g.order, nil
}
func (g *portalCheckoutGateway) FindOrder(context.Context, bizbilling.OrderIntent) (*bizbilling.GatewayOrder, error) {
	if g.order.ID == "" {
		return nil, nil
	}
	return &g.order, nil
}
func (g *portalCheckoutGateway) FetchPayment(context.Context, string) (bizbilling.CapturedPayment, error) {
	return bizbilling.CapturedPayment{ID: "pay_fixture", OrderID: g.order.ID, Gateway: g.ref, AmountPaisa: g.order.Intent.AmountPaisa, Currency: "INR", Captured: g.captured, Status: map[bool]string{true: "captured", false: "authorized"}[g.captured]}, nil
}
func (g *portalCheckoutGateway) VerifyCheckout(_, _, signature string) error {
	if signature != "fixture-valid" {
		return bizbilling.ErrInvalidGatewayProof
	}
	return nil
}

type portalHTTPFixture struct {
	es            *elasticsearch.Client
	lifecycle     *services.BizLifecycleService
	portal        *services.BizPortalService
	merchants     *services.BizGatewayConnectionService
	checkout      *services.BizCheckoutService
	gateway       *portalCheckoutGateway
	appID, userID string
}

func newPortalHTTPFixture(t *testing.T) portalHTTPFixture {
	if testing.Short() {
		t.Skip("requires owned billing fixture")
	}
	manifest := testutil.RequireOwnedBizFixture(t)
	es, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{manifest.Services["elasticsearch"].URL}, DisableRetry: true})
	require.NoError(t, err)
	ctx := context.Background()
	logger := zap.NewNop()
	templates := &database.IndexTemplates{}
	for index, template := range map[string]map[string]interface{}{
		"users": templates.GetUsersTemplate(), "frn_biz_invoices": templates.GetBizInvoicesTemplate(), "frn_biz_payments": templates.GetBizPaymentsTemplate(), "frn_biz_configs": templates.GetBizConfigsTemplate(),
		bizbillingrepo.OperationIndex: templates.GetBizOperationsTemplate(), bizbillingrepo.JournalIndex: templates.GetBizJournalTemplate(), bizbillingrepo.StreamIndex: templates.GetBizStreamsTemplate(), bizbillingrepo.CustomerBalanceIndex: templates.GetBizCustomerBalancesTemplate(),
		bizbillingrepo.LifecycleStateIndex: templates.GetBizLifecycleStatesTemplate(), bizbillingrepo.GatewayConnectionIndex: templates.GetBizGatewayConnectionsTemplate(), bizbillingrepo.GatewayEffectIndex: templates.GetBizGatewayEffectsTemplate(), bizbillingrepo.PortalTokenIndex: templates.GetBizPortalTokensTemplate(),
	} {
		response, err := es.Indices.Exists([]string{index})
		require.NoError(t, err)
		exists := response.StatusCode == 200
		response.Body.Close()
		if exists {
			continue
		}
		body, err := json.Marshal(template)
		require.NoError(t, err)
		response, err = es.Indices.Create(index, es.Indices.Create.WithBody(bytes.NewReader(body)))
		require.NoError(t, err)
		require.False(t, response.IsError())
		response.Body.Close()
	}
	appID, userID := uuid.NewString(), uuid.NewString()
	state := bizbilling.DefaultLifecycleState(appID)
	state.LifecycleVersion = 2
	state.Status = "active"
	state.PrimaryCurrency = "INR"
	stateRepo := bizbillingrepo.NewLifecycleStateRepo(es)
	require.NoError(t, stateRepo.Mutate(ctx, appID, func(s *bizbilling.LifecycleState) error { *s = state; return nil }))
	gate := services.NewBizLifecycleGate(stateRepo)
	stores := bizbillingrepo.NewStores(es, logger)
	config := services.NewBizConfigService(bizbillingrepo.NewConfigRepo(es, logger), logger)
	portal := services.NewBizPortalService(stores, "", logger)
	lifecycle := services.NewBizLifecycleService(bizbillingrepo.NewLifecycleRepo(es, logger), stores, nil, config, nil, portal, nil, nil, nil, "INR", logger)
	ops := services.NewBizOperationService(es, logger)
	ops.SetLifecycleGate(gate)
	lifecycle.SetOperationService(ops)
	lifecycle.SetLifecycleGate(gate)
	codec, err := secrets.NewConnectionCodec("fixture:" + base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{37}, 32)))
	require.NoError(t, err)
	merchants := services.NewBizGatewayConnectionService(es, gate, codec, func(context.Context, bizbilling.MerchantCredentials) error { return nil }, logger)
	key, secret, hook := "rzp_test_biz1", "owned-api-secret", "owned-hook-secret"
	snapshot, err := merchants.Save(ctx, appID, bizbilling.GatewayTest, 0, bizbilling.MerchantPatch{AccountID: "acc_fixture", KeyID: &key, KeySecret: &secret, WebhookSecret: &hook})
	require.NoError(t, err)
	gateway := &portalCheckoutGateway{ref: bizbilling.GatewayRef{ConnectionID: snapshot.ConnectionID, AccountID: snapshot.AccountID, Mode: snapshot.Mode, CredentialRevision: snapshot.Revision}}
	checkout := services.NewBizCheckoutService(es, lifecycle, merchants, gateway, logger)
	checkout.SetNewCollectionEnabled(true)
	return portalHTTPFixture{es: es, lifecycle: lifecycle, portal: portal, merchants: merchants, checkout: checkout, gateway: gateway, appID: appID, userID: userID}
}

func TestBizCheckoutPortalHTTPFlow(t *testing.T) {
	f := newPortalHTTPFixture(t)
	es, lifecycle, portal, checkout, gateway, appID, userID := f.es, f.lifecycle, f.portal, f.checkout, f.gateway, f.appID, f.userID
	ctx, logger := context.Background(), zap.NewNop()
	handler := NewBizPortalHandler(BizBillingServices{Portal: portal, Lifecycle: lifecycle}, logger)
	handler.SetCheckoutService(checkout)
	app := fiber.New()
	app.Get("/portal/:token/invoices/:id", handler.GetInvoice)
	app.Post("/portal/:token/invoices/:id/pay", handler.PayInvoice)
	app.Post("/portal/:token/invoices/:id/verify-payment", handler.VerifyInvoicePayment)
	inv := &bizbilling.BizInvoice{ID: uuid.NewString(), AppID: appID, UserID: userID, Currency: "INR", GatewayMode: bizbilling.GatewayTest, Status: bizbilling.InvoiceStatusOpen, InvoiceNumber: "FIXTURE", TotalPaisa: 1000, SubtotalPaisa: 1000, AmountDuePaisa: 1000, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	require.NoError(t, bizbillingrepo.NewStore[bizbilling.BizInvoice](es, "frn_biz_invoices", logger).Create(ctx, inv.ID, inv))
	token, err := portal.CreateToken(ctx, appID, userID, time.Hour)
	require.NoError(t, err)
	request := func(method, path, body string, status int) map[string]any {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		response, err := app.Test(r, -1)
		require.NoError(t, err)
		defer response.Body.Close()
		var result map[string]any
		require.NoError(t, json.NewDecoder(response.Body).Decode(&result))
		require.Equal(t, status, response.StatusCode, result)
		return result
	}
	path := "/portal/" + token.Token + "/invoices/" + inv.ID
	read := request("GET", path, "", 200)
	require.Equal(t, inv.ID, read["data"].(map[string]any)["id"])
	require.Zero(t, gateway.creates, "reading an invoice must not create an order")
	paid := request("POST", path+"/pay", `{"gateway_mode":"live"}`, 200)
	require.Equal(t, "order_fixture", paid["data"].(map[string]any)["gateway_order_id"])
	request("POST", path+"/pay", "", 200)
	require.Equal(t, 1, gateway.creates)
	proof := `{"razorpay_order_id":"order_fixture","razorpay_payment_id":"pay_fixture","razorpay_signature":"fixture-valid"}`
	request("POST", path+"/verify-payment", proof, 400)
	gateway.captured = true
	request("POST", path+"/verify-payment", proof, 200)
	request("POST", path+"/verify-payment", proof, 200)
	final := request("GET", path, "", 200)["data"].(map[string]any)
	require.Equal(t, "paid", final["status"])
	require.Equal(t, float64(1000), final["amount_paid_paisa"])
	require.Equal(t, 1, gateway.creates)
	request("POST", "/portal/invalid/invoices/"+inv.ID+"/pay", "", 401)
}
