package integration

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/testutil"
)

// Restores into new, uniquely named indices, never over an existing index.
// The repository is registered by the owned rehearsal runner, not this test.
func TestBizMigrationRehearsalPreservesHistory(t *testing.T) {
	if testing.Short() {
		t.Skip("owned snapshot rehearsal runs explicitly without -short")
	}
	m := testutil.RequireOwnedBizFixture(t)
	repo := os.Getenv("FRN_BIZ_SNAPSHOT_REPO")
	require.Equal(t, "frn-billing-"+m.RunID, repo, "owned snapshot runner is required")
	name := "frn_rehearsal_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	restored := name + "_restored"
	snapshot := "run-" + strings.ReplaceAll(uuid.NewString(), "-", "")
	client := &http.Client{Timeout: 60 * time.Second}
	request := func(method, path string, body any) map[string]any {
		var raw []byte
		if body != nil {
			var err error
			raw, err = json.Marshal(body)
			require.NoError(t, err)
		}
		req, err := http.NewRequest(method, m.Services["elasticsearch"].URL+path, bytes.NewReader(raw))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		res, err := client.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()
		require.Less(t, res.StatusCode, 300, "owned snapshot request %s %s", method, path)
		var data map[string]any
		require.NoError(t, json.NewDecoder(res.Body).Decode(&data))
		return data
	}
	t.Cleanup(func() {
		// A valid manifest alone never authorizes deletion of another index.
		testutil.RequireOwnedBizFixture(t)
		for _, suffix := range []string{"invoices", "payments", "journal", "subscriptions", "users"} {
			for _, index := range []string{name + "_" + suffix, restored + "_" + suffix} {
				require.True(t, strings.HasPrefix(index, "frn_rehearsal_"))
				req, _ := http.NewRequest("DELETE", m.Services["elasticsearch"].URL+"/"+index, nil)
				res, err := client.Do(req)
				if err == nil {
					res.Body.Close()
				}
			}
		}
		req, _ := http.NewRequest("DELETE", m.Services["elasticsearch"].URL+"/_snapshot/"+repo+"/"+snapshot, nil)
		res, err := client.Do(req)
		if err == nil {
			res.Body.Close()
		}
	})
	// Mixed old/new-shaped financial sources retain IDs, numbers, explicit
	// currencies, integer amounts and unknown extension fields through restore.
	sources := map[string]map[string]any{
		"invoices": {
			"legacy-invoice": map[string]any{"id": "legacy-invoice", "invoice_number": "INV-000007", "currency": "USD", "total_paisa": 1180, "amount_paid_paisa": 400, "amount_due_paisa": 780, "extension": map[string]any{"keep": []any{"unknown", true}}},
			"v2-invoice":     map[string]any{"id": "v2-invoice", "invoice_number": "INV-000008", "currency": "INR", "total_paisa": 3500000, "amount_paid_paisa": 3500000, "amount_due_paisa": 0, "billing_sequence": 8},
		},
		"payments":      {"v2-payment": map[string]any{"id": "v2-payment", "currency": "INR", "amount_paisa": 3500000, "status": "success", "billing_sequence": 9, "future_extension": "preserve"}},
		"journal":       {"opening": map[string]any{"id": "opening", "sequence": 1, "currency": "INR", "available_paisa": 400}, "settlement": map[string]any{"id": "settlement", "sequence": 9, "kind": "payment_recorded", "invoice_id": "v2-invoice", "payment_id": "v2-payment", "currency": "INR", "amount_paisa": 3500000}},
		"subscriptions": {"legacy-subscription": map[string]any{"id": "legacy-subscription", "currency": "USD", "current_period_start": "2026-01-31T00:00:00Z", "current_period_end": "2026-02-28T00:00:00Z", "status": "active"}},
		"users":         {"explicit": map[string]any{"user_id": "explicit", "balance_paisa": 400, "biz_balance_currency": "INR"}, "ambiguous": map[string]any{"user_id": "ambiguous", "balance_paisa": 100, "future_extension": true}},
	}
	indices := []string{}
	for suffix, documents := range sources {
		index := name + "_" + suffix
		indices = append(indices, index)
		request("PUT", "/"+index, map[string]any{"settings": map[string]any{"number_of_shards": 1, "number_of_replicas": 0}, "mappings": map[string]any{"dynamic": false}})
		for id, source := range documents {
			request("PUT", "/"+index+"/_doc/"+id+"?refresh=true", source)
		}
	}
	hashes := func(prefix string) map[string]string {
		out := map[string]string{}
		for suffix, documents := range sources {
			index := prefix + "_" + suffix
			require.EqualValues(t, len(documents), request("GET", "/"+index+"/_count", nil)["count"])
			for id := range documents {
				data := request("GET", "/"+index+"/_doc/"+id, nil)
				raw, err := json.Marshal(data["_source"])
				require.NoError(t, err)
				out[suffix+"/"+id] = fmt.Sprintf("%x", sha256.Sum256(raw))
			}
		}
		return out
	}
	before := hashes(name)
	result := request("PUT", "/_snapshot/"+repo+"/"+snapshot+"?wait_for_completion=true", map[string]any{"indices": strings.Join(indices, ","), "include_global_state": false})
	require.Equal(t, "SUCCESS", result["snapshot"].(map[string]any)["state"])
	request("POST", "/_snapshot/"+repo+"/"+snapshot+"/_restore?wait_for_completion=true", map[string]any{"indices": strings.Join(indices, ","), "include_global_state": false, "rename_pattern": "^" + name + "_(.*)$", "rename_replacement": restored + "_$1"})
	require.Equal(t, before, hashes(restored))
	require.Equal(t, before, hashes(name), "restore must leave original records untouched")
}
