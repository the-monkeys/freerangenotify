package payment

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
)

func TestBizObsoleteOrderRequiresPositiveUnpaidProof(t *testing.T) {
	ref := bizbilling.GatewayRef{ConnectionID: "fixture", AccountID: "acc_fixture", Mode: bizbilling.GatewayTest, CredentialRevision: 1}
	intent := bizbilling.OrderIntent{OperationID: "op", InvoiceID: "invoice", Scope: bizbilling.Scope{AppID: "app", UserID: "user", Currency: "INR", Mode: bizbilling.GatewayTest}, Gateway: ref, AmountPaisa: 1000, Currency: "INR", Receipt: "receipt"}
	for _, tc := range []struct {
		name, status   string
		paid, attempts any
		receipt        string
		httpStatus     int
		unpaid         bool
	}{
		{"never_attempted", "created", 0, 0, "receipt", 200, true},
		{"authorized_or_pending", "attempted", 0, 1, "receipt", 200, false},
		{"captured", "paid", 1000, 1, "receipt", 200, false},
		{"missing_state", "created", nil, nil, "receipt", 200, false},
		{"wrong_binding", "created", 0, 0, "other", 200, false},
		{"unavailable", "created", 0, 0, "receipt", 503, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "GET", r.Method)
				require.Equal(t, "/v1/orders/order_fixture", r.URL.Path)
				w.WriteHeader(tc.httpStatus)
				json.NewEncoder(w).Encode(map[string]any{"id": "order_fixture", "entity": "order", "amount": 1000, "currency": "INR", "receipt": tc.receipt, "status": tc.status, "amount_paid": tc.paid, "attempts": tc.attempts, "notes": map[string]string{"frn_operation": "op", "frn_invoice": "invoice", "frn_app": "app"}})
			}))
			defer server.Close()
			endpoint, err := url.Parse(server.URL)
			require.NoError(t, err)
			credentials := bizbilling.MerchantCredentials{AppID: "app", Reference: ref, KeyID: "rzp_test_fixture", KeySecret: "local-fixture"}
			gateway := &bizRazorpayGateway{credentials: credentials, apiCredentials: credentials, client: &http.Client{Transport: bizProbeTransport{endpoint}}}
			unpaid, err := gateway.OrderIsUnpaid(context.Background(), bizbilling.GatewayOrder{ID: "order_fixture", Intent: intent})
			require.Equal(t, tc.unpaid, unpaid)
			if tc.unpaid {
				require.NoError(t, err)
			}
			if tc.receipt != "receipt" || tc.httpStatus != 200 {
				require.Error(t, err)
			}
		})
	}
}

func TestBizRecurringProofUsesExactSuccessorAndRetainsBinding(t *testing.T) {
	ref := bizbilling.GatewayRef{ConnectionID: "fixture", AccountID: "acc_fixture", Mode: bizbilling.GatewayTest, CredentialRevision: 1}
	next := ref
	next.CredentialRevision = 2
	intent := bizbilling.RecurringIntent{OperationID: "operation", SubscriptionID: "subscription", Scope: bizbilling.Scope{AppID: "app", UserID: "user", Currency: "INR", Mode: bizbilling.GatewayTest}, Gateway: ref, PlanID: "plan_fixture", Name: "Monthly", Period: "monthly", Interval: 1, AmountPaisa: 1000, Currency: "INR", TotalCount: 12, StartOnAuthorization: true, ExpireBy: time.Unix(1900000000, 0).UTC()}
	reads := 0
	foreign := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "GET", r.Method)
		key, _, _ := r.BasicAuth()
		require.Equal(t, "rzp_test_new", key)
		reads++
		if r.URL.Path == "/v1/subscriptions/sub_fixture" {
			notes := recurringSubscriptionNotes(intent)
			if foreign {
				notes["frn_account"] = "acc_foreign"
			}
			json.NewEncoder(w).Encode(map[string]any{"id": "sub_fixture", "entity": "subscription", "plan_id": "plan_fixture", "status": "authenticated", "quantity": 1, "total_count": 12, "start_at": 0, "expire_by": intent.ExpireBy.Unix(), "notes": notes})
		} else {
			require.Equal(t, "/v1/plans/plan_fixture", r.URL.Path)
			json.NewEncoder(w).Encode(map[string]any{"id": "plan_fixture", "entity": "plan", "period": "monthly", "interval": 1, "item": map[string]any{"name": "Monthly", "amount": 1000, "currency": "INR"}, "notes": map[string]string{"frn_operation": "operation", "frn_app": "app"}})
		}
	}))
	defer server.Close()
	endpoint, err := url.Parse(server.URL)
	require.NoError(t, err)
	factory := NewBizRazorpayFactory(func(_ context.Context, actual bizbilling.GatewayRef) (bizbilling.MerchantCredentials, error) {
		key := "rzp_test_old"
		if actual.CredentialRevision == 2 {
			key = "rzp_test_new"
		}
		return bizbilling.MerchantCredentials{AppID: "app", Reference: actual, KeyID: key, KeySecret: "fixture"}, nil
	}, &http.Client{Transport: bizProbeTransport{endpoint}})
	remote, err := factory.FetchSubscriptionForProof(context.Background(), ref, next, "sub_fixture")
	require.NoError(t, err)
	require.Equal(t, intent, remote.Intent)
	require.Equal(t, 2, reads)
	foreign = true
	_, err = factory.FetchSubscriptionForProof(context.Background(), ref, next, "sub_fixture")
	require.ErrorIs(t, err, bizbilling.ErrInvalidGatewayProof)
	other := next
	other.AccountID = "acc_other"
	before := reads
	_, err = factory.FetchSubscriptionForProof(context.Background(), ref, other, "sub_fixture")
	require.ErrorIs(t, err, bizbilling.ErrInvalidGatewayProof)
	require.Equal(t, before, reads)
}
