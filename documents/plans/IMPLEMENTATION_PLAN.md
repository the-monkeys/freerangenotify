## Plan: Credit-Based Pricing Rollout (Updated)

Replace the current message-quota model with a shared credit wallet per workspace, where each delivery burns channel-specific credits (email 15, SMS 700, WhatsApp 1000, in-app/webhook/SSE 1).

Updated policy constraints:
- BYOC must also consume credits.
- BYOC deduction rule: 1 credit per BYOC message for WhatsApp and SMS (and keep this map configurable so other channels can be changed later without code deploy).
- No legacy subscription migration is required right now because there are no active subscriptions.
- Credit rates must be globally changeable through FRN admin CLI commands, and updates should apply to everyone with minimal runtime overhead.

**Target Architecture for Configurability + Performance**
1. Database as source of truth for active billing rate card (versioned documents).
2. In-memory rate-card cache in API + worker processes for hot-path reads (no per-message DB call).
3. Redis pub/sub invalidation channel for near-real-time propagation after CLI updates.
4. Periodic fallback refresh (for missed pub/sub events) every 30-60s.
5. Versioned activation model (`active_version`) so updates are atomic and reversible.

**Steps**
1. Phase 1 - Pricing Domain Refactor (blocks all other steps)
2. Update /Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/domain/billing/rates.go:
3. Add credit-native fields to `PlanTier`: `CreditsIncluded`, `ChannelCreditCost`, `BYOCChannelCreditCost`, `OveragePerMessage`.
4. Keep existing `IncludedQuotas`/`OverageRates` only for short compatibility window, then mark deprecated.
5. Encode PRICING_PLAN bundle values for free/starter/pro/growth/scale.
6. Encode defaults: system credits (email 15, SMS 700, WhatsApp 1000, in-app/webhook/SSE 1).
7. Encode BYOC defaults: WhatsApp 1 credit/message, SMS 1 credit/message.
8. Update /Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/domain/billing/usage.go:
9. Extend `UsageEvent` with `CreditsUsed` and `BillingMode` (`credit_system` | `credit_byoc` | `platform`).
10. Extend `UsageSummary` with `CreditsConsumed`.
11. Create /Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/domain/billing/credits.go:
12. Add `CreditBalance`, `CreditLedgerEntry`, and `CreditReservation` contracts (reserve/commit/release pattern).
13. Add repository interfaces: `CreditBalanceRepository`, `CreditLedgerRepository`, `RateCardRepository`.

14. Phase 2 - Persistence and Index Templates (depends on Phase 1)
15. Update /Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/infrastructure/database/index_templates.go:
16. Add templates for `frn_credit_balances`, `frn_credit_ledger`, and `frn_billing_rate_cards`.
17. Add template for `frn_billing_runtime` (optional single-doc active pointer) or store active version in the rate-card document itself.
18. Extend `subscriptions` mapping with explicit credit snapshot fields (`credits_total`, `credits_remaining`, `credits_expire_at`).
19. Update /Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/infrastructure/database/index_manager.go:
20. Register new billing indices.
21. Update /Users/pranavtripathi/Documents/monkeys/freerangenotify/cmd/migrate/main.go:
22. Keep down/status index list aligned with new billing indices.
23. Create /Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/infrastructure/billingrepo/es_credit_balance_repo.go and /Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/infrastructure/billingrepo/es_credit_ledger_repo.go.
24. Create /Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/infrastructure/billingrepo/es_rate_card_repo.go for versioned rate-card CRUD + activate.

25. Phase 3 - Dynamic Global Rate Config + Admin CLI (new)
26. Create /Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/usecases/services/ratecard_service.go:
27. Load active rate card into in-memory cache (atomic pointer/map).
28. Expose `GetChannelCreditCost(channel, credentialSource)` for hot path.
29. Expose `RefreshActiveRateCard()` and `ActivateVersion(version)`.
30. Publish and subscribe on Redis channel (example: `billing:ratecard:updated`) to invalidate caches cluster-wide.
31. Add fallback periodic refresh ticker in API/worker startup paths.
32. Update /Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/container/container.go:
33. Wire `RateCardService` + cache refresher + pub/sub listener into server and worker.
34. Add CLI command surface under /Users/pranavtripathi/Documents/monkeys/freerangenotify/cmd/frn:
35. New commands (examples):
36. `frn admin billing rates show`
37. `frn admin billing rates set --channel whatsapp --system 1000 --byoc 1`
38. `frn admin billing rates set --channel sms --system 700 --byoc 1`
39. `frn admin billing rates activate --version <v>`
40. `frn admin billing rates rollback --version <v-1>`
41. Command behavior:
42. Write new version to DB.
43. Atomically mark active version.
44. Publish invalidation event to Redis.
45. All API/worker instances refresh local cache; changes apply for everyone without restart.

46. Phase 4 - Subscription, Renewal, and Checkout Credit Allocation (depends on Phases 1-3)
47. Update /Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/domain/license/models.go:
48. Add first-class fields for `credits_total`, `credits_remaining`, `credits_expire_at`.
49. Keep Metadata fallback only during compatibility window.
50. Update /Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/interfaces/http/handlers/subscription_cycle.go:
51. Refactor `applySubscriptionRenewal` for credit allocation and 12-month validity.
52. Update /Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/interfaces/http/handlers/payment_handler.go:
53. On `VerifyPayment`, allocate credits and return updated credit summary.
54. Update /Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/interfaces/http/handlers/renewal_handler.go:
55. Reuse same allocation path for admin renewals.

56. Phase 5 - Enforcement in Delivery Path (depends on Phases 1-4)
57. Create /Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/usecases/services/credit_service.go:
58. Expose `ReserveForNotification`, `CommitOnSuccess`, `ReleaseOnFailure`, `GetUsageSnapshot`.
59. Integrate with `RateCardService` lookup so worker always uses latest active costs from local cache.
60. Update /Users/pranavtripathi/Documents/monkeys/freerangenotify/cmd/worker/processor.go:
61. Before provider send, reserve credits for both system and BYOC sends.
62. For BYOC sends, deduct according to `BYOCChannelCreditCost` map (default 1 for WhatsApp/SMS).
63. On success, commit burn + append ledger entry.
64. On failure/cancel/retry, release reservation.
65. Enforce daily caps for expensive channels (WhatsApp/SMS) from config.
66. Update /Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/infrastructure/providers/manager.go:
67. Keep usage event emission, include `CreditsUsed`, `BillingMode`, and effective rate-card version metadata.
68. Do not deduct credits in manager; worker owns reservation lifecycle.

69. Phase 6 - Billing APIs and Config Surface (depends on Phases 1-5)
70. Update /Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/interfaces/http/handlers/billing_handler.go:
71. `GetUsage`: return `credits_consumed`, `credits_remaining`, `credits_total`, `usage_percent`.
72. `GetUsageBreakdown`: include `message_count` + `credits_consumed` per channel and mode.
73. `GetRates`: return active global rate-card version with system + BYOC credit maps.
74. Add admin endpoints for rate-card introspection/validation if needed.
75. Update /Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/interfaces/http/routes/routes.go for any new billing/admin routes.
76. Update /Users/pranavtripathi/Documents/monkeys/freerangenotify/internal/config/config.go, /Users/pranavtripathi/Documents/monkeys/freerangenotify/config/config.yaml, and /Users/pranavtripathi/Documents/monkeys/freerangenotify/config/config.prod.yaml:
77. Add cache/polling/pubsub config (refresh interval, channel name, fail-open/fail-closed behavior).
78. Keep feature flags for staged rollout.

79. Phase 7 - Frontend Contract + UX (depends on Phase 6)
80. Update /Users/pranavtripathi/Documents/monkeys/freerangenotify/ui/src/types/index.ts:
81. Extend billing contracts with credits fields + rate-card version metadata.
82. Update /Users/pranavtripathi/Documents/monkeys/freerangenotify/ui/src/services/api.ts:
83. Type `getUsage`, `getUsageBreakdown`, `getRates` with credit + BYOC fields.
84. Refactor /Users/pranavtripathi/Documents/monkeys/freerangenotify/ui/src/pages/WorkspaceBilling.tsx:
85. Show BYOC = 1 credit/message policy in breakdown legend.
86. Add active rate-card version and effective-at timestamp for transparency.
87. Refactor pricing data duplication via /Users/pranavtripathi/Documents/monkeys/freerangenotify/ui/src/constants/pricing.ts.
88. Update /Users/pranavtripathi/Documents/monkeys/freerangenotify/ui/src/pages/Pricing.tsx and /Users/pranavtripathi/Documents/monkeys/freerangenotify/ui/src/pages/LandingPage.tsx to display updated credit model.
89. Update /Users/pranavtripathi/Documents/monkeys/freerangenotify/ui/src/hooks/useRazorpayCheckout.ts to show granted credits clearly.

90. Phase 8 - Tests and Validation (depends on all prior phases)
91. Add/Update backend tests:
92. BYOC sends consume 1 credit/message for WhatsApp and SMS.
93. System sends consume configured channel costs.
94. Rate-card update via CLI propagates to all workers without restart.
95. Hot path does not query DB per message when cache is warm.
96. Fallback refresh recovers from missed pub/sub events.
97. Add/Update frontend tests for BYOC policy display and rate-card version rendering.
98. Run smoke tests across checkout, send flow, and runtime rate updates.

**Relevant files**
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

**Verification**
1. `go test ./internal/domain/billing/... ./internal/interfaces/http/handlers/... ./cmd/worker/... ./cmd/frn/...`
2. Validate APIs: `GET /v1/billing/usage`, `GET /v1/billing/usage/breakdown`, `GET /v1/billing/rates`.
3. Confirm BYOC WhatsApp/SMS deduct exactly 1 credit each using active rate-card defaults.
4. Run CLI rate update command and verify changes apply to all workers without restart.
5. Confirm no DB reads in per-message hot path when cache is warm.
6. Validate fallback polling catches missed invalidation events.
7. UI smoke test on billing and pricing screens for BYOC and rate-card visibility.

**Decisions**
- Shared credit pool across channels is required.
- BYOC is not free: BYOC WhatsApp/SMS consume 1 credit/message by default.
- Credit deduction values must be globally configurable via FRN admin CLI.
- DB is source of truth, but delivery hot path reads from in-memory cache to avoid latency.
- Cache invalidation is event-driven (Redis pub/sub) with periodic fallback refresh.
- No legacy subscription migration/backfill scope is needed now.

**Further Considerations**
1. Introduce version pinning option later if some enterprise tenants need frozen rates while others move to latest.
2. Add CLI dry-run/validate mode before activation to prevent accidental global mispricing.
3. Track propagation lag metric (`ratecard_update_to_worker_apply_ms`) for operational confidence.
