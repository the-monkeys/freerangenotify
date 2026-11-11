package handlers

import (
	"context"
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"go.uber.org/zap"
	"io"
	"net/http/httptest"
	"os"
	"testing"
)

func TestBizLegacyFinancialDocumentsRoundTrip(t *testing.T) {
	raw, err := os.ReadFile("../../../../tests/fixtures/biz-billing/legacy-contracts.json")
	require.NoError(t, err)
	var fixtures map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &fixtures))
	for name, model := range map[string]interface{}{"invoice": &bizbilling.BizInvoice{}, "subscription": &bizbilling.BizSubscription{}} {
		require.NoError(t, json.Unmarshal(fixtures[name], model))
		roundTrip, err := json.Marshal(model)
		require.NoError(t, err)
		var want, got map[string]interface{}
		require.NoError(t, json.Unmarshal(fixtures[name], &want))
		require.NoError(t, json.Unmarshal(roundTrip, &got))
		for key, value := range want {
			require.Equal(t, value, got[key], name+" "+key)
		}
		require.NotEqual(t, "automatic", got["collection_mode"])
	}
}

func TestBizLegacyGuardPreservesRequestContext(t *testing.T) {
	type key struct{}
	app := fiber.New()
	h := NewBizBillingHandler(BizBillingServices{}, zap.NewNop())
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("app_id", "fixture-app")
		c.SetUserContext(context.WithValue(c.UserContext(), key{}, "kept"))
		return c.Next()
	})
	app.Post("/biz", h.GuardMutations, func(c *fiber.Ctx) error { v, _ := c.UserContext().Value(key{}).(string); return c.SendString(v) })
	res, err := app.Test(httptest.NewRequest("POST", "/biz", nil))
	require.NoError(t, err)
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	require.Equal(t, "kept", string(body))
}
