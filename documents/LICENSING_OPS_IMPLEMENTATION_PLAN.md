# Licensing + Ops Security Implementation Plan

## Scope
This document defines an implementation plan for:
- Hosted mode (Scenario A): secure ops actions from `frn` CLI for subscription exemption and account deletion.
- Self-hosted mode (Scenario B): installer-led binary deployment with compile-time licensing enforcement.

Goals:
- No regression to existing API behavior for normal users.
- Backward compatibility for current data model and running clusters.
- No new bypass path for subscription or license enforcement.

---

## Architecture Decisions

1. Separate control planes:
- User/API plane: existing JWT + API-key flows remain unchanged.
- Ops plane: new machine-auth routes (`/v1/ops/*`) for privileged actions only.

2. Two build targets:
- Hosted binary: runtime-configurable licensing.
- Self-hosted binary: compile-time locked licensing (`selfhosted` build tag).

3. Self-hosted distribution:
- No Docker image/compose/env distribution for customer installs.
- `frn install` downloads self-hosted binaries and sets up systemd.

4. Preserve existing DB schema compatibility:
- Reuse `subscriptions` index and existing user/app/tenant models.
- Avoid destructive index migrations.

---

## Phase 0: Safety Baseline (No Functional Change)

### 0.1 Add feature flags/config gates
Files:
- `internal/config/config.go`

Changes:
- Add `security.ops_secret` (env: `FREERANGE_SECURITY_OPS_SECRET`).
- Add `security.ops_enabled` (default `false`).
- Validate: if `ops_enabled=true`, `ops_secret` must be non-empty.

Backward compatibility:
- Defaults keep behavior unchanged.
- Existing deployments run without any new mandatory config.

### 0.2 Add rollout metrics/log fields
Files:
- `internal/interfaces/http/middleware/*`
- `internal/interfaces/http/handlers/*`

Changes:
- Structured logs include `auth_plane=user|ops`, `deployment_mode`, `route_group`.
- No response contract changes.

---

## Phase 1: Close Existing Critical Loophole (Hosted + Self-hosted)

### 1.1 Introduce OpsAuth middleware
Files:
- New: `internal/interfaces/http/middleware/ops_auth.go`

Behavior:
- Accept only `Authorization: Bearer ops:<secret>`.
- Constant-time compare with configured `security.ops_secret`.
- Reject all JWT and API-key tokens for ops endpoints.

Security properties:
- No user session token can invoke ops actions.

### 1.2 Split licensing routes by privilege
Files:
- `internal/interfaces/http/routes/routes.go`

Changes:
- Keep read-only endpoints under admin JWT:
  - `GET /v1/admin/licensing/subscriptions`
  - `GET /v1/admin/licensing/subscriptions/:id`
- Move write endpoints to ops routes:
  - `POST /v1/ops/subscriptions`
  - `PUT /v1/ops/subscriptions/:id`
  - `POST /v1/ops/subscriptions/renew`
- Add ops user deletion endpoint:
  - `DELETE /v1/ops/users/:user_id`

Backward compatibility:
- Existing admin dashboard reads remain unchanged.
- Existing clients using admin write endpoints: provide 1-release deprecation shim:
  - Old endpoint returns `410` + migration hint, or internally proxies only when `ops_enabled=false` in dev.

### 1.3 Add OpsHandler
Files:
- New: `internal/interfaces/http/handlers/ops_handler.go`
- Reuse service logic from:
  - `internal/usecases/services/auth_service_impl.go`
  - subscription repo/service

Operations:
- Renew/create/update subscription with strict validation:
  - `months` bounded (1..24)
  - `plan/status` allowlist
  - target identity exists
- Delete user with cascade using existing delete flow (single source of truth).

Backward compatibility:
- No DB schema change.
- Uses existing repositories and indexes.

---

## Phase 2: Deployment-mode Enforcement and Build Separation

### 2.1 Build-tag based licensing overrides
Files:
- New: `internal/config/licensing_default.go` (`//go:build !selfhosted`)
- New: `internal/config/licensing_selfhosted.go` (`//go:build selfhosted`)
- `internal/config/config.go`

Behavior:
- Hosted build: `LicensingOverrides() == nil`.
- Self-hosted build forces:
  - `licensing.enabled=true`
  - `licensing.deployment_mode=self_hosted`
  - `licensing.fail_mode=fail_closed`

Backward compatibility:
- Hosted build remains fully compatible with current env/config behavior.
- Self-hosted behavior is intentionally stricter.

### 2.2 Mode-claim verification
Files:
- `internal/domain/license/self_hosted.go`

Changes:
- Add claim check (example `mode=self_hosted`).
- On mismatch, return `StateInvalid` and refuse licensed actions.

Backward compatibility:
- For already issued keys without `mode` claim, temporary compatibility window:
  - accept missing claim for N weeks with warning logs.
  - after cutover, require claim.

---

## Phase 3: Self-hosted Installer (CLI-driven, systemd)

### 3.1 Rewrite `frn install`
Files:
- `cmd/frn/install.go`
- New: `cmd/frn/systemd.go`
- New: `cmd/frn/download.go`
- New: `cmd/frn/license_verify_remote.go`

Flow:
1. Prompt: ES URL/user/pass, Redis host/port/pass, license key, install path.
2. Preflight: validate ES/Redis reachability.
3. Remote verify: call central endpoint (`api.freerangenotify.com`) with license key.
4. Download self-hosted `server` and `worker` binaries + checksums.
5. Verify checksum/signature.
6. Write config file (infra + license key only).
7. Generate and install systemd units.
8. Start services and health-check.

Important:
- Do not generate docker-compose or .env in self-hosted path.
- Keep existing `frn install` behavior available via explicit `--legacy-docker` for one release if needed.

Backward compatibility:
- Existing users can still run current docker deployment until migration deadline.

### 3.2 CLI config enhancements
Files:
- `cmd/frn/config.go`
- `cmd/frn/config_cmd.go`

Changes:
- Add fields:
  - `OpsSecret`
  - `ReleaseChannel`
  - `InstallerEndpoint`
- Keep old fields (`APIURL`, `APIKey`, `AdminToken`) unchanged.

Backward compatibility:
- Existing config file loads without migration errors.
- New fields optional.

---

## Phase 4: Hosted Ops CLI Commands

### 4.1 New admin command group
Files:
- `cmd/frn/main.go`
- New: `cmd/frn/admin.go`

Commands:
- `frn admin renew-license --user-id ... --months 1 --reason ...`
- `frn admin delete-account --user-id ... --reason ...`

Auth:
- Uses `OpsSecret` only for `/v1/ops/*`.
- Never reuses JWT admin token for ops endpoints.

Safety:
- `--confirm` required for delete.
- Idempotency key header for renew/delete.

Backward compatibility:
- Existing `frn license status/request/attach/verify` remain unchanged.

---

## Phase 5: Self-hosted Remote Verification + Heartbeat

### 5.1 Remote verifier in self-hosted checker
Files:
- `internal/domain/license/self_hosted.go`
- New: `internal/domain/license/remote_verifier.go`

Behavior:
- Periodically verify with central server.
- Cache decision with TTL/grace.
- Fail mode honored (`fail_closed` in self-hosted build).

### 5.2 Heartbeat service
Files:
- New: `internal/platform/licenseheartbeat/*`
- `cmd/server/main.go`

Behavior:
- Emit signed heartbeat every fixed interval.
- Includes instance ID, license fingerprint, version.

Backward compatibility:
- Hosted mode heartbeat optional and off by default.

---

## Backend Backward Compatibility Rules

1. Keep all existing user-facing routes and contracts unchanged.
2. Read-only licensing APIs under `/v1/admin/licensing/*` stay available.
3. Write licensing operations move to `/v1/ops/*` with a deprecation transition.
4. Existing DB indexes (`subscriptions`, `users`, `applications`) remain unchanged.
5. Any added fields in documents must be optional and ignored by old readers.

---

## Database Compatibility Plan

No mandatory schema migration required.

Data rules:
- Reuse `subscriptions` index and `license.Subscription` model.
- Keep `tenant_id/app_id/status/current_period_*` semantics unchanged.
- Optional metadata additions only (`updated_by`, `change_reason`, `source=ops_cli`).

Index safety:
- If mappings need extension for metadata fields, use additive mapping update.
- Never rename/delete existing fields.

---

## Frontend Impact

Current requirement does not need frontend changes.

Optional (later):
- Show read-only banner in Billing page for "ops-granted exemption".
- No UI for dangerous ops actions.

Files if later needed:
- `ui/src/pages/TenantDetail.tsx`
- `ui/src/services/api.ts`

Backward compatibility:
- UI continues using existing billing endpoints unchanged.

---

## Security Validation Checklist

Hosted (Scenario A):
- [x] Regular JWT cannot call `/v1/ops/*`.
- [x] API key cannot call `/v1/ops/*`.
- [x] Ops secret rotation documented (restart currently required).
- [x] Subscription write paths are only in ops plane.
- [x] Audit log created for every renew/delete.

Self-hosted (Scenario B):
- [x] Binary built with `selfhosted` tag forces licensing on.
- [x] Env/config cannot disable licensing.
- [x] Mode mismatch (`hosted` config on self-hosted key) is rejected.
- [x] Installer refuses invalid license before binary download.
- [x] No docker-compose/.env emitted by installer.
- [x] Remote verify and heartbeat work through proxies/firewalls (documented).

Cross-cutting:
- [x] Replay-protected ops requests (idempotency key + timestamp tolerance).
- [x] Rate-limit `/v1/ops/*` aggressively.
- [x] Unit tests for new auth plane middleware.
- [x] Integration tests for new auth plane.

Operational notes:
- Ops secret rotation: current middleware captures the secret at route setup time; rotate by updating config and restarting server.
- Proxy/firewall support: remote verification and heartbeat use Go `http.Client` with default proxy behavior (`HTTP_PROXY`, `HTTPS_PROXY`, `NO_PROXY`). Ensure outbound allow rules for licensing verify/heartbeat endpoints.

---

## Test Plan

1. Unit tests
- `ops_auth` token parsing/validation/constant-time compare.
- route registration matrix by mode + flag.
- self-hosted licensing override behavior.

2. Integration tests
- Hosted:
  - normal user JWT denied on ops routes.
  - ops secret accepted.
  - renew/delete affect expected records only.
- Self-hosted:
  - install flow blocks invalid license.
  - systemd units generated correctly.
  - licensing cannot be disabled via config edits.

3. Regression tests
- Existing auth, billing, notifications, and delete-own-account flows remain unchanged.

---

## Rollout Strategy

1. Release A:
- Ship ops auth + route split + deprecation headers.
- Keep old write endpoints disabled by default.

2. Release B:
- Enable new `frn admin` commands internally.
- Start logging attempted access to deprecated routes.

3. Release C:
- Remove old write endpoints.
- Enforce self-hosted installer path for new customers.

---

## Residual Risk Statement (Honest)

There is no absolute "zero loophole" in an open-source project if an attacker recompiles source with protections removed. What we can do is:
- Eliminate runtime misconfiguration loopholes.
- Move privileged actions to a separate auth plane.
- Enforce compile-time licensing in self-hosted binary.
- Add remote verification + heartbeat for tamper detection.
- Back everything with legal licensing terms.

This is industry-standard practical security for open-core/self-hosted licensing systems.
