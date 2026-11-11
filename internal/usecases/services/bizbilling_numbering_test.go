package services

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/database"
	"github.com/the-monkeys/freerangenotify/internal/testutil"
	"go.uber.org/zap"
)

type numberingReadBarrier struct {
	base     http.RoundTripper
	arrivals atomic.Int32
	ready    chan struct{}
}

func (b *numberingReadBarrier) RoundTrip(r *http.Request) (*http.Response, error) {
	res, err := b.base.RoundTrip(r)
	if err == nil && r.Method == http.MethodGet && strings.Contains(r.URL.Path, "frn_biz_configs/_doc/") {
		if b.arrivals.Add(1) == 2 {
			close(b.ready)
		}
		select {
		case <-b.ready:
		case <-r.Context().Done():
			res.Body.Close()
			return nil, r.Context().Err()
		}
	}
	return res, err
}

func TestBizNumberAllocationConcurrent(t *testing.T) {
	_, es := operationFixture(t, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	res, err := es.Indices.Exists([]string{"frn_biz_configs"})
	require.NoError(t, err)
	exists := res.StatusCode == 200
	res.Body.Close()
	if !exists {
		data, err := json.Marshal((&database.IndexTemplates{}).GetBizConfigsTemplate())
		require.NoError(t, err)
		res, err = es.Indices.Create("frn_biz_configs", es.Indices.Create.WithBody(bytes.NewReader(data)))
		require.NoError(t, err)
		require.False(t, res.IsError())
		res.Body.Close()
	}
	ensureNumberingInvoiceIndex(t, es)
	appID := uuid.NewString()
	repo := bizbillingrepo.NewConfigRepo(es, zap.NewNop())
	require.NoError(t, repo.Upsert(ctx, &bizbilling.BizConfig{AppID: appID, ConfigType: bizbilling.ConfigTypeNumbering, Data: map[string]interface{}{"invoice_prefix": "INV", "next_invoice_number": 1}}))
	// Synchronize the two original GET responses. A subsequent CAS implementation
	// retries against the changed revision; the blind upsert allocates INV1 twice.
	barrier := &numberingReadBarrier{base: http.DefaultTransport, ready: make(chan struct{})}
	manifest := testutil.RequireOwnedBizFixture(t)
	client, err := elasticsearch.NewClient(elasticsearch.Config{Transport: barrier, Addresses: []string{manifest.Services["elasticsearch"].URL}})
	require.NoError(t, err)
	service := NewBizConfigService(bizbillingrepo.NewConfigRepo(client, zap.NewNop()), zap.NewNop())
	numbers := make(chan string, 2)
	failures := make(chan error, 2)
	var workers sync.WaitGroup
	for i := 0; i < 2; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			number, err := service.NextNumber(ctx, appID, NumberKindInvoice)
			numbers <- number
			failures <- err
		}()
	}
	workers.Wait()
	close(numbers)
	close(failures)
	for err := range failures {
		require.NoError(t, err)
	}
	unique := map[string]bool{}
	for number := range numbers {
		unique[number] = true
	}
	require.Len(t, unique, 2, "each accepted document must receive its own number")
}

func TestBizNumberingConfigCannotResetCounter(t *testing.T) {
	_, es := operationFixture(t, nil)
	ctx := context.Background()
	res, err := es.Indices.Exists([]string{"frn_biz_configs"})
	require.NoError(t, err)
	exists := res.StatusCode == 200
	res.Body.Close()
	if !exists {
		data, err := json.Marshal((&database.IndexTemplates{}).GetBizConfigsTemplate())
		require.NoError(t, err)
		res, err = es.Indices.Create("frn_biz_configs", es.Indices.Create.WithBody(bytes.NewReader(data)))
		require.NoError(t, err)
		require.False(t, res.IsError())
		res.Body.Close()
	}
	ensureNumberingInvoiceIndex(t, es)
	for _, mode := range []string{"lowered", "omitted"} {
		t.Run(mode, func(t *testing.T) {
			appID := uuid.NewString()
			repo := bizbillingrepo.NewConfigRepo(es, zap.NewNop())
			service := NewBizConfigService(repo, zap.NewNop())
			require.NoError(t, repo.Upsert(ctx, &bizbilling.BizConfig{AppID: appID, ConfigType: bizbilling.ConfigTypeNumbering, Data: map[string]interface{}{"invoice_prefix": "INV", "next_invoice_number": 1}}))
			first, err := service.NextNumber(ctx, appID, NumberKindInvoice)
			require.NoError(t, err)
			require.Equal(t, "INV-000001", first)
			data := map[string]interface{}{"invoice_prefix": "INV"}
			if mode == "lowered" {
				data["next_invoice_number"] = 1
			}
			err = repo.Upsert(ctx, &bizbilling.BizConfig{AppID: appID, ConfigType: bizbilling.ConfigTypeNumbering, Data: data})
			if mode == "lowered" {
				require.Error(t, err, "allocated document numbers cannot be reused")
			} else {
				require.NoError(t, err)
			}
			next, err := service.NextNumber(ctx, appID, NumberKindInvoice)
			require.NoError(t, err)
			require.Equal(t, "INV-000002", next)
		})
	}
}

func ensureNumberingInvoiceIndex(t *testing.T, es *elasticsearch.Client) {
	t.Helper()
	res, err := es.Indices.Exists([]string{"frn_biz_invoices"})
	require.NoError(t, err)
	exists := res.StatusCode == 200
	res.Body.Close()
	if exists {
		return
	}
	data, err := json.Marshal((&database.IndexTemplates{}).GetBizInvoicesTemplate())
	require.NoError(t, err)
	res, err = es.Indices.Create("frn_biz_invoices", es.Indices.Create.WithBody(bytes.NewReader(data)))
	require.NoError(t, err)
	require.False(t, res.IsError())
	res.Body.Close()
}

func TestBizNumberPrefixCanStartNewSeries(t *testing.T) {
	_, es := operationFixture(t, nil)
	ctx := context.Background()
	ensureNumberingInvoiceIndex(t, es)
	res, err := es.Indices.Exists([]string{"frn_biz_configs"})
	require.NoError(t, err)
	exists := res.StatusCode == 200
	res.Body.Close()
	if !exists {
		data, err := json.Marshal((&database.IndexTemplates{}).GetBizConfigsTemplate())
		require.NoError(t, err)
		res, err = es.Indices.Create("frn_biz_configs", es.Indices.Create.WithBody(bytes.NewReader(data)))
		require.NoError(t, err)
		require.False(t, res.IsError())
		res.Body.Close()
	}
	appID := uuid.NewString()
	repo := bizbillingrepo.NewConfigRepo(es, zap.NewNop())
	service := NewBizConfigService(repo, zap.NewNop())
	update := func(prefix string, next int64) error {
		return repo.Upsert(ctx, &bizbilling.BizConfig{AppID: appID, ConfigType: bizbilling.ConfigTypeNumbering, Data: map[string]interface{}{"invoice_prefix": prefix, "next_invoice_number": next}})
	}
	require.NoError(t, update("INV-2026", 1))
	number, err := service.NextNumber(ctx, appID, NumberKindInvoice)
	require.NoError(t, err)
	require.Equal(t, "INV-2026-000001", number)
	require.NoError(t, update("INV-2027", 1))
	number, err = service.NextNumber(ctx, appID, NumberKindInvoice)
	require.NoError(t, err)
	require.Equal(t, "INV-2027-000001", number)
	require.Error(t, update("INV-2026", 1))
	require.NoError(t, update("INV-2026", 2))
	number, err = service.NextNumber(ctx, appID, NumberKindInvoice)
	require.NoError(t, err)
	require.Equal(t, "INV-2026-000002", number)
}
