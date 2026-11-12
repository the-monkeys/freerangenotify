package database

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestBizGatewayMappingUpgradeIsAdditive(t *testing.T) {
	for _, reject := range []bool{false, true} {
		t.Run(map[bool]string{false: "adds fields", true: "fails closed"}[reject], func(t *testing.T) {
			writes := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Elastic-Product", "Elasticsearch")
				if r.Method == http.MethodHead {
					return
				}
				require.Equal(t, http.MethodPut, r.Method)
				require.Equal(t, "/frn_biz_gateway_effects/_mapping", r.URL.Path)
				var body map[string]any
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				require.Len(t, body, 1, "only additive properties, never settings or documents")
				properties := body["properties"].(map[string]any)
				require.Equal(t, "keyword", properties["gateway_payment_id"].(map[string]any)["type"])
				writes++
				if reject {
					w.WriteHeader(http.StatusBadRequest)
				}
				_, _ = w.Write([]byte(`{"acknowledged":true}`))
			}))
			defer server.Close()
			es, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{server.URL}, DisableRetry: true})
			require.NoError(t, err)
			manager := NewIndexManager(&ElasticsearchClient{Client: es}, zap.NewNop(), true)
			success, _ := manager.createIndex(context.Background(), "frn_biz_gateway_effects", (&IndexTemplates{}).GetBizGatewayEffectsTemplate())
			require.Equal(t, !reject, success)
			require.Equal(t, 1, writes)
			success, _ = manager.createIndex(context.Background(), "applications", (&IndexTemplates{}).GetApplicationsTemplate())
			require.True(t, success)
			require.Equal(t, 1, writes, "existing customer data indices remain untouched")
		})
	}
}
