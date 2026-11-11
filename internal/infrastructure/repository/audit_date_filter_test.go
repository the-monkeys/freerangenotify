package repository_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/audit"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"github.com/the-monkeys/freerangenotify/internal/testutil"
	"go.uber.org/zap"
)

func TestAuditDateFilteringKeepsTenantScope(t *testing.T) {
	var queries []map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		var body map[string]interface{}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		queries = append(queries, body)
		if r.URL.Path == "/audit_logs/_count" {
			require.NotContains(t, body, "sort")
			require.NotContains(t, body, "size")
			io.WriteString(w, `{"count":2}`)
		} else {
			io.WriteString(w, `{"hits":{"total":{"value":2},"hits":[]}}`)
		}
	}))
	defer server.Close()
	es, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{server.URL}})
	require.NoError(t, err)
	repo := repository.NewAuditRepository(es, zap.NewNop())
	from := time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 1)
	filter := audit.Filter{AppID: "owned", AppIDs: []string{"owned", "also-owned"}, FromDate: &from, ToDate: &to, ToDateExclusive: true, Limit: 7, Offset: 2}
	_, err = repo.List(context.Background(), filter)
	require.NoError(t, err)
	count, err := repo.Count(context.Background(), filter)
	require.NoError(t, err)
	require.Equal(t, int64(2), count)
	require.Equal(t, queries[0]["query"], queries[1]["query"])
	encoded, err := json.Marshal(queries[0])
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"gte":"2026-11-11T00:00:00Z"`)
	require.Contains(t, string(encoded), `"lt":"2026-11-12T00:00:00Z"`)
	require.Contains(t, string(encoded), `"terms":{"app_id":["owned","also-owned"]}`)
	require.Contains(t, string(encoded), `"term":{"app_id":"owned"}`)
	require.Equal(t, float64(7), queries[0]["size"])
	require.Equal(t, float64(2), queries[0]["from"])
}

func TestAuditDateFilteringElasticsearch(t *testing.T) {
	if testing.Short() || os.Getenv("FRN_BIZ_REQUIRED") != "1" {
		t.Skip("requires owned billing fixture")
	}
	m := testutil.RequireOwnedBizFixture(t)
	es, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{m.Services["elasticsearch"].URL}})
	require.NoError(t, err)
	data, err := json.Marshal((&database.IndexTemplates{}).GetAuditLogsTemplate())
	require.NoError(t, err)
	res, err := es.Indices.Exists([]string{"audit_logs"})
	require.NoError(t, err)
	exists := res.StatusCode == 200
	res.Body.Close()
	if !exists {
		res, err = es.Indices.Create("audit_logs", es.Indices.Create.WithBody(bytes.NewReader(data)))
		require.NoError(t, err)
		require.False(t, res.IsError())
		res.Body.Close()
	}
	repo := repository.NewAuditRepository(es, zap.NewNop())
	ctx := context.Background()
	from := time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, 1)
	appID := uuid.NewString()
	for i, entry := range []struct {
		app string
		at  time.Time
	}{{appID, from}, {appID, to.Add(-time.Millisecond)}, {appID, to}, {"foreign", from}, {appID, from.Add(-time.Millisecond)}} {
		require.NoError(t, repo.Create(ctx, &audit.AuditLog{AuditID: appID + []string{"start", "last", "next", "foreign", "previous"}[i], AppID: entry.app, CreatedAt: entry.at, Action: "create", Resource: "invoice"}))
	}
	res, err = es.Indices.Refresh(es.Indices.Refresh.WithIndex("audit_logs"))
	require.NoError(t, err)
	res.Body.Close()
	filter := audit.Filter{AppIDs: []string{appID}, FromDate: &from, ToDate: &to, ToDateExclusive: true, Limit: 1, Offset: 1}
	count, err := repo.Count(ctx, filter)
	require.NoError(t, err)
	require.Equal(t, int64(2), count)
	logs, err := repo.List(ctx, filter)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	require.Equal(t, appID+"start", logs[0].AuditID)
}
