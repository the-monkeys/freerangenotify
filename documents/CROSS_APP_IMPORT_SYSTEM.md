# Cross-App Import System

## Overview

The import system enables **resource sharing between applications** without duplicating data. It works by creating **resource links** — lightweight cross-app references stored in the `app_resource_links` Elasticsearch index — that allow a *target* application to see and use resources owned by a *source* application.

**Key principle: Import does NOT copy data.** It creates pointers (links). The actual resource (user, template, workflow, etc.) remains owned by the source app.

---

## Architecture

```
Source App (b4321bd1)              Target App (e6722431)
┌──────────────────────┐          ┌──────────────────────┐
│ Owns:                │          │ Owns:                │
│  - Template A        │◄─LINK───│  (link to Template A)│
│  - Template B        │◄─LINK───│  (link to Template B)│
│  - User X            │◄─LINK───│  (link to User X)    │
│  - User Y            │◄─LINK───│  (link to User Y)    │
└──────────────────────┘          └──────────────────────┘
                                         │
                                         ▼
                              app_resource_links index
                              ┌──────────────────────┐
                              │ link_id: uuid         │
                              │ target_app_id: e672.. │
                              │ source_app_id: b432.. │
                              │ resource_type: "templates" │
                              │ resource_id: <tmpl-uuid>   │
                              │ linked_by: <user-uuid>     │
                              │ linked_at: <timestamp>     │
                              └──────────────────────┘
```

---

## Resource Link Model

**Domain:** `internal/domain/resourcelink/models.go`

```go
type Link struct {
    LinkID       string       // Unique ID for this link record
    TargetAppID  string       // The app that imported (consumer)
    SourceAppID  string       // The app that owns the resource (producer)
    ResourceType ResourceType // "users", "templates", "workflows", etc.
    ResourceID   string       // The actual resource's ID (e.g., template UUID)
    LinkedBy     string       // User who performed the import
    LinkedAt     time.Time    // When the link was created
}
```

**Supported resource types:**
| Type | Constant |
|------|----------|
| Users | `TypeUser = "users"` |
| Templates | `TypeTemplate = "templates"` |
| Workflows | `TypeWorkflow = "workflows"` |
| Digest Rules | `TypeDigest = "digest_rules"` |
| Topics | `TypeTopic = "topics"` |
| Providers | `TypeProvider = "providers"` |

---

## API Endpoints

All endpoints are JWT-authenticated (admin/owner required on both apps).

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| `POST` | `/v1/apps/:id/import` | `ImportHandler.Import` | Import resources from source to target app |
| `GET` | `/v1/apps/:id/links` | `ImportHandler.ListLinks` | List all resource links for an app |
| `DELETE` | `/v1/apps/:id/links` | `ImportHandler.UnlinkAll` | Remove all links for an app |
| `DELETE` | `/v1/apps/:id/links/:link_id` | `ImportHandler.Unlink` | Remove a specific link |

### Import Request Body

```json
{
  "source_app_id": "b4321bd1-b343-4b21-98e2-17f2f39480a7",
  "resources": ["users", "templates"]
}
```

---

## Import Flow (Step by Step)

**Handler:** `internal/interfaces/http/handlers/import_handler.go` → `Import()`

1. Parse request: extract `target_app_id` from URL, `source_app_id` + `resources` from body.
2. **Validate**: source ≠ target, resource types are valid.
3. **Verify admin access** on both source AND target apps (checks `app.AdminUserID` or membership with admin/owner role).
4. **For each resource type** requested:
   a. Call `listResourceIDs()` — queries the source app's actual resources (users, templates, etc.) up to 10,000 items.
   b. For each resource ID, check `linkRepo.Exists()` — skip if already linked (dedup).
   c. Create a `Link` record for each new resource.
   d. `linkRepo.BulkCreate()` — batch-insert all links into ES.
5. Return summary: `{ "linked": {"templates": 29, "users": 138}, "skipped": {} }`

**Critical detail:** No data is copied. The template/user/workflow documents stay in ES with their original `app_id`. Only link records are created.

---

## How Linked Resources Surface in Reads

### ListTemplates (GET /v1/templates)
**File:** `template_handler.go` → `ListTemplates()`

```go
if h.linkRepo != nil {
    linkedIDs, _ := h.linkRepo.GetAllLinkedResourceIDs(c.Context(), appID, resourcelink.TypeTemplate)
    if len(linkedIDs) > 0 {
        filter.LinkedIDs = linkedIDs
    }
}
```

The ES query includes both `app_id = targetApp` AND `id IN linkedIDs`, so linked templates appear alongside owned templates in the list.

### ListUsers (GET /v1/users)
**File:** `user_handler.go` — same pattern: queries linked user IDs and includes them in the filter.

### GetTemplate / GetByID
**File:** `template_handler.go` → `GetTemplate()` → calls `service.GetByID(ctx, id, appID)`

```go
func (s *TemplateService) GetByID(ctx context.Context, id, appID string) (*templateDomain.Template, error) {
    tmpl, err := s.repo.GetByID(ctx, id)
    if tmpl.AppID != appID {
        return nil, fmt.Errorf("template not found")
    }
    return tmpl, nil
}
```

**⚠️ GAP: No link fallback.** If `appID` doesn't match the template's `app_id`, it returns "not found" — even if the template is linked/imported. The handler does NOT check `linkRepo.Exists()` for a fallback.

### RenderTemplate (POST /v1/templates/:id/render)
**File:** `template_handler.go` → `RenderTemplate()` → calls `service.Render()`

Same ownership check as `GetByID`. **Same gap** — no link-aware fallback.

---

## Delete with Ownership Transfer

When a template is deleted from the **source** app and a consumer (target) app has imported it:

**File:** `template_handler.go` → `DeleteTemplate()`

```
1. Check if linkRepo has consumers:
   linkRepo.ListBySourceAndResource(ctx, appID, TypeTemplate, id)

2. If consumers exist:
   a. Transfer ownership: tmpl.AppID = consumers[0].TargetAppID
   b. Delete the first consumer's link (no longer needed — they own it now)
   c. Update remaining consumers' links: sourceAppID = newOwner
   d. Return 204 (no content deleted, just transferred)

3. If no consumers: actually delete the template
```

Same pattern exists for users in `user_handler.go`.

---

## Known Gaps / Issues

### 1. GetByID and Render Don't Check Links
`GetTemplate()` and `RenderTemplate()` call `service.GetByID()` which only checks `tmpl.AppID == appID`. If the requesting app has a link to the template but doesn't own it, the request returns 404.

**Impact:** An app that imported templates can list them but cannot view or render individual ones by ID.

**This is the root cause of the 404 on template `2c37e5c3`:** It's owned by `b4321bd1` but app `e6722431` tried to render it via API key. The link exists but Render doesn't check it.

### 2. Worker Bypasses Ownership
The worker (`cmd/worker/processor.go`) calls `templateRepo.GetByID()` directly — NO `appID` ownership check. This means notifications for linked templates will work at delivery time, but the API-level preview/render will fail.

### 3. No Link Check on Update
`UpdateTemplate()` only allows the owning app to update. Imported apps cannot modify linked templates (which is correct for shared resources, but should be documented).

### 4. Template Variable Syntax Not Validated
The `renderTemplate()` function uses Go's `text/template` which requires `{{.var}}` syntax. Templates stored with `{{var}}` (no dot) fail at render time with "function not defined". The `extractVariables()` regex accepts both `{{.var}}` and `{{var}}`, but the actual Go template parser does not.

---

## File Reference

| File | Role |
|------|------|
| `internal/domain/resourcelink/models.go` | Domain model, Repository interface |
| `internal/interfaces/http/handlers/import_handler.go` | Import, ListLinks, Unlink, UnlinkAll handlers |
| `internal/interfaces/http/handlers/template_handler.go` | ListTemplates (link-aware), GetTemplate/Render (NOT link-aware), Delete (transfer-aware) |
| `internal/interfaces/http/handlers/user_handler.go` | ListUsers (link-aware), DeleteUser (transfer-aware) |
| `internal/interfaces/http/routes/routes.go` | Route registration (lines 325-329) |
| `internal/usecases/template_service.go` | Render/GetByID ownership check (no link fallback) |
| `cmd/worker/processor.go` | Worker template resolution (no ownership check) |
