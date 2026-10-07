package billingrepo

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"go.uber.org/zap"
)

// The search is deliberately stale. Only the atomic update response is valid
// evidence of the rejected admission; storage errors must never look depleted.
func TestReserveAtomicObservation(t *testing.T) {
	for _, tc := range []struct {
		name      string
		status    int
		body      string
		wantFunds bool
	}{
		{"contention", 200, `{"result":"noop","get":{"_source":{"id":"sub-1","tenant_id":"t","credits_total":1500,"credits_remaining":1500,"credits_reserved":800}}}`, true},
		{"conflict", 409, `{"error":"version_conflict_engine_exception"}`, false},
		{"database", 500, `{"error":"insufficient credits is just an unrelated storage message"}`, false},
		{"malformed", 200, `{"result":"noop"}`, false},
		{"missing_counters", 200, `{"result":"noop","get":{"_source":{"id":"sub-1","tenant_id":"t"}}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Elastic-Product", "Elasticsearch")
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/subscriptions/_search" {
					fmt.Fprint(w, `{"hits":{"total":{"value":1},"hits":[{"_source":{"id":"sub-1","tenant_id":"t","credits_remaining":1500,"credits_reserved":0}}]}}`)
					return
				}
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer srv.Close()
			es, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{srv.URL}, DisableRetry: true})
			if err != nil {
				t.Fatal(err)
			}
			repo := NewSubscriptionCreditBalanceRepo(repository.NewSubscriptionRepository(es, zap.NewNop()), zap.NewNop())
			b, err := repo.ReserveCredits(context.Background(), "t", 800)
			if tc.wantFunds {
				if !errors.Is(err, billing.ErrInsufficientCredits) {
					t.Fatalf("expected capacity rejection, balance=%+v error=%v", b, err)
				}
			} else if err == nil || errors.Is(err, billing.ErrInsufficientCredits) {
				t.Fatalf("storage/protocol failure converted to funds: balance=%+v error=%v", b, err)
			}
		})
	}
}
