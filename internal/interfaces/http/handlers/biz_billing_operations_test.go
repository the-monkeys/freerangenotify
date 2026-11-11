package handlers

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"go.uber.org/zap"
)

func TestBizOperationErrorStatuses(t *testing.T) {
	for _, test := range []struct {
		err    error
		status int
	}{
		{bizbilling.ErrCurrencyMismatch, 409}, {bizbilling.ErrIdempotencyConflict, 409}, {bizbilling.ErrAutomaticCollectionInProgress, 409}, {bizbilling.ErrConflict, 409}, {bizbilling.ErrActiveLegacyWriter, 503}, {bizbilling.ErrReconciliationRequired, 503}, {bizbilling.ErrInvalidMoney, 400}, {bizbilling.ErrMoneyOverflow, 400},
	} {
		handler := NewBizBillingHandler(BizBillingServices{}, zap.NewNop())
		app := fiber.New()
		app.Post("/action", func(c *fiber.Ctx) error { return handler.handleError(c, test.err) })
		res, err := app.Test(httptest.NewRequest("POST", "/action", nil))
		require.NoError(t, err)
		res.Body.Close()
		require.Equal(t, test.status, res.StatusCode, test.err)
	}
}

func TestBizBalanceRequestKeepsOptionalCurrency(t *testing.T) {
	var request bizbilling.AdjustBalanceRequest
	require.NoError(t, json.Unmarshal([]byte(`{"amount_paisa":100,"reason":"top up","currency":"USD"}`), &request))
	encoded, err := json.Marshal(request)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"currency":"USD"`, "currency must not be silently discarded at the API boundary")
}

func TestBizOperationIdempotencyKeyValidation(t *testing.T) {
	for _, test := range []struct {
		key    string
		status int
	}{{"", 200}, {"invoice-request-1", 200}, {strings.Repeat("a", 201), 400}} {
		handler := NewBizBillingHandler(BizBillingServices{}, zap.NewNop())
		app := fiber.New()
		app.Use(func(c *fiber.Ctx) error { c.Locals("app_id", "app"); return c.Next() })
		app.Use(handler.GuardMutations)
		app.Post("/action", func(c *fiber.Ctx) error { return c.SendStatus(200) })
		req := httptest.NewRequest("POST", "/action", nil)
		if test.key != "" {
			req.Header.Set("Idempotency-Key", test.key)
		}
		res, err := app.Test(req)
		require.NoError(t, err)
		res.Body.Close()
		require.Equal(t, test.status, res.StatusCode)
	}
}

func TestBizOperationStatusScopesAndSanitizes(t *testing.T) {
	command := bizbilling.Command{Kind: bizbilling.CommandBalanceAdjustment, Scope: bizbilling.Scope{AppID: "owned", UserID: "customer", Currency: "INR"}, BalanceAdjustment: &bizbilling.BalanceAdjustment{DeltaPaisa: 100, Reason: "SECRET_FIXTURE_INTENT"}}
	intent, err := json.Marshal(command)
	require.NoError(t, err)
	hash := sha256.Sum256(intent)
	op := bizbilling.Operation{SchemaVersion: bizbilling.FinancialOperationSchemaVersion, ID: "operation", AppID: "owned", Scope: command.Scope, Command: command, IntentHash: fmt.Sprintf("%x", hash), State: bizbilling.OperationAccepted}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		json.NewEncoder(w).Encode(map[string]interface{}{"found": true, "_seq_no": 0, "_primary_term": 1, "_source": op})
	}))
	defer server.Close()
	es, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{server.URL}})
	require.NoError(t, err)
	for _, test := range []struct {
		app    string
		status int
	}{{"owned", 200}, {"foreign", 404}, {"", 401}} {
		handler := NewBizBillingHandler(BizBillingServices{}, zap.NewNop())
		handler.SetOperationService(services.NewBizOperationService(es, zap.NewNop()))
		app := fiber.New()
		app.Use(func(c *fiber.Ctx) error {
			if test.app != "" {
				c.Locals("app_id", test.app)
			}
			return c.Next()
		})
		app.Get("/operations/:id", handler.GetBillingOperation)
		res, err := app.Test(httptest.NewRequest("GET", "/operations/operation", nil))
		require.NoError(t, err)
		data, err := io.ReadAll(res.Body)
		res.Body.Close()
		require.NoError(t, err)
		require.Equal(t, test.status, res.StatusCode)
		require.NotContains(t, string(data), "SECRET_FIXTURE_INTENT")
		require.NotContains(t, string(data), "intent_hash")
	}
}
