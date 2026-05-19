## Plan: Credit-Based Pricing Rollout (Aligned with `PRICING_PLAN.md`)

`PRICING_PLAN.md` is the source of truth for all pricing and credit constants. This plan defines implementation steps that must stay aligned with it.

Core model to implement:

- Shared credit wallet per workspace.
- Base unit: **1 credit = ₹0.01**.
- Channel burn rates:
  - In-app / Webhook / SSE: **1 credit**
  - Email: **3 credits**
  - SMS: **80 credits**
  - WhatsApp: **108 credits**
- Credits valid for 12 months; unused credits expire.
- Guardrails are mandatory: daily caps (WhatsApp/SMS), API key rate limits, spike detection, no negative balances without billing approval.

## Canonical Pricing Snapshot (must match `PRICING_PLAN.md`)

### Tier credit bundles

| Tier                      | Price   | Credits  |
| ------------------------- | ------- | -------- |
| Free (1 month onboarding) | ₹0      | 500      |
| Starter                   | ₹500    | 15,000   |
| Pro                       | ₹1,499  | 55,000   |
| Growth                    | ₹4,999  | 1,85,000 |
| Scale                     | ₹14,999 | 5,50,000 |

Free-tier hard caps:

- WhatsApp: up to 2/day
- SMS: up to 3/day

### Overage pricing

| Channel  | Overage Charge |
| -------- | -------------- |
| In-app   | ₹0.03          |
| Email    | ₹0.06          |
| SMS      | ₹1.50          |
| WhatsApp | ₹2.00          |

## Target Architecture for Configurability + Performance

1. Database as source of truth for active billing rate card (versioned documents).
2. In-memory rate-card cache in API + worker processes for hot-path reads (no per-message DB call).
3. Redis pub/sub invalidation channel for near-real-time propagation after CLI updates.
4. Periodic fallback refresh (every 30-60s) to recover from missed pub/sub events.
5. Versioned activation model (`active_version`) so updates are atomic and reversible.

## Steps

### 1. Phase 1 - Pricing Domain Refactor (blocks all other phases)

1. Update `/Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/domain/billing/rates.go`:
   - Add credit-native fields to `PlanTier`: `CreditsIncluded`, `ChannelCreditCost`, `OveragePerMessage`, `CreditValueINR`.
   - Keep legacy `IncludedQuotas`/`OverageRates` only for short compatibility window, then mark deprecated.
   - Encode tier bundles exactly from `PRICING_PLAN.md` (Free/Starter/Pro/Growth/Scale).
   - Encode channel credit costs exactly: in-app/webhook/SSE 1, email 3, SMS 80, WhatsApp 108.
   - Encode overage prices exactly: in-app 0.03, email 0.06, SMS 1.50, WhatsApp 2.00.
2. Update `/Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/domain/billing/usage.go`:
   - Extend `UsageEvent` with `CreditsUsed` and `RateCardVersion`.
   - Extend `UsageSummary` with `CreditsConsumed` and `OverageAmount`.
3. Create `/Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/domain/billing/credits.go`:
   - Add `CreditBalance`, `CreditLedgerEntry`, and `CreditReservation` contracts (reserve/commit/release).
   - Add repository interfaces: `CreditBalanceRepository`, `CreditLedgerRepository`, `RateCardRepository`.

### 2. Phase 2 - Persistence and Index Templates (depends on Phase 1)

1. Update `/Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/infrastructure/database/index_templates.go`:
   - Add templates for `frn_credit_balances`, `frn_credit_ledger`, and `frn_billing_rate_cards`.
   - Add optional runtime pointer index (`frn_billing_runtime`) if active version is decoupled.
   - Extend `subscriptions` mapping with `credits_total`, `credits_remaining`, `credits_expire_at`.
2. Update `/Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/infrastructure/database/index_manager.go` and `/Users/pranavtripathi/Documents/monkeys/freerangenotify/cmd/migrate/main.go`:
   - Register and manage new billing indices in up/down/status paths.
3. Create:
   - `/Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/infrastructure/billingrepo/es_credit_balance_repo.go`
   - `/Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/infrastructure/billingrepo/es_credit_ledger_repo.go`
   - `/Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/infrastructure/billingrepo/es_rate_card_repo.go`

### 3. Phase 3 - Dynamic Global Rate Config + Admin CLI (depends on Phase 2)

1. Create `/Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/usecases/services/ratecard_service.go`:
   - Load active rate card into in-memory cache.
   - Expose `GetChannelCreditCost(channel)` for delivery hot path.
   - Expose `RefreshActiveRateCard()` and `ActivateVersion(version)`.
   - Publish and subscribe on Redis channel (e.g., `billing:ratecard:updated`) for invalidation.
2. Update `/Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/container/container.go`:
   - Wire `RateCardService` + cache refresher + pub/sub listener into API and worker processes.
3. Add CLI commands under `cmd/frn` (documented in [CLI_ADMIN_REFERENCE.md](../CLI_ADMIN_REFERENCE.md)):
   - `frn admin billing rates show`
   - `frn admin billing rates set --channel email --credits 3`
   - `frn admin billing rates set --channel sms --credits 80`
   - `frn admin billing rates set --channel whatsapp --credits 108`
   - `frn admin billing rates activate --version <v>`
   - `frn admin billing rates rollback --version <v-1>`
   - `frn admin grant-credits --user-id <uuid> --credits <n> --reason "<audit>"`
4. Command behavior:
   - Write new version to DB.
   - Atomically mark active version.
   - Publish invalidation event.
   - All API/worker instances refresh local cache without restart.

### 4. Phase 4 - Subscription, Renewal, and Checkout Credit Allocation (depends on Phases 1-3)

1. Update `/Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/domain/license/models.go`:
   - Add first-class fields for `credits_total`, `credits_remaining`, `credits_expire_at`.
2. Update:
   - `/Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/interfaces/http/handlers/subscription_cycle.go`
   - `/Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/interfaces/http/handlers/payment_handler.go`
   - `/Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/interfaces/http/handlers/renewal_handler.go`
3. Allocation rules:
   - Grant credits by selected tier exactly per `PRICING_PLAN.md`.
   - Set validity to 12 months.
   - Enforce free-tier onboarding window (1 month).

### 5. Phase 5 - Enforcement in Delivery Path (depends on Phases 1-4)

1. Create `/Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/usecases/services/credit_service.go`:
   - `ReserveForNotification`, `CommitOnSuccess`, `ReleaseOnFailure`, `GetUsageSnapshot`.
2. Update `/Users/pranavtripathi/Documents/monkeys/freerangenotify/cmd/worker/processor.go`:
   - Reserve credits before provider send using active channel credit cost.
   - Commit burn + append ledger on success.
   - Release reservation on failure/cancel/retry.
   - Enforce daily caps for WhatsApp and SMS from active plan/guardrail config.
3. Update `/Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/infrastructure/providers/manager.go`:
   - Keep usage event emission; include `CreditsUsed` and `RateCardVersion`.
   - Do not perform direct credit deduction in provider manager.

### 6. Phase 6 - Billing APIs and Config Surface (depends on Phases 1-5)

1. Update `/Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/interfaces/http/handlers/billing_handler.go`:
   - `GetUsage`: return `credits_consumed`, `credits_remaining`, `credits_total`, `usage_percent`.
   - `GetUsageBreakdown`: return `message_count`, `credits_consumed`, `overage_amount` per channel.
   - `GetRates`: return active rate-card version and canonical channel credit costs.
2. Update `/Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/interfaces/http/routes/routes.go` for new billing/admin routes.
3. Update:
   - `/Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/config/config.go`
   - `/Users/pranavtripathi/Documents/monkeys/freerangenotify/config/config.yaml`
   - `/Users/pranavtripathi/Documents/monkeys/freerangenotify/config/config.prod.yaml`
   - Add cache refresh interval, pub/sub channel, and enforcement toggles.

### 7. Phase 7 - Frontend Contract + UX (depends on Phase 6)

1. Update:
   - `/Users/pranavtripathi/Documents/monkeys/freerangenotify/ui/src/types/index.ts`
   - `/Users/pranavtripathi/Documents/monkeys/freerangenotify/ui/src/services/api.ts`
2. Refactor:
   - `/Users/pranavtripathi/Documents/monkeys/freerangenotify/ui/src/constants/pricing.ts` as single pricing source used by UI pages.
3. Update:
   - `/Users/pranavtripathi/Documents/monkeys/freerangenotify/ui/src/pages/WorkspaceBilling.tsx`
   - `/Users/pranavtripathi/Documents/monkeys/freerangenotify/ui/src/pages/Pricing.tsx`
   - `/Users/pranavtripathi/Documents/monkeys/freerangenotify/ui/src/pages/LandingPage.tsx`
   - `/Users/pranavtripathi/Documents/monkeys/freerangenotify/ui/src/hooks/useRazorpayCheckout.ts`
4. UI must show:
   - Exact per-channel burn rates (1/3/80/108).
   - Tier bundle credits and 12-month validity.
   - Overage charges from source pricing plan.
   - Free-tier hard caps (WhatsApp/SMS).
   - Active rate-card version/effective timestamp.

### 8. Phase 8 - Tests and Validation (depends on all prior phases)

1. Add/update backend tests for:
   - Correct credit deduction: in-app/webhook/SSE 1, email 3, SMS 80, WhatsApp 108.
   - Correct tier allocation for Free/Starter/Pro/Growth/Scale.
   - Correct overage charge mapping.
   - Daily cap enforcement for WhatsApp/SMS.
   - Cache-backed hot path behavior and invalidation recovery.
2. Add/update frontend tests for:
   - Pricing and billing screen constants matching source plan.
   - Usage and overage visualization.
3. Run smoke tests across checkout, renewal, send flow, and rate update propagation.

## Relevant files

- /Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/domain/billing/rates.go
- /Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/domain/billing/usage.go
- /Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/domain/billing/credits.go
- /Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/domain/license/models.go
- /Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/interfaces/http/handlers/subscription_cycle.go
- /Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/interfaces/http/handlers/payment_handler.go
- /Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/interfaces/http/handlers/renewal_handler.go
- /Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/interfaces/http/handlers/billing_handler.go
- /Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/interfaces/http/routes/routes.go
- /Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/infrastructure/database/index_templates.go
- /Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/infrastructure/database/index_manager.go
- /Users/pranavtripathi/Documents/monkeys/freerangenotify/cmd/migrate/main.go
- /Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/infrastructure/billingrepo/es_usage_repo.go
- /Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/infrastructure/billingrepo/es_credit_balance_repo.go
- /Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/infrastructure/billingrepo/es_credit_ledger_repo.go
- /Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/infrastructure/billingrepo/es_rate_card_repo.go
- /Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/usecases/services/credit_service.go
- /Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/usecases/services/ratecard_service.go
- /Users/pranavtripathi/Documents/monkeys/freerangenotify/cmd/worker/processor.go
- /Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/infrastructure/providers/manager.go
- /Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/config/config.go
- /Users/pranavtripathi/Documents/monkeys/freerangenotify/config/config.yaml
- /Users/pranavtripathi/Documents/monkeys/freerangenotify/config/config.prod.yaml
- /Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/container/container.go
- /Users/pranavtripathi/Documents/monkeys/freerangenotify/cmd/frn/admin.go
- /Users/pranavtripathi/Documents/monkeys/freerangenotify/cmd/frn/config_cmd.go
- /Users/pranavtripathi/Documents/monkeys/freerangenotify/cmd/frn/billing_rates.go
- /Users/pranavtripathi/Documents/monkeys/freerangenotify/ui/src/types/index.ts
- /Users/pranavtripathi/Documents/monkeys/freerangenotify/ui/src/services/api.ts
- /Users/pranavtripathi/Documents/monkeys/freerangenotify/ui/src/pages/WorkspaceBilling.tsx
- /Users/pranavtripathi/Documents/monkeys/freerangenotify/ui/src/constants/pricing.ts
- /Users/pranavtripathi/Documents/monkeys/freerangenotify/ui/src/pages/Pricing.tsx
- /Users/pranavtripathi/Documents/monkeys/freerangenotify/ui/src/pages/LandingPage.tsx
- /Users/pranavtripathi/Documents/monkeys/freerangenotify/ui/src/hooks/useRazorpayCheckout.ts

## Verification

1. `go test ./internal/domain/billing/... ./internal/interfaces/http/handlers/... ./cmd/worker/... ./cmd/frn/...`
2. Validate APIs:
   - `GET /v1/billing/usage`
   - `GET /v1/billing/usage/breakdown`
   - `GET /v1/billing/rates`
3. Confirm channel burns exactly match source plan: 1/3/80/108.
4. Confirm tier credit allocation matches source plan.
5. Confirm overage mapping matches source plan.
6. Confirm free-tier hard caps are enforced.
7. Confirm no DB reads in per-message hot path when cache is warm.
8. Confirm fallback polling catches missed invalidation events.

## Decisions

- `PRICING_PLAN.md` is canonical; implementation constants must be derived from it.
- Shared credit pool across channels is required.
- Rate card remains globally configurable via FRN admin CLI.
- DB is source of truth; API/worker use in-memory cache for hot path.
- Cache invalidation is Redis pub/sub with periodic fallback refresh.
- No legacy subscription migration/backfill is currently required.

## Further considerations

1. Add CLI dry-run/validate before rate activation to avoid mispricing.
2. Add propagation lag metric (`ratecard_update_to_worker_apply_ms`) for operational confidence.
3. Optionally add enterprise rate-card pinning in a later phase.
