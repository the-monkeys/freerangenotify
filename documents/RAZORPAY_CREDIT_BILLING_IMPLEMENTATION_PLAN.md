# Razorpay Credit Billing Implementation Plan

Status: Draft for approval

## 1. Goal

Make Razorpay checkout work correctly with the credit/token billing system.

The important product rule is:

- The database must decide what a plan costs and how many credits it gives.
- If the active billing configuration says `starter = INR 500 + 2000 credits`, checkout and payment verification must allocate exactly 2000 credits.
- Existing production businesses must keep working with the current subscription records and current default plans.

## 2. Current State

Backend:

- Razorpay provider exists in `internal/infrastructure/payment/razorpay.go`.
- Checkout APIs exist:
  - `POST /v1/billing/checkout`
  - `POST /v1/billing/verify-payment`
  - `POST /v1/billing/webhook`
- Subscription credit fields exist on `license.Subscription`:
  - `credits_total`
  - `credits_remaining`
  - `credits_reserved`
  - `credits_expire_at`
- Channel credit burn rates are DB-backed via `billing_rate_cards`.
- Plan purchase bundles are still mostly hardcoded in `internal/domain/billing/rates.go`.

Frontend:

- `ui/src/hooks/useRazorpayCheckout.ts` opens Razorpay Checkout.
- `ui/src/pages/WorkspaceBilling.tsx` hardcodes checkout to `pro`.
- `ui/src/constants/pricing.ts` contains static plan prices and credit bundles.
- `/docs/pricing` is rendered from `ui/src/docs/pricing.md`.

Main gaps:

- Selecting `starter`, `growth`, or custom plans is not wired through Billing UI.
- The backend does not yet use DB-backed purchase bundles for checkout.
- Payment verification recomputes plan credits instead of using a checkout-time immutable purchase snapshot.

## 3. High-Level Design

Use Razorpay only as the payment collection gateway. FreeRangeNotify remains the source of truth for:

- plan id
- amount
- currency
- credits included
- validity period
- active pricing version

Checkout flow:

1. UI loads active billing plans from backend.
2. User selects a plan.
3. Backend resolves that plan from the active DB billing configuration.
4. Backend creates a Razorpay order for the configured amount.
5. Backend stores a pending checkout snapshot on the subscription.
6. UI opens Razorpay Checkout.
7. Backend verifies Razorpay signature.
8. Backend allocates credits from the stored snapshot, not from current live config.
9. Razorpay webhook is an idempotent fallback for captured payments.

Why snapshot:

- If an admin changes `starter` from 15,000 credits to 2,000 credits while a user is paying, the user still receives the exact bundle attached to their checkout order.

## 4. Low-Level Design

### 4.1 Billing Plan Bundle Model

Modify: `internal/domain/billing/credits.go`

Add:

```go
type PlanBundle struct {
    ID              string    `json:"id"`
    Name            string    `json:"name"`
    AmountPaisa     int64     `json:"amount_paisa"`
    Currency        string    `json:"currency"`
    CreditsIncluded int64     `json:"credits_included"`
    ValidityDays    int       `json:"validity_days"`
    Active          bool      `json:"active"`
    DisplayOrder    int       `json:"display_order"`
    Metadata        map[string]interface{} `json:"metadata,omitempty"`
}
```

Extend:

```go
type RateCard struct {
    ...
    Plans map[string]PlanBundle `json:"plans,omitempty"`
}
```

Backward compatibility:

- Existing `billing_rate_cards` documents without `plans` remain valid.
- If active DB card has no `plans`, fallback to current defaults from `billing.DefaultRates()`.

### 4.2 Plan Resolution Service

Modify: `internal/usecases/services/ratecard_service.go`

Add:

- `GetCheckoutPlan(planID string) (billing.PlanBundle, bool)`
- `ListCheckoutPlans() []billing.PlanBundle`
- default plan bootstrap that copies current defaults:
  - free: 500 credits, INR 0
  - starter: 15,000 credits, INR 500
  - pro: 55,000 credits, INR 1,499
  - growth: 185,000 credits, INR 4,999
  - scale: 550,000 credits, INR 14,999

Custom DB behavior:

- If DB says `starter.amount_paisa = 50000` and `starter.credits_included = 2000`, checkout uses 50000 paisa and verification allocates 2000 credits.

### 4.3 Elasticsearch Mapping

Modify: `internal/infrastructure/database/index_templates.go`

Update `billing_rate_cards` mapping to support:

- `plans`
- `plans.<plan_id>.amount_paisa`
- `plans.<plan_id>.credits_included`
- `plans.<plan_id>.currency`
- `plans.<plan_id>.validity_days`
- `plans.<plan_id>.active`
- `plans.<plan_id>.display_order`

No destructive migration is required.

### 4.4 Payment Checkout Snapshot

Modify: `internal/interfaces/http/handlers/payment_handler.go`

On `CreateOrder`:

- Resolve plan from DB-backed plan manager.
- Reject free plans from gateway checkout.
- Allow active `free` subscriptions to upgrade.
- Block active paid subscriptions from duplicate payment unless top-up behavior is explicitly added later.
- Create Razorpay order with:
  - `amount`
  - `currency`
  - short unique `receipt`
  - `notes.tenant_id`
  - `notes.plan_id`
  - `notes.credits_included`
  - `notes.rate_card_version`
- Store pending checkout snapshot:

```text
pending_checkout_order_id
pending_checkout_plan_id
pending_checkout_plan_name
pending_checkout_amount_paisa
pending_checkout_currency
pending_checkout_credits
pending_checkout_validity_days
pending_checkout_rate_card_version
pending_checkout_provider
pending_checkout_created_at
```

On `VerifyPayment`:

- Verify Razorpay signature.
- Load pending checkout by user subscription.
- Ensure order id matches snapshot.
- Allocate credits from snapshot.
- Store final payment metadata:

```text
last_payment_id
last_order_id
last_payment_at
last_paid_amount_paisa
last_paid_currency
last_paid_credits
last_paid_plan_id
last_rate_card_version
```

Do not recompute credits from live plan config during verification.

### 4.5 Credit Allocation

Modify: `internal/interfaces/http/handlers/subscription_cycle.go`

Add a helper such as:

```go
func applyPaidCreditAllocation(sub *license.Subscription, allocation CreditAllocation)
```

Where allocation contains:

- plan id
- credits purchased
- validity days
- payment method
- payment metadata

Behavior:

- Set paid plan active.
- Reset reserved credits to zero for the new allocation.
- Set `credits_total` and `credits_remaining` from checkout snapshot.
- Set `credits_expire_at` from validity days.
- Preserve existing metadata not related to pending checkout.

Open decision:

- Current implementation behaves like plan renewal/replacement.
- If product wants top-up behavior, credits should be added to remaining balance instead of replaced. This should be decided before coding.

### 4.6 Webhook Idempotency

Modify: `internal/interfaces/http/handlers/payment_handler.go`

Webhook behavior:

- Verify Razorpay webhook signature.
- For `payment.captured`, resolve tenant/order from notes.
- If payment/order already processed, return 200 without changing credits.
- If UI verification already completed, do nothing.
- If UI verification did not complete, apply the pending checkout snapshot.
- For `payment.failed`, record failure metadata only.

Idempotency keys:

- `last_payment_id`
- `last_order_id`
- optional `processed_payment_ids` metadata list if needed

### 4.7 Backend Billing APIs

Modify: `internal/interfaces/http/handlers/billing_handler.go`

Add user-facing endpoint:

- `GET /v1/billing/plans`

Response:

```json
{
  "currency": "INR",
  "active_version": "v3",
  "plans": [
    {
      "id": "starter",
      "name": "Starter",
      "amount_paisa": 50000,
      "currency": "INR",
      "credits_included": 2000,
      "validity_days": 365,
      "active": true
    }
  ]
}
```

Add admin endpoints:

- `GET /v1/admin/billing/plans`
- `POST /v1/admin/billing/plans/set`
- `POST /v1/admin/billing/rates/activate` continues to activate whole billing versions

Plan set request:

```json
{
  "id": "starter",
  "name": "Starter",
  "amount_paisa": 50000,
  "currency": "INR",
  "credits_included": 2000,
  "validity_days": 365,
  "active": true,
  "display_order": 20
}
```

### 4.8 Routes

Modify: `internal/interfaces/http/routes/routes.go`

Add:

- `billing.Get("/plans", c.BillingHandler.GetPlans)`
- `adminBilling.Get("/plans", c.BillingHandler.AdminGetPlans)`
- `adminBilling.Post("/plans/set", c.BillingHandler.AdminSetPlan)`

Do not remove old tenant billing routes.

## 5. Frontend Design

### 5.1 Types

Modify: `ui/src/types/index.ts`

Add:

```ts
export interface BillingPlanBundle {
  id: string;
  name: string;
  amount_paisa: number;
  currency: string;
  credits_included: number;
  validity_days: number;
  active: boolean;
  display_order?: number;
}
```

### 5.2 API Client

Modify: `ui/src/services/api.ts`

Add:

- `billingAPI.getPlans()`
- typed `checkoutBilling(planId: string)`
- typed `verifyPayment(payload)`

### 5.3 Billing UI

Modify:

- `ui/src/pages/WorkspaceBilling.tsx`
- `ui/src/hooks/useRazorpayCheckout.ts`
- `ui/src/components/PricingSection.tsx`
- `ui/src/pages/Pricing.tsx`
- `ui/src/pages/LandingPage.tsx`

Required changes:

- Fetch live plans from `/v1/billing/plans` for authenticated checkout.
- Stop hardcoding `initiateCheckout("pro")`.
- Pass selected plan id to checkout.
- Display amount and credits from backend.
- Keep static `ui/src/constants/pricing.ts` only as public/unauthenticated fallback.

User behavior:

- If authenticated and clicks Starter, checkout starts for `starter`.
- If DB changed Starter to INR 500 + 2000 credits, UI displays and buys that bundle.

### 5.4 Razorpay Hook

Modify: `ui/src/hooks/useRazorpayCheckout.ts`

Use backend response for:

- key id
- order id
- amount
- currency
- plan name
- credits included

Do not derive amount or credits in the browser.

## 6. Environment Changes

Existing env variables are enough:

```env
FREERANGE_PAYMENT_PROVIDER=razorpay
FREERANGE_PAYMENT_RAZORPAY_KEY_ID=
FREERANGE_PAYMENT_RAZORPAY_KEY_SECRET=
FREERANGE_PAYMENT_RAZORPAY_WEBHOOK_SECRET=
FREERANGE_PAYMENT_RAZORPAY_CURRENCY=INR
```

No new env variable is required for plan customization. Plan customization belongs in DB.

Optional future env:

- `FREERANGE_PAYMENT_RAZORPAY_WEBHOOK_TOLERANCE_SECONDS`

Only add this if webhook replay tolerance becomes configurable.

## 7. Backward Compatibility

Must preserve production behavior for existing 21 businesses:

- Existing subscriptions remain valid.
- Existing `free`, `starter`, `pro`, `growth`, `scale` names remain valid.
- Existing active rate cards without `plans` fallback to code defaults.
- Existing pending checkouts with only `pending_checkout_tier` still verify using old fallback logic.
- Existing tenant billing APIs stay mounted but remain deprecated/UI-unused.
- Existing credit fields on subscription are not renamed.
- No destructive migration.

Safe rollout:

1. Deploy code that supports both old and new plan sources.
2. Let bootstrap create default plans if DB plans are missing.
3. Add admin/custom plan through DB.
4. Verify `/v1/billing/plans`.
5. Enable UI plan selection.

## 8. Tests

### 8.1 Unit Tests

Add/extend:

- `internal/usecases/services/ratecard_service_test.go`
  - DB plan overrides defaults.
  - Missing DB plans fallback to defaults.
  - Inactive plan is not checkoutable.
  - INR 500 + 2000 credits resolves correctly.

- `internal/interfaces/http/handlers/payment_handler_test.go`
  - checkout uses DB amount and credits.
  - free plan cannot create payment order.
  - active free subscription can upgrade.
  - active paid subscription blocks duplicate checkout.
  - verification allocates snapshot credits.
  - verification still works if DB plan changes after order creation.
  - mismatched order id is rejected.

- `internal/infrastructure/payment/razorpay_test.go`
  - receipt is short and unique.
  - payment signature verification uses Razorpay order/payment format.

- `internal/interfaces/http/handlers/billing_handler_test.go`
  - `/v1/billing/plans` returns active DB plans.
  - fallback plans are returned when DB has no plans.

### 8.2 Integration Tests

Add/extend under `tests/integration/`:

- Seed active billing config with custom Starter:
  - INR 500
  - 2000 credits
- Create checkout.
- Verify payment through mock provider or deterministic test provider.
- Assert subscription has exactly:
  - `plan = starter`
  - `credits_total = 2000`
  - `credits_remaining = 2000`
  - payment metadata from snapshot
- Assert old rate-card document without `plans` does not break billing APIs.
- Assert webhook replay is idempotent.

### 8.3 Frontend Verification

Run:

```powershell
cmd /c npm run build
```

If UI test infrastructure exists later:

- plan card click sends selected plan id
- billing page displays live backend credits
- checkout hook uses backend amount, not constants

## 9. Documentation Updates

### 9.1 `documents/`

Update:

- `documents/razorpay-payment-integration-spec.md`
  - Add DB-backed credit plan catalog.
  - Add checkout snapshot rules.
  - Add idempotent webhook behavior.
  - Link Razorpay official docs.

- `documents/plans/PRICING_PLAN.md`
  - State that hardcoded plan values are fallback defaults.
  - State that production pricing comes from active DB billing configuration.

- `documents/API_DOCUMENTATION.md`
  - Add `/v1/billing/plans`.
  - Add admin billing plan endpoints.
  - Add checkout response shape.

- `documents/CLI_ADMIN_REFERENCE.md`
  - If CLI support is added, document how admins update plan bundles.

### 9.2 UI Official Docs

Update:

- `ui/src/docs/pricing.md`
  - Explain credit packs are served from active billing configuration.
  - Keep examples but mark values as current defaults.
  - Mention admins may configure custom credit bundles.

Optional:

- Add `ui/src/docs/billing.md` only if pricing docs become too large.

### 9.3 External Official References

Use these Razorpay docs while implementing:

- Standard Checkout integration: `https://razorpay.com/docs/payments/payment-gateway/web-integration/standard/integration-steps/`
- Orders API: `https://razorpay.com/docs/api/orders/create/`
- Payment signature verification: `https://razorpay.com/docs/payments/payment-gateway/web-integration/standard/integration-steps/#15-verify-payment-signature`
- Webhooks: `https://razorpay.com/docs/webhooks/`

## 10. Acceptance Criteria Mapping

Backend changes:

- DB-backed plan bundles exist.
- Checkout resolves amount and credits from DB.
- Payment verification uses immutable checkout snapshot.
- Webhook is idempotent.
- Existing plans and subscriptions remain compatible.

Frontend changes:

- Billing UI fetches plans from backend.
- User can select Starter/Pro/Growth/Scale/custom active plans.
- UI no longer hardcodes Pro checkout.
- Browser never computes amount or credits.

`.env` changes:

- No new required env.
- Existing Razorpay env documented and validated.

Unit and integration tests:

- Unit tests cover plan resolution, checkout, verification, webhook idempotency.
- Integration tests prove custom DB credits are allocated.

Documentation:

- `documents/` architecture/API docs updated.
- `ui/src/docs/pricing.md` updated for official UI docs.

Production compatibility:

- Existing 21 businesses keep working.
- No destructive migration.
- Old rate-card/subscription shapes remain valid.

Custom DB credits:

- If DB says INR 500 gives 2000 credits, checkout charges INR 500 and allocates exactly 2000 credits.

## 11. Open Decisions Before Coding

1. Renewal vs top-up:
   - Should buying a paid pack replace the current credit wallet, or add to remaining credits?

2. Multiple active paid purchases:
   - Should already-paid users be blocked from checkout, or allowed to buy another pack as top-up?

3. Credit expiry:
   - One expiry date on subscription is current behavior.
   - True per-allocation expiry requires ledger-aware wallet lots and is a bigger change.

4. Admin UI:
   - Should custom plan editing be API-only for now, or exposed in dashboard?

5. Enterprise/custom plan:
   - Should enterprise be checkout-disabled contact-us only, or support custom Razorpay payment links later?
