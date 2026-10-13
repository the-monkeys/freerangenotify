package repository_test

import (
	"context"
	"encoding/json"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/billingrepo"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"go.uber.org/zap"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCreditCounterUpdatesDoNotWaitForSearchRefresh(t *testing.T) {
	var updateRefresh string
	source := `{"id":"sub","tenant_id":"tenant","status":"active","billing_model":"credits","credits_total":100,"credits_remaining":100,"credits_reserved":1,"current_period_start":"2026-01-01T00:00:00Z","current_period_end":"2030-01-01T00:00:00Z","credits_expire_at":"2030-01-01T00:00:00Z"}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "_search"):
			w.Write([]byte(`{"hits":{"total":{"value":1},"hits":[{"_source":` + source + `}]}}`))
		case strings.Contains(r.URL.Path, "_update"):
			updateRefresh = r.URL.Query().Get("refresh")
			w.Write([]byte(`{"result":"updated","get":{"_source":` + source + `}}`))
		default:
			w.Write([]byte(`{"_source":` + source + `}`))
		}
	}))
	defer server.Close()
	client, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{server.URL}})
	require.NoError(t, err)
	repo := repository.NewSubscriptionRepository(client, zap.NewNop())
	balance := billingrepo.NewSubscriptionCreditBalanceRepo(repo, zap.NewNop())
	out, err := balance.ReserveCredits(context.Background(), "tenant", 1)
	require.NoError(t, err)
	require.Equal(t, int64(1), out.CreditsReserved)
	require.Empty(t, updateRefresh, "wallet source is returned atomically; search refresh is not needed per send")
}
func TestActiveSubscriptionQueryPreservesFractionalTime(t *testing.T) {
	now := time.Date(2026, 10, 11, 12, 0, 0, 500000000, time.UTC)
	var body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		w.Header().Set("Content-Type", "application/json")
		var query map[string]any
		json.NewDecoder(r.Body).Decode(&query)
		data, _ := json.Marshal(query)
		body = string(data)
		w.Write([]byte(`{"hits":{"total":{"value":0},"hits":[]}}`))
	}))
	defer server.Close()
	client, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{server.URL}})
	require.NoError(t, err)
	_, err = repository.NewSubscriptionRepository(client, zap.NewNop()).GetActiveSubscription(context.Background(), "tenant", "", now)
	require.NoError(t, err)
	require.Contains(t, body, now.Format(time.RFC3339Nano), "a renewal at .5 seconds must be discoverable in the same second")
}
