package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/the-monkeys/freerangenotify/internal/domain/billing"
	"github.com/the-monkeys/freerangenotify/internal/domain/license"
	"go.uber.org/zap"
)

type paymentTestProvider struct{}

func (p *paymentTestProvider) CreateOrder(ctx context.Context, tenantID, tier string, amountPaisa int64) (billing.CheckoutResponse, error) {
	return billing.CheckoutResponse{
		OrderID:   "order_test",
		Tier:      tier,
		AmountINR: amountPaisa,
		Currency:  "INR",
		KeyID:     "rzp_test_key",
	}, nil
}

func (p *paymentTestProvider) VerifyPayment(ctx context.Context, verification billing.PaymentVerification) error {
	return nil
}

func (p *paymentTestProvider) VerifyWebhook(payload []byte, signature string) (billing.WebhookEvent, error) {
	return billing.WebhookEvent{}, nil
}

type paymentTestSubRepo struct {
	sub         *license.Subscription
	createCalls int
	updateCalls int
}

func (r *paymentTestSubRepo) Create(ctx context.Context, sub *license.Subscription) error {
	r.sub = sub
	r.createCalls++
	return nil
}

func (r *paymentTestSubRepo) GetByID(ctx context.Context, id string) (*license.Subscription, error) {
	if r.sub != nil && r.sub.ID == id {
		return r.sub, nil
	}
	return nil, nil
}

func (r *paymentTestSubRepo) Update(ctx context.Context, sub *license.Subscription) error {
	r.sub = sub
	r.updateCalls++
	return nil
}

func (r *paymentTestSubRepo) Delete(ctx context.Context, id string) error {
	return nil
}

func (r *paymentTestSubRepo) List(ctx context.Context, filter license.SubscriptionFilter) ([]*license.Subscription, error) {
	if r.sub == nil || (filter.TenantID != "" && r.sub.TenantID != filter.TenantID) {
		return nil, nil
	}
	return []*license.Subscription{r.sub}, nil
}

func (r *paymentTestSubRepo) GetActiveSubscription(ctx context.Context, tenantID, appID string, now time.Time) (*license.Subscription, error) {
	if r.sub == nil || r.sub.TenantID != tenantID || !r.sub.IsActiveAt(now) {
		return nil, nil
	}
	return r.sub, nil
}

func newPaymentTestApp(handler *PaymentHandler) *fiber.App {
	app := fiber.New()
	app.Post("/checkout", func(c *fiber.Ctx) error {
		c.Locals("user_id", "user-1")
		return handler.CreateOrder(c)
	})
	return app
}

func postCheckout(t *testing.T, app *fiber.App, tier string) *http.Response {
	t.Helper()
	body, err := json.Marshal(map[string]string{"tier": tier})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/checkout", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestCreateOrderAllowsActiveFreeSubscriptionUpgrade(t *testing.T) {
	now := time.Now().UTC()
	repo := &paymentTestSubRepo{sub: &license.Subscription{
		ID:                 "sub-free",
		TenantID:           "user-1",
		Plan:               "free",
		Status:             license.SubscriptionStatusActive,
		CurrentPeriodStart: now.Add(-time.Hour),
		CurrentPeriodEnd:   now.Add(time.Hour),
		CreatedAt:          now.Add(-time.Hour),
		Metadata:           map[string]interface{}{},
	}}
	handler := NewPaymentHandler(&paymentTestProvider{}, repo, nil, billing.DefaultRates(), true, zap.NewNop())
	app := newPaymentTestApp(handler)

	resp := postCheckout(t, app, "pro")
	if resp.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, fiber.StatusOK)
	}
	if repo.updateCalls != 1 {
		t.Fatalf("update calls = %d, want 1", repo.updateCalls)
	}
	if got := repo.sub.Metadata["pending_checkout_tier"]; got != "pro" {
		t.Fatalf("pending checkout tier = %v, want pro", got)
	}
	if got := repo.sub.Metadata["pending_checkout_order_id"]; got != "order_test" {
		t.Fatalf("pending checkout order id = %v, want order_test", got)
	}
}

func TestCreateOrderRejectsAlreadyPaidActiveSubscription(t *testing.T) {
	now := time.Now().UTC()
	repo := &paymentTestSubRepo{sub: &license.Subscription{
		ID:                 "sub-pro",
		TenantID:           "user-1",
		Plan:               "pro",
		Status:             license.SubscriptionStatusActive,
		CurrentPeriodStart: now.Add(-time.Hour),
		CurrentPeriodEnd:   now.Add(time.Hour),
		CreatedAt:          now.Add(-time.Hour),
	}}
	handler := NewPaymentHandler(&paymentTestProvider{}, repo, nil, billing.DefaultRates(), true, zap.NewNop())
	app := newPaymentTestApp(handler)

	resp := postCheckout(t, app, "pro")
	if resp.StatusCode != fiber.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.StatusCode, fiber.StatusConflict)
	}
	if repo.updateCalls != 0 {
		t.Fatalf("update calls = %d, want 0", repo.updateCalls)
	}
}

func TestCreateOrderRejectsFreeCheckout(t *testing.T) {
	repo := &paymentTestSubRepo{}
	handler := NewPaymentHandler(&paymentTestProvider{}, repo, nil, billing.DefaultRates(), true, zap.NewNop())
	app := newPaymentTestApp(handler)

	resp := postCheckout(t, app, "free")
	if resp.StatusCode != fiber.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, fiber.StatusBadRequest)
	}
	if repo.createCalls != 0 || repo.updateCalls != 0 {
		t.Fatalf("create/update calls = %d/%d, want 0/0", repo.createCalls, repo.updateCalls)
	}
}
