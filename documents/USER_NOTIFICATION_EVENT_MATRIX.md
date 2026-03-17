# User Notification Event Matrix

## Purpose
This document defines when users should receive notifications across:
- In-app notifications
- Email
- WhatsApp
- SMS

Use this as the product policy reference for event-triggered messaging.

## Channel Rules
- In-app: default channel for product activity and non-urgent updates.
- Email: for important account, billing, invitation, and security events.
- WhatsApp: for high-importance or conversational alerts where user opted in.
- SMS: for critical, time-sensitive alerts and verification fallback.
- Do not send the same low-priority event to all channels by default.
- Respect user preferences, quiet hours, country/consent constraints, and regulatory requirements.

## Event Matrix
Legend: `Y` = should send, `O` = optional/conditional, `N` = do not send by default.

| Event | In-App | Email | WhatsApp | SMS | Trigger | Priority | Notes |
|---|---|---|---|---|---|---|---|
| Registration OTP requested | N | Y | O | O | User starts signup | High | Email primary; WhatsApp/SMS only if configured fallback. |
| Registration OTP resent | N | Y | O | O | User requests resend | High | Apply resend rate limits. |
| Registration completed | Y | Y | O | N | OTP verified, account created | Medium | Welcome in-app + welcome email. |
| First login success | Y | O | N | N | First successful login | Low | Useful onboarding marker in bell feed. |
| New login from unknown device/location | Y | Y | O | O | Risk signal on login | Critical | Security alert; SMS/WhatsApp only for opted-in security alerts. |
| Password reset requested | N | Y | O | O | Forgot-password flow | High | Avoid revealing account existence. |
| Password successfully changed | Y | Y | O | O | Password reset/change success | Critical | Security confirmation event. |
| MFA/2FA enabled | Y | Y | N | N | User enables MFA | Medium | Security posture update. |
| MFA/2FA disabled | Y | Y | O | O | User disables MFA | Critical | Elevated-risk action; consider SMS fallback. |
| Email address changed | Y | Y | O | O | Profile security update | Critical | Notify old and new email where possible. |
| Phone number changed | Y | Y | O | O | Profile security update | High | Security event, track in audit logs. |
| Account deletion requested | Y | Y | O | O | Deletion initiated | Critical | Include recovery/support instructions if allowed. |
| Account deletion completed | N | Y | N | N | Deletion finalized | Critical | Final compliance confirmation email. |
| Team/app invite received | Y | Y | O | N | Member invited to app/team | High | In-app should deep-link to invite context. |
| Organization invite received | Y | Y | O | N | Member invited to org | High | In-app + email is baseline. |
| Invite accepted | Y | O | N | N | Invite accepted by member | Medium | Notify inviter/admin in-app. |
| Role changed (owner/admin/editor/viewer) | Y | Y | O | N | RBAC change | High | Security-sensitive if privilege reduced/elevated. |
| API key created/rotated/revoked | Y | Y | O | O | Credential lifecycle action | Critical | Security event; optional SMS for high-security tenants. |
| Webhook/provider credentials changed | Y | Y | O | N | Provider config update | High | Operational security event. |
| Subscription trial starting | Y | Y | O | N | Trial provisioned | Medium | Onboarding + value reminder. |
| Trial ending soon (7/3/1 day) | Y | Y | O | O | Scheduled reminders | High | Escalate cadence near expiry. |
| Subscription renewed | Y | Y | O | N | Renewal success | Medium | Include next billing/expiry date. |
| Payment failed | Y | Y | O | O | Billing failure | Critical | SMS optional for near service interruption. |
| Subscription expired | Y | Y | O | O | License/subscription expired | Critical | Blocking state; include remediation path. |
| Notification delivery failed repeatedly (DLQ threshold) | Y | Y | O | O | Reliability alert | Critical | Target app owners/admins only. |
| Provider outage/degradation detected | Y | Y | O | O | Health monitor threshold | Critical | Incident-level operational alert. |
| Daily/weekly usage summary available | Y | O | N | N | Scheduled digest | Low | Keep to in-app unless user subscribed to digest email. |
| Compliance/legal policy update | Y | Y | N | N | Policy revision published | Medium | Email required for legal-significant updates. |

## Delivery Policy by Priority
- Critical: In-app + Email required. WhatsApp/SMS optional but recommended for security and outage events.
- High: In-app or Email required. Add WhatsApp/SMS based on urgency and consent.
- Medium: In-app default. Email optional.
- Low: In-app only, preferably digest/batched.

## Preference and Compliance Requirements
- Require explicit opt-in for WhatsApp and SMS.
- Support per-category notification settings (security, billing, product, marketing).
- Enforce quiet hours for non-critical messages.
- Keep security and billing alerts non-mutable by marketing preferences.
- Log notification decisioning in audit trails (event, channels chosen, suppression reason).

## Implementation Plan (Phase A: Selected 8 Events)
Scope for this phase:
- Registration completed
- First login success
- Password successfully changed
- Organization invite received
- Invite accepted
- Trial ending soon (7/3/1 day)
- Subscription renewed
- Subscription expired

### 1) Shared Foundation
1. Add event codes and template keys (single source of truth).
2. Add one orchestration entry-point for event fanout decisions.
3. Reuse `dashboard_notifications` for in-app bell notifications.
4. Keep email send best-effort but log structured failures.
5. Keep WhatsApp/SMS as optional adapters behind feature/config flags.

Proposed event codes:
- `auth.registration_completed`
- `auth.first_login_success`
- `auth.password_changed`
- `org.invite_received`
- `org.invite_accepted`
- `billing.trial_ending_7d`
- `billing.trial_ending_3d`
- `billing.trial_ending_1d`
- `billing.subscription_renewed`
- `billing.subscription_expired`

### 2) Event-by-Event Implementation

#### A. Registration completed
- Trigger point: `internal/usecases/services/auth_service_impl.go` in `VerifyRegistrationOTP` after successful user creation and token generation.
- In-app: create dashboard notification (`welcome` category).
- Email: send welcome email template.
- WhatsApp/SMS: optional; only if consented and provider configured.
- Idempotency: guard by metadata key (event code + user_id + day).

#### B. First login success
- Trigger point: `internal/usecases/services/auth_service_impl.go` in `Login` and `SSOLogin`.
- Condition: only when `LastLoginAt` was zero/empty before update.
- In-app: onboarding marker notification.
- Email: optional disabled by default.

#### C. Password successfully changed
- Trigger points:
	- `ResetPassword` success path.
	- `ChangePassword` success path.
- In-app: security notification.
- Email: required security confirmation.
- WhatsApp/SMS: optional security alert path.

#### D. Organization invite received
- Current status: already implemented in tenant invite flow.
- Trigger point: `internal/usecases/services/tenant_service_impl.go` in `InviteMember`.
- Action: keep as baseline and align template/event code naming.

#### E. Invite accepted
- Trigger points:
	- Tenant membership accept flow.
	- App membership accept flow.
- In-app: notify inviter/admin user(s).
- Email: optional (off by default).
- Note: unaccepted invites should auto-expire after TTL; no rejection notification required.

#### F. Trial ending soon (7/3/1 day)
- Trigger mechanism: scheduled job/worker scan on `subscriptions` index.
- Data source: `current_period_end`, status in `trial|active`.
- In-app: reminder notifications at day offsets.
- Email: reminder emails at day offsets.
- WhatsApp/SMS: optional only for 1-day reminder and opted-in users.
- Dedupe key: `event_code + tenant_id + period_end + offset_day`.

#### G. Subscription renewed
- Trigger points:
	- Ops renew endpoint (`/v1/ops/subscriptions/renew`).
	- Any future billing webhook renewal path.
- In-app: renewal confirmation.
- Email: required renewal confirmation with next expiry date.

#### H. Subscription expired
- Trigger mechanism: scheduled job marks expiry state transitions.
- In-app: blocking/urgent notification.
- Email: required expiration alert.
- WhatsApp/SMS: optional urgent alert for opted-in users.
- Ensure single transition event per period end.

### 3) Technical Touchpoints
- Auth service: `internal/usecases/services/auth_service_impl.go`
- Tenant invite flow: `internal/usecases/services/tenant_service_impl.go`
- Team invite flow parity: `internal/usecases/services/team_service_impl.go`
- Dashboard notifier: `internal/infrastructure/dashboard_notifier.go`
- Dashboard APIs (bell source): `internal/interfaces/http/handlers/dashboard_notification_handler.go`
- UI bell consumer: `ui/src/components/NotificationBell.tsx`
- Subscription repository: `internal/infrastructure/repository/subscription_repository.go`
- Scheduled job location: worker service (`cmd/worker` + usecase scheduler)

### 4) Delivery Order
1. Foundation: event codes + template registry + event dispatcher.
2. Auth events: registration completed, first login, password changed.
3. Invite response event.
4. Billing events: renewed, expired.
5. Trial reminder scheduler (7/3/1).
6. Optional channels (WhatsApp/SMS) behind flags.

### 5) Testing and Acceptance
1. Unit tests per trigger function (event emitted exactly once).
2. Integration tests for dashboard bell endpoints (`/v1/admin/notifications*`).
3. Integration tests for email side effects (mock provider or SMTP test harness).
4. Scheduler tests for 7/3/1 offsets and dedupe behavior.
5. Backward compatibility check: existing login/register/invite flows remain successful if notification send fails.

### 6) Definition of Done (for each event)
- Event emitted at correct trigger point.
- In-app notification visible in bell feed when required.
- Email delivered (or retry recorded) when required.
- Optional channels only fire with explicit consent + config.
- Idempotency and audit logging validated by tests.

## Phase A Progress

| Event | Event Code | Done | Notes |
|---|---|---|---|
| Registration completed | `auth.registration_completed` | Yes | Implemented in auth registration verify flow (in-app + welcome email). |
| First login success | `auth.first_login_success` | Yes | Implemented in login and SSO first-login condition (in-app). |
| Password successfully changed | `auth.password_changed` | Yes | Implemented in reset/change password success paths (in-app + email). |
| Organization invite received | `org.invite_received` | Existing | Already present in tenant invite flow; kept as baseline behavior. |
| Invite accepted | `org.invite_accepted` | Yes | Implemented via membership claim flow; notifies inviter/admin in-app. |
| Trial ending soon (7/3/1 day) | `billing.trial_ending_{7d,3d,1d}` | No | Pending scheduled worker scan with dedupe keys. |
| Subscription renewed | `billing.subscription_renewed` | Yes | Implemented in ops renew path with in-app + best-effort email fanout. |
| Subscription expired | `billing.subscription_expired` | No | Pending expiry transition worker path and single-fire guard. |

## Suggested Next Implementation Set
1. Implement subscription expiry transition and `billing.subscription_expired` fanout.
2. Add trial reminder scheduler (7/3/1-day sequence) with dedupe keys.
3. Add template registry by event key (`event_code`) to avoid hardcoded message text.
4. Add per-user channel preferences and legal consent flags for WhatsApp/SMS.
5. Add team invite flow parity if any path still bypasses dashboard notifications.
