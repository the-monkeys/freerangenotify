package bizbillingrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/testutil"
	"go.uber.org/zap"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

type casFixtureDoc struct {
	ID     string `json:"id"`
	Value  int    `json:"value"`
	Name   string `json:"name,omitempty"`
	Nested struct {
		Label string `json:"label,omitempty"`
	} `json:"nested"`
}

func ownedCASStore(t *testing.T) (*Store[casFixtureDoc], *elasticsearch.Client, string) {
	t.Helper()
	if testing.Short() || os.Getenv("FRN_BIZ_REQUIRED") != "1" {
		t.Skip("run using the owned billing fixture runner")
	}
	m := testutil.RequireOwnedBizFixture(t)
	es, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{m.Services["elasticsearch"].URL}})
	require.NoError(t, err)
	index := "frn_biz_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	res, err := es.Indices.Create(index, es.Indices.Create.WithBody(strings.NewReader(`{"settings":{"number_of_shards":1,"number_of_replicas":0}}`)))
	require.NoError(t, err)
	require.False(t, res.IsError())
	res.Body.Close()
	t.Cleanup(func() {
		testutil.RequireOwnedBizFixture(t)
		res, err := es.Indices.Delete([]string{index})
		if err == nil {
			res.Body.Close()
		}
	})
	return NewStore[casFixtureDoc](es, index, zap.NewNop()), es, index
}
func TestBizCreateDoesNotOverwrite(t *testing.T) {
	store, _, _ := ownedCASStore(t)
	ctx := context.Background()
	require.NoError(t, store.Create(ctx, "doc", &casFixtureDoc{ID: "doc", Value: 445}))
	require.ErrorIs(t, store.Create(ctx, "doc", &casFixtureDoc{ID: "doc", Value: 0}), bizbilling.ErrConflict)
	got, err := store.Get(ctx, "doc")
	require.NoError(t, err)
	require.Equal(t, 445, got.Value)
}
func TestBizCASNoLostUpdate(t *testing.T) {
	store, _, _ := ownedCASStore(t)
	ctx := context.Background()
	require.NoError(t, store.Create(ctx, "doc", &casFixtureDoc{ID: "doc"}))
	var calls atomic.Int32
	var arrived sync.WaitGroup
	arrived.Add(2)
	var workers sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			_, err := store.Update(ctx, "doc", func(d *casFixtureDoc) error {
				if calls.Add(1) <= 2 {
					arrived.Done()
					arrived.Wait()
				}
				d.Value++
				return nil
			})
			errs <- err
		}()
	}
	workers.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	got, err := store.Get(ctx, "doc")
	require.NoError(t, err)
	require.Equal(t, 2, got.Value, fmt.Sprintf("lost update after %d mutations", calls.Load()))
}

func TestBizCASRejectsStaleRevision(t *testing.T) {
	store, _, _ := ownedCASStore(t)
	ctx := context.Background()
	require.NoError(t, store.Create(ctx, "doc", &casFixtureDoc{ID: "doc", Value: 1}))
	doc, version, err := store.GetVersioned(ctx, "doc")
	require.NoError(t, err)
	doc.Value = 2
	wrongTerm := version
	wrongTerm.PrimaryTerm++
	require.ErrorIs(t, store.ReplaceVersioned(ctx, "doc", doc, wrongTerm), bizbilling.ErrConflict)
	require.NoError(t, store.ReplaceVersioned(ctx, "doc", doc, version))
	doc.Value = 3
	require.ErrorIs(t, store.ReplaceVersioned(ctx, "doc", doc, version), bizbilling.ErrConflict)
	got, err := store.Get(ctx, "doc")
	require.NoError(t, err)
	require.Equal(t, 2, got.Value)
}

func TestBizCASPreservesUnknownFieldsElasticsearch(t *testing.T) {
	store, es, index := ownedCASStore(t)
	ctx := context.Background()
	res, err := es.Index(index, strings.NewReader(`{"id":"doc","value":1,"name":"remove","future_field":"retained","large":9007199254740993,"nested":{"label":"old","future_nested":"retained"}}`), es.Index.WithDocumentID("doc"))
	require.NoError(t, err)
	require.False(t, res.IsError())
	res.Body.Close()
	_, err = store.Update(ctx, "doc", func(d *casFixtureDoc) error { d.Value++; d.Name = ""; d.Nested.Label = "new"; return nil })
	require.NoError(t, err)
	res, err = es.Get(index, "doc")
	require.NoError(t, err)
	defer res.Body.Close()
	var result struct {
		Source map[string]json.RawMessage `json:"_source"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&result))
	require.JSONEq(t, `"retained"`, string(result.Source["future_field"]))
	require.Equal(t, "9007199254740993", string(result.Source["large"]))
	require.NotContains(t, result.Source, "name")
	require.JSONEq(t, `{"label":"new","future_nested":"retained"}`, string(result.Source["nested"]))
}
