# Template System Code Readout (Backend + Frontend)

## Scope
This document summarizes the current template implementation across the Go backend and React frontend, based on code in `internal/` and `ui/src/`.

## Backend Architecture

### Core Domain Model
File: `internal/domain/template/models.go`

- `Template` includes core content and lifecycle fields:
- `id`, `app_id`, `environment_id`, `name`, `description`, `channel`, `subject`, `body`, `variables`, `metadata`, `version`, `status`, `locale`, timestamps, and audit fields.
- Phase 6 content controls are modeled as:
- `controls []TemplateControl`
- `control_values map[string]interface{}`
- `TemplateControl` supports UI-editable control types: `text`, `textarea`, `url`, `color`, `image`, `number`, `boolean`, `select`.

### Service Layer
File: `internal/usecases/template_service.go`

- `Create`:
- Defaults locale to `en`.
- Validates channel against allowed list.
- Auto-detects variables from body if omitted.
- Validates used variables vs declared variables.
- Prevents duplicate `(app_id, name, locale)` active template.
- Persists with version `1` and status `active`.

- `Update`:
- Loads by ID and enforces app ownership.
- Supports partial updates for description/webhook/subject/body/variables/metadata/status.
- Re-extracts variables if body changes and variables are not explicitly provided.
- Validates status enum.
- Persists in place (same template ID).

- `Render`:
- Loads template, verifies app ownership and `status == active`.
- Uses Go `text/template` to render body with provided data.

- Versioning:
- `CreateVersion` snapshots current template into a new document/version.
- `Rollback` copies target version into a new version (history-preserving).
- `Diff` compares two versions and returns field-level changes.

- Helper behavior:
- Variable extraction regex: `{{ .var }}` and `{{var}}` forms.
- Validation excludes Go template keywords from variable checks.

### Repository Layer (Elasticsearch)
File: `internal/infrastructure/database/template_repository.go`

- Index name: `templates`.
- CRUD operations use ES index/get/search/delete APIs.
- `GetByAppAndName` returns latest `active` template sorted by version desc.
- `List` defaults to `status=active` if status filter is omitted.
- `CreateVersion`:
- Fetches existing versions.
- Increments version number.
- Generates new template ID.
- Marks prior active versions as `inactive`.
- `GetVersions` sorts by `version desc`.
- `GetByVersion` queries exact version.

### HTTP Layer
Files:
- `internal/interfaces/http/routes/routes.go`
- `internal/interfaces/http/handlers/template_handler.go`
- `internal/interfaces/http/dto/template_dto.go`

Routes under `/v1/templates` include:
- `GET /library`
- `POST /library/:name/clone`
- `POST /`
- `GET /`
- `GET /:id`
- `PUT /:id`
- `DELETE /:id`
- `POST /:id/render`
- `POST /:id/rollback`
- `GET /:id/diff`
- `POST /:id/test`
- `GET /:id/controls`
- `PUT /:id/controls`
- `POST /:app_id/:name/versions`
- `GET /:app_id/:name/versions`
- `GET /:app_id/:name/versions/:version`

Handler highlights:
- App ownership is enforced via `app_id` from auth context.
- Environment scoping uses optional `environment_id` from context.
- List supports linked-template reads through `resourcelink` repo.
- Template library is loaded from seed templates.
- Test-send is SMTP-only and currently uses email address payload.

### Seeded Library Templates
File: `internal/seed/templates.go`

- Contains reusable templates across channels (`email`, `push`, `sms`, `webhook`, `sse`).
- Categorized by metadata (e.g., `transactional`, `newsletter`, `notification`).
- Includes multiple newsletter-grade HTML templates and long-form examples.

### Template Usage in Notification Flow
File: `internal/usecases/notification_service.go`

- `Send` resolves `template_id` by UUID or by name.
- `resolveTemplate` order:
- UUID lookup.
- Name with locale `en`.
- Name with empty locale fallback.
- Channel can be inferred from resolved template if missing in request.
- Template body/subject is copied into notification content before queueing.

### Container Wiring
File: `internal/container/container.go`

- `TemplateService` wired with template repository and logger.
- `TemplateHandler` wired with service plus optional SMTP provider.
- SMTP provider is created best-effort for template test-send endpoint.

## Frontend Architecture

### Primary Template UI
File: `ui/src/components/AppTemplates.tsx`

- Full template management UI:
- List and pagination.
- Create/edit form.
- Channel selection and webhook target selection.
- Rich body editing via `TemplateEditor`.
- Variable extraction from body while editing.
- Per-template preview rendering.
- Version history modal.
- Save version and rollback actions.
- Diff viewer panel, test panel, controls panel.

### Editor
File: `ui/src/components/TemplateEditor.tsx`

- Non-email channels use plain textarea.
- Email channel supports 3 modes:
- `builder` (block builder)
- `visual` (contentEditable toolbar)
- `html` (raw HTML textarea)
- Includes live iframe preview for email content.

### Advanced Panels
Files:
- `ui/src/components/templates/TemplateDiffViewer.tsx`
- `ui/src/components/templates/TemplateTestPanel.tsx`
- `ui/src/components/templates/TemplateControlsPanel.tsx`

- Diff panel lets user choose two versions and fetches server diff.
- Test panel supports user selection, variable JSON, preview, and send test action.
- Controls panel fetches controls/values and allows grouped editing.

### Template Library Page
File: `ui/src/pages/TemplateLibrary.tsx`

- Fetches app API key, loads server-side library templates.
- Groups by category.
- Allows preview and clone/import into app.

### Frontend API Client + Types
Files:
- `ui/src/services/api.ts`
- `ui/src/types/index.ts`

- `templatesAPI` mirrors backend endpoints for CRUD/render/version/diff/test/controls/library.
- Type definitions exist for base and advanced template flows.

## Contract Mismatches Found (Important)

These are current backend/frontend mismatches that will likely cause runtime issues:

1. Rollback payload key mismatch
- Backend expects `{"version": <int>}` via `dto.RollbackRequest`.
- Frontend sends `{"target_version": <int>}` via `TemplateRollbackRequest`.

2. Diff response shape mismatch
- Backend returns `TemplateDiff` as:
- `from_version`, `to_version`, `changes: []FieldChange` where each item is `{field, from, to}`.
- Frontend expects `changes` as object map:
- `Record<string, { old, new }>`.

3. Test-send payload mismatch
- Backend endpoint `POST /templates/:id/test` expects:
- `to_email`, `sample_data`.
- Frontend test panel sends:
- `user_id`, `variables`.

4. Controls update payload mismatch
- Backend `UpdateControls` parses request body directly as `ControlValues` map.
- Frontend sends wrapped payload:
- `{ control_values: { ... } }`.
- Current backend will store `control_values` as a literal key, not merge actual field keys.

5. Controls fetch response key mismatch
- Backend response uses `control_values`.
- Frontend type expects `values`.

6. Update handler ignores locale updates
- DTO supports `locale` in update request.
- Handler does not map `locale` into `template.UpdateRequest`.
- Domain `UpdateRequest` does not currently include locale pointer either.

## Suggested Next Workstream

1. Align API contracts first (rollback/diff/test/controls) to remove UI breakage.
2. Decide source-of-truth contract style:
- Option A: update backend to match frontend payloads.
- Option B: update frontend to current backend contract.
3. Add integration tests for template advanced endpoints:
- rollback, diff, test-send, controls get/update.
4. Add a single shared API contract reference (OpenAPI or generated TS types) to prevent future drift.

## Key Files Reviewed

Backend:
- `internal/domain/template/models.go`
- `internal/usecases/template_service.go`
- `internal/infrastructure/database/template_repository.go`
- `internal/interfaces/http/handlers/template_handler.go`
- `internal/interfaces/http/dto/template_dto.go`
- `internal/interfaces/http/routes/routes.go`
- `internal/usecases/notification_service.go`
- `internal/container/container.go`
- `internal/seed/templates.go`

Frontend:
- `ui/src/components/AppTemplates.tsx`
- `ui/src/components/TemplateEditor.tsx`
- `ui/src/components/templates/TemplateDiffViewer.tsx`
- `ui/src/components/templates/TemplateTestPanel.tsx`
- `ui/src/components/templates/TemplateControlsPanel.tsx`
- `ui/src/pages/TemplateLibrary.tsx`
- `ui/src/services/api.ts`
- `ui/src/types/index.ts`

## UX + Feature Plan (Requested)

All requested items are feasible. The main blocker is current API contract drift between frontend and backend; that must be fixed first to make UI behavior reliable.

### 1. Button Guidance (Hover Help + Inline Help)

Problem:
- Template action buttons (`Preview`, `Compare`, `Test`, `Controls`, `Save Version`, `Edit`, `Delete`) are not self-explanatory for first-time users.

Plan:
- Add hover tooltips on every action button with concise purpose and side effects.
- Add one inline `How actions work` info row at top of templates tab.
- Add destructive-action warning copy for `Delete` and rollback-related actions.

Suggested tooltip copy:
- `Preview`: "Render this template with sample data without sending."
- `Compare`: "Compare two saved versions field-by-field."
- `Test`: "Send a test notification/email to verify delivery."
- `Controls`: "Edit business-safe content fields without editing raw template HTML."
- `Save Version`: "Create a new immutable version snapshot from current content."
- `Edit`: "Modify template body, subject, variables, and metadata."
- `Delete`: "Permanently delete this template. This cannot be undone."

Frontend files:
- `ui/src/components/AppTemplates.tsx`

Acceptance criteria:
- Every action button has hover help.
- Keyboard focus also reveals the same help text (accessibility parity).
- The templates tab shows a concise action explainer for non-technical users.

### 2. Verify All Buttons End-to-End (Frontend + Backend)

Problem:
- Several action flows are currently mismatched at API contract level.

Plan:
- Build a verification matrix and execute endpoint and UI checks for each action.
- Fix contract mismatches before UX polish.

Verification matrix (must pass):
- `Preview` -> `POST /v1/templates/:id/render`
- `Compare` -> `GET /v1/templates/:id/diff`
- `Test` -> `POST /v1/templates/:id/test`
- `Controls` read -> `GET /v1/templates/:id/controls`
- `Controls` write -> `PUT /v1/templates/:id/controls`
- `Save Version` -> `POST /v1/templates/:app_id/:name/versions`
- `Edit` -> `PUT /v1/templates/:id`
- `Delete` -> `DELETE /v1/templates/:id`
- `History Restore` -> `POST /v1/templates/:id/rollback`

Required contract fixes before test pass:
- Rollback body: align `version` vs `target_version`.
- Diff shape: align array vs map response contract.
- Test payload: align `to_email/sample_data` vs `user_id/variables`.
- Controls payload and response keys: align `control_values` handling.

Validation strategy:
- Backend integration tests for each endpoint (happy path + validation errors + auth scope).
- Frontend UI smoke test for each button action.
- Optional Playwright flow for templates tab regression.

### 3. Persist Variable Values As New Defaults

Problem:
- Users fill many variables in preview/test; values reset on refresh and become tedious.

Plan (two-layer persistence):
- Layer A (fast UX): Persist last-used values in browser local storage keyed by template ID.
- Layer B (shared/team default): Persist curated defaults server-side in template metadata or controls values.

Recommended data model:
- Keep runtime preview/test values in `localStorage` key:
- `frn:template:last_values:{app_id}:{template_id}`
- Add explicit `Save as default` action to write to backend default store.

Backend option (recommended):
- Reuse `control_values` as persisted defaults for user-editable variables.
- For variables not in controls, store under `metadata.sample_data` as canonical preview defaults.

UX behavior:
- On load, hydrate values from backend defaults first, then overlay local unsaved draft values.
- On render/test, auto-save current values as draft.
- On explicit `Save as default`, persist to backend and invalidate cache.

Acceptance criteria:
- Refresh does not erase in-progress variable values.
- Saved defaults appear on all devices/sessions for same app/template.
- User can reset to template defaults in one click.

### 4. Editable Preview Panel Without Raw HTML Editing

Problem:
- Right-side preview currently displays rendered output only.
- Non-technical users need easy edits without touching template HTML tags.

Plan:
- Convert preview panel into a split view:
- Left: generated variable form (friendly fields).
- Right: live rendered preview.
- Keep raw HTML editor available separately for advanced users only.

Implementation approach:
- Generate form from `controls` when present.
- Fallback to inferred fields from template variables.
- Update preview on debounce (300-500 ms) and manual `Render` button.
- Keep template HTML immutable unless user enters `Edit` mode.

Acceptance criteria:
- User can modify visible content through fields and see instant preview updates.
- No accidental mutation of template source HTML from preview panel.

### 5. Non-Technical UX for Large Variable Sets

Problem:
- Large JSON payload editing in `PREVIEW DATA (JSON)` is hard for non-technical users.

Plan:
- Replace raw JSON-first UX with schema/form-first UX.
- Keep JSON as an advanced toggle.

Form UX requirements:
- Group fields by section (`Hero`, `CTA`, `Footer`, etc.) via controls group metadata.
- Search/filter fields by name.
- Collapsible groups with required-field markers.
- Type-aware inputs (`url`, `color`, `image`, `select`, `boolean`, number).
- Inline validation and error messages before render/test.
- Import/export sample data JSON for power users.

Acceptance criteria:
- A non-technical user can render/test template without touching JSON.
- Large templates remain usable with field groups and search.

## Suggested Implementation Phases

### Phase 0: Contract Stabilization (Must Do First)
- Align frontend/backend API contracts for rollback, diff, test, controls.
- Update DTO/types and backward compatibility handling where needed.
- Add endpoint tests to lock contract.

### Phase 1: Usability Baseline
- Add tooltips and templates-tab helper text.
- Add verification badges/logging around each button action result.

### Phase 2: Variable Persistence
- Local draft persistence in browser.
- Backend `Save as default` and `Reset` support.

### Phase 3: Form-Driven Preview/Test UX
- Introduce variable form panel with grouped controls.
- Add live preview updates and safe editing flow.

### Phase 4: Hardening + Regression
- End-to-end tests for all button flows.
- Performance checks on templates with high variable counts.
- Error-state UX polish (clear remediation messaging).

## Additional Suggestions For Ease of Use

1. Add "Template Health" indicators:
- Missing required variables.
- Invalid URLs/colors.
- Empty subject for email channel.

2. Add "Variable Presets":
- Save named sample datasets (e.g., "US Customer", "Enterprise Trial").

3. Add "Channel Preview Modes":
- Email desktop/mobile frame.
- SMS character count.
- Webhook JSON lint + schema hint.

4. Add "Safe Delete" guardrail:
- Show where template is used (workflows/default template) before delete.

5. Add "Recently Used Templates" quick actions in notification send UI.

## Execution Recommendation

Recommended order:
1. Contract fixes and tests.
2. Tooltips/help text.
3. Variable persistence.
4. Form-driven preview/test UX.

This order gives immediate reliability, then immediate clarity, then major usability gains.

## Implementation Status (2026-03-17)

### Completed

1. Branch and scope
- Working branch created: `feature/template-ux-contract`.

2. API contract stabilization (compatibility)
- Rollback now supports both `version` and `target_version` payload styles.
- Controls read now returns both `control_values` and `values` for compatibility.
- Controls update accepts both raw control map and wrapped `{ control_values: {...} }` body.
- Diff UI supports both backend diff shapes (array and map forms).
- Test send UI now uses backend payload contract (`to_email`, `sample_data`).

3. Core backend bug fix
- `TemplateService.Update` now persists `Controls` and `ControlValues` correctly.

4. Templates tab guidance
- Added action explainer text in templates tab.
- Added per-button hover help for `Preview`, `Compare`, `Test`, `Controls`, `Save Version`, `Edit`, `Delete`.

5. Variable persistence and easier editing
- Local persistence for preview/test variable drafts.
- `Save as Default` and `Reset Defaults` implemented via `metadata.sample_data` updates.
- Quick variable form added for preview and test workflows.
- Search/filter added for large variable sets in preview slide panel and test panel.

6. Safe editable preview panel
- Right-side preview converted to split workspace:
- Left: variable form (data edits only)
- Right: rendered preview output
- No direct raw HTML mutation from this preview workspace.

7. Validation completed
- `go test ./internal/interfaces/http/handlers ./internal/usecases` -> pass
- `go test ./internal/interfaces/http/dto` -> no test files
- `npm --prefix ui run build` -> pass (`tsc` + `vite build`)

### Pending

1. Full end-to-end verification matrix execution (runtime)
- Need API-level runtime verification for every template action in a running environment:
- render, diff, test send, controls get/update, create version, update, delete, rollback.

2. Automated regression coverage
- Add/expand integration tests for template endpoints and contract behavior.
- Add frontend e2e smoke flow for templates tab actions.

3. Large-form UX enhancements (recommended)
- Group variables by control group in preview/test forms.
- Add required-field indicators and inline type validation in quick forms.

4. Optional product improvements
- Template health indicators.
- Named variable presets.
- Safe delete impact analysis (where template is referenced).

## Verification Checklist

- `Preview` button wiring: Completed
- `Compare` button wiring: Completed
- `Test` button wiring: Completed
- `Controls` button wiring: Completed
- `Save Version` button wiring: Existing flow retained
- `Edit` button wiring: Existing flow retained
- `Delete` button wiring: Existing flow retained
- Hover help + inline action guidance: Completed
- Local variable persistence: Completed
- Server-backed default persistence: Completed
- Editable preview workspace without HTML editing: Completed
- Runtime matrix in live stack: Pending
- Automated e2e regression suite: Pending

### Live Runtime Verification Results (2026-03-17)

Environment:
- `docker-compose` stack running.
- API health check returned `200` on `http://localhost:8080/health`.

Endpoint checks executed with real API key and temporary templates:
- `POST /v1/templates/`: Pass
- `GET /v1/templates/:id`: Pass
- `POST /v1/templates/:id/render`: Pass
- `GET /v1/templates/:id/controls`: Pass (`control_values` + `values` alias both present)
- `PUT /v1/templates/:id/controls` with wrapped body: Pass
- `PUT /v1/templates/:id`: Pass
- `POST /v1/templates/:app_id/:name/versions`: Pass
- `GET /v1/templates/:app_id/:name/versions`: Pass
- `GET /v1/templates/:id/diff`: Pass
- `POST /v1/templates/:id/rollback` using `target_version`: Pass
- `POST /v1/templates/:id/test`: Pass (successful run observed)
- `DELETE /v1/templates/:id`: Pass

Issue found during live verification:
- `GET /v1/templates/?name=<unique_name>&limit=20&offset=0` did not return a freshly created template in repeated checks.
- Direct `GET /v1/templates/:id` succeeded for the same records.
- This suggests a list/filter behavior issue (possibly name filtering, environment scoping, or list query semantics) and should be treated as a bug to investigate.

Recommended follow-up for this issue:
1. Add handler/repository test that creates a template then asserts list-by-name returns it.
2. Inspect `TemplateRepository.List` name filter (`match` on `name`) and adjust to deterministic exact/filter behavior as needed.
3. Validate whether environment context (`environment_id`) in middleware is narrowing list unexpectedly.
