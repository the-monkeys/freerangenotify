package handlers

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"go.uber.org/zap"
)

func TestBizPreviewAndIssuedDocumentHTTP(t *testing.T) {
	f := newPortalHTTPFixture(t)
	ctx, logger := context.Background(), zap.NewNop()
	users := repository.NewUserRepository(f.es, logger)
	require.NoError(t, users.Create(ctx, &user.User{UserID: f.userID, AppID: f.appID, FullName: "HTTP Customer", Email: "http-customer@example.invalid"}))
	config := services.NewBizConfigService(bizbillingrepo.NewConfigRepo(f.es, logger), logger)
	lifecycle := services.NewBizLifecycleService(bizbillingrepo.NewLifecycleRepo(f.es, logger), bizbillingrepo.NewStores(f.es, logger), nil, config, nil, nil, nil, nil, users, "INR", logger)
	lifecycle.SetOperationService(f.operations)
	lifecycle.SetLifecycleGate(services.NewBizLifecycleGate(bizbillingrepo.NewLifecycleStateRepo(f.es)))
	h := NewBizBillingHandler(BizBillingServices{Lifecycle: lifecycle}, logger)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error { c.Locals("app_id", f.appID); return c.Next() })
	app.Post("/invoices/preview", h.PreviewInvoice)
	app.Post("/invoices", h.CreateInvoice)
	app.Post("/invoices/:id/issue", h.IssueInvoice)
	app.Get("/invoices/:id/document", h.GetInvoiceDocument)
	request := func(method, path, body string, status int) map[string]any {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		response, err := app.Test(req, 30000)
		require.NoError(t, err)
		defer response.Body.Close()
		var payload map[string]any
		require.NoError(t, json.NewDecoder(response.Body).Decode(&payload))
		require.Equal(t, status, response.StatusCode, payload)
		return payload
	}
	body := `{"user_id":"` + f.userID + `","currency":"INR","line_items":[{"description":"Service","quantity":1,"unit_price":1000}]}`
	preview := request("POST", "/invoices/preview", body, 200)["data"].(map[string]any)
	draft := request("POST", "/invoices", body, 201)["data"].(map[string]any)
	id := draft["id"].(string)
	request("POST", "/invoices/"+id+"/issue", "{}", 200)
	document := request("GET", "/invoices/"+id+"/document", "", 200)["data"].(map[string]any)
	require.Equal(t, preview["total_paisa"], document["total_paisa"])
	require.Zero(t, document["amount_paid_paisa"])
	require.Equal(t, "HTTP Customer", document["customer"].(map[string]any)["name"])
	request("GET", "/invoices/foreign/document", "", 404)
}
