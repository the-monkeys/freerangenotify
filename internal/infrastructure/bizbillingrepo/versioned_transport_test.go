package bizbillingrepo

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"go.uber.org/zap"
)

func transportCASStore(t *testing.T, handler http.HandlerFunc) *Store[casFixtureDoc] {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	es, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{server.URL}, DisableRetry: true})
	require.NoError(t, err)
	return NewStore[casFixtureDoc](es, "fixture", zap.NewNop())
}

func TestBizCASPreservesUnknownFields(t *testing.T) {
	var written map[string]json.RawMessage
	store := transportCASStore(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			io.WriteString(w, `{"found":true,"_seq_no":7,"_primary_term":2,"_source":{"id":"doc","value":1,"name":"remove","future_field":"retained","large":9007199254740993,"nested":{"label":"old","future_nested":"retained"}}}`)
			return
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&written))
		io.WriteString(w, `{"result":"updated"}`)
	})
	_, err := store.Update(context.Background(), "doc", func(d *casFixtureDoc) error {
		d.Value++
		d.Name = ""
		d.Nested.Label = "new"
		return nil
	})
	require.NoError(t, err)
	require.JSONEq(t, `"retained"`, string(written["future_field"]))
	require.Equal(t, "9007199254740993", string(written["large"]))
	require.NotContains(t, written, "name")
	require.JSONEq(t, `{"label":"new","future_nested":"retained"}`, string(written["nested"]))
}

func TestBizCASBoundsRetries(t *testing.T) {
	var writes atomic.Int32
	store := transportCASStore(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			io.WriteString(w, `{"found":true,"_seq_no":7,"_primary_term":2,"_source":{"id":"doc","value":1}}`)
			return
		}
		writes.Add(1)
		w.WriteHeader(http.StatusConflict)
		io.WriteString(w, `{"error":{"type":"version_conflict_engine_exception"}}`)
	})
	_, err := store.Update(context.Background(), "doc", func(d *casFixtureDoc) error { d.Value++; return nil })
	require.ErrorIs(t, err, bizbilling.ErrConflict)
	require.Equal(t, int32(8), writes.Load())
}

func TestBizLifecycleMissingIndexFailsClosed(t *testing.T) {
	store := transportCASStore(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `{"error":{"type":"index_not_found_exception"},"status":404}`)
	})
	_, err := NewLifecycleStateRepo(store.es).Load(context.Background(), "app")
	require.Error(t, err, "an unavailable lifecycle index must not reset an active app to legacy")
}

func TestBizCASWriteGuardsRevision(t *testing.T) {
	store := transportCASStore(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			io.WriteString(w, `{"found":true,"_seq_no":7,"_primary_term":2,"_source":{"id":"doc","value":1}}`)
			return
		}
		require.Equal(t, "7", r.URL.Query().Get("if_seq_no"))
		require.Equal(t, "2", r.URL.Query().Get("if_primary_term"))
		io.WriteString(w, `{"result":"updated"}`)
	})
	doc, version, err := store.GetVersioned(context.Background(), "doc")
	require.NoError(t, err)
	doc.Value++
	require.NoError(t, store.ReplaceVersioned(context.Background(), "doc", doc, version))
	require.Error(t, store.ReplaceVersioned(context.Background(), "another", doc, version))
	require.Error(t, store.ReplaceVersioned(context.Background(), "doc", doc, Version{SeqNo: 7, PrimaryTerm: 2}))
}

func TestBizCASDoesNotRetryUnknownWriteOutcome(t *testing.T) {
	var writes atomic.Int32
	store := transportCASStore(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			io.WriteString(w, `{"found":true,"_seq_no":7,"_primary_term":2,"_source":{"id":"doc","value":1}}`)
			return
		}
		writes.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		connection, _, err := w.(http.Hijacker).Hijack()
		if err == nil {
			connection.Close()
		}
	})
	_, err := store.Update(context.Background(), "doc", func(d *casFixtureDoc) error { d.Value++; return nil })
	require.ErrorContains(t, err, "outcome unknown")
	require.Equal(t, int32(1), writes.Load(), "only a confirmed 409 can repeat the mutation")
}

func TestBizSearchKeepsMinorUnitPrecision(t *testing.T) {
	store := transportCASStore(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"hits":{"total":{"value":1},"hits":[{"_source":{"id":"doc","value":9007199254740993}}]}}`)
	})
	docs, total, err := store.Find(context.Background(), map[string]interface{}{"query": map[string]interface{}{"match_all": map[string]interface{}{}}})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, docs, 1)
	require.Equal(t, int64(9007199254740993), int64(docs[0].Value))
}

func TestBizLifecycleListsKeepMinorUnitPrecision(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		io.WriteString(w, `{"hits":{"total":{"value":1},"hits":[{"_source":{"id":"doc","app_id":"app","total_paisa":9007199254740993,"amount_paisa":9007199254740993,"quantity":9007199254740993}}]}}`)
	}))
	defer server.Close()
	es, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{server.URL}, DisableRetry: true})
	require.NoError(t, err)
	repo := NewLifecycleRepo(es, zap.NewNop())
	invoices, _, err := repo.ListInvoices(context.Background(), bizbilling.InvoiceFilter{AppID: "app"})
	require.NoError(t, err)
	require.Len(t, invoices, 1)
	require.Equal(t, int64(9007199254740993), invoices[0].TotalPaisa)
	payments, _, err := repo.ListPayments(context.Background(), bizbilling.PaymentFilter{AppID: "app"})
	require.NoError(t, err)
	require.Len(t, payments, 1)
	require.Equal(t, int64(9007199254740993), payments[0].AmountPaisa)
}
