# Phase 4 — Content & Templates: Detailed Design Document

> **Status:** Ready for Implementation
> **Dependencies:** Phase 3 Complete (verified `go build ./...` clean)
> **Duration:** 3 weeks (Weeks 13–16)
> **Feature Count:** 5 features (2 backend, 1 library expansion, 1 frontend, 1 cross-cutting)

---

## Table of Contents

- [Section 0: Concepts](#section-0-concepts)
- [Section 1: Codebase Audit](#section-1-codebase-audit)
  - [1.1 Template Domain Model](#11-template-domain-model)
  - [1.2 Template Repository (Elasticsearch)](#12-template-repository-elasticsearch)
  - [1.3 Template Service Layer](#13-template-service-layer)
  - [1.4 Template HTTP Handler & Routes](#14-template-http-handler--routes)
  - [1.5 Seed Library Architecture](#15-seed-library-architecture)
  - [1.6 Newsletter Templates (Current State)](#16-newsletter-templates-current-state)
  - [1.7 Gaps & Debt](#17-gaps--debt)
- [Section 2: Feature 4.1 — Template Versioning: Diff & Rollback](#section-2-feature-41--template-versioning-diff--rollback)
  - [2.1 Current Versioning State](#21-current-versioning-state)
  - [2.2 Rollback Endpoint](#22-rollback-endpoint)
  - [2.3 Diff Endpoint](#23-diff-endpoint)
  - [2.4 Version-Aware Update Flow](#24-version-aware-update-flow)
  - [2.5 API Endpoints](#25-api-endpoints)
  - [2.6 Implementation](#26-implementation)
- [Section 3: Feature 4.2 — Newsletter Template Library Expansion](#section-3-feature-42--newsletter-template-library-expansion)
  - [3.1 Design Principles](#31-design-principles)
  - [3.2 New Templates](#32-new-templates)
  - [3.3 Template Architecture (Preserved)](#33-template-architecture-preserved)
  - [3.4 Implementation](#34-implementation)
- [Section 4: Feature 4.3 — Template Categories & Discovery](#section-4-feature-43--template-categories--discovery)
  - [4.1 Category Model](#41-category-model)
  - [4.2 Library Filtering API](#42-library-filtering-api)
  - [4.3 Implementation](#43-implementation)
- [Section 5: Feature 4.4 — Block-Based Email Editor](#section-5-feature-44--block-based-email-editor)
  - [5.1 Architecture Decision](#51-architecture-decision)
  - [5.2 Editor Metadata Convention](#52-editor-metadata-convention)
  - [5.3 Frontend Components](#53-frontend-components)
  - [5.4 Implementation](#54-implementation)
- [Section 6: Feature 4.5 — Newsletter Delivery Features](#section-6-feature-45--newsletter-delivery-features)
  - [6.1 Unsubscribe Link Injection](#61-unsubscribe-link-injection)
  - [6.2 Preheader Text Convention](#62-preheader-text-convention)
  - [6.3 Send Test Endpoint](#63-send-test-endpoint)
  - [6.4 Implementation](#64-implementation)
- [Section 7: Wiring & Container Integration](#section-7-wiring--container-integration)
- [Section 8: Metrics](#section-8-metrics)
- [Section 9: File Inventory](#section-9-file-inventory)
- [Section 10: Backward Compatibility](#section-10-backward-compatibility)
- [Section 11: Implementation Order](#section-11-implementation-order)

---

## Section 0: Concepts

### Template Versioning (Diff & Rollback)

Every time a template is updated, the system creates an immutable snapshot (version). Users can compare any two versions side-by-side and rollback to a previous version. Rollback does **not** delete history — it creates a new version whose content is copied from the target version.

**Example:** A user edits their "welcome_email" template 5 times. Version 3 had the best copy. They hit "Rollback to v3" — the system creates version 6 with the exact content from version 3. All 6 versions remain in history.

### Newsletter Template Library

FreeRangeNotify ships with a curated set of production-ready newsletter templates that users can clone into their apps. Each template is a complete, responsive HTML email designed for real inbox rendering (Gmail, Outlook, Apple Mail). Templates use Go's `//go:embed` directive for large HTML files, keeping the seed code clean and the HTML files independently editable.

**Example:** A user browses the template library, sees "product_launch" (a modern launch announcement with hero image, feature grid, and CTA), clicks "Clone to My App", and instantly has a working template they fill with their own variables and send.

### Template Categories

Library templates are organized into categories — `transactional`, `newsletter`, `marketing`, `system`. Users can filter the library by category to find relevant templates quickly. Categories are a read-only label on library templates, not a separate entity.

### Block-Based Email Editor

A drag-and-drop visual email editor embedded in the dashboard UI. Users build newsletter layouts from blocks (heading, text, image, button, divider, columns) instead of writing raw HTML. The editor produces standard HTML stored in the template `body` field, while the block structure is preserved in `metadata.editor_blocks` for round-trip editing.

**Example:** A marketing team member opens the editor, drags in a hero image block, a two-column text block, and a CTA button. The editor generates responsive HTML. They save the template, and it renders identically whether opened in the visual editor or the raw HTML view.

### Send Test

Before deploying a notification template to production, users can send a test rendering to a specific email address. The system renders the template with sample data and delivers it through the configured SMTP provider, so the user can verify rendering in a real email client.

---

## Section 1: Codebase Audit

### 1.1 Template Domain Model

**File:** `internal/domain/template/models.go`

```go
type Template struct {
    ID            string                 `json:"id" es:"template_id"`
    AppID         string                 `json:"app_id" es:"app_id"`
    Name          string                 `json:"name" es:"name"`
    Description   string                 `json:"description" es:"description"`
    Channel       string                 `json:"channel" es:"channel"`
    WebhookTarget string                 `json:"webhook_target,omitempty" es:"webhook_target"`
    Subject       string                 `json:"subject,omitempty" es:"subject"`
    Body          string                 `json:"body" es:"body"`
    Variables     []string               `json:"variables" es:"variables"`
    Metadata      map[string]interface{} `json:"metadata" es:"metadata"`
    Version       int                    `json:"version" es:"version"`
    Status        string                 `json:"status" es:"status"`
    Locale        string                 `json:"locale" es:"locale"`
    CreatedBy     string                 `json:"created_by" es:"created_by"`
    UpdatedBy     string                 `json:"updated_by" es:"updated_by"`
    CreatedAt     time.Time              `json:"created_at" es:"created_at"`
    UpdatedAt     time.Time              `json:"updated_at" es:"updated_at"`
}
```

**Key observations:**
- `Metadata map[string]interface{}` is already present — this is where editor blocks and category labels will live, requiring **zero model changes** for the block editor.
- `Version int` field already exists. Versioning foundation is built.
- `Channel` validation in `CreateRequest` is `oneof=push email sms webhook` — does not include `sse`, `slack`, `discord`, `whatsapp` from Phase 3. This needs fixing in Phase 4.

### 1.2 Template Repository (Elasticsearch)

**File:** `internal/infrastructure/database/template_repository.go`

| Method | Status | Notes |
|---|---|---|
| `Create` | ✅ Complete | UUID gen, timestamps, defaults version=1, status="active" |
| `GetByID` | ✅ Complete | Direct ES document GET |
| `GetByAppAndName` | ✅ Complete | Sorts by version DESC, returns highest active version |
| `Update` | ✅ Complete | Overwrites entire document in-place |
| `List` | ✅ Complete | Dynamic bool query, excludes archived, limit 50 |
| `Delete` | ✅ Complete | Hard delete |
| `GetVersions` | ✅ Complete | ES query: `(app_id AND name AND locale)`, sorted version DESC, limit 100 |
| `CreateVersion` | ✅ Complete | Finds latest version number, increments, deactivates previous active versions, creates new document |

**Key observations:**
- `GetVersions` and `CreateVersion` are **fully implemented** — not stubs.
- `CreateVersion` properly deactivates prior active versions when creating a new one.
- `GetByAppAndName` already returns the latest active version by sorting version DESC.
- No `GetByVersion` method exists (needed for rollback) — must be added.

### 1.3 Template Service Layer

**File:** `internal/usecases/template_service.go`

| Method | Status |
|---|---|
| `Create` | ✅ |
| `GetByID` | ✅ |
| `GetByName` | ✅ |
| `Update` | ✅ (in-place overwrite, NOT version-aware) |
| `Delete` | ✅ |
| `List` | ✅ |
| `Render` | ✅ (uses `text/template` stdlib) |
| `CreateVersion` | ✅ (clones current, repo handles version increment) |
| `GetVersions` | ✅ (pass-through to repo) |

**Render pipeline:** `text/template.New("notification").Parse(body)` → `Execute(data)`. Variables use `{{.varName}}` syntax with the data map as the dot context.

**Gap:** `Update` does in-place overwrite. The master plan specifies that updates should create new versions instead of overwriting. This is the most impactful change in Feature 4.1.

### 1.4 Template HTTP Handler & Routes

**File:** `internal/interfaces/http/handlers/template_handler.go`

| Handler | Route | Method |
|---|---|---|
| `CreateTemplate` | `POST /v1/templates/` | Create new template |
| `GetTemplate` | `GET /v1/templates/:id` | Get by ID |
| `ListTemplates` | `GET /v1/templates/` | List with filters |
| `UpdateTemplate` | `PUT /v1/templates/:id` | In-place update |
| `DeleteTemplate` | `DELETE /v1/templates/:id` | Hard delete |
| `RenderTemplate` | `POST /v1/templates/:id/render` | Render with variables |
| `CreateTemplateVersion` | `POST /v1/templates/:app_id/:name/versions` | Create version snapshot |
| `GetTemplateVersions` | `GET /v1/templates/:app_id/:name/versions` | List all versions |
| `GetLibrary` | `GET /v1/templates/library` | Get pre-built templates |
| `CloneFromLibrary` | `POST /v1/templates/library/:name/clone` | Clone library template into app |

**Route registration order** in `routes.go`:
```go
templates.Get("/library", ...)           // Before /:id to avoid conflict
templates.Post("/library/:name/clone", ...)
templates.Post("/", ...)
templates.Get("/", ...)
templates.Get("/:id", ...)
// ...
```

Library routes are registered **before** `/:id` routes — correct. New version/rollback/diff routes must also respect this ordering.

### 1.5 Seed Library Architecture

**File:** `internal/seed/templates.go`

```go
//go:embed newsletter_editorial.html
var newsletterEditorialHTML string

var LibraryTemplates = []template.Template{
    { Name: "welcome_email", Channel: "email", Body: `<div>...</div>`, ... },
    // ... 7 more inline templates ...
    { Name: "newsletter_editorial", Channel: "email", Body: newsletterEditorialHTML,
      Metadata: map[string]interface{}{
          "sample_data": map[string]interface{}{ /* 37 variables with sample values */ },
      },
    },
}
```

**Pattern rules to preserve:**
1. **Small templates** → inline Go string literals in `templates.go`
2. **Large templates** (>100 lines HTML) → `//go:embed filename.html` in separate file
3. **Sample data** → stored in `Metadata["sample_data"]` for preview rendering
4. **All templates** → appended to the single `LibraryTemplates` slice
5. **CloneFromLibrary** → iterates `LibraryTemplates` by name, copies all fields into a `CreateRequest`

This pattern is clean and must be preserved for all new newsletter templates.

### 1.6 Newsletter Templates (Current State)

| Template | Style | Lines | Variables | Embed |
|---|---|---|---|---|
| `monkeys_weekly_digest` | Dark theme (#0d0d0d bg, #161616 cards), indigo accents (#6366f1) | ~200 (inline) | 32 | Inline string |
| `newsletter_editorial` | Light/white Beefree design, Merriweather font, 15 row sections | 1,531 | 37 | `//go:embed newsletter_editorial.html` |

Both are production-quality, responsive HTML emails with:
- Mobile-responsive tables
- MSO (Outlook) conditionals
- Preheader text support
- Social footer links
- Unsubscribe link variable
- CTA buttons with inline styles

### 1.7 Gaps & Debt

| Gap | Impact | Phase 4 Fix |
|---|---|---|
| `Update` overwrites in-place, no automatic versioning | Users can lose previous content | Feature 4.1: Version-aware updates |
| No rollback endpoint | Cannot restore previous versions | Feature 4.1: Rollback API |
| No diff endpoint | Cannot compare versions | Feature 4.1: Diff API |
| Only 2 newsletter templates in library | Users need more starting points | Feature 4.2: 5 new newsletter templates |
| No template categories | Hard to find relevant templates in library | Feature 4.3: Categories |
| No visual editor | Non-developers can't create newsletters | Feature 4.4: Block editor |
| `CreateRequest.Channel` validation doesn't include Phase 3 channels | `sse`, `slack`, `discord`, `whatsapp` channels fail validation | Fix in Feature 4.3 |
| No "Send Test" capability | Users can't preview in real email clients | Feature 4.5: Send test endpoint |
| No `GetByVersion` in repository | Cannot fetch a specific version for rollback | Feature 4.1: New repo method |

---

## Section 2: Feature 4.1 — Template Versioning: Diff & Rollback

### 2.1 Current Versioning State

The versioning **foundation** is solid:
- `CreateVersion` in the repository increments the version number, deactivates prior active versions, and creates a new ES document.
- `GetVersions` returns all versions sorted by version DESC.
- `GetByAppAndName` returns the latest active version.

What's missing is three pieces: **rollback**, **diff**, and **version-aware updates**.

### 2.2 Rollback Endpoint

Rollback creates a **new version** whose content is copied from the target version. It never deletes history.

**Service method:**

```go
// internal/usecases/template_service.go

func (s *TemplateService) Rollback(ctx context.Context, templateID, appID string, targetVersion int, updatedBy string) (*templateDomain.Template, error) {
    // 1. Fetch current template to verify ownership
    current, err := s.repo.GetByID(ctx, templateID)
    if err != nil {
        return nil, err
    }
    if current.AppID != appID {
        return nil, pkgerrors.ErrForbidden("template belongs to a different app")
    }

    // 2. Fetch the target version
    target, err := s.repo.GetByVersion(ctx, current.AppID, current.Name, current.Locale, targetVersion)
    if err != nil {
        return nil, pkgerrors.ErrNotFound("version not found")
    }

    // 3. Create a new version with the target's content
    rollback := &templateDomain.Template{
        AppID:         current.AppID,
        Name:          current.Name,
        Description:   target.Description,
        Channel:       target.Channel,
        WebhookTarget: target.WebhookTarget,
        Subject:       target.Subject,
        Body:          target.Body,
        Variables:     target.Variables,
        Metadata:      target.Metadata,
        Locale:        current.Locale,
        Status:        "active",
        CreatedBy:     updatedBy,
        UpdatedBy:     updatedBy,
    }

    if err := s.repo.CreateVersion(ctx, rollback); err != nil {
        return nil, err
    }

    s.logger.Info("template rolled back",
        zap.String("template_id", templateID),
        zap.String("name", current.Name),
        zap.Int("from_version", current.Version),
        zap.Int("to_version", targetVersion),
        zap.Int("new_version", rollback.Version),
    )

    return rollback, nil
}
```

**New repository method:**

```go
// internal/infrastructure/database/template_repository.go

func (r *TemplateRepository) GetByVersion(ctx context.Context, appID, name, locale string, version int) (*template.Template, error) {
    query := map[string]any{
        "query": map[string]any{
            "bool": map[string]any{
                "must": []map[string]any{
                    {"term": {"app_id": appID}},
                    {"term": {"name.keyword": name}},
                    {"term": {"locale": locale}},
                    {"term": {"version": version}},
                },
            },
        },
        "size": 1,
    }
    // Execute search and return single result
}
```

**Domain interface update:**

```go
// internal/domain/template/models.go — add to Repository interface

GetByVersion(ctx context.Context, appID, name, locale string, version int) (*Template, error)
```

### 2.3 Diff Endpoint

Returns a field-by-field comparison between two versions. The body diff is a simple line-level comparison (not a character-level diff algorithm — keep it practical).

**Service method:**

```go
// internal/usecases/template_service.go

type TemplateDiff struct {
    FromVersion int                    `json:"from_version"`
    ToVersion   int                    `json:"to_version"`
    Changes     []FieldChange          `json:"changes"`
}

type FieldChange struct {
    Field    string      `json:"field"`
    From     interface{} `json:"from"`
    To       interface{} `json:"to"`
}

func (s *TemplateService) Diff(ctx context.Context, appID, name, locale string, fromVersion, toVersion int) (*TemplateDiff, error) {
    from, err := s.repo.GetByVersion(ctx, appID, name, locale, fromVersion)
    if err != nil {
        return nil, pkgerrors.ErrNotFound("version " + strconv.Itoa(fromVersion) + " not found")
    }
    to, err := s.repo.GetByVersion(ctx, appID, name, locale, toVersion)
    if err != nil {
        return nil, pkgerrors.ErrNotFound("version " + strconv.Itoa(toVersion) + " not found")
    }

    if from.AppID != appID || to.AppID != appID {
        return nil, pkgerrors.ErrForbidden("template belongs to a different app")
    }

    diff := &TemplateDiff{
        FromVersion: fromVersion,
        ToVersion:   toVersion,
    }

    // Compare scalar fields
    if from.Subject != to.Subject {
        diff.Changes = append(diff.Changes, FieldChange{Field: "subject", From: from.Subject, To: to.Subject})
    }
    if from.Body != to.Body {
        diff.Changes = append(diff.Changes, FieldChange{Field: "body", From: from.Body, To: to.Body})
    }
    if from.Description != to.Description {
        diff.Changes = append(diff.Changes, FieldChange{Field: "description", From: from.Description, To: to.Description})
    }
    if from.Channel != to.Channel {
        diff.Changes = append(diff.Changes, FieldChange{Field: "channel", From: from.Channel, To: to.Channel})
    }
    if !reflect.DeepEqual(from.Variables, to.Variables) {
        diff.Changes = append(diff.Changes, FieldChange{Field: "variables", From: from.Variables, To: to.Variables})
    }
    if !reflect.DeepEqual(from.Metadata, to.Metadata) {
        diff.Changes = append(diff.Changes, FieldChange{Field: "metadata", From: from.Metadata, To: to.Metadata})
    }

    return diff, nil
}
```

### 2.4 Version-Aware Update Flow

Currently `Update` does an in-place overwrite. The new behavior: an update **creates a new version** and deactivates the previous active version.

**Modified update flow in `template_service.go`:**

```go
func (s *TemplateService) Update(ctx context.Context, id, appID string, req *templateDomain.UpdateRequest) (*templateDomain.Template, error) {
    existing, err := s.repo.GetByID(ctx, id)
    if err != nil {
        return nil, err
    }
    if existing.AppID != appID {
        return nil, pkgerrors.ErrForbidden("template belongs to a different app")
    }

    // Apply partial updates to build the new version content
    updated := *existing
    if req.Name != nil {
        updated.Name = *req.Name
    }
    if req.Description != nil {
        updated.Description = *req.Description
    }
    if req.Subject != nil {
        updated.Subject = *req.Subject
    }
    if req.Body != nil {
        updated.Body = *req.Body
    }
    if req.Variables != nil {
        updated.Variables = *req.Variables
    }
    if req.Metadata != nil {
        updated.Metadata = req.Metadata
    }
    if req.Status != nil {
        // Status-only updates (archive, deactivate) are in-place — no new version
        existing.Status = *req.Status
        existing.UpdatedBy = req.UpdatedBy
        existing.UpdatedAt = time.Now()
        return existing, s.repo.Update(ctx, existing)
    }
    if req.WebhookTarget != nil {
        updated.WebhookTarget = *req.WebhookTarget
    }

    updated.UpdatedBy = req.UpdatedBy
    updated.CreatedBy = req.UpdatedBy

    // Create a new version instead of in-place overwrite
    if err := s.repo.CreateVersion(ctx, &updated); err != nil {
        return nil, err
    }

    s.logger.Info("template updated (new version created)",
        zap.String("name", updated.Name),
        zap.Int("version", updated.Version),
    )

    return &updated, nil
}
```

**Important:** Status-only updates (e.g., archiving a template) remain in-place overwrites — creating a new version for a status toggle is excessive. Only content changes (body, subject, variables, metadata) trigger a new version.

### 2.5 API Endpoints

| Method | Path | Description |
|---|---|---|
| `POST` | `/v1/templates/:id/rollback` | Rollback to a specific version |
| `GET` | `/v1/templates/:id/diff` | Diff between two versions |

**Rollback request body:**
```json
{
    "version": 3
}
```

**Rollback response:** Returns the newly created template version (same shape as the template object).

**Diff query parameters:**
```
GET /v1/templates/:id/diff?from=3&to=5
```

**Diff response:**
```json
{
    "from_version": 3,
    "to_version": 5,
    "changes": [
        { "field": "subject", "from": "Welcome!", "to": "Welcome to {{.product}}!" },
        { "field": "body", "from": "<div>old</div>", "to": "<div>new</div>" },
        { "field": "variables", "from": ["name"], "to": ["name", "product"] }
    ]
}
```

### 2.6 Implementation

**Files to modify:**

| Action | File | Change |
|---|---|---|
| **MODIFY** | `internal/domain/template/models.go` | Add `GetByVersion` to Repository interface; add `Rollback` + `Diff` to Service interface; add `TemplateDiff`, `FieldChange` types |
| **MODIFY** | `internal/infrastructure/database/template_repository.go` | Add `GetByVersion` method |
| **MODIFY** | `internal/usecases/template_service.go` | Add `Rollback`, `Diff` methods; change `Update` to create new version for content changes |
| **MODIFY** | `internal/interfaces/http/handlers/template_handler.go` | Add `RollbackTemplate`, `DiffTemplate` handlers |
| **MODIFY** | `internal/interfaces/http/routes/routes.go` | Register `POST /:id/rollback`, `GET /:id/diff` |

---

## Section 3: Feature 4.2 — Newsletter Template Library Expansion

### 3.1 Design Principles

1. **Preserve the existing pattern:** `//go:embed` for large HTML, inline for small. All templates append to `LibraryTemplates`.
2. **Production-quality HTML:** Every template must render correctly in Gmail, Outlook (MSO conditionals), Apple Mail, and mobile clients.
3. **Variable-driven:** Templates use `{{.varName}}` syntax. All variable names are declared in the `Variables` field.
4. **Sample data included:** Every newsletter template includes `Metadata["sample_data"]` with realistic placeholder values for preview rendering.
5. **Responsive by default:** All newsletter HTML uses the table-layout pattern with `@media` breakpoints at 660px.
6. **Modern design:** Clean typography, generous whitespace, accessible color contrast, inline SVG-safe icons.

### 3.2 New Templates

#### 3.2.1 `product_launch` — Product Launch Announcement

**Theme:** Clean white background, bold hero section, feature grid (2×2), CTA button.

**Layout:**
```
┌─────────────────────────────────────┐
│  Logo + Tagline                     │
├─────────────────────────────────────┤
│  🎉 Hero Image (full width)        │
├─────────────────────────────────────┤
│  Product Name + One-liner           │
│  Body paragraph                     │
├──────────────────┬──────────────────┤
│  Feature 1       │  Feature 2       │
│  Icon + Title    │  Icon + Title    │
│  Description     │  Description     │
├──────────────────┼──────────────────┤
│  Feature 3       │  Feature 4       │
│  Icon + Title    │  Icon + Title    │
│  Description     │  Description     │
├──────────────────┴──────────────────┤
│  [ Try It Now → ]  CTA Button       │
├─────────────────────────────────────┤
│  Footer: Unsubscribe + Social       │
└─────────────────────────────────────┘
```

**Variables (19):**
```
logo_url, tagline, hero_image_url,
product_name, product_tagline, product_description,
feature1_icon, feature1_title, feature1_desc,
feature2_icon, feature2_title, feature2_desc,
feature3_icon, feature3_title, feature3_desc,
feature4_icon, feature4_title, feature4_desc,
cta_text, cta_url, unsubscribe_url
```

**Embed:** `//go:embed newsletter_product_launch.html`

---

#### 3.2.2 `changelog_release` — Changelog / Release Notes

**Theme:** Dark sidebar (#1a1a2e) header band, light body, monospace code snippets, version badges.

**Layout:**
```
┌─────────────────────────────────────┐
│  Logo        Version Badge (v2.4.0) │
├─────────────────────────────────────┤
│  "What's New" Heading               │
│  Release summary paragraph          │
├─────────────────────────────────────┤
│  🟢 Added                           │
│  • Feature 1 description            │
│  • Feature 2 description            │
├─────────────────────────────────────┤
│  🔧 Fixed                           │
│  • Fix 1 description                │
│  • Fix 2 description                │
├─────────────────────────────────────┤
│  ⚠️  Breaking Changes               │
│  • Change 1 description             │
├─────────────────────────────────────┤
│  [ View Full Changelog → ]          │
├─────────────────────────────────────┤
│  Footer                             │
└─────────────────────────────────────┘
```

**Variables (18):**
```
logo_url, product_name, version, release_date, release_summary,
added_1, added_2, added_3,
fixed_1, fixed_2, fixed_3,
breaking_1, breaking_2,
changelog_url, docs_url,
unsubscribe_url, company_name, company_address
```

**Embed:** `//go:embed newsletter_changelog.html`

---

#### 3.2.3 `event_invitation` — Event / Webinar Invitation

**Theme:** Gradient header (indigo→purple), centered layout, countdown-style date display, speaker cards.

**Layout:**
```
┌─────────────────────────────────────┐
│  ░░ Gradient Header ░░              │
│  EVENT TYPE BADGE                   │
│  Event Title (large)                │
│  📅 Date  ·  🕐 Time  ·  📍 Venue  │
├─────────────────────────────────────┤
│  Event description / pitch          │
├──────────────────┬──────────────────┤
│  Speaker 1       │  Speaker 2       │
│  Photo + Name    │  Photo + Name    │
│  Title           │  Title           │
├──────────────────┴──────────────────┤
│  [ Register Now → ]                 │
├─────────────────────────────────────┤
│  "Add to Calendar" link             │
├─────────────────────────────────────┤
│  Footer                             │
└─────────────────────────────────────┘
```

**Variables (20):**
```
logo_url, event_type, event_title, event_date, event_time,
event_venue, event_description,
speaker1_name, speaker1_title, speaker1_image_url,
speaker2_name, speaker2_title, speaker2_image_url,
register_url, calendar_url,
unsubscribe_url, company_name, company_address,
social_twitter_url, social_linkedin_url
```

**Embed:** `//go:embed newsletter_event_invitation.html`

---

#### 3.2.4 `weekly_roundup_light` — Weekly Roundup (Light Theme)

**Theme:** Light mode counterpart to `monkeys_weekly_digest`. White background, subtle gray cards (#f8f9fa), blue accents (#2563eb). Clean sans-serif (Inter/system).

**Layout:**
```
┌─────────────────────────────────────┐
│  Logo         Week of Month DD      │
├─────────────────────────────────────┤
│  "This Week's Highlights"           │
├─────────────────────────────────────┤
│  Featured Story (image + excerpt)   │
├──────────────────┬──────────────────┤
│  Story 1 (thumb  │  Story 2 (thumb  │
│  + title + meta) │  + title + meta) │
├──────────────────┼──────────────────┤
│  Story 3 (thumb  │  Story 4 (thumb  │
│  + title + meta) │  + title + meta) │
├──────────────────┴──────────────────┤
│  "Quick Links" — 3 inline links     │
├─────────────────────────────────────┤
│  CTA Banner (gradient)              │
├─────────────────────────────────────┤
│  Footer                             │
└─────────────────────────────────────┘
```

**Variables (28):**
```
logo_url, digest_title, date_range,
featured_url, featured_image, featured_title, featured_excerpt, featured_topic,
story1_url, story1_image, story1_title, story1_topic,
story2_url, story2_image, story2_title, story2_topic,
story3_url, story3_image, story3_title, story3_topic,
story4_url, story4_image, story4_title, story4_topic,
quicklink1_text, quicklink1_url,
quicklink2_text, quicklink2_url,
quicklink3_text, quicklink3_url,
cta_text, cta_url, unsubscribe_url
```

**Embed:** `//go:embed newsletter_weekly_roundup_light.html`

---

#### 3.2.5 `community_spotlight` — Community / User Spotlight

**Theme:** Warm tones, cream background (#fefce8), amber accents (#f59e0b). Personal, approachable design.

**Layout:**
```
┌─────────────────────────────────────┐
│  Logo + "Community Spotlight"       │
├─────────────────────────────────────┤
│  Featured Member/Project            │
│  Large photo + Name + Bio           │
│  Quote block (italic, left border)  │
├─────────────────────────────────────┤
│  Q&A or Interview Excerpt           │
│  Q: Question → A: Answer (×3)       │
├─────────────────────────────────────┤
│  "Project Showcase" heading         │
│  Screenshot + description + link    │
├─────────────────────────────────────┤
│  [ Read Full Interview → ]          │
├─────────────────────────────────────┤
│  "More from the Community"          │
│  3 small link items                 │
├─────────────────────────────────────┤
│  Footer                             │
└─────────────────────────────────────┘
```

**Variables (26):**
```
logo_url, edition_number,
member_name, member_title, member_image_url, member_bio, member_quote,
q1, a1, q2, a2, q3, a3,
project_title, project_description, project_image_url, project_url,
interview_url,
link1_title, link1_url,
link2_title, link2_url,
link3_title, link3_url,
unsubscribe_url, company_name
```

**Embed:** `//go:embed newsletter_community_spotlight.html`

### 3.3 Template Architecture (Preserved)

The existing architecture is **fully preserved**. The pattern for adding new newsletter templates:

```go
// internal/seed/templates.go

//go:embed newsletter_product_launch.html
var newsletterProductLaunchHTML string

//go:embed newsletter_changelog.html
var newsletterChangelogHTML string

//go:embed newsletter_event_invitation.html
var newsletterEventInvitationHTML string

//go:embed newsletter_weekly_roundup_light.html
var newsletterWeeklyRoundupLightHTML string

//go:embed newsletter_community_spotlight.html
var newsletterCommunitySpotlightHTML string
```

Each template entry in `LibraryTemplates` follows the exact same structure as `newsletter_editorial`:

```go
{
    Name:        "product_launch",
    Description: "Modern product launch announcement with hero image, feature grid, and CTA",
    Channel:     "email",
    Subject:     "{{.product_name}} — {{.tagline}}",
    Body:        newsletterProductLaunchHTML,
    Variables:   []string{"logo_url", "tagline", "hero_image_url", ...},
    Metadata: map[string]interface{}{
        "category": "newsletter",
        "sample_data": map[string]interface{}{
            "product_name":   "FreeRangeNotify v2.0",
            "tagline":        "The future of notifications is here",
            // ... all variables with realistic values
        },
    },
    Locale: "en",
    Status: "active",
}
```

### 3.4 Implementation

| Action | File | Change |
|---|---|---|
| **CREATE** | `internal/seed/newsletter_product_launch.html` | ~400 lines, responsive email HTML |
| **CREATE** | `internal/seed/newsletter_changelog.html` | ~350 lines, responsive email HTML |
| **CREATE** | `internal/seed/newsletter_event_invitation.html` | ~400 lines, responsive email HTML |
| **CREATE** | `internal/seed/newsletter_weekly_roundup_light.html` | ~450 lines, responsive email HTML |
| **CREATE** | `internal/seed/newsletter_community_spotlight.html` | ~400 lines, responsive email HTML |
| **MODIFY** | `internal/seed/templates.go` | Add 5 `//go:embed` directives + 5 template entries to `LibraryTemplates` |

---

## Section 4: Feature 4.3 — Template Categories & Discovery

### 4.1 Category Model

Categories are **not** a separate entity. They are a string label stored in the template `Metadata["category"]` field. This keeps the domain model unchanged and avoids a new Elasticsearch index.

**Defined categories:**

| Category | Description | Templates |
|---|---|---|
| `transactional` | Triggered by user actions (signups, purchases, resets) | welcome_email, password_reset, order_confirmation |
| `newsletter` | Recurring content digests and editorial emails | monkeys_weekly_digest, newsletter_editorial, product_launch, changelog_release, event_invitation, weekly_roundup_light, community_spotlight |
| `notification` | Short-form alerts (push, SMS, SSE, webhook) | push_alert, sms_verification, webhook_event, sse_realtime |

### 4.2 Library Filtering API

**Enhanced `GetLibrary` handler:**

```
GET /v1/templates/library?category=newsletter
```

The handler filters `seed.LibraryTemplates` by `Metadata["category"]` when the `category` query parameter is provided. No parameter = return all.

**Handler change in `template_handler.go`:**

```go
func (h *TemplateHandler) GetLibrary(c *fiber.Ctx) error {
    category := c.Query("category", "")

    templates := seed.LibraryTemplates
    if category != "" {
        var filtered []templateDomain.Template
        for _, t := range templates {
            if cat, ok := t.Metadata["category"].(string); ok && cat == category {
                filtered = append(filtered, t)
            }
        }
        templates = filtered
    }

    return c.JSON(fiber.Map{
        "templates": templates,
        "total":     len(templates),
    })
}
```

**Also: fix `CreateRequest.Channel` validation** to include Phase 3 channels:

```go
// internal/domain/template/models.go — CreateRequest
Channel string `json:"channel" validate:"required,oneof=push email sms webhook sse slack discord whatsapp"`
```

### 4.3 Implementation

| Action | File | Change |
|---|---|---|
| **MODIFY** | `internal/seed/templates.go` | Add `Metadata["category"]` to all existing 9 templates |
| **MODIFY** | `internal/interfaces/http/handlers/template_handler.go` | Add category filter to `GetLibrary` |
| **MODIFY** | `internal/domain/template/models.go` | Fix `Channel` validation in `CreateRequest` |

---

## Section 5: Feature 4.4 — Block-Based Email Editor

### 5.1 Architecture Decision

The block editor is a **frontend-only feature**. The backend already stores templates as HTML strings in the `body` field — the editor is purely a visual authoring tool that produces HTML.

**Technology:** [email-builder-js](https://github.com/usewaypoint/email-builder-js) (MIT license, React component, produces responsive HTML).

**Why email-builder-js:**
- MIT licensed, actively maintained
- React-native component (the UI is already React/Vite)
- Outputs standard HTML — no vendor lock-in
- Supports custom blocks and theming
- Used by production email platforms

**Backend impact:** Zero. The `body` field receives HTML regardless of whether it was typed manually or generated by the editor. The block structure is stored in `metadata.editor_blocks` for round-trip editing — this field is opaque to the backend.

### 5.2 Editor Metadata Convention

When a template is created or edited via the block editor, the frontend stores the editor state in the template's `Metadata` field:

```json
{
    "editor_type": "block",
    "editor_blocks": { /* email-builder-js document JSON */ },
    "editor_version": "1.0"
}
```

When a template is created via the HTML editor (raw code), the metadata is:

```json
{
    "editor_type": "html"
}
```

**Round-trip behavior:**
- If `editor_type == "block"`, the dashboard opens the visual editor and loads from `editor_blocks`.
- If `editor_type == "html"` or absent, the dashboard opens the raw HTML editor.
- On save from the block editor, both `body` (rendered HTML) and `metadata.editor_blocks` (editor state) are updated together.

### 5.3 Frontend Components

**New files in `ui/src/`:**

#### `ui/src/components/templates/EmailEditor.tsx`

```tsx
import { EmailBuilder, EmailBuilderProvider } from '@usewaypoint/email-builder';
import { useState, useCallback } from 'react';

interface EmailEditorProps {
    initialBlocks?: Record<string, unknown>;
    onSave: (html: string, blocks: Record<string, unknown>) => void;
}

export function EmailEditor({ initialBlocks, onSave }: EmailEditorProps) {
    const [document, setDocument] = useState(initialBlocks ?? DEFAULT_DOCUMENT);

    const handleSave = useCallback(() => {
        const html = renderToHtml(document);
        onSave(html, document);
    }, [document, onSave]);

    return (
        <EmailBuilderProvider>
            <div className="email-editor-container">
                <EmailBuilder
                    document={document}
                    onChange={setDocument}
                />
                <div className="editor-actions">
                    <Button onClick={handleSave}>Save Template</Button>
                </div>
            </div>
        </EmailBuilderProvider>
    );
}
```

#### `ui/src/components/templates/EditorToggle.tsx`

```tsx
interface EditorToggleProps {
    mode: 'block' | 'html';
    onChange: (mode: 'block' | 'html') => void;
}

export function EditorToggle({ mode, onChange }: EditorToggleProps) {
    return (
        <div className="editor-toggle">
            <button
                className={mode === 'block' ? 'active' : ''}
                onClick={() => onChange('block')}
            >
                Visual Editor
            </button>
            <button
                className={mode === 'html' ? 'active' : ''}
                onClick={() => onChange('html')}
            >
                HTML Editor
            </button>
        </div>
    );
}
```

#### Integration in Template Create/Edit Form

The existing template form component gains an `EditorToggle` at the top of the body input section. When "Visual Editor" is selected, the `EmailEditor` component replaces the textarea. On save, the form includes both the rendered HTML body and the editor blocks in the template metadata.

### 5.4 Implementation

| Action | File | Change |
|---|---|---|
| **CREATE** | `ui/src/components/templates/EmailEditor.tsx` | Block editor wrapper component |
| **CREATE** | `ui/src/components/templates/EditorToggle.tsx` | Toggle between block/HTML editor |
| **MODIFY** | `ui/package.json` | Add `@usewaypoint/email-builder` dependency |
| **MODIFY** | Template form component (TBD — depends on current UI structure) | Integrate editor toggle + EmailEditor |

---

## Section 6: Feature 4.5 — Newsletter Delivery Features

### 6.1 Unsubscribe Link Injection

Every newsletter template should include an `{{.unsubscribe_url}}` variable. The worker automatically injects this variable at render time if:
1. The template's `Metadata["category"]` is `"newsletter"`
2. The notification payload does not already include `unsubscribe_url`

**Worker-side injection in `processor.go`:**

```go
// Before rendering, inject unsubscribe URL for newsletter templates
if category, ok := tmpl.Metadata["category"].(string); ok && category == "newsletter" {
    if _, exists := data["unsubscribe_url"]; !exists {
        data["unsubscribe_url"] = fmt.Sprintf(
            "%s/v1/users/%s/unsubscribe?app_id=%s",
            cfg.Server.BaseURL,
            notification.UserID,
            notification.AppID,
        )
    }
}
```

This is a convenience feature — users can always override by providing their own `unsubscribe_url` in the notification payload.

### 6.2 Preheader Text Convention

Preheader text is the preview text shown in email clients next to the subject line. The existing `newsletter_editorial` template already supports this via `{{.preheader_text}}`.

**Convention:** All newsletter library templates include a hidden preheader element:

```html
<div style="display:none;font-size:1px;color:#ffffff;line-height:1px;max-height:0;max-width:0;opacity:0;overflow:hidden;">
    {{.preheader_text}}
</div>
```

This is a template convention, not a backend feature. All new newsletter HTML files in Feature 4.2 include this element.

### 6.3 Send Test Endpoint

A new endpoint that renders a template with sample data and sends it to a specified email address for preview.

**Endpoint:**

```
POST /v1/templates/:id/test
Body: {
    "to_email": "dave@monkeys.com.co",
    "sample_data": { "name": "Dave", "product": "Monkeys" }
}
```

If `sample_data` is omitted, the system uses `Metadata["sample_data"]` from the template (if available — newsletter library templates include this).

**Handler:**

```go
func (h *TemplateHandler) SendTest(c *fiber.Ctx) error {
    id := c.Params("id")
    appID := c.Locals("app_id").(string)

    var req dto.SendTestRequest
    if err := c.BodyParser(&req); err != nil {
        return pkgerrors.ErrBadRequest("invalid request body")
    }

    if req.ToEmail == "" {
        return pkgerrors.ErrBadRequest("to_email is required")
    }

    // Get template
    tmpl, err := h.service.GetByID(c.Context(), id, appID)
    if err != nil {
        return err
    }

    // Determine sample data
    sampleData := req.SampleData
    if sampleData == nil {
        if sd, ok := tmpl.Metadata["sample_data"].(map[string]interface{}); ok {
            sampleData = sd
        }
    }
    if sampleData == nil {
        return pkgerrors.ErrBadRequest("sample_data required — template has no built-in sample data")
    }

    // Render
    rendered, err := h.service.Render(c.Context(), id, appID, sampleData)
    if err != nil {
        return err
    }

    // Send via SMTP provider
    if err := h.smtpProvider.Send(c.Context(), providers.EmailMessage{
        To:      req.ToEmail,
        Subject: "[TEST] " + renderSubject(tmpl.Subject, sampleData),
        Body:    rendered,
    }); err != nil {
        return pkgerrors.ErrInternal("failed to send test email: " + err.Error())
    }

    return c.JSON(fiber.Map{
        "status":  "sent",
        "to":      req.ToEmail,
        "subject": "[TEST] " + renderSubject(tmpl.Subject, sampleData),
    })
}
```

**Note:** The handler needs access to the SMTP provider. This is injected via the container — `TemplateHandler` gains an optional `smtpProvider` field. If SMTP is not configured, the endpoint returns a 503 with a clear message.

### 6.4 Implementation

| Action | File | Change |
|---|---|---|
| **MODIFY** | `internal/interfaces/http/handlers/template_handler.go` | Add `SendTest` handler; add `smtpProvider` field to `TemplateHandler` |
| **MODIFY** | `internal/interfaces/http/routes/routes.go` | Register `POST /v1/templates/:id/test` |
| **MODIFY** | `cmd/worker/processor.go` | Add unsubscribe URL injection for newsletter category templates |
| **MODIFY** | `internal/container/container.go` | Pass SMTP provider to `TemplateHandler` constructor |

---

## Section 7: Wiring & Container Integration

### 7.1 Container Changes

```go
// internal/container/container.go

// TemplateHandler now accepts an optional SMTP provider for Send Test
func (c *Container) buildTemplateHandler() {
    var smtpProvider providers.SMTPProvider
    if c.Config.Providers.SMTP.Enabled {
        smtpProvider = c.SMTPProvider // Already initialized in container
    }

    c.TemplateHandler = handlers.NewTemplateHandler(
        c.TemplateService,
        smtpProvider,    // NEW: optional, can be nil
        c.Logger,
    )
}
```

### 7.2 Route Registration

```go
// internal/interfaces/http/routes/routes.go — template routes

templates := v1.Group("/templates")
templates.Use(auth)
templates.Get("/library", c.TemplateHandler.GetLibrary)
templates.Post("/library/:name/clone", c.TemplateHandler.CloneFromLibrary)
templates.Post("/", c.TemplateHandler.CreateTemplate)
templates.Get("/", c.TemplateHandler.ListTemplates)
templates.Get("/:id", c.TemplateHandler.GetTemplate)
templates.Put("/:id", c.TemplateHandler.UpdateTemplate)
templates.Delete("/:id", c.TemplateHandler.DeleteTemplate)
templates.Post("/:id/render", c.TemplateHandler.RenderTemplate)
templates.Post("/:id/rollback", c.TemplateHandler.RollbackTemplate)  // NEW
templates.Get("/:id/diff", c.TemplateHandler.DiffTemplate)           // NEW
templates.Post("/:id/test", c.TemplateHandler.SendTest)              // NEW
templates.Post("/:app_id/:name/versions", c.TemplateHandler.CreateTemplateVersion)
templates.Get("/:app_id/:name/versions", c.TemplateHandler.GetTemplateVersions)
```

**Route ordering:** The new `/:id/rollback`, `/:id/diff`, and `/:id/test` routes are registered **after** the catch-all CRUD routes. This works because Fiber's radix tree router matches literal segments (`/rollback`, `/diff`, `/test`) before parameter segments (`:id`). However, to be safe, these specific sub-resource routes should be registered **before** `/:app_id/:name/versions` to avoid ambiguity.

---

## Section 8: Metrics

All new features add Prometheus metrics to `internal/infrastructure/metrics/metrics.go`:

| Metric | Type | Labels | Feature |
|---|---|---|---|
| `frn_template_versions_created_total` | Counter | `app_id` | 4.1 Versioning |
| `frn_template_rollbacks_total` | Counter | `app_id` | 4.1 Rollback |
| `frn_template_diffs_total` | Counter | `app_id` | 4.1 Diff |
| `frn_template_library_clones_total` | Counter | `template_name`, `category` | 4.3 Categories |
| `frn_template_test_sends_total` | Counter | `app_id`, `status` | 4.5 Send Test |

---

## Section 9: File Inventory

### New Files (7)

| File | Feature | Description |
|---|---|---|
| `internal/seed/newsletter_product_launch.html` | 4.2 | Product launch announcement template (~400 lines) |
| `internal/seed/newsletter_changelog.html` | 4.2 | Changelog/release notes template (~350 lines) |
| `internal/seed/newsletter_event_invitation.html` | 4.2 | Event/webinar invitation template (~400 lines) |
| `internal/seed/newsletter_weekly_roundup_light.html` | 4.2 | Light-theme weekly roundup template (~450 lines) |
| `internal/seed/newsletter_community_spotlight.html` | 4.2 | Community spotlight template (~400 lines) |
| `ui/src/components/templates/EmailEditor.tsx` | 4.4 | Block-based email editor wrapper |
| `ui/src/components/templates/EditorToggle.tsx` | 4.4 | Toggle between block/HTML editing modes |

### Modified Files (8)

| File | Features | Changes |
|---|---|---|
| `internal/domain/template/models.go` | 4.1, 4.3 | Add `GetByVersion` to repo interface; add `Rollback`, `Diff` to service interface; add `TemplateDiff`, `FieldChange` types; fix `Channel` validation |
| `internal/infrastructure/database/template_repository.go` | 4.1 | Add `GetByVersion` method |
| `internal/usecases/template_service.go` | 4.1 | Add `Rollback`, `Diff` methods; make `Update` version-aware for content changes |
| `internal/interfaces/http/handlers/template_handler.go` | 4.1, 4.3, 4.5 | Add `RollbackTemplate`, `DiffTemplate`, `SendTest` handlers; add category filter to `GetLibrary`; add `smtpProvider` field |
| `internal/interfaces/http/routes/routes.go` | 4.1, 4.5 | Register rollback, diff, test routes |
| `internal/seed/templates.go` | 4.2, 4.3 | Add 5 `//go:embed` directives + 5 template entries; add `Metadata["category"]` to all 9 existing templates |
| `internal/container/container.go` | 4.5 | Pass SMTP provider to `TemplateHandler` |
| `cmd/worker/processor.go` | 4.5 | Add unsubscribe URL auto-injection for newsletter categories |
| `ui/package.json` | 4.4 | Add `@usewaypoint/email-builder` dependency |

---

## Section 10: Backward Compatibility

### Zero Breaking Changes

| Concern | Guarantee |
|---|---|
| **Existing templates** | All existing templates continue to work. The `Metadata["category"]` addition is additive — templates without a category are still returned by unfiltered `GetLibrary` calls. |
| **Existing API** | All current endpoints retain their behavior. New endpoints (`/rollback`, `/diff`, `/test`) are additive. |
| **Version-aware Update** | The `PUT /v1/templates/:id` endpoint now creates new versions for content changes, but `status`-only updates remain in-place. Callers see the same response shape — the `version` field is already in the template model. |
| **Seed library** | The `LibraryTemplates` slice grows from 9 to 14 entries. The `CloneFromLibrary` handler iterates by name — no code change needed. |
| **Template rendering** | The `Render` method is unchanged. `text/template` stdlib rendering works the same. |
| **No new Elasticsearch indices** | All data continues to use the existing `templates` index. No migration step needed. |
| **No new config entries** | All features use existing config (`smtp`, `server.base_url`). No new YAML/env vars. |

### Upgrade Path

1. Deploy new server binary (new endpoints become available, new library templates visible)
2. Deploy new worker binary (unsubscribe URL injection activates)
3. Update dashboard UI (block editor becomes available)
4. No migration required — no new indices, no schema changes

---

## Section 11: Implementation Order

```
Week 1: Features 4.1 + 4.3 (Backend)
├── Day 1-2: Template Versioning (Diff & Rollback)
│   ├── Add GetByVersion to repo interface + implementation
│   ├── Add Rollback service method + handler
│   ├── Add Diff service method + handler
│   ├── Modify Update to be version-aware
│   └── Register new routes
│
├── Day 3: Template Categories
│   ├── Add Metadata["category"] to all existing seed templates
│   ├── Add category filter to GetLibrary handler
│   └── Fix Channel validation to include Phase 3 channels
│
└── Day 4-5: Unit tests for versioning + categories

Week 2: Feature 4.2 + 4.5 (Templates + Newsletter Features)
├── Day 1-2: Create newsletter HTML templates
│   ├── newsletter_product_launch.html
│   ├── newsletter_changelog.html
│   ├── newsletter_event_invitation.html
│   ├── newsletter_weekly_roundup_light.html
│   └── newsletter_community_spotlight.html
│
├── Day 3: Wire templates into seed/templates.go
│   ├── Add //go:embed directives
│   ├── Add LibraryTemplates entries with sample_data
│   └── Verify all templates render with text/template
│
├── Day 4: Newsletter delivery features
│   ├── Add SendTest handler + route
│   ├── Wire SMTP provider to TemplateHandler
│   └── Add unsubscribe URL injection in worker
│
└── Day 5: Integration tests + email client preview testing

Week 3: Feature 4.4 (Frontend)
├── Day 1-2: Block editor integration
│   ├── Install @usewaypoint/email-builder in ui/
│   ├── Create EmailEditor.tsx component
│   ├── Create EditorToggle.tsx component
│   └── Integrate into template create/edit form
│
├── Day 3-4: UI polish + testing
│   ├── Test round-trip (create in editor → save → reopen → edit)
│   ├── Test library clone → open in editor
│   └── Mobile responsiveness of editor UI
│
└── Day 5: Prometheus metrics + final review
    ├── Add versioning/rollback/test-send metrics
    ├── go build ./... verification
    └── go vet ./... verification
```

### Dependency Chain

```
Feature 4.1 (Versioning)   ←── No dependencies, start first
Feature 4.3 (Categories)   ←── No dependencies, parallel with 4.1
Feature 4.2 (Templates)    ←── Depends on 4.3 (needs category labels)
Feature 4.5 (Newsletter)   ←── Depends on 4.2 (templates must exist for testing)
Feature 4.4 (Block Editor)  ←── Independent of backend features, can parallelize
```

---

## Appendix A: Newsletter Design Specifications

### Color Palettes

| Template | Background | Card/Section | Primary Accent | Text | Muted Text |
|---|---|---|---|---|---|
| `product_launch` | `#ffffff` | `#f8f9fa` | `#4f46e5` (indigo) | `#1f2937` | `#6b7280` |
| `changelog_release` | `#ffffff` | `#f3f4f6` | `#1a1a2e` (navy header) | `#111827` | `#6b7280` |
| `event_invitation` | `#ffffff` | `#faf5ff` | gradient `#4f46e5→#7c3aed` | `#1f2937` | `#6b7280` |
| `weekly_roundup_light` | `#ffffff` | `#f8f9fa` | `#2563eb` (blue) | `#1f2937` | `#6b7280` |
| `community_spotlight` | `#fefce8` (cream) | `#ffffff` | `#f59e0b` (amber) | `#1c1917` | `#78716c` |

### Typography

All templates use system font stack for maximum compatibility:
```css
font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
```

Exception: `changelog_release` uses monospace for version numbers and code references:
```css
font-family: 'SF Mono', 'Fira Code', 'Courier New', monospace;
```

### Responsive Breakpoints

All templates use a single breakpoint at `max-width: 660px` (matching the existing `newsletter_editorial.html` convention). At this breakpoint:
- Two-column layouts stack to single column
- Hero images become 100% width
- Font sizes increase slightly for readability
- Button padding increases for touch targets

### Email Client Compatibility

All templates must be tested against:
- Gmail (web + mobile)
- Outlook 2019+ (MSO conditional comments for table layout)
- Apple Mail (macOS + iOS)
- Yahoo Mail
- Thunderbird

MSO conditional pattern (used in all templates):
```html
<!--[if mso]>
<table role="presentation" cellpadding="0" cellspacing="0" width="640"><tr><td>
<![endif]-->
<!-- ... content ... -->
<!--[if mso]>
</td></tr></table>
<![endif]-->
```
