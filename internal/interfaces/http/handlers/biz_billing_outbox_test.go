package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/domain/channelconnection"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"go.uber.org/zap"
)

func TestBizOutboxAPIRequiresOwnershipAndHidesSecrets(t *testing.T) {
	row := bizbilling.NotificationDelivery{ID: "delivery", AppID: "owned", Scope: bizbilling.Scope{AppID: "owned", UserID: "customer", Currency: "INR"}, Status: "unknown", FailureCode: "admission_unconfirmed", LeaseOwner: "PRIVATE_LEASE", SensitivePayload: &channelconnection.EncryptedSecret{Ciphertext: "PRIVATE_CIPHER"}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		json.NewEncoder(w).Encode(map[string]interface{}{"found": true, "_seq_no": 0, "_primary_term": 1, "_source": row})
	}))
	defer server.Close()
	es, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{server.URL}})
	require.NoError(t, err)
	for _, tc := range []struct {
		app, method string
		status      int
	}{{"owned", "GET", 200}, {"foreign", "GET", 404}, {"", "GET", 401}, {"owned", "POST", 409}, {"foreign", "POST", 404}} {
		t.Run(tc.app+tc.method, func(t *testing.T) {
			h := NewBizBillingHandler(BizBillingServices{}, zap.NewNop())
			h.SetOutboxService(services.NewBizOutboxService(es, nil, nil, nil, nil, zap.NewNop()))
			app := fiber.New()
			app.Use(func(c *fiber.Ctx) error {
				if tc.app != "" {
					c.Locals("app_id", tc.app)
				}
				return c.Next()
			})
			app.Get("/deliveries/:id", h.GetNotificationDelivery)
			app.Post("/deliveries/:id/retry", h.RetryNotificationDelivery)
			path := "/deliveries/delivery"
			if tc.method == "POST" {
				path += "/retry"
			}
			res, err := app.Test(httptest.NewRequest(tc.method, path, nil))
			require.NoError(t, err)
			body, err := io.ReadAll(res.Body)
			res.Body.Close()
			require.NoError(t, err)
			require.Equal(t, tc.status, res.StatusCode, string(body))
			require.NotContains(t, string(body), "PRIVATE_")
			require.NotContains(t, string(body), "sensitive_payload")
		})
	}
}
