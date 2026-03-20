# Resource Link Deletion & App Cascade Cleanup

## Problem Statement

When resources (users, templates, workflows, etc.) are imported/linked from App-A into App-B, and App-A is subsequently deleted:
1. The linked resources appear in App-B but are broken/useless
2. The underlying resource data is never cleaned up (orphaned in Elasticsearch)
3. Resource link records become stale pointers to a non-existent source app

The root cause is that **app deletion has no cascade cleanup** — it only deletes the application document itself.

---

## Current Architecture

### How Resource Linking Works

- Resources (users, templates, etc.) live in their own ES indices with an `app_id` field identifying the owner app.
- `POST /apps/:id/import` creates `Link` records in the `app_resource_links` ES index.
- Each link stores: `source_app_id`, `target_app_id`, `resource_type`, `resource_id`.
- When a handler lists resources (e.g. templates), it calls `linkRepo.GetLinkedAppIDs()` and expands the ES query to include both the app's own resources AND resources from linked source apps.
- Resources are **never duplicated** — links are read-only pointers.

### Key Files

| File | Role |
|---|---|
| `internal/domain/resourcelink/models.go` | Link model, Repository interface |
| `internal/infrastructure/repository/resource_link_repository.go` | ES repo (index: `app_resource_links`) |
| `internal/interfaces/http/handlers/import_handler.go` | Import/Unlink HTTP handlers |
| `internal/usecases/services/application_service_impl.go` | App Delete (bare — no cleanup) |
| `internal/usecases/services/auth_service_impl.go` | Account Delete cascade |
| `internal/interfaces/http/handlers/template_handler.go` | Merges linked templates into list |
| `internal/interfaces/http/handlers/user_handler.go` | Merges linked users into list |
| `internal/interfaces/http/handlers/workflow_handler.go` | Merges linked workflows into list |
| `internal/interfaces/http/handlers/topic_handler.go` | Merges linked topics into list |
| `internal/infrastructure/database/index_templates.go` | ES index mappings |

---

## Bugs Found

### Bug 1: App Deletion Does ZERO Cleanup

**File:** `internal/usecases/services/application_service_impl.go` — `Delete()` method

The `ApplicationServiceImpl.Delete()` only deletes the application document. It does **not** delete:
- Users, templates, workflows, notifications, topics, digest rules, environments
- Resource links (where this app is source OR target)
- App memberships, audit logs

All child data becomes permanently orphaned in Elasticsearch.

### Bug 2: Account Deletion Uses Wrong Index & Field for Resource Links

**File:** `internal/usecases/services/auth_service_impl.go` — `deleteAccountCascade()` method

The account cascade deletes from `"resource_links"` using field `"app_id"`:

```go
s.deleteByTerms(ctx, "resource_links", "app_id", appIDList)
```

**Two problems:**
1. The index is actually named `app_resource_links`, not `resource_links`
2. The index has no `app_id` field — it uses `target_app_id` and `source_app_id`

This line **silently does nothing** — resource links are never cleaned up on account deletion.

### Bug 3: Linked Resources Break When Source App Is Deleted

When App-A's resources are linked into App-B and App-A gets deleted:
- The link records still point to App-A as `source_app_id`
- The resources themselves still exist (Bug #1 — never deleted)
- But App-A is gone, so the UI shows broken source-app references
- If Bug #1 were fixed (resources deleted with app), the links would point to non-existent resources

---

## Implementation Plan

The solution uses a **reference counting / adoption** approach: resources shared across apps are retained as long as at least one app references them. When the last reference is removed, the resource is deleted.

### Phase 1: Add Repository Methods

**File:** `internal/domain/resourcelink/models.go`

Add to `Repository` interface:

```go
// ListBySource returns all links where this app is the source (its resources are shared out).
ListBySource(ctx context.Context, sourceAppID string, resourceType *ResourceType) ([]*Link, error)

// DeleteAllBySource deletes all links where this app is the source.
DeleteAllBySource(ctx context.Context, sourceAppID string) error

// CountByResource counts how many target apps link to a specific resource from a specific source.
CountByResource(ctx context.Context, sourceAppID string, resourceType ResourceType, resourceID string) (int64, error)
```

**File:** `internal/infrastructure/repository/resource_link_repository.go`

Implement the three new methods using ES queries on `source_app_id`.

---

### Phase 2: Resource Adoption + Cascade Deletion Logic

**New file:** `internal/usecases/services/app_deletion_cascade.go`

Create a `CascadeDeleter` struct that encapsulates the deletion logic. It needs access to:
- Resource link repository
- User, Template, Workflow, Topic, Digest repositories (to update `app_id` or delete)
- Application repository
- Notification repository (to delete — notifications are never shared)

#### Core Algorithm

For each resource type (users, templates, workflows, digest_rules, topics, providers):

```
For each resource owned by the deleting app:
    links = findLinks(source_app_id=appID, resource_id=resourceID)
    if len(links) > 0:
        // Resource is shared — adopt into the first consumer app
        newOwnerAppID = links[0].target_app_id
        resource.app_id = newOwnerAppID
        updateResource(resource)
        
        // The first consumer is now the owner — delete its link (it owns the resource directly)
        deleteLink(links[0])
        
        // Remaining consumers: update source_app_id to point to the new owner
        for link in links[1:]:
            link.source_app_id = newOwnerAppID
            updateLink(link)
    else:
        // No consumers — safe to delete the resource entirely
        deleteResource(resource)
```

#### Non-Shareable Data (Always Deleted)

These are app-scoped and never linked across apps:
- `notifications` — delivery records
- `workflow_executions` — execution history
- `workflow_schedules` — cron schedules
- `environments` — multi-env configs
- `audit_logs` / `audits` — audit trail
- `memberships` — app team memberships

---

### Phase 3: Wire Into App Deletion

**File:** `internal/usecases/services/application_service_impl.go`

Enhance `Delete()` to call the cascade deleter:

```go
func (s *ApplicationServiceImpl) Delete(ctx context.Context, appID string) error {
    // 1. Verify app exists
    existing, err := s.repo.GetByID(ctx, appID)
    if err != nil { return err }
    if existing == nil { return errors.NotFound("Application", appID) }

    // 2. Run cascade: adopt shared resources, delete unshared ones
    if err := s.cascadeDeleter.DeleteAppResources(ctx, appID); err != nil {
        s.logger.Error("Cascade deletion failed", zap.String("app_id", appID), zap.Error(err))
        return err
    }

    // 3. Delete the application document itself
    if err := s.repo.Delete(ctx, appID); err != nil {
        return errors.DatabaseError("delete application", err)
    }

    s.logger.Info("Application deleted with full cascade", zap.String("app_id", appID))
    return nil
}
```

The `CascadeDeleter` needs to be injected into `ApplicationServiceImpl` via the container.

---

### Phase 4: Fix Account Deletion Cascade

**File:** `internal/usecases/services/auth_service_impl.go`

Replace the broken resource_links cleanup in `deleteAccountCascade()`:

```go
// BEFORE (broken — wrong index name AND wrong field):
s.deleteByTerms(ctx, "resource_links", "app_id", appIDList)

// AFTER: reuse the app cascade deleter for each owned app
for _, appID := range appIDList {
    if err := s.cascadeDeleter.DeleteAppResources(ctx, appID); err != nil {
        s.logger.Warn("Failed to cascade-delete app resources during account purge",
            zap.String("app_id", appID), zap.Error(err))
    }
}
```

This ensures account deletion follows the exact same adoption/cleanup logic as individual app deletion.

---

### Phase 5: UI — Orphan Detection (Defensive)

**File:** `ui/src/components/apps/AppImport.tsx`

In the "Current Links" section where links are grouped by `source_app_id`:
- Resolve source app name for display (currently just shows the raw ID)
- If the source app no longer exists (404 on get), show an "orphaned" badge
- Allow bulk cleanup of orphaned links

After Phases 1–4 are implemented, orphan links should never occur. This is purely defensive/migration support.

---

## Summary of Changes

| Phase | File | Change |
|---|---|---|
| 1 | `domain/resourcelink/models.go` | Add `ListBySource`, `DeleteAllBySource`, `CountByResource` to Repository interface |
| 1 | `repository/resource_link_repository.go` | Implement the 3 new methods |
| 2 | **NEW** `usecases/services/app_deletion_cascade.go` | Resource adoption + safe deletion logic |
| 3 | `services/application_service_impl.go` | Inject CascadeDeleter, call it in `Delete()` |
| 3 | `container/container.go` | Wire CascadeDeleter into the DI container |
| 4 | `services/auth_service_impl.go` | Fix index name `resource_links` → `app_resource_links`, replace broken cleanup with cascade deleter call |
| 5 | `ui/src/components/apps/AppImport.tsx` | Defensive orphan detection + cleanup UI |

---

## Design Rationale

- **No data duplication**: Resources stay in one ES index. Only the `app_id` field is updated when ownership transfers.
- **No broken references**: Links are updated in-place when a source app is deleted. The first consumer becomes the new owner; other consumers' links are updated to point to the new owner.
- **Safe deletion**: Resources with zero consumers outside the deleting app are fully deleted — no orphaned data.
- **Backward compatible**: Existing links and resources work unchanged. The fix only changes what happens at deletion time.
- **Consistent**: Both app deletion and account deletion use the same cascade logic, preventing drift.
