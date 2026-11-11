package repository_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/user"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/repository"
	"go.uber.org/zap"
)

func TestUserBillingUpdateClearsZeroBalance(t *testing.T) {
	var body map[string]json.RawMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		io.WriteString(w, `{"result":"updated"}`)
	}))
	defer server.Close()
	es, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{server.URL}})
	require.NoError(t, err)
	require.NoError(t, repository.NewUserRepository(es, zap.NewNop()).Update(context.Background(), &user.User{UserID: "user", AppID: "app", BalancePaisa: 0}))
	var doc map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body["doc"], &doc))
	require.Contains(t, doc, "balance_paisa", "zero must clear old stored credit, not disappear via omitempty")
	require.JSONEq(t, `0`, string(doc["balance_paisa"]))
}

func TestUserBillingProfileUpdateCannotOverwriteMoney(t *testing.T) {
	var body map[string]json.RawMessage
	released := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		io.WriteString(w, `{"result":"updated"}`)
	}))
	defer server.Close()
	es, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{server.URL}})
	require.NoError(t, err)
	repo := repository.NewUserRepository(es, zap.NewNop()).(*repository.UserRepository)
	repo.SetBillingWriteGuard(func(_ context.Context, app string) (bool, func(), error) {
		require.Equal(t, "app", app)
		return true, func() { released = true }, nil
	})
	require.NoError(t, repo.Update(context.Background(), &user.User{UserID: "user", AppID: "app", Email: "updated@example.invalid", BalancePaisa: 0}))
	var doc map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body["doc"], &doc))
	require.NotContains(t, doc, "balance_paisa", "stale profile values must not replace journal-backed credit")
	require.JSONEq(t, `"updated@example.invalid"`, string(doc["email"]))
	require.True(t, released)
}

func TestUserBillingCreateAcceptsVerifiedReceipt(t *testing.T) {
	var body map[string]json.RawMessage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		require.Equal(t, "create", r.URL.Query().Get("op_type"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		io.WriteString(w, `{"result":"created"}`)
	}))
	defer server.Close()
	es, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{server.URL}})
	require.NoError(t, err)
	repo := repository.NewUserRepository(es, zap.NewNop()).(*repository.UserRepository)
	repo.SetBillingWriteGuard(func(context.Context, string) (bool, func(), error) { return true, func() {}, nil })
	verified := false
	repo.SetBillingBalanceAuthorizer(func(_ context.Context, profile *user.User) error {
		require.Equal(t, int64(1), profile.BizBalanceSequence)
		require.Equal(t, "INR", profile.BizBalanceCurrency)
		verified = true
		return nil
	})
	require.NoError(t, repo.Create(context.Background(), &user.User{UserID: "user", AppID: "app", Email: "fixture@example.invalid", BalancePaisa: 100, BizBalanceSequence: 1, BizBalanceCurrency: "INR"}))
	require.True(t, verified)
	require.JSONEq(t, `100`, string(body["balance_paisa"]))
	require.JSONEq(t, `1`, string(body["biz_balance_sequence"]))
}

func TestUserBillingBulkCreatesDoNotReplaceProfiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		first := strings.SplitN(string(body), "\n", 2)[0]
		var action map[string]json.RawMessage
		require.NoError(t, json.Unmarshal([]byte(first), &action))
		require.Contains(t, action, "create", "v2 imports cannot replace existing customer profiles and financial mirrors")
		io.WriteString(w, `{"items":[{"create":{"_id":"user","status":409,"error":{"type":"version_conflict_engine_exception"}}}]}`)
	}))
	defer server.Close()
	es, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{server.URL}})
	require.NoError(t, err)
	repo := repository.NewUserRepository(es, zap.NewNop()).(*repository.UserRepository)
	repo.SetBillingWriteGuard(func(context.Context, string) (bool, func(), error) { return true, func() {}, nil })
	outcomes, err := repo.BulkCreateResults(context.Background(), []*user.User{{UserID: "user", AppID: "app", Email: "fixture@example.invalid"}})
	require.NoError(t, err)
	require.Len(t, outcomes, 1)
	require.Error(t, outcomes[0])
}
