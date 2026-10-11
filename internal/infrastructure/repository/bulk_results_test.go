package repository_test

import (
	"context"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"go.uber.org/zap"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBulkCreateDetectsItemFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"errors":true,"items":[{"index":{"_id":"a","status":201}},{"index":{"_id":"b","status":400,"error":{"type":"mapper_parsing_exception","reason":"rejected"}}}]}`))
	}))
	defer server.Close()
	client, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{server.URL}})
	require.NoError(t, err)
	repo := repository.NewBaseRepository(client, "fixture", zap.NewNop())
	require.Error(t, repo.BulkCreate(context.Background(), map[string]any{"a": map[string]string{"name": "A"}, "b": map[string]string{"name": "B"}}))
}
