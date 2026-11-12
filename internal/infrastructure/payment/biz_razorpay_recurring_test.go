package payment

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
)

func TestBizAutomaticRazorpayWireContracts(t *testing.T) {
	ctx := context.Background()
	ref := bizbilling.GatewayRef{ConnectionID: "fixture-business", AccountID: "acc_fixture", Mode: bizbilling.GatewayTest, CredentialRevision: 1}
	intent := bizbilling.RecurringIntent{OperationID: "fixture-recurring", SubscriptionID: "fixture-subscription", Scope: bizbilling.Scope{AppID: "fixture-app", UserID: "fixture-user", Currency: "INR", Mode: bizbilling.GatewayTest}, Gateway: ref, Name: "Monthly service", AmountPaisa: 1180, Currency: "INR", Period: "monthly", Interval: 1, TotalCount: 12, StartAt: time.Unix(1900000000, 0).UTC(), ExpireBy: time.Unix(1899900000, 0).UTC()}
	status := "created"
	posts := 0
	failPause := false
	plan := map[string]any{"id": "plan_fixture", "entity": "plan", "period": "monthly", "interval": 1, "item": map[string]any{"name": intent.Name, "amount": int64(1180), "currency": "INR"}, "notes": map[string]string{"frn_operation": intent.OperationID, "frn_app": intent.Scope.AppID}}
	notes := map[string]string{"frn_operation": intent.OperationID, "frn_subscription": intent.SubscriptionID, "frn_app": intent.Scope.AppID, "frn_user": intent.Scope.UserID, "frn_connection": ref.ConnectionID, "frn_account": ref.AccountID, "frn_mode": "test", "frn_revision": "1"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key, secret, ok := r.BasicAuth()
		require.True(t, ok)
		require.Equal(t, "rzp_test_biz1", key)
		require.Equal(t, "owned-api-secret", secret)
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "POST" {
			posts++
		}
		remote := func() map[string]any {
			return map[string]any{"id": "sub_fixture", "entity": "subscription", "plan_id": "plan_fixture", "status": status, "quantity": 1, "total_count": 12, "start_at": intent.StartAt.Unix(), "expire_by": intent.ExpireBy.Unix(), "notes": notes}
		}
		switch r.Method + " " + r.URL.Path {
		case "POST /v1/plans":
			var body struct {
				Period   string `json:"period"`
				Interval int    `json:"interval"`
				Item     struct {
					Name     string `json:"name"`
					Amount   int64  `json:"amount"`
					Currency string `json:"currency"`
				} `json:"item"`
				Notes map[string]string `json:"notes"`
			}
			decoder := json.NewDecoder(r.Body)
			decoder.DisallowUnknownFields()
			require.NoError(t, decoder.Decode(&body))
			require.Equal(t, "monthly", body.Period)
			require.Equal(t, 1, body.Interval)
			require.Equal(t, int64(1180), body.Item.Amount)
			require.Equal(t, "INR", body.Item.Currency)
			require.Equal(t, intent.OperationID, body.Notes["frn_operation"])
			json.NewEncoder(w).Encode(plan)
		case "GET /v1/plans":
			json.NewEncoder(w).Encode(map[string]any{"entity": "collection", "count": 1, "items": []any{plan}})
		case "GET /v1/plans/plan_fixture":
			json.NewEncoder(w).Encode(plan)
		case "POST /v1/subscriptions":
			var body struct {
				PlanID         string            `json:"plan_id"`
				TotalCount     int               `json:"total_count"`
				Quantity       int               `json:"quantity"`
				CustomerNotify bool              `json:"customer_notify"`
				StartAt        int64             `json:"start_at"`
				ExpireBy       int64             `json:"expire_by"`
				Notes          map[string]string `json:"notes"`
			}
			decoder := json.NewDecoder(r.Body)
			decoder.DisallowUnknownFields()
			require.NoError(t, decoder.Decode(&body))
			require.Equal(t, "plan_fixture", body.PlanID)
			require.Equal(t, 12, body.TotalCount)
			require.Equal(t, 1, body.Quantity)
			require.False(t, body.CustomerNotify)
			require.Equal(t, intent.StartAt.Unix(), body.StartAt)
			require.Equal(t, intent.ExpireBy.Unix(), body.ExpireBy)
			require.Equal(t, notes, body.Notes)
			json.NewEncoder(w).Encode(remote())
		case "GET /v1/subscriptions":
			json.NewEncoder(w).Encode(map[string]any{"entity": "collection", "count": 1, "items": []any{remote()}})
		case "GET /v1/subscriptions/sub_fixture":
			json.NewEncoder(w).Encode(remote())
		case "POST /v1/subscriptions/sub_fixture/cancel":
			var body struct {
				CycleEnd bool `json:"cancel_at_cycle_end"`
			}
			decoder := json.NewDecoder(r.Body)
			decoder.DisallowUnknownFields()
			require.NoError(t, decoder.Decode(&body))
			require.False(t, body.CycleEnd)
			status = "cancelled"
			json.NewEncoder(w).Encode(remote())
		case "POST /v1/subscriptions/sub_fixture/pause", "POST /v1/subscriptions/sub_fixture/resume":
			var body map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			if r.URL.Path == "/v1/subscriptions/sub_fixture/pause" {
				require.Equal(t, map[string]string{"pause_at": "now"}, body)
				if failPause {
					w.WriteHeader(503)
					json.NewEncoder(w).Encode(map[string]string{"error": "owned-api-secret"})
					return
				}
				status = "paused"
			} else {
				require.Equal(t, map[string]string{"resume_at": "now"}, body)
				status = "active"
			}
			json.NewEncoder(w).Encode(remote())
		default:
			t.Errorf("unexpected provider call %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	endpoint, err := url.Parse(server.URL)
	require.NoError(t, err)
	factory := NewBizRazorpayFactory(func(context.Context, bizbilling.GatewayRef) (bizbilling.MerchantCredentials, error) {
		return bizbilling.MerchantCredentials{AppID: "fixture-app", Reference: ref, KeyID: "rzp_test_biz1", KeySecret: "owned-api-secret"}, nil
	}, &http.Client{Transport: bizProbeTransport{endpoint}})
	base, err := factory.For(ctx, ref)
	require.NoError(t, err)
	gateway, ok := base.(bizbilling.RecurringGateway)
	require.True(t, ok, "Razorpay adapter must implement the optional recurring contract")
	require.NoError(t, gateway.CheckRecurring(ctx))
	require.Zero(t, posts, "readiness must not create provider resources")
	createdPlan, err := gateway.CreateRecurringPlan(ctx, intent)
	require.NoError(t, err)
	require.Equal(t, "plan_fixture", createdPlan.ID)
	foundPlan, err := gateway.FindRecurringPlan(ctx, intent)
	require.NoError(t, err)
	require.Equal(t, createdPlan, *foundPlan)
	intent.PlanID = createdPlan.ID
	created, err := gateway.CreateSubscription(ctx, intent)
	require.NoError(t, err)
	require.Equal(t, "sub_fixture", created.ID)
	found, err := gateway.FindSubscription(ctx, intent)
	require.NoError(t, err)
	require.Equal(t, created, *found)
	status = "authenticated"
	authorized, err := gateway.FetchSubscription(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "authenticated", authorized.Status)
	paused, err := gateway.ChangeSubscription(ctx, created.ID, bizbilling.SubscriptionPause)
	require.NoError(t, err)
	require.Equal(t, "paused", paused.Status)
	resumed, err := gateway.ChangeSubscription(ctx, created.ID, bizbilling.SubscriptionResume)
	require.NoError(t, err)
	require.Equal(t, "active", resumed.Status)
	cancelled, err := gateway.ChangeSubscription(ctx, created.ID, bizbilling.SubscriptionCancel)
	require.NoError(t, err)
	require.Equal(t, "cancelled", cancelled.Status)
	require.Equal(t, 5, posts, "one versioned plan, subscription, pause, resume and cancellation")
	failPause = true
	_, err = gateway.ChangeSubscription(ctx, created.ID, bizbilling.SubscriptionPause)
	require.ErrorIs(t, err, bizbilling.ErrGatewayPending)
	require.NotContains(t, err.Error(), "owned-api-secret")
	proof, ok := base.(interface {
		VerifySubscriptionAuthorization(string, string, string) error
	})
	require.True(t, ok)
	mac := hmac.New(sha256.New, []byte("owned-api-secret"))
	mac.Write([]byte("pay_fixture|sub_fixture"))
	require.NoError(t, proof.VerifySubscriptionAuthorization("pay_fixture", "sub_fixture", hex.EncodeToString(mac.Sum(nil))))
	require.Error(t, proof.VerifySubscriptionAuthorization("pay_fixture", "sub_other", hex.EncodeToString(mac.Sum(nil))))
}
