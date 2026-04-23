# Subscription, Payment, and Limits File Map

Last updated: 2026-04-09

This document lists all significant backend and frontend files related to:
- subscription lifecycle
- payment checkout and verification
- plan and medium-wise quotas/rates
- usage metering and billing breakdowns
- enforcement paths (license gate, daily limits, throttle)

## Canonical File Inventory

### 1) Plan and Medium-Wise Billing Definitions
- [internal/domain/billing/rates.go](../internal/domain/billing/rates.go)
- [internal/domain/billing/calculator.go](../internal/domain/billing/calculator.go)
- [internal/domain/billing/usage.go](../internal/domain/billing/usage.go)
- [internal/domain/billing/provider.go](../internal/domain/billing/provider.go)

### 2) Payment Flow (Checkout, Verify, Webhooks)
- [internal/interfaces/http/handlers/payment_handler.go](../internal/interfaces/http/handlers/payment_handler.go)
- [internal/infrastructure/payment/razorpay.go](../internal/infrastructure/payment/razorpay.go)
- [internal/infrastructure/payment/mock.go](../internal/infrastructure/payment/mock.go)
- [internal/interfaces/http/routes/routes.go](../internal/interfaces/http/routes/routes.go)

### 3) Subscription Lifecycle and Licensing
- [internal/domain/license/models.go](../internal/domain/license/models.go)
- [internal/domain/license/repository.go](../internal/domain/license/repository.go)
- [internal/domain/license/checker.go](../internal/domain/license/checker.go)
- [internal/domain/license/hosted.go](../internal/domain/license/hosted.go)
- [internal/domain/license/self_hosted.go](../internal/domain/license/self_hosted.go)
- [internal/domain/license/options.go](../internal/domain/license/options.go)
- [internal/domain/license/remote_verifier.go](../internal/domain/license/remote_verifier.go)
- [internal/infrastructure/repository/subscription_repository.go](../internal/infrastructure/repository/subscription_repository.go)
- [internal/interfaces/http/middleware/license_check.go](../internal/interfaces/http/middleware/license_check.go)
- [internal/interfaces/http/handlers/licensing_handler.go](../internal/interfaces/http/handlers/licensing_handler.go)
- [internal/interfaces/http/handlers/subscription_cycle.go](../internal/interfaces/http/handlers/subscription_cycle.go)
- [internal/interfaces/http/handlers/renewal_handler.go](../internal/interfaces/http/handlers/renewal_handler.go)
- [internal/interfaces/http/handlers/ops_handler.go](../internal/interfaces/http/handlers/ops_handler.go)
- [internal/interfaces/http/handlers/billing_handler.go](../internal/interfaces/http/handlers/billing_handler.go)
- [internal/usecases/services/auth_service_impl.go](../internal/usecases/services/auth_service_impl.go)

### 4) Billing Usage Metering and Aggregation
- [internal/infrastructure/billingrepo/es_usage_repo.go](../internal/infrastructure/billingrepo/es_usage_repo.go)
- [internal/infrastructure/providers/manager.go](../internal/infrastructure/providers/manager.go)
- [internal/infrastructure/providers/provider.go](../internal/infrastructure/providers/provider.go)

Provider files setting billing metadata (`billing_channel`) and/or credential source for channel metering:
- [internal/infrastructure/providers/apns_provider.go](../internal/infrastructure/providers/apns_provider.go)
- [internal/infrastructure/providers/custom_provider.go](../internal/infrastructure/providers/custom_provider.go)
- [internal/infrastructure/providers/discord_provider.go](../internal/infrastructure/providers/discord_provider.go)
- [internal/infrastructure/providers/fcm_provider.go](../internal/infrastructure/providers/fcm_provider.go)
- [internal/infrastructure/providers/inapp_provider.go](../internal/infrastructure/providers/inapp_provider.go)
- [internal/infrastructure/providers/mailgun_provider.go](../internal/infrastructure/providers/mailgun_provider.go)
- [internal/infrastructure/providers/postmark_provider.go](../internal/infrastructure/providers/postmark_provider.go)
- [internal/infrastructure/providers/resend_provider.go](../internal/infrastructure/providers/resend_provider.go)
- [internal/infrastructure/providers/sendgrid_provider.go](../internal/infrastructure/providers/sendgrid_provider.go)
- [internal/infrastructure/providers/ses_provider.go](../internal/infrastructure/providers/ses_provider.go)
- [internal/infrastructure/providers/slack_provider.go](../internal/infrastructure/providers/slack_provider.go)
- [internal/infrastructure/providers/smtp_provider.go](../internal/infrastructure/providers/smtp_provider.go)
- [internal/infrastructure/providers/sse_provider.go](../internal/infrastructure/providers/sse_provider.go)
- [internal/infrastructure/providers/teams_provider.go](../internal/infrastructure/providers/teams_provider.go)
- [internal/infrastructure/providers/twilio_provider.go](../internal/infrastructure/providers/twilio_provider.go)
- [internal/infrastructure/providers/vonage_provider.go](../internal/infrastructure/providers/vonage_provider.go)
- [internal/infrastructure/providers/webhook_provider.go](../internal/infrastructure/providers/webhook_provider.go)
- [internal/infrastructure/providers/whatsapp_provider.go](../internal/infrastructure/providers/whatsapp_provider.go)

### 5) App/User Limits and Throttle (Non-Plan Limits)
- [internal/usecases/notification_service.go](../internal/usecases/notification_service.go)
- [internal/domain/notification/errors.go](../internal/domain/notification/errors.go)
- [internal/interfaces/http/handlers/notification_handler.go](../internal/interfaces/http/handlers/notification_handler.go)
- [internal/infrastructure/limiter/limiter.go](../internal/infrastructure/limiter/limiter.go)
- [internal/infrastructure/limiter/redis_limiter.go](../internal/infrastructure/limiter/redis_limiter.go)
- [internal/infrastructure/limiter/subscriber_throttle.go](../internal/infrastructure/limiter/subscriber_throttle.go)
- [cmd/worker/processor.go](../cmd/worker/processor.go)
- [cmd/worker/main.go](../cmd/worker/main.go)
- [internal/domain/application/models.go](../internal/domain/application/models.go)
- [internal/interfaces/http/dto/application_dto.go](../internal/interfaces/http/dto/application_dto.go)
- [internal/interfaces/http/handlers/application_handler.go](../internal/interfaces/http/handlers/application_handler.go)
- [internal/usecases/services/application_service_impl.go](../internal/usecases/services/application_service_impl.go)
- [internal/domain/user/models.go](../internal/domain/user/models.go)
- [internal/interfaces/http/dto/user_dto.go](../internal/interfaces/http/dto/user_dto.go)
- [internal/interfaces/http/handlers/user_handler.go](../internal/interfaces/http/handlers/user_handler.go)
- [internal/interfaces/http/middleware/ops_rate_limit.go](../internal/interfaces/http/middleware/ops_rate_limit.go)

### 6) Tenant Billing Surfaces
- [internal/domain/tenant/models.go](../internal/domain/tenant/models.go)
- [internal/interfaces/http/handlers/tenant_handler.go](../internal/interfaces/http/handlers/tenant_handler.go)
- [internal/usecases/services/tenant_service_impl.go](../internal/usecases/services/tenant_service_impl.go)

### 7) Config, Wiring, Indices, and Runtime
- [internal/config/config.go](../internal/config/config.go)
- [internal/container/container.go](../internal/container/container.go)
- [internal/infrastructure/database/index_templates.go](../internal/infrastructure/database/index_templates.go)
- [internal/infrastructure/database/index_manager.go](../internal/infrastructure/database/index_manager.go)
- [internal/infrastructure/database/manager.go](../internal/infrastructure/database/manager.go)
- [cmd/migrate/main.go](../cmd/migrate/main.go)
- [cmd/server/main.go](../cmd/server/main.go)
- [cmd/server/license_runtime.go](../cmd/server/license_runtime.go)
- [internal/platform/licenseheartbeat/service.go](../internal/platform/licenseheartbeat/service.go)
- [config/config.yaml](../config/config.yaml)
- [config/config.prod.yaml](../config/config.prod.yaml)
- [.env](../.env)

### 8) Frontend Billing, Checkout, and Limits UI
- [ui/src/services/api.ts](../ui/src/services/api.ts)
- [ui/src/hooks/useRazorpayCheckout.ts](../ui/src/hooks/useRazorpayCheckout.ts)
- [ui/src/pages/WorkspaceBilling.tsx](../ui/src/pages/WorkspaceBilling.tsx)
- [ui/src/pages/Welcome.tsx](../ui/src/pages/Welcome.tsx)
- [ui/src/pages/VerifyOTP.tsx](../ui/src/pages/VerifyOTP.tsx)
- [ui/src/contexts/AuthContext.tsx](../ui/src/contexts/AuthContext.tsx)
- [ui/src/App.tsx](../ui/src/App.tsx)
- [ui/src/components/Sidebar.tsx](../ui/src/components/Sidebar.tsx)
- [ui/src/types/index.ts](../ui/src/types/index.ts)
- [ui/src/pages/AppDetail.tsx](../ui/src/pages/AppDetail.tsx)
- [ui/src/components/AppUsers.tsx](../ui/src/components/AppUsers.tsx)
- [ui/src/pages/Pricing.tsx](../ui/src/pages/Pricing.tsx)
- [ui/src/pages/LandingPage.tsx](../ui/src/pages/LandingPage.tsx)
- [ui/src/pages/TermsOfService.tsx](../ui/src/pages/TermsOfService.tsx)

### 9) CLI/Admin Subscription and License Operations
- [cmd/frn/admin.go](../cmd/frn/admin.go)
- [cmd/frn/license.go](../cmd/frn/license.go)
- [cmd/frn/install.go](../cmd/frn/install.go)
- [cmd/frn/install_selfhosted.go](../cmd/frn/install_selfhosted.go)

### 10) Tests Related to Subscription/License/Rate Limit Paths
- [internal/interfaces/http/middleware/license_check_test.go](../internal/interfaces/http/middleware/license_check_test.go)
- [internal/domain/license/hosted_test.go](../internal/domain/license/hosted_test.go)
- [internal/domain/license/self_hosted_test.go](../internal/domain/license/self_hosted_test.go)
- [internal/interfaces/http/middleware/ops_rate_limit_test.go](../internal/interfaces/http/middleware/ops_rate_limit_test.go)
- [tests/integration/ops_auth_plane_test.go](../tests/integration/ops_auth_plane_test.go)
- [tests/integration/user_external_id_test.go](../tests/integration/user_external_id_test.go)
- [internal/platform/licenseheartbeat/service_test.go](../internal/platform/licenseheartbeat/service_test.go)
- [cmd/frn/install_selfhosted_test.go](../cmd/frn/install_selfhosted_test.go)

## Compact Matrix

| File | Concern | Defines/Enforces/Displays |
|---|---|---|
| [internal/domain/billing/rates.go](../internal/domain/billing/rates.go) | Plan tiers, per-medium quotas, overage, BYOC/platform fees | Defines |
| [internal/domain/billing/calculator.go](../internal/domain/billing/calculator.go) | Invoice and usage cost computation | Defines |
| [internal/domain/billing/usage.go](../internal/domain/billing/usage.go) | Usage event and summary contracts | Defines |
| [internal/domain/billing/provider.go](../internal/domain/billing/provider.go) | Payment provider interface/contracts | Defines |
| [internal/interfaces/http/handlers/billing_handler.go](../internal/interfaces/http/handlers/billing_handler.go) | Billing usage/subscription/rates APIs | Displays |
| [internal/interfaces/http/handlers/payment_handler.go](../internal/interfaces/http/handlers/payment_handler.go) | Checkout, payment verification, webhook activation | Enforces |
| [internal/interfaces/http/handlers/subscription_cycle.go](../internal/interfaces/http/handlers/subscription_cycle.go) | Renewal cycle metadata and message_limit rollover | Defines |
| [internal/interfaces/http/handlers/renewal_handler.go](../internal/interfaces/http/handlers/renewal_handler.go) | Admin renew behavior and updated limits | Enforces |
| [internal/interfaces/http/handlers/licensing_handler.go](../internal/interfaces/http/handlers/licensing_handler.go) | Subscription CRUD/listing in licensing plane | Enforces |
| [internal/interfaces/http/handlers/ops_handler.go](../internal/interfaces/http/handlers/ops_handler.go) | Ops renewal and subscription extension | Enforces |
| [internal/interfaces/http/middleware/license_check.go](../internal/interfaces/http/middleware/license_check.go) | Blocks protected endpoints with 402 when invalid | Enforces |
| [internal/domain/license/hosted.go](../internal/domain/license/hosted.go) | Hosted subscription validity logic | Enforces |
| [internal/domain/license/self_hosted.go](../internal/domain/license/self_hosted.go) | Self-hosted license validity logic | Enforces |
| [internal/domain/license/models.go](../internal/domain/license/models.go) | Subscription status/lifecycle model | Defines |
| [internal/infrastructure/repository/subscription_repository.go](../internal/infrastructure/repository/subscription_repository.go) | Subscriptions persistence and active lookup | Defines |
| [internal/infrastructure/payment/razorpay.go](../internal/infrastructure/payment/razorpay.go) | Razorpay order/signature/webhook handling | Enforces |
| [internal/infrastructure/payment/mock.go](../internal/infrastructure/payment/mock.go) | Mock payment provider for dev/testing | Defines |
| [internal/infrastructure/billingrepo/es_usage_repo.go](../internal/infrastructure/billingrepo/es_usage_repo.go) | Usage event storage and aggregation by channel/source | Defines |
| [internal/infrastructure/providers/manager.go](../internal/infrastructure/providers/manager.go) | Emits billing usage events on successful sends | Enforces |
| [internal/infrastructure/providers/*_provider.go](../internal/infrastructure/providers) | Channel tagging and credential-source metadata for billing | Defines |
| [internal/usecases/notification_service.go](../internal/usecases/notification_service.go) | App daily email/user daily limits at send path | Enforces |
| [internal/infrastructure/limiter/redis_limiter.go](../internal/infrastructure/limiter/redis_limiter.go) | Sliding-window and daily limit counters | Enforces |
| [internal/infrastructure/limiter/subscriber_throttle.go](../internal/infrastructure/limiter/subscriber_throttle.go) | Per-user per-channel hourly/daily throttle | Enforces |
| [cmd/worker/processor.go](../cmd/worker/processor.go) | Worker-side throttle + license blocking | Enforces |
| [cmd/worker/main.go](../cmd/worker/main.go) | Billing emitter/throttle wiring | Enforces |
| [internal/domain/application/models.go](../internal/domain/application/models.go) | App-level rate_limit, daily_email_limit, subscriber_throttle schema | Defines |
| [internal/domain/user/models.go](../internal/domain/user/models.go) | User-level daily_limit and channel throttle schema | Defines |
| [internal/interfaces/http/handlers/application_handler.go](../internal/interfaces/http/handlers/application_handler.go) | App settings update path for limits | Enforces |
| [internal/interfaces/http/handlers/user_handler.go](../internal/interfaces/http/handlers/user_handler.go) | User settings update path for daily limits | Enforces |
| [internal/interfaces/http/middleware/ops_rate_limit.go](../internal/interfaces/http/middleware/ops_rate_limit.go) | Ops plane request throttling | Enforces |
| [internal/domain/tenant/models.go](../internal/domain/tenant/models.go) | Tenant billing fields (tier, max apps, throughput) | Defines |
| [internal/interfaces/http/handlers/tenant_handler.go](../internal/interfaces/http/handlers/tenant_handler.go) | Tenant billing read endpoint and legacy checkout bridge | Displays |
| [internal/usecases/services/tenant_service_impl.go](../internal/usecases/services/tenant_service_impl.go) | Tenant upgrade billing and active subscription upsert | Enforces |
| [internal/container/container.go](../internal/container/container.go) | Billing feature wiring, rate-card init, payment provider select | Defines |
| [internal/config/config.go](../internal/config/config.go) | Billing/payment/features/limits configuration defaults and validation | Defines |
| [internal/infrastructure/database/index_templates.go](../internal/infrastructure/database/index_templates.go) | ES mappings for subscriptions and related limit fields | Defines |
| [internal/interfaces/http/routes/routes.go](../internal/interfaces/http/routes/routes.go) | Route exposure and middleware attachment for billing/licensing | Enforces |
| [ui/src/services/api.ts](../ui/src/services/api.ts) | Billing and payment frontend API calls | Displays |
| [ui/src/hooks/useRazorpayCheckout.ts](../ui/src/hooks/useRazorpayCheckout.ts) | Frontend checkout and verify-payment flow | Enforces |
| [ui/src/pages/WorkspaceBilling.tsx](../ui/src/pages/WorkspaceBilling.tsx) | Subscription status, quotas, billing breakdown UI | Displays |
| [ui/src/pages/Welcome.tsx](../ui/src/pages/Welcome.tsx) | Trial acceptance UI flow | Displays |
| [ui/src/contexts/AuthContext.tsx](../ui/src/contexts/AuthContext.tsx) | Trial welcome gating state after OTP verify | Displays |
| [ui/src/pages/VerifyOTP.tsx](../ui/src/pages/VerifyOTP.tsx) | Routes user into trial onboarding when needed | Displays |
| [ui/src/pages/AppDetail.tsx](../ui/src/pages/AppDetail.tsx) | App-level limit settings UI | Displays |
| [ui/src/components/AppUsers.tsx](../ui/src/components/AppUsers.tsx) | User-level daily limit settings UI | Displays |
| [ui/src/types/index.ts](../ui/src/types/index.ts) | Billing/subscription/usage/limit TS contracts | Defines |
| [cmd/frn/admin.go](../cmd/frn/admin.go) | Ops CLI renewal command surface | Enforces |
| [cmd/frn/license.go](../cmd/frn/license.go) | CLI license/subscription admin commands | Enforces |
| [internal/interfaces/http/middleware/license_check_test.go](../internal/interfaces/http/middleware/license_check_test.go) | License gate behavior tests | Displays |
| [internal/domain/license/hosted_test.go](../internal/domain/license/hosted_test.go) | Hosted checker tests | Displays |
| [internal/domain/license/self_hosted_test.go](../internal/domain/license/self_hosted_test.go) | Self-hosted checker tests | Displays |

## Notes

- In current architecture, medium-wise plan quotas are defined clearly in billing rate card files and surfaced via billing APIs/UI.
- Hard request/send blocking is primarily enforced by licensing checks, daily limits, and throttle paths.
- `message_limit` appears in billing and cycle metadata/reporting paths; strict per-message hard-stop by plan quota is not the primary enforcement path compared with license/limit middleware and worker checks.
