package handlers

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"go.uber.org/zap"
)

func TestBizAutomaticAuthorizationHTTPRequiresScopedCustomerAndKey(t *testing.T) {
	f := newPortalHTTPFixture(t)
	ctx := context.Background()
	// Creating the collection preference must not call a gateway. Deliberately
	// omit the factory so a misplaced external call fails this contract.
	service := services.NewBizRecurringService(f.es, f.lifecycle, f.merchants, nil, zap.NewNop())
	subs := bizbillingrepo.NewStore[bizbilling.BizSubscription](f.es, "frn_biz_subscriptions", zap.NewNop())
	res, err := f.es.Indices.Exists([]string{"frn_biz_subscriptions"})
	require.NoError(t, err)
	exists := res.StatusCode == 200
	res.Body.Close()
	if !exists {
		raw, err := json.Marshal((&database.IndexTemplates{}).GetBizSubscriptionsTemplate())
		require.NoError(t, err)
		res, err = f.es.Indices.Create("frn_biz_subscriptions", f.es.Indices.Create.WithBody(strings.NewReader(string(raw))))
		require.NoError(t, err)
		require.False(t, res.IsError())
		res.Body.Close()
	}
	sub := &bizbilling.BizSubscription{ID: uuid.NewString(), AppID: f.appID, UserID: f.userID, Currency: "INR", GatewayMode: bizbilling.GatewayTest, AutomaticCycles: 12, Status: bizbilling.SubscriptionStatusActive}
	require.NoError(t, subs.Create(ctx, sub.ID, sub))
	admin := NewBizBillingHandler(BizBillingServices{Lifecycle: f.lifecycle}, zap.NewNop())
	admin.SetRecurringService(service)
	portal := NewBizPortalHandler(BizBillingServices{Lifecycle: f.lifecycle, Portal: f.portal}, zap.NewNop())
	portal.SetRecurringService(service)
	app := fiber.New()
	app.Post("/subscriptions/:id/collection-mode", func(c *fiber.Ctx) error { c.Locals("app_id", f.appID); return admin.ChangeSubscriptionCollection(c) })
	app.Post("/portal/:token/subscriptions/:id/authorize", portal.AuthorizeSubscription)
	request := func(path, key, body string, status int) map[string]any {
		req := httptest.NewRequest("POST", path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", key)
		response, err := app.Test(req, 30000)
		require.NoError(t, err)
		defer response.Body.Close()
		var payload map[string]any
		require.NoError(t, json.NewDecoder(response.Body).Decode(&payload))
		require.Equal(t, status, response.StatusCode, payload)
		return payload
	}
	request("/subscriptions/"+sub.ID+"/collection-mode", "", `{"collection_mode":"automatic"}`, 400)
	request("/subscriptions/"+sub.ID+"/collection-mode", "choose-automatic", `{"collection_mode":"automatic"}`, 200)
	stored, err := subs.Get(ctx, sub.ID)
	require.NoError(t, err)
	require.Equal(t, bizbilling.CollectionAutomatic, stored.CollectionMode)
	require.Empty(t, stored.GatewaySubscriptionID)
	token, err := f.portal.CreateToken(ctx, f.appID, uuid.NewString(), time.Hour)
	require.NoError(t, err)
	request("/portal/invalid/subscriptions/"+sub.ID+"/authorize", "authorize", `{"mode":"test"}`, 401)
	request("/portal/"+token.Token+"/subscriptions/"+sub.ID+"/authorize", "authorize", `{"mode":"test","user_id":"`+sub.UserID+`"}`, 404)
}
