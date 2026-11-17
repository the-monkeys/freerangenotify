package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/infrastructure/bizbillingrepo"
	"github.com/the-monkeys/freerangenotify/internal/testutil"
	"github.com/the-monkeys/freerangenotify/internal/usecases/services"
	"go.uber.org/zap"
)

func TestBizBillingRequiredSuiteCannotSkip(t *testing.T) {
	child := exec.Command(os.Args[0], "-test.run=^TestBizBillingV2E2E$", "-test.v")
	child.Env = append(os.Environ(), "FRN_BIZ_REQUIRED=1", "FREERANGE_FEATURES_BIZ_BILLING_ENABLED=false", "FRN_BIZ_FIXTURE_MANIFEST=")
	out, err := child.CombinedOutput()
	require.Error(t, err, "misconfigured required suite must fail")
	require.Contains(t, string(out), "FAIL")
	require.NotContains(t, string(out), "SKIP")
}

// This suite creates app-unique records only. It never clears indices, deletes
// historical data, calls a payment vendor or requires real gateway credentials.
func TestBizBillingV2E2E(t *testing.T) {
	if testing.Short() {
		t.Skip("real owned HTTP/ES/Redis suite; mandatory verifier runs without -short")
	}
	require.Equal(t, "1", os.Getenv("FRN_BIZ_REQUIRED"), "required owned billing fixture")
	require.Equal(t, "true", os.Getenv("FREERANGE_FEATURES_BIZ_BILLING_ENABLED"), "billing must be enabled; never skip")
	m := testutil.RequireOwnedBizFixture(t)
	require.Equal(t, m.APIURL, os.Getenv("INTEGRATION_BASE_URL"))
	admin := os.Getenv("INTEGRATION_ADMIN_TOKEN")
	require.NotEmpty(t, admin)
	client := &http.Client{Timeout: 30 * time.Second}
	call := func(method, path, token, key string, body any, status int) map[string]any {
		var raw []byte
		if body != nil {
			var err error
			raw, err = json.Marshal(body)
			require.NoError(t, err)
		}
		req, err := http.NewRequest(method, m.APIURL+"/v1"+path, bytes.NewReader(raw))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		res, err := client.Do(req)
		require.NoError(t, err)
		defer res.Body.Close()
		require.Equal(t, status, res.StatusCode, "%s %s", method, strings.Split(path, "?")[0])
		var out map[string]any
		require.NoError(t, json.NewDecoder(res.Body).Decode(&out))
		return out
	}
	data := func(value map[string]any) map[string]any {
		out, ok := value["data"].(map[string]any)
		require.True(t, ok)
		return out
	}
	createApp := func(name string) (string, string) {
		a := data(call("POST", "/apps", admin, "", map[string]any{"app_name": name + " " + uuid.NewString()}, 201))
		return a["app_id"].(string), a["api_key"].(string)
	}
	app, key := createApp("Required v2 billing fixture")
	_, legacyKey := createApp("Required legacy billing fixture")
	legacyBefore := data(call("GET", "/biz/lifecycle", legacyKey, "", nil, 200))
	require.EqualValues(t, 1, legacyBefore["lifecycle_version"])
	customer := data(call("POST", "/users", key, "", map[string]any{"email": uuid.NewString() + "@example.invalid", "full_name": "Required fixture customer"}, 201))
	userID := customer["user_id"].(string)
	// Exercise the actual reviewed upgrade, without letting an HTTP caller attest
	// writer readiness. Only this synthetic application is selected.
	es, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{m.Services["elasticsearch"].URL}})
	require.NoError(t, err)
	ops := services.NewBizOperationService(es, zap.NewNop())
	gate := services.NewBizLifecycleGate(bizbillingrepo.NewLifecycleStateRepo(es))
	ops.SetLifecycleGate(gate)
	migration := services.NewBizMigrationService(es, gate, ops, zap.NewNop())
	migration.SetNewCollectionEnabled(true)
	ctx := context.Background()
	dry, err := migration.Prepare(ctx, app, services.MigrationOptions{DryRun: true, PrimaryCurrency: "INR"})
	require.NoError(t, err)
	require.True(t, dry.Ready)
	reviewed, err := migration.Prepare(ctx, app, services.MigrationOptions{PrimaryCurrency: "INR", ExpectedReportID: dry.ID, WritersUpgraded: true})
	require.NoError(t, err)
	require.NoError(t, migration.Activate(ctx, app, reviewed.ID))
	active := data(call("GET", "/biz/lifecycle", key, "", nil, 200))
	require.EqualValues(t, 2, active["lifecycle_version"])
	product := data(call("POST", "/biz/products", key, "", map[string]any{"name": "Fixture service", "tax_rate": 18}, 201))
	plan := data(call("POST", "/biz/plans", key, "", map[string]any{"product_id": product["id"], "name": "INR monthly", "currency": "INR", "amount_paisa": 1000, "billing_cycle": "monthly", "pricing_model": "flat"}, 201))
	call("POST", "/biz/subscriptions", key, uuid.NewString(), map[string]any{"user_id": userID, "plan_id": plan["id"], "collection_mode": "invoice", "trial_days": 7, "quantity": 1}, 201)
	invoice := data(call("POST", "/biz/invoices", key, uuid.NewString(), map[string]any{"user_id": userID, "currency": "INR", "line_items": []any{map[string]any{"description": "Fixture service", "quantity": 1, "unit_price": 1000, "tax_exempt": true}}}, 201))
	id := invoice["id"].(string)
	issued := data(call("POST", "/biz/invoices/"+id+"/issue", key, uuid.NewString(), map[string]any{}, 200))
	require.EqualValues(t, 1000, issued["amount_due_paisa"])
	paymentKey := uuid.NewString()
	paymentBody := map[string]any{"method": "cash", "amount_paisa": 1000, "reference": "FIXTURE-STAFF-CONFIRMED"}
	payment := data(call("POST", "/biz/invoices/"+id+"/payments", key, paymentKey, paymentBody, 201))
	replay := data(call("POST", "/biz/invoices/"+id+"/payments", key, paymentKey, paymentBody, 201))
	require.Equal(t, payment["id"], replay["id"])
	paid := data(call("GET", "/biz/invoices/"+id, key, "", nil, 200))
	require.EqualValues(t, 1000, paid["amount_paid_paisa"])
	require.EqualValues(t, 0, paid["amount_due_paisa"])
	call("GET", "/biz/invoices/"+id, legacyKey, "", nil, 404)
	call("POST", "/biz/payments/"+payment["id"].(string)+"/refund", key, uuid.NewString(), map[string]any{"amount_paisa": 200, "reason": "Fixture cash returned"}, 200)
	afterRefund := data(call("GET", "/biz/invoices/"+id, key, "", nil, 200))
	require.EqualValues(t, 1000, afterRefund["amount_paid_paisa"], "refund preserves received history")
	call("GET", "/biz/users/"+userID+"/statement", key, "", nil, 200)
	call("GET", "/biz/notification-deliveries", key, "", nil, 200)
	link := data(call("POST", "/biz/users/"+userID+"/portal-link", key, "", map[string]any{}, 201))
	portalURL, ok := link["url"].(string)
	require.True(t, ok)
	pieces := strings.Split(strings.TrimRight(portalURL, "/"), "/")
	token := pieces[len(pieces)-1]
	req, err := http.NewRequest("GET", m.APIURL+"/v1/portal/"+token+"/invoices/"+id, nil)
	require.NoError(t, err)
	response, err := client.Do(req)
	require.NoError(t, err)
	response.Body.Close()
	require.Equal(t, 200, response.StatusCode)
	require.Equal(t, "no-store", response.Header.Get("Cache-Control"))
	legacyAfter := data(call("GET", "/biz/lifecycle", legacyKey, "", nil, 200))
	require.Equal(t, legacyBefore["lifecycle_version"], legacyAfter["lifecycle_version"])
}
