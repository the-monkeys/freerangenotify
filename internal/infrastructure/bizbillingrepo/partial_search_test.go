package bizbillingrepo

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"go.uber.org/zap"
)

func TestBizFindRejectsPartialSearch(t *testing.T) {
	for _, test := range []struct {
		name    string
		timeout bool
		failed  int
	}{{"timeout", true, 0}, {"failed shard", false, 1}} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Elastic-Product", "Elasticsearch")
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"timed_out":%t,"_shards":{"total":2,"successful":1,"failed":%d},"hits":{"total":{"value":1,"relation":"eq"},"hits":[{"_id":"invoice","_source":{"id":"invoice"}}]}}`, test.timeout, test.failed)
			}))
			defer server.Close()
			es, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{server.URL}, DisableRetry: true})
			require.NoError(t, err)
			rows, total, err := NewStore[bizbilling.BizInvoice](es, "frn_biz_invoices", zap.NewNop()).Find(context.Background(), map[string]interface{}{"size": 200})
			require.ErrorIs(t, err, bizbilling.ErrReconciliationRequired)
			require.Nil(t, rows)
			require.Zero(t, total)
		})
	}
}
