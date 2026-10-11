package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/application"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
	"go.uber.org/zap"
)

type breakdownAppFixture struct{ application.Repository }

func (*breakdownAppFixture) List(context.Context, application.ApplicationFilter) ([]*application.Application, error) {
	return []*application.Application{{AppID: "app-1", AdminUserID: "user-1"}}, nil
}

type breakdownUsageFixture struct{ billing.UsageRepository }

func (*breakdownUsageFixture) GetSummary(context.Context, []string, time.Time, time.Time) ([]billing.UsageSummary, error) {
	return []billing.UsageSummary{{Channel: "email", MessageCount: 20, CreditsConsumed: 100}}, nil
}

func TestUsageBreakdownCannotInventAnExhaustedWallet(t *testing.T) {
	now := time.Now().UTC()
	h := NewBillingHandler(&paymentTestSubRepo{sub: &license.Subscription{ID: "sub-1", TenantID: "user-1", Plan: "free",
		Status: license.SubscriptionStatusActive, CurrentPeriodStart: now.Add(-time.Hour), CurrentPeriodEnd: now.Add(time.Hour),
		CreditsTotal: 1500, CreditsRemaining: 0, Metadata: map[string]interface{}{"billing_model": "credits"}}}, &breakdownAppFixture{}, billing.DefaultRates(), zap.NewNop())
	h.SetUsageRepo(&breakdownUsageFixture{}, true)
	app := fiber.New()
	app.Get("/breakdown", func(c *fiber.Ctx) error { c.Locals("user_id", "user-1"); return h.GetUsageBreakdown(c) })
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/breakdown", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != 200 || body["credits_remaining"] != float64(0) || body["credits_available"] != float64(0) || body["credits_consumed"] != float64(1500) {
		t.Fatalf("incomplete metering invented wallet credits: %+v", body)
	}
	if body["metered_credits_consumed"] != float64(100) {
		t.Fatalf("delivery breakdown lost its metered total: %+v", body)
	}
}

func TestBillingHandler_GetUsage_exposes_available_and_reserved_credits(t *testing.T) {
	now := time.Now().UTC()
	handler := NewBillingHandler(&paymentTestSubRepo{sub: &license.Subscription{
		ID:                 "sub-1",
		TenantID:           "user-1",
		Plan:               "free",
		Status:             license.SubscriptionStatusActive,
		CreditsTotal:       1500,
		CreditsRemaining:   445,
		CreditsReserved:    445,
		CurrentPeriodStart: now.AddDate(0, -1, 0),
		CurrentPeriodEnd:   now.AddDate(1, 0, 0),
		Metadata: map[string]interface{}{
			"billing_model": "credits",
		},
	}}, nil, nil, zap.NewNop())

	app := fiber.New()
	app.Get("/usage", func(c *fiber.Ctx) error {
		c.Locals("user_id", "user-1")
		return handler.GetUsage(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/usage", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d body = %s", resp.StatusCode, body)
	}

	var got map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got["credits_reserved"] != float64(445) {
		t.Fatalf("credits_reserved = %v want 445", got["credits_reserved"])
	}
	if got["credits_available"] != float64(0) {
		t.Fatalf("credits_available = %v want 0", got["credits_available"])
	}
	if got["credits_remaining"] != float64(445) {
		t.Fatalf("credits_remaining = %v want 445", got["credits_remaining"])
	}
	if got["credits_consumed"] != float64(1055) {
		t.Fatalf("credits_consumed = %v want 1055", got["credits_consumed"])
	}
	pct, ok := got["usage_percent"].(float64)
	if !ok {
		t.Fatalf("usage_percent = %v", got["usage_percent"])
	}
	wantPct := float64(1055) / float64(1500) * 100
	if pct < wantPct-0.01 || pct > wantPct+0.01 {
		t.Fatalf("usage_percent = %v want %v", pct, wantPct)
	}
}
