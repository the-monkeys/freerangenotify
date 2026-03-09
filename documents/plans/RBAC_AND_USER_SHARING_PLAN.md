# Implementation Plan: RBAC Enforcement & Cross-App User Sharing

**Status**: Draft  
**Date**: 2026-03-08  
**Backward Compatibility**: Required (existing API contracts, ES indices, and data must continue to work)

---

## Part A: Fix RBAC So Invited Members Can Access Applications

### Current Gaps

| Handler / Route | Current Behavior | Required Behavior |
|---|---|---|
| `ApplicationHandler.GetByID` | Hard-checks `AdminUserID == userID` → blocks all invited members | Allow if user has **any** membership (viewer+) |
| `ApplicationHandler.Update` | Hard-checks `AdminUserID == userID` | Allow if membership role has `manage_app` permission (owner/admin) |
| `ApplicationHandler.Delete` | Hard-checks `AdminUserID == userID` | Allow if membership role has `manage_app` permission (owner only) |
| `ApplicationHandler.GetSettings` | Hard-checks `AdminUserID == userID` | Allow if user has **any** membership (viewer+) — settings are read-only for viewers |
| `ApplicationHandler.UpdateSettings` | Hard-checks `AdminUserID == userID` + RBAC middleware on one route | Allow if `manage_app` permission |
| `ApplicationHandler.RegenerateAPIKey` | Hard-checks `AdminUserID == userID` | Allow if `manage_app` permission (owner only — this is destructive) |
| `TeamHandler.*` | `verifyAppOwnership` checks `AdminUserID` only | Allow if `manage_members` permission (owner/admin) |
| Dashboard resource pages (users, templates, notifications) | JWT-authenticated users cannot see these — they require API key auth | Need a JWT-to-app-context bridge for dashboard views |

### Viewer Permission Mapping (what each role can see/do in the dashboard)

| Resource | Owner | Admin | Editor | Viewer |
|---|---|---|---|---|
| View app details & settings | Yes | Yes | Yes | Yes (read-only) |
| Edit app settings | Yes | Yes | No | No |
| Delete app | Yes | No | No | No |
| Regenerate API key | Yes | No | No | No |
| View users/subscribers | Yes | Yes | Yes | Yes |
| Manage users (create/update/delete) | Yes | Yes | Yes | No |
| View templates | Yes | Yes | Yes | Yes |
| Create/edit templates | Yes | Yes | Yes | No |
| Send notifications | Yes | Yes | Yes | No |
| View notification logs | Yes | Yes | Yes | Yes |
| View audit logs | Yes | Yes | No | Yes (via `view_audit`) |
| Manage team members | Yes | Yes | No | No |
| View environments | Yes | Yes | Yes | Yes |
| Manage environments | Yes | Yes | No | No |

### Implementation Steps

#### Step 1: Add `checkAppAccess` helper to ApplicationHandler

Replace the scattered `AdminUserID != userID` checks with a centralized helper:

```go
// checkAppAccess verifies the user has access to the app (owner OR member).
// Returns the app, the user's effective role, and an error.
func (h *ApplicationHandler) checkAppAccess(ctx context.Context, fiberCtx *fiber.Ctx, appID string) (*application.Application, auth.Role, error) {
    userID := fiberCtx.Locals("user_id").(string)
    
    app, err := h.service.GetByID(ctx, appID)
    if err != nil {
        return nil, "", err
    }
    
    // Owner always has access
    if app.AdminUserID == userID {
        return app, auth.RoleOwner, nil
    }
    
    // Check membership
    if h.membershipRepo != nil {
        membership, mErr := h.membershipRepo.GetByAppAndUser(ctx, appID, userID)
        if mErr == nil && membership != nil {
            return app, membership.Role, nil
        }
    }
    
    return nil, "", errors.Forbidden("You do not have access to this application")
}
```

#### Step 2: Update all handler methods to use `checkAppAccess`

- `GetByID`: Call `checkAppAccess`, return app data for any role.
- `Update`: Call `checkAppAccess`, require `manage_app` permission.
- `Delete`: Call `checkAppAccess`, require owner only.
- `GetSettings`: Call `checkAppAccess`, allow any role (read-only).
- `UpdateSettings`: Call `checkAppAccess`, require `manage_app`.
- `RegenerateAPIKey`: Call `checkAppAccess`, require owner only.

#### Step 3: Fix TeamHandler to allow admin-role members

Replace `verifyAppOwnership` with `verifyAppPermission` that checks `manage_members` permission via membership.

#### Step 4: Add RBAC middleware to ALL JWT-protected app routes

Currently only 2 routes use `RequirePermission`. Expand to all:

```
PUT    /v1/apps/:id           → manage_app
DELETE /v1/apps/:id           → manage_app (owner only at handler level)
PUT    /v1/apps/:id/settings  → manage_app
POST   /v1/apps/:id/regenerate-key → manage_app
POST   /v1/apps/:id/providers → manage_app
DELETE /v1/apps/:id/providers/:pid → manage_app
POST   /v1/apps/:id/environments → manage_app
DELETE /v1/apps/:id/environments/:eid → manage_app
POST   /v1/apps/:id/environments/promote → manage_app
GET    /v1/apps/:id/team      → manage_members
POST   /v1/apps/:id/team      → manage_members
PUT    /v1/apps/:id/team/:mid → manage_members
DELETE /v1/apps/:id/team/:mid → manage_members
```

Read-only routes (`GET /v1/apps/:id`, `GET /v1/apps/:id/settings`, etc.) need only a "is member?" check, no specific permission.

#### Step 5: Add "viewer_logs" permission for Viewer role

Currently, `RoleViewer` only has `view_logs`. Add `view_app` permission:

```go
var RolePermissions = map[Role][]Permission{
    RoleOwner:  {PermManageApp, PermManageMembers, PermManageTemplates, PermSendNotifications, PermViewLogs, PermViewAudit, PermViewApp},
    RoleAdmin:  {PermManageApp, PermManageMembers, PermManageTemplates, PermSendNotifications, PermViewLogs, PermViewAudit, PermViewApp},
    RoleEditor: {PermManageTemplates, PermSendNotifications, PermViewLogs, PermViewApp},
    RoleViewer: {PermViewLogs, PermViewApp},
}
```

#### Step 6: Dashboard JWT-to-App bridge for resource pages

The dashboard currently loads users, templates, and notifications via **API key** auth. When a viewer accesses an app via the dashboard (JWT auth), the frontend needs to use JWT endpoints.

**Two approaches (choose one)**:

**Option A — Expose resource read endpoints under JWT auth** (Recommended):
Add new JWT-authenticated read-only routes:
```
GET /v1/apps/:id/users       → list users (requires view_app)
GET /v1/apps/:id/templates   → list templates (requires view_app)
GET /v1/apps/:id/notifications → list notifications (requires view_logs)
```

These reuse existing service methods but authenticate via JWT + membership check instead of API key.

**Option B — Frontend injects API key**:
After loading app details (which now works for members), the frontend stores the API key and uses it for subsequent calls. **Risk**: Exposes API key to members who shouldn't have it (viewers).

**Recommendation**: Option A for security.

---

## Part B: Cross-Application User (Subscriber) Sharing

### Current Model

```
User (subscriber) ──1:1──> Application
```

Each user record has a mandatory `app_id`. There is no concept of a subscriber existing independently of an application.

### Problem

- With 5,000 users in App A, creating App B requires duplicating all 5,000 records.
- User preferences, devices, and contact info become out-of-sync across apps.
- Customer has to maintain N copies of the same subscriber for N applications.

### Proposed Model: Organizational User Pool

Introduce a **Contact** (or **Subscriber**) entity that lives at the **organization/account level**, separate from app-specific bindings.

```
Contact (org-level)           AppSubscription (binding)           Application
┌─────────────────┐          ┌──────────────────────┐          ┌─────────────────┐
│ contact_id       │──1:N──▷│ subscription_id       │◁──N:1──│ app_id           │
│ org_id (account) │         │ contact_id            │         │ app_name         │
│ email            │         │ app_id                │         │ ...              │
│ phone            │         │ external_id (per-app) │         └─────────────────┘
│ timezone         │         │ preferences (per-app) │
│ language         │         │ devices (per-app)     │
│ global_prefs     │         │ created_at            │
│ created_at       │         └──────────────────────┘
│ updated_at       │
└─────────────────┘
```

### Why This Works

1. **Contact info is centralized**: Email, phone, timezone update once.
2. **Preferences are per-app**: Each app can have different channel settings.
3. **Devices can be per-app or shared**: Configurable.
4. **Backward compatible**: Existing single-app users become contacts with one subscription.

### Backward Compatibility Strategy

This is a **large schema change**. We need a phased rollout:

#### Phase 1: Feature-flagged behind `shared_subscribers_enabled`

- New ES index: `contacts` (org-level subscriber pool)
- New ES index: `app_subscriptions` (contact-to-app bindings)
- New domain entities: `Contact`, `AppSubscription`
- New service: `ContactService` with CRUD + cross-app linking

#### Phase 2: Dual-Write Migration

- When `shared_subscribers_enabled = true`:
  - `POST /v1/users` creates both a contact AND an app_subscription.
  - `GET /v1/users` queries via app_subscriptions joined to contacts.
  - Old `users` index continues to be written for backward compat.
- When `shared_subscribers_enabled = false`:
  - Behavior is identical to current (no breaking change).

#### Phase 3: New API Surface

```
# Contact Management (org-level)
POST   /v1/contacts              → Create contact in org pool
GET    /v1/contacts              → List contacts in org
GET    /v1/contacts/:id          → Get contact
PUT    /v1/contacts/:id          → Update contact info
DELETE /v1/contacts/:id          → Delete contact

# App Subscription (link contact to app)
POST   /v1/apps/:id/subscribers       → Link contact(s) to app
GET    /v1/apps/:id/subscribers       → List app subscribers
DELETE /v1/apps/:id/subscribers/:cid  → Unlink contact from app
PUT    /v1/apps/:id/subscribers/:cid/preferences → Per-app preferences

# Bulk operations
POST   /v1/apps/:id/subscribers/import → Import contacts from another app
```

#### Phase 4: Migration Tool

A CLI command to migrate existing per-app users to the new model:

```bash
./migrate contacts --from-app <app_id> [--to-org <org_id>]
```

This reads all users from the `users` index for an app, creates `contact` records (deduplicating by email), and creates `app_subscription` records pointing back.

### Impact on Notification Service

The `NotificationService.Send` method currently resolves users by `UserID` in the `users` index. With the new model:

1. If `shared_subscribers_enabled`, resolve via `app_subscriptions` → `contacts`.
2. If disabled, resolve via existing `users` index (no change).
3. The `SendRequest` continues to accept `user_id` — the resolution layer is transparent.

---

## Execution Order

| Priority | Task | Effort | Risk |
|---|---|---|---|
| **P0** | Fix RBAC: `checkAppAccess` helper + update all handlers | 1 day | Low — no schema change |
| **P0** | Fix RBAC: Add JWT-authenticated read routes for dashboard | 1 day | Low — additive routes |
| **P1** | Fix RBAC: Update frontend to use membership-aware API calls | 1 day | Medium — UI changes |
| **P1** | Add `PermViewApp` permission to role matrix | 30 min | Low |
| **P2** | Design & implement Contact/AppSubscription domain entities | 2 days | Medium |
| **P2** | Implement dual-write subscriber service | 2 days | Medium — migration |
| **P3** | New API surface for contacts + subscriptions | 1 day | Low |
| **P3** | Migration CLI tool | 1 day | Medium |
| **P3** | Frontend UI for subscriber management across apps | 2 days | Medium |

---

## Files to Modify (RBAC Fix)

### Backend
- `internal/domain/auth/models.go` — Add `PermViewApp` permission
- `internal/interfaces/http/handlers/application_handler.go` — Replace ownership checks with `checkAppAccess`
- `internal/interfaces/http/handlers/team_handler.go` — Replace `verifyAppOwnership` with permission check
- `internal/interfaces/http/routes/routes.go` — Add RBAC middleware to all app routes
- `internal/interfaces/http/middleware/rbac_middleware.go` — Add `RequireAnyMembership` middleware variant

### Frontend
- `ui/src/services/api.ts` — Add JWT-authenticated app resource endpoints
- `ui/src/pages/AppDetail.tsx` — Use membership-aware access
- `ui/src/components/apps/AppTeam.tsx` — Allow admin-role members to manage team

## Files to Create (User Sharing — future)

### Backend
- `internal/domain/contact/models.go` — Contact + AppSubscription entities
- `internal/infrastructure/repository/contact_repository.go`
- `internal/infrastructure/repository/app_subscription_repository.go`
- `internal/usecases/contact_service.go`
- `internal/interfaces/http/handlers/contact_handler.go`
- `internal/interfaces/http/handlers/subscriber_handler.go`
