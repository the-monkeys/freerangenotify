# Pricing Rebalance — Fair for Customers, Profitable for Us

> **Branch:** `feature/razorpay-credit-billing`
> **Date:** 2026-05-31
> **Status:** APPROVED — Implementing via DB migration (no code changes to `rates.go`)

---

## 1. Problem Statement

The current credit-based pricing has several issues that hurt both customers and us:

| Issue | Impact |
|-------|--------|
| Email sold at a loss (₹0.03 charge vs ₹0.021 carrier cost — zero margin after GST) | We lose money on every email |
| SMS/WhatsApp overage rates are 5–8× carrier cost | Bill shock → customer churn |
| Free tier too small (500 credits = 6 SMS messages) | Developers bounce before evaluating |
| "Lite" plan (₹155) is confusing | Too small, sits awkwardly between Free and Starter |
| `CarrierCosts()` in code uses SES pricing, not Zoho | Internal tracking is wrong |

---

## 2. Actual Carrier Costs (What We Pay)

### Email — Zoho ZeptoMail
- **₹0.021 per email** ($2.50 per 10,000 emails, ~₹208/10k)
- Pay-as-you-go, credits valid 6 months
- Transactional only (OTPs, confirmations, alerts)

### SMS — Twilio (India outbound)
- **₹6.93 per SMS segment** ($0.0832/segment at ~₹83.3/USD)
- Additional carrier surcharges may apply
- DLT compliance costs managed separately

### WhatsApp — Twilio + Meta
- **Utility/Authentication:** ₹0.56/msg (Meta ₹0.145 + Twilio $0.005 ≈ ₹0.42)
- **Marketing:** ₹1.51/msg (Meta ₹1.09 + Twilio $0.005 ≈ ₹0.42)
- Service window messages (customer-initiated within 24h): Meta free, Twilio ₹0.42 only

### Push / In-App / SSE / Webhook
- **₹0.00** — FCM/APNS are free, webhooks are self-hosted

---

## 3. BYOC (Bring Your Own Credentials)

When customers configure their own provider credentials (Twilio account, SMTP server, SendGrid API key, Meta WhatsApp credentials), they pay the carrier directly. We charge only a **platform fee** for orchestration, routing, and delivery tracking.

**Detection is already implemented** in `InferCredentialSource()` (`internal/domain/billing/legacy.go`):
- Email: Customer has own SMTP host or SendGrid API key → `byoc`
- WhatsApp: Customer has own Twilio SID+Token or Meta credentials → `byoc`
- SMS: Customer has own Twilio SID+Token → `byoc`

**Live billing path** is in `credit_service.go` (`CreditService.reserveCredits()` → `RateCardManager.GetChannelCreditCost()`). `calculator.go` exists but is **not** wired into the live credit-deduction flow — it's reserved for invoice generation. Source-based dispatch:
- `CredSourceSystem` → deduct credits from wallet via `CreditService`
- `CredSourceBYOC` → flat platform fee per message (no credit deduction)
- `CredSourcePlatform` → free (push/inapp/SSE/webhook)

> **Note:** The live path does **not** charge overage. `CreditService.reserveCredits()` only deducts credits and rejects when the wallet is empty. Overage rates currently only matter for `calculator.go` (future invoice generation) and the legacy quota path.

---

## 4. Proposed New Rate Card

### 4.1 Credit Value
**1 credit = ₹0.01** (unchanged — keeps math simple for customers)

### 4.2 Channel Credit Costs (system credentials)

| Channel | Current Credits | New Credits | ₹ Per Msg | Our Cost | Margin | Markup |
|---------|----------------|-------------|-----------|----------|--------|--------|
| In-App | 1 | **1** | ₹0.01 | ₹0.00 | ₹0.01 | ∞ |
| Webhook | 1 | **1** | ₹0.01 | ₹0.00 | ₹0.01 | ∞ |
| SSE | 1 | **1** | ₹0.01 | ₹0.00 | ₹0.01 | ∞ |
| Email | 3 | **5** | ₹0.05 | ₹0.021 | ₹0.029 | 2.4× |
| WhatsApp | 108 | **80** | ₹0.80 | ₹0.56 | ₹0.24 | 1.4× |
| SMS | 80 | **800** | ₹8.00 | ₹6.93 | ₹1.07 | 1.15× |

> **SMS:** Twilio India outbound is $0.0832/segment (₹6.93). We price at ₹8.00 to maintain a thin margin instead of taking a loss. Customers who need cheap SMS at scale should configure BYOC (their own Twilio account) and pay only the ₹0.05 platform fee.

### 4.3 Overage Rates (after credits exhausted)

| Channel | Current (paisa/msg) | New (paisa/msg) | ₹ Per Msg | vs In-Plan |
|---------|--------------------|--------------------|-----------|------------|
| In-App | 3 | **2** | ₹0.02 | 2× |
| Email | 6 | **8** | ₹0.08 | 1.6× |
| WhatsApp | 200 | **120** | ₹1.20 | 1.5× |
| SMS | 150 | **1200** | ₹12.00 | 1.5× |

**Design principle:** Overage is 1.5–2× in-plan price. Enough incentive to upgrade, but not punishing.

> **Note:** Overage rates currently only affect future invoice generation (`calculator.go`) and the legacy quota path. The live credit-deduction flow in `credit_service.go` rejects requests when the wallet hits zero; it does not auto-charge overage.

### 4.4 BYOC Platform Fees

| Channel | Current (paisa/msg) | New (paisa/msg) | ₹ Per Msg |
|---------|--------------------|--------------------|-----------|
| Email | 2 | **2** | ₹0.02 |
| WhatsApp | 3 | **5** | ₹0.05 |
| SMS | 3 | **5** | ₹0.05 |
| Push | 0 | **0** | Free |

**Rationale:** BYOC customers pay their own carrier costs. Our ₹0.02–₹0.05/msg covers the cost of running the orchestration infrastructure (routing, retries, delivery tracking, analytics).

### 4.5 Plan Bundles

| Plan | Price | Credits | Emails | WhatsApp | SMS | ₹/Credit | Display Order |
|------|-------|---------|--------|----------|-----|----------|---------------|
| **Free** | ₹0 | 1,500 | 300 | 18 | 1 | free | 10 |
| **Starter** | ₹499 | 35,000 | 7,000 | 437 | 43 | ₹0.0143 | 20 |
| **Pro** | ₹1,499 | 120,000 | 24,000 | 1,500 | 150 | ₹0.0125 | 30 |
| **Growth** | ₹4,999 | 450,000 | 90,000 | 5,625 | 562 | ₹0.0111 | 40 |
| **Scale** | ₹14,999 | 1,600,000 | 320,000 | 20,000 | 2,000 | ₹0.0094 | 50 |

**Key changes:**
- **Free:** 500 → 1,500 credits (3× increase). Developers can evaluate without burning the tier in a day.
- **Lite plan removed:** ₹155 for 4,800 credits was too small. Clean tier progression now.
- **Starter:** ₹500 → ₹499 (psychology pricing), 22,000 → 35,000 credits.
- **Pro:** Credits 92,000 → 120,000 (round number, easier to communicate).
- **Growth/Scale:** Adjusted proportionally to new credit costs.
- **All plans:** Validity 365 days (unchanged).

### 4.6 Volume Discount Curve

Higher plans get cheaper per-credit rates:

```
Free:    free (₹0/credit)
Starter: ₹0.0143/credit  (baseline)
Pro:     ₹0.0125/credit  (12% discount)
Growth:  ₹0.0111/credit  (22% discount)
Scale:   ₹0.0094/credit  (34% discount)
```

---

## 5. Revenue Projections

### Per-Channel Revenue at Pro Plan (100,000 credits)

| If customer sends | Volume | Credits Used | Revenue | Our Cost | Gross Margin |
|-------------------|--------|-------------|---------|----------|-------------|
| 20,000 emails | 100% email | 100,000 | ₹1,499 | ₹420 | ₹1,079 (72%) |
| 500 SMS | 100% SMS | 100,000 | ₹1,499 | ₹3,465 | −₹1,966 (loss) |
| 1,250 WhatsApp | 100% WA | 100,000 | ₹1,499 | ₹700 | ₹799 (53%) |
| Blended (typical) | 70/10/20 | 100,000 | ₹1,499 | ₹595 | ₹904 (60%) |

**Typical customer mix (70% email, 10% SMS, 20% WhatsApp) yields ~60% gross margin** on the Pro plan. SMS-only customers are rare and would likely use BYOC (their own Twilio).

---

## 6. Backward Compatibility

### 6.1 Legacy Billing Model (pre-credit customers)
- Code path: `BillingModel(sub) == "legacy"` → uses `legacy.go` quota system
- **Zero changes.** Legacy rate cards, quota enforcement, and overage rates untouched.
- Legacy customers continue until they explicitly upgrade to a credit plan.

### 6.2 Existing Credit Billing Customers
- Rate card is versioned in Elasticsearch (`billing_rate_cards` index)
- New version created → old version stays in the index
- In-flight reservations store `rate_card_version` and settle at old prices
- Customer credit balances are **not recalculated** — they keep remaining credits
- New credit costs apply to **new reservations only**

### 6.3 BYOC Customers
- `InferCredentialSource()` logic unchanged
- Detection of own credentials (SMTP host, SendGrid API key, Twilio SID, Meta tokens) works the same
- Only platform fee amounts change (SMS/WA: 3→5 paisa)

### 6.4 Schema Compatibility
- `RateCard` struct: no new fields, same JSON shape
- `PlanBundle` struct: no new fields
- Elasticsearch index mappings: no changes
- API responses: same shape, different values

---

## 7. Configurability (Offers & Promotions)

The rate card system is already fully configurable via admin CLI:

### Adjust credit costs on the fly
```bash
# Holiday promo: reduce WhatsApp cost temporarily
frn billing rates set --channel whatsapp --credits 40 --admin-token $TOKEN

# Revert after promo
frn billing rates set --channel whatsapp --credits 80 --admin-token $TOKEN
```

### Create promotional plans
```bash
# Diwali special: Starter plan with 50,000 credits instead of 30,000
frn billing plans set --id starter --name "Starter (Diwali)" \
  --amount-paisa 49900 --credits-included 50000 --admin-token $TOKEN
```

### Grant bonus credits to a specific user
```bash
# Give a customer 10,000 bonus credits (ops-only command)
frn admin grant-credits --user-id <id> --amount 10000 \
  --reason "launch promo" --ops-secret $OPS_SECRET
```

### Version management
```bash
# Activate new rates
frn billing rates activate --version <version> --admin-token $TOKEN

# Something went wrong? Rollback instantly
frn billing rates rollback --version <old-version> --admin-token $TOKEN
```

### Rate card refresh
- Active rate card is cached in-memory, refreshed every 45 seconds (`config.yaml: billing.rate_card_refresh_seconds: 45`)
- Instant invalidation via Redis pub/sub channel (`billing:ratecard:updated`)
- All running server instances pick up changes without restart

---

## 8. Implementation

**Approach: pure DB migration. No changes to `rates.go`, handlers, routes, or services.** Pricing has to remain customizable at runtime (offers, promos, per-channel tweaks) — hardcoding into `DefaultRates()` would force a redeploy for every price change. The migration writes the new rate-card version + plan bundles into Elasticsearch and activates them.

### New file

#### `cmd/migrate/rebalance_pricing_2026.go`
Idempotent migration step, registered alongside existing migrations in `cmd/migrate/main.go`. Logic:

1. Check the active rate card. If `version == "rebalance-2026"` already → skip (re-runnable).
2. Build a new `RateCard` with the channel credit costs and overage rates from §4.2 / §4.3.
3. Build the 5 `PlanBundle` documents from §4.5 and attach them to the rate card.
4. Call the same persistence path used by the admin handler (`RateCardService.CreateVersion` → `Activate`) so the existing pub/sub broadcast (`billing:ratecard:updated`) fires and all running pods refresh their in-memory cache within seconds.
5. Delete-by-omission: the new rate card simply does not include a `lite` plan. The old rate-card versions retain it for audit, but it disappears from `/v1/billing/plans` because that endpoint reads only from the active card's `Plans` map.
6. **SMS goodwill grants** are intentionally **not** bundled into this migration. If operators choose to offer them, use the existing per-tenant command: `frn admin grant-credits --user-id <id> --amount <n> --reason sms_rebalance_goodwill_2026 --ops-secret $OPS_SECRET`. Keeps the migration single-purpose and reversible.

### Files NOT modified
- `internal/domain/billing/rates.go` — `DefaultRates()` stays as the cold-start fallback; live pricing comes from ES.
- `calculator.go`, `legacy.go`, `credit_service.go`, `ratecard_service.go`, `payment_handler.go`, `subscription_cycle.go` — all read from `RateCardManager`, which the migration updates.
- All billing API handlers (`/v1/billing/*`, `/v1/admin/billing/*`) — unchanged. Same endpoints, same response shapes, new values served from the new rate-card version.
- `WorkspaceBilling.tsx` — unchanged; reads from `/v1/billing/plans`.

### BYOC fee caveat
`BYOCFees` lives on the `PlanTier` Go struct only and is **not** persisted in the `RateCard` ES document. The current values (email 2, whatsapp 3, sms 3 paisa) are already close enough to the target (2/5/5). Bumping whatsapp/sms BYOC from 3→5 paisa is not worth a code change — deferring to a future release.

---

## 9. Deployment Steps

1. **Deploy** the build containing `rebalance_pricing_2026.go`.
2. **Run migration:** `docker-compose exec notification-service /app/migrate` — same command used for first-run bootstrap; the new step is idempotent and skips if `rebalance-2026` is already active.
3. **Pub/sub broadcast** fires automatically → all running pods refresh within 1–2 seconds. No restart required.
4. **Verify:**
   - `curl /v1/billing/rates` → see new channel costs
   - `curl /v1/billing/plans` → see 5 plans (no `lite`)
   - Send a test notification → confirm credit deduction matches new rates
   - UI → reload `WorkspaceBilling.tsx`, confirm new prices/credits render
5. **Optional SMS goodwill grants** — use the existing per-tenant CLI; not part of this migration.
6. **Rollback** (if needed):
   ```bash
   curl -X POST /v1/admin/billing/rates/rollback \
     -H "Authorization: Bearer $ADMIN_JWT" \
     -d '{"version":"<previous-version>"}'
   ```

---

## 10. Resolved Decisions

| # | Question | Decision |
|---|----------|----------|
| 1 | SMS carrier | Twilio India outbound (₹6.93/segment) |
| 2 | Grandfathering | No. New rates apply to new reservations only; existing wallet balances untouched. Optional SMS goodwill credit grant available. |
| 3 | WhatsApp template types | Flat (₹0.80 / 80 credits). Carrier-cost split tracked internally only. |
| 4 | Lite plan | Drop. Deleted by migration. |
| 5 | SMS pricing | Price at cost + thin margin (₹8.00 = 800 credits). Customers who need cheap SMS use BYOC. |
| 6 | Existing 21 tenants | Auto-migrate. Subscriptions untouched. Pricing change takes effect on next reservation. SMS-heavy users may get one-time goodwill credits. |
