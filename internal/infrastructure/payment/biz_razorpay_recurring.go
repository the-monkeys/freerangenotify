package payment

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/the-monkeys/freerangenotify/internal/domain/bizbilling"
)

// Razorpay represents an empty notes object as [] on some older resources.
type recurringNotes map[string]string

func (n *recurringNotes) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("[]")) || bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		*n = nil
		return nil
	}
	return json.Unmarshal(data, (*map[string]string)(n))
}

type razorpayRecurringPlan struct {
	ID       string `json:"id"`
	Entity   string `json:"entity"`
	Period   string `json:"period"`
	Interval int    `json:"interval"`
	Item     struct {
		Name     string `json:"name"`
		Amount   int64  `json:"amount"`
		Currency string `json:"currency"`
	} `json:"item"`
	Notes recurringNotes `json:"notes"`
}

type razorpaySubscription struct {
	ID           string         `json:"id"`
	Entity       string         `json:"entity"`
	PlanID       string         `json:"plan_id"`
	Status       string         `json:"status"`
	Quantity     int            `json:"quantity"`
	TotalCount   int            `json:"total_count"`
	StartAt      int64          `json:"start_at"`
	ExpireBy     int64          `json:"expire_by"`
	CurrentStart int64          `json:"current_start"`
	CurrentEnd   int64          `json:"current_end"`
	Notes        recurringNotes `json:"notes"`
}

func (g *bizRazorpayGateway) CheckRecurring(ctx context.Context) error {
	for _, path := range []string{"/plans?count=1", "/subscriptions?count=1"} {
		var result struct {
			Entity string            `json:"entity"`
			Count  int               `json:"count"`
			Items  []json.RawMessage `json:"items"`
		}
		if err := g.request(ctx, http.MethodGet, path, nil, &result); err != nil {
			return err
		}
		if result.Entity != "collection" || result.Count != len(result.Items) || result.Count < 0 || result.Count > 1 {
			return bizbilling.ErrInvalidGatewayProof
		}
	}
	return nil
}

func (g *bizRazorpayGateway) validRecurringIntent(intent bizbilling.RecurringIntent) bool {
	return intent.Scope.Validate() == nil && intent.Scope.AppID == g.credentials.AppID && intent.Gateway == g.credentials.Reference && (intent.Gateway.Mode == bizbilling.GatewayTest) == (intent.Scope.Mode == bizbilling.GatewayTest) && intent.OperationID != "" && intent.SubscriptionID != "" && intent.Currency == "INR" && intent.Scope.Currency == intent.Currency && intent.AmountPaisa >= 100 && intent.AmountPaisa <= 4294967295 && intent.TotalCount >= 1 && intent.TotalCount <= 120 && intent.Interval >= 1 && (intent.Period == "weekly" || intent.Period == "monthly" || intent.Period == "yearly") && intent.StartAt.Unix() > 0 && intent.ExpireBy.Unix() > 0
}

func (g *bizRazorpayGateway) checkedRecurringPlan(raw razorpayRecurringPlan, intent bizbilling.RecurringIntent) (bizbilling.GatewayRecurringPlan, error) {
	if !providerEntityID.MatchString(raw.ID) || raw.Entity != "plan" || raw.Notes["frn_operation"] != intent.OperationID || raw.Notes["frn_app"] != intent.Scope.AppID || raw.Item.Amount != intent.AmountPaisa || raw.Item.Currency != intent.Currency || raw.Period != intent.Period || raw.Interval != intent.Interval {
		return bizbilling.GatewayRecurringPlan{}, bizbilling.ErrInvalidGatewayProof
	}
	return bizbilling.GatewayRecurringPlan{ID: raw.ID, OperationID: intent.OperationID, AmountPaisa: raw.Item.Amount, Currency: raw.Item.Currency, Period: raw.Period, Interval: raw.Interval}, nil
}

func (g *bizRazorpayGateway) CreateRecurringPlan(ctx context.Context, intent bizbilling.RecurringIntent) (bizbilling.GatewayRecurringPlan, error) {
	if !g.validRecurringIntent(intent) {
		return bizbilling.GatewayRecurringPlan{}, bizbilling.ErrInvalidGatewayProof
	}
	request := map[string]any{"period": intent.Period, "interval": intent.Interval, "item": map[string]any{"name": intent.Name, "amount": intent.AmountPaisa, "currency": intent.Currency}, "notes": map[string]string{"frn_operation": intent.OperationID, "frn_app": intent.Scope.AppID}}
	var result razorpayRecurringPlan
	if err := g.request(ctx, http.MethodPost, "/plans", request, &result); err != nil {
		return bizbilling.GatewayRecurringPlan{}, err
	}
	return g.checkedRecurringPlan(result, intent)
}

func (g *bizRazorpayGateway) FindRecurringPlan(ctx context.Context, intent bizbilling.RecurringIntent) (*bizbilling.GatewayRecurringPlan, error) {
	if !g.validRecurringIntent(intent) {
		return nil, bizbilling.ErrInvalidGatewayProof
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var found *bizbilling.GatewayRecurringPlan
	for offset := 0; offset < 100000; offset += 100 {
		var result struct {
			Entity string                  `json:"entity"`
			Count  int                     `json:"count"`
			Items  []razorpayRecurringPlan `json:"items"`
		}
		query := url.Values{"count": {"100"}, "skip": {strconv.Itoa(offset)}, "from": {strconv.FormatInt(intent.ExpireBy.Add(-25*time.Hour).Unix(), 10)}}
		if err := g.request(ctx, http.MethodGet, "/plans?"+query.Encode(), nil, &result); err != nil {
			return nil, err
		}
		if result.Entity != "collection" || result.Count != len(result.Items) || result.Count > 100 {
			return nil, bizbilling.ErrInvalidGatewayProof
		}
		for _, raw := range result.Items {
			if raw.Notes["frn_operation"] != intent.OperationID {
				continue
			}
			plan, err := g.checkedRecurringPlan(raw, intent)
			if err != nil {
				return nil, err
			}
			if found != nil {
				return nil, bizbilling.ErrReconciliationRequired
			}
			found = &plan
		}
		if result.Count < 100 {
			return found, nil
		}
	}
	return nil, bizbilling.ErrGatewayPending
}

func recurringSubscriptionNotes(intent bizbilling.RecurringIntent) map[string]string {
	return map[string]string{"frn_operation": intent.OperationID, "frn_subscription": intent.SubscriptionID, "frn_app": intent.Scope.AppID, "frn_user": intent.Scope.UserID, "frn_connection": intent.Gateway.ConnectionID, "frn_account": intent.Gateway.AccountID, "frn_mode": string(intent.Gateway.Mode), "frn_revision": strconv.FormatInt(intent.Gateway.CredentialRevision, 10)}
}

func (g *bizRazorpayGateway) checkedSubscription(ctx context.Context, raw razorpaySubscription) (bizbilling.GatewaySubscription, error) {
	var result bizbilling.GatewaySubscription
	if !providerEntityID.MatchString(raw.ID) || !providerEntityID.MatchString(raw.PlanID) || raw.Entity != "subscription" || raw.Quantity != 1 || raw.StartAt <= 0 || raw.ExpireBy <= 0 || raw.TotalCount < 1 || raw.TotalCount > 120 {
		return result, bizbilling.ErrInvalidGatewayProof
	}
	switch raw.Status {
	case "created", "authenticated", "active", "pending", "halted", "paused", "cancelled", "completed", "expired":
	default:
		return result, bizbilling.ErrInvalidGatewayProof
	}
	revision, err := strconv.ParseInt(raw.Notes["frn_revision"], 10, 64)
	if err != nil {
		return result, bizbilling.ErrInvalidGatewayProof
	}
	ref := bizbilling.GatewayRef{ConnectionID: raw.Notes["frn_connection"], AccountID: raw.Notes["frn_account"], Mode: bizbilling.GatewayMode(raw.Notes["frn_mode"]), CredentialRevision: revision}
	if ref != g.credentials.Reference || raw.Notes["frn_app"] != g.credentials.AppID {
		return result, bizbilling.ErrInvalidGatewayProof
	}
	var plan razorpayRecurringPlan
	if err = g.request(ctx, http.MethodGet, "/plans/"+url.PathEscape(raw.PlanID), nil, &plan); err != nil {
		return result, err
	}
	if plan.ID != raw.PlanID || plan.Entity != "plan" || plan.Notes["frn_operation"] != raw.Notes["frn_operation"] || plan.Notes["frn_app"] != raw.Notes["frn_app"] {
		return result, bizbilling.ErrInvalidGatewayProof
	}
	scope := bizbilling.Scope{AppID: raw.Notes["frn_app"], UserID: raw.Notes["frn_user"], Currency: plan.Item.Currency}
	if ref.Mode == bizbilling.GatewayTest {
		scope.Mode = bizbilling.GatewayTest
	}
	intent := bizbilling.RecurringIntent{OperationID: raw.Notes["frn_operation"], SubscriptionID: raw.Notes["frn_subscription"], Scope: scope, Gateway: ref, Name: plan.Item.Name, PlanID: plan.ID, Period: plan.Period, Interval: plan.Interval, AmountPaisa: plan.Item.Amount, Currency: plan.Item.Currency, TotalCount: raw.TotalCount, StartAt: time.Unix(raw.StartAt, 0).UTC(), ExpireBy: time.Unix(raw.ExpireBy, 0).UTC()}
	if !g.validRecurringIntent(intent) {
		return result, bizbilling.ErrInvalidGatewayProof
	}
	result = bizbilling.GatewaySubscription{ID: raw.ID, PlanID: raw.PlanID, Intent: intent, Status: raw.Status}
	if raw.CurrentStart > 0 {
		result.CurrentStart = time.Unix(raw.CurrentStart, 0).UTC()
	}
	if raw.CurrentEnd > 0 {
		result.CurrentEnd = time.Unix(raw.CurrentEnd, 0).UTC()
	}
	return result, nil
}

func (g *bizRazorpayGateway) CreateSubscription(ctx context.Context, intent bizbilling.RecurringIntent) (bizbilling.GatewaySubscription, error) {
	if !g.validRecurringIntent(intent) || !providerEntityID.MatchString(intent.PlanID) {
		return bizbilling.GatewaySubscription{}, bizbilling.ErrInvalidGatewayProof
	}
	request := map[string]any{"plan_id": intent.PlanID, "quantity": 1, "total_count": intent.TotalCount, "customer_notify": false, "start_at": intent.StartAt.Unix(), "expire_by": intent.ExpireBy.Unix(), "notes": recurringSubscriptionNotes(intent)}
	var result razorpaySubscription
	if err := g.request(ctx, http.MethodPost, "/subscriptions", request, &result); err != nil {
		return bizbilling.GatewaySubscription{}, err
	}
	return g.checkedSubscription(ctx, result)
}

func (g *bizRazorpayGateway) FetchSubscription(ctx context.Context, id string) (bizbilling.GatewaySubscription, error) {
	if !providerEntityID.MatchString(id) {
		return bizbilling.GatewaySubscription{}, bizbilling.ErrInvalidGatewayProof
	}
	var result razorpaySubscription
	if err := g.request(ctx, http.MethodGet, "/subscriptions/"+url.PathEscape(id), nil, &result); err != nil {
		return bizbilling.GatewaySubscription{}, err
	}
	if result.ID != id {
		return bizbilling.GatewaySubscription{}, bizbilling.ErrInvalidGatewayProof
	}
	return g.checkedSubscription(ctx, result)
}

func (g *bizRazorpayGateway) FindSubscription(ctx context.Context, intent bizbilling.RecurringIntent) (*bizbilling.GatewaySubscription, error) {
	if !g.validRecurringIntent(intent) || !providerEntityID.MatchString(intent.PlanID) {
		return nil, bizbilling.ErrInvalidGatewayProof
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var found *bizbilling.GatewaySubscription
	for offset := 0; offset < 100000; offset += 100 {
		var result struct {
			Entity string                 `json:"entity"`
			Count  int                    `json:"count"`
			Items  []razorpaySubscription `json:"items"`
		}
		query := url.Values{"plan_id": {intent.PlanID}, "count": {"100"}, "skip": {strconv.Itoa(offset)}, "from": {strconv.FormatInt(intent.ExpireBy.Add(-25*time.Hour).Unix(), 10)}}
		if err := g.request(ctx, http.MethodGet, "/subscriptions?"+query.Encode(), nil, &result); err != nil {
			return nil, err
		}
		if result.Entity != "collection" || result.Count != len(result.Items) || result.Count > 100 {
			return nil, bizbilling.ErrInvalidGatewayProof
		}
		for _, raw := range result.Items {
			if raw.Notes["frn_operation"] != intent.OperationID {
				continue
			}
			sub, err := g.checkedSubscription(ctx, raw)
			if err != nil {
				return nil, err
			}
			if found != nil {
				return nil, bizbilling.ErrReconciliationRequired
			}
			found = &sub
		}
		if result.Count < 100 {
			return found, nil
		}
	}
	return nil, bizbilling.ErrGatewayPending
}

func (g *bizRazorpayGateway) ChangeSubscription(ctx context.Context, id string, action bizbilling.SubscriptionAction) (bizbilling.GatewaySubscription, error) {
	if !providerEntityID.MatchString(id) {
		return bizbilling.GatewaySubscription{}, bizbilling.ErrInvalidGatewayProof
	}
	var body map[string]any
	switch action {
	case bizbilling.SubscriptionCancel:
		body = map[string]any{"cancel_at_cycle_end": false}
	case bizbilling.SubscriptionPause:
		body = map[string]any{"pause_at": "now"}
	case bizbilling.SubscriptionResume:
		body = map[string]any{"resume_at": "now"}
	default:
		return bizbilling.GatewaySubscription{}, bizbilling.ErrInvalidGatewayProof
	}
	var result razorpaySubscription
	if err := g.request(ctx, http.MethodPost, "/subscriptions/"+url.PathEscape(id)+"/"+string(action), body, &result); err != nil {
		return bizbilling.GatewaySubscription{}, err
	}
	if result.ID != id {
		return bizbilling.GatewaySubscription{}, bizbilling.ErrInvalidGatewayProof
	}
	return g.checkedSubscription(ctx, result)
}

func (g *bizRazorpayGateway) VerifySubscriptionAuthorization(payment, subscription, signature string) error {
	if !providerEntityID.MatchString(payment) || !providerEntityID.MatchString(subscription) {
		return bizbilling.ErrInvalidGatewayProof
	}
	actual, err := hex.DecodeString(signature)
	if err != nil {
		return bizbilling.ErrInvalidGatewayProof
	}
	mac := hmac.New(sha256.New, []byte(g.credentials.KeySecret))
	mac.Write([]byte(payment + "|" + subscription))
	if !hmac.Equal(actual, mac.Sum(nil)) {
		return bizbilling.ErrInvalidGatewayProof
	}
	return nil
}
