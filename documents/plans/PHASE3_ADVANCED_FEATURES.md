# Phase 3 — Advanced Features: Implementation Plan

> **Parent:** [UI_API_INTEGRATION_PLAN.md](UI_API_INTEGRATION_PLAN.md)  
> **Prerequisite:** [PHASE2_CORE_API_SURFACES.md](PHASE2_CORE_API_SURFACES.md) (completed)  
> **Duration:** ~2 weeks  
> **Goal:** Build advanced template capabilities (diff viewer, rollback, test send, content controls), team/RBAC management, custom provider registration, multi-environment with env switcher and promotion, and a standalone audit logs page.

---

## Table of Contents

1. [Task Breakdown](#1-task-breakdown)
2. [Dependency Order](#2-dependency-order)
3. [Task 1: Template Diff Viewer](#3-task-1-template-diff-viewer)
4. [Task 2: Template Rollback Enhancement](#4-task-2-template-rollback-enhancement)
5. [Task 3: Template Test Send Panel](#5-task-3-template-test-send-panel)
6. [Task 4: Template Content Controls Panel](#6-task-4-template-content-controls-panel)
7. [Task 5: AppTemplates Integration (Diff/Rollback/Test/Controls Buttons)](#7-task-5-apptemplates-integration)
8. [Task 6: Team Management Tab](#8-task-6-team-management-tab)
9. [Task 7: Custom Providers Tab](#9-task-7-custom-providers-tab)
10. [Task 8: Multi-Environment Tab & Env Switcher](#10-task-8-multi-environment-tab--env-switcher)
11. [Task 9: Audit Logs Page](#11-task-9-audit-logs-page)
12. [Task 10: Sidebar & Route Registration](#12-task-10-sidebar--route-registration)
13. [Task 11: AppDetail Tab Extension (Team/Providers/Environments)](#13-task-11-appdetail-tab-extension)
14. [Task 12: Build Verification](#14-task-12-build-verification)
15. [Acceptance Criteria](#15-acceptance-criteria)

---

## 1. Task Breakdown

| # | Task | New Files | Modified Files | Est. |
|---|------|-----------|----------------|------|
| 1 | Template Diff Viewer | 1 | 0 | 3h |
| 2 | Template Rollback Enhancement | 0 | 0 | 1h |
| 3 | Template Test Send Panel | 1 | 0 | 3h |
| 4 | Template Content Controls Panel | 1 | 0 | 3h |
| 5 | AppTemplates Integration | 0 | 1 | 3h |
| 6 | Team Management Tab | 1 | 0 | 4h |
| 7 | Custom Providers Tab | 1 | 0 | 3h |
| 8 | Multi-Environment Tab & Env Switcher | 1 | 0 | 5h |
| 9 | Audit Logs Page | 1 | 0 | 4h |
| 10 | Sidebar & Route Registration | 0 | 2 | 30m |
| 11 | AppDetail Tab Extension | 0 | 1 | 2h |
| 12 | Build Verification | 0 | 0 | 30m |
| **Total** | | **7** | **4** | **~32h** |

### New Files (7)

| File | Task | Description |
|------|------|-------------|
| `components/templates/TemplateDiffViewer.tsx` | 1 | Side-by-side version comparison with field-level change highlighting |
| `components/templates/TemplateTestPanel.tsx` | 3 | Slide panel for test send with user picker, variable editor, and live preview |
| `components/templates/TemplateControlsPanel.tsx` | 4 | Dynamic form generated from template `controls` array with grouping |
| `components/apps/AppTeam.tsx` | 6 | Team member list, invite form, role management, permission info |
| `components/apps/AppProviders.tsx` | 7 | Custom provider list, register form, signing key reveal |
| `components/apps/AppEnvironments.tsx` | 8 | Environment cards, create form, promote dialog, env switcher integration |
| `pages/audit/AuditLogsList.tsx` | 9 | Filterable audit log table with detail slide panel |

### Modified Files (4)

| File | Task | Changes |
|------|------|---------|
| `components/AppTemplates.tsx` | 5 | Add buttons for diff, rollback, test send, and controls; wire up the 4 new template components |
| `components/Sidebar.tsx` | 10 | Add "Audit Logs" nav item under ADMIN section |
| `App.tsx` | 10 | Add `/audit` lazy-loaded route |
| `pages/AppDetail.tsx` | 11 | Extend tabs from 8 to 11, add Team/Providers/Environments tabs with permission gating |

---

## 2. Dependency Order

```
Task 1 (Diff Viewer)  ─────────────┐
Task 2 (Rollback)  ─────────────────┤
Task 3 (Test Panel) ────────────────┼─→ Task 5 (AppTemplates Integration) ───┐
Task 4 (Controls Panel) ────────────┘                                        │
                                                                             │
Task 6 (Team) ──────────────────────────┐                                    │
Task 7 (Providers) ─────────────────────┼─→ Task 11 (AppDetail Tabs) ────────┤
Task 8 (Environments) ──────────────────┘                                    │
                                                                             │
Task 9 (Audit Logs) ────→ Task 10 (Sidebar & Routes) ───────────────────────┤
                                                                             │
                                                                      Task 12 (Build Verify)
```

**Critical path:** Tasks 1–4 → Task 5 (template sub-components must exist before wiring into AppTemplates).  
**Parallelizable:** Tasks 1+2+3+4 (all independent components), Tasks 6+7+8+9 (all independent pages/tabs).

---

## 3. Task 1: Template Diff Viewer

**File:** `ui/src/components/templates/TemplateDiffViewer.tsx`  
**Action:** Create new file.

### Purpose

Side-by-side comparison of two template versions. Users pick two versions from dropdowns and see field-level changes highlighted with add/remove colors.

### API Dependencies (already in api.ts)

```ts
templatesAPI.diff(apiKey, templateId, fromVersion, toVersion) → TemplateDiffResponse
templatesAPI.getVersions(apiKey, appId, templateName) → TemplateVersion[]
```

### Types (already in types/index.ts)

```ts
interface TemplateDiffResponse {
    from_version: number;
    to_version: number;
    changes: Record<string, { old: any; new: any }>;
}

interface TemplateVersion {
    id: string;
    version: number;
    subject?: string;
    body: string;
    created_at: string;
}
```

### Props Interface

```tsx
interface TemplateDiffViewerProps {
    apiKey: string;
    appId: string;
    templateId: string;
    templateName: string;
    versions: TemplateVersion[];
    open: boolean;
    onOpenChange: (open: boolean) => void;
}
```

### Component Structure

```
┌─ Compare Versions ────────────────────────────────────────────┐
│                                                                │
│  From: [v1 ▼]              To: [v3 ▼]         [Compare]       │
│                                                                │
│ ┌─────────── Changes ──────────────────────────────────────┐   │
│ │                                                          │   │
│ │  subject                                                 │   │
│ │  ─ "Welcome to {{app_name}}"          (red bg, old)      │   │
│ │  + "Welcome aboard, {{user_name}}!"   (green bg, new)    │   │
│ │                                                          │   │
│ │  body                                                    │   │
│ │  ─ "<p>Hello there...</p>"            (red bg, old)      │   │
│ │  + "<p>Hi {{user_name}}...</p>"       (green bg, new)    │   │
│ │                                                          │   │
│ │  variables                                               │   │
│ │  ─ ["app_name"]                       (red bg, old)      │   │
│ │  + ["app_name", "user_name"]          (green bg, new)    │   │
│ │                                                          │   │
│ └──────────────────────────────────────────────────────────┘   │
│                                                                │
│                                               [Close]          │
└────────────────────────────────────────────────────────────────┘
```

### Implementation Details

1. **Container**: Use `SlidePanel` (named export from `../ui/slide-panel`) for consistency with existing UX.
2. **Version Selectors**: Two `Select` dropdowns, populated from the `versions` prop. Default from=first, to=latest.
3. **Compare Button**: Calls `templatesAPI.diff()`. Shows `Spinner` while loading.
4. **Diff Display**: Iterate over `changes` keys. For each key:
   - Show key name as a label (bold, `text-sm font-medium`).
   - Old value: `bg-red-50 text-red-800 border-l-2 border-red-400` with `−` prefix.
   - New value: `bg-green-50 text-green-800 border-l-2 border-green-400` with `+` prefix.
   - For multi-line values (body), use `<pre>` with `whitespace-pre-wrap`.
   - For array values (variables), `JSON.stringify(val)`.
5. **Empty State**: If no changes, show "No differences between these versions."
6. **Error Handling**: Toast on API failure, disable Compare button while loading.

### Imports

```tsx
import React, { useState } from 'react';
import { templatesAPI } from '../../services/api';
import type { TemplateVersion, TemplateDiffResponse } from '../../types';
import { SlidePanel } from '../ui/slide-panel';
import { Button } from '../ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select';
import { Label } from '../ui/label';
import { Spinner } from '../ui/spinner';
import { toast } from 'sonner';
```

### Acceptance Criteria

- [ ] Two version dropdowns populate from `versions` prop
- [ ] Clicking Compare calls `templatesAPI.diff()` and renders field-level changes
- [ ] Old values shown in red, new values shown in green
- [ ] Multi-line body content displayed in `<pre>` blocks
- [ ] Loading spinner during API call
- [ ] Error toast on failure
- [ ] "No differences" message when versions are identical
- [ ] `tsc --noEmit` passes

---

## 4. Task 2: Template Rollback Enhancement

**File:** No new file — this is logic integrated into Task 5 (AppTemplates).  
**Action:** Enhance the existing Version History dialog in `AppTemplates.tsx`.

### Current State

`AppTemplates.tsx` already has a Version History dialog (lines 716–780) with a "Restore" button per version that calls `templatesAPI.update()` to overwrite the current template. This is a basic restore — it doesn't use the dedicated rollback API.

### Target State

Replace the manual restore with the proper `templatesAPI.rollback()` API call and add a confirm dialog.

### API

```ts
templatesAPI.rollback(apiKey, templateId, { target_version: number }) → void
```

### Changes to AppTemplates.tsx (Task 5)

1. Replace `handleRestoreVersion` implementation:
   ```tsx
   // Before: manually copies body/subject via update
   // After: uses rollback API
   const handleRollback = async (version: number) => {
       if (!versionHistoryTemplate) return;
       try {
           await templatesAPI.rollback(apiKey, versionHistoryTemplate.id, { target_version: version });
           toast.success(`Rolled back to v${version}. A new version has been created.`);
           setVersionHistoryTemplate(null);
           fetchTemplates();
       } catch (err: any) {
           toast.error(err?.response?.data?.error || 'Rollback failed');
       }
   };
   ```

2. Add a `ConfirmDialog` before executing rollback:
   - Title: "Rollback Template"
   - Description: "This will create a new version (v{N+1}) with the content from v{target}. The current version will not be deleted."
   - Confirm button: "Rollback"

### Acceptance Criteria

- [ ] "Restore" button renamed to "Rollback to v{N}"
- [ ] Confirm dialog appears before rollback execution
- [ ] Calls `templatesAPI.rollback()` instead of manual update
- [ ] Toast shows success with new version info
- [ ] Version list and template list both refresh after rollback
- [ ] Error toast on failure

---

## 5. Task 3: Template Test Send Panel

**File:** `ui/src/components/templates/TemplateTestPanel.tsx`  
**Action:** Create new file.

### Purpose

A slide-out panel for sending a test notification from a template. Users pick a recipient user, edit template variables as JSON, preview the rendered output, and send a test.

### API Dependencies (already in api.ts)

```ts
templatesAPI.render(apiKey, templateId, { variables }) → rendered content
templatesAPI.sendTest(apiKey, templateId, { user_id, variables }) → void
usersAPI.list(apiKey) → { users: User[] }
```

### Types (already in types/index.ts)

```ts
interface TemplateTestRequest {
    user_id: string;
    variables?: Record<string, any>;
}

interface RenderTemplateRequest {
    variables: Record<string, string>;
}
```

### Props Interface

```tsx
interface TemplateTestPanelProps {
    apiKey: string;
    template: Template;
    open: boolean;
    onOpenChange: (open: boolean) => void;
}
```

### Component Layout

```
┌─ Send Test Notification ──────────────────────────┐
│                                                    │
│  Template:  [template.name] (channel badge)        │
│                                                    │
│  Recipient User:                                   │
│  [Select → loads from usersAPI.list] ▼             │
│                                                    │
│  Variables (JSON):                                 │
│  ┌────────────────────────────────────────────────┐│
│  │ {                                              ││
│  │   "user_name": "Alice",                        ││
│  │   "order_id": "ORD-12345"                      ││
│  │ }                                              ││
│  └────────────────────────────────────────────────┘│
│  Hint: This template expects: user_name, order_id  │
│                                                    │
│  ┌─ Preview ──────────────────────────────────────┐│
│  │ (rendered HTML/text output here)               ││
│  └────────────────────────────────────────────────┘│
│                                                    │
│            [Preview]  [Send Test]                  │
└────────────────────────────────────────────────────┘
```

### Implementation Details

1. **Container**: `SlidePanel` with title "Send Test Notification".
2. **Template Info**: Display `template.name` and a `Badge` showing `template.channel`.
3. **User Picker**: `Select` dropdown. On panel open, fetch users via `usersAPI.list(apiKey)`. Show `external_id` or `email` as display. Store selected `user_id` (internal UUID).
4. **Variables Editor**: `Textarea` for JSON input. Pre-populate with template's `variables` array as keys with empty string values:
   ```ts
   const defaultVars = (template.variables || []).reduce((acc, v) => {
       acc[v] = '';
       return acc;
   }, {} as Record<string, string>);
   ```
5. **Hint**: Dynamic text: "This template expects: {template.variables.join(', ')}" — shown below the textarea. If no variables, show "This template has no variables."
6. **Preview Button**: Calls `templatesAPI.render(apiKey, template.id, { variables: parsedJson })`. Renders result in a bordered div. For HTML content (email channel), use `dangerouslySetInnerHTML` inside a sandboxed container. For other channels, show as plain text in `<pre>`.
7. **Send Test Button**: Validates user is selected and JSON is valid. Calls `templatesAPI.sendTest(apiKey, template.id, { user_id, variables })`. Success toast: "Test notification sent to {userEmail}."
8. **JSON Validation**: On blur or before Preview/Send, parse the textarea with `JSON.parse()`. Show inline error text in red if invalid: "Invalid JSON — check your syntax."
9. **Loading States**: Separate loading booleans for preview and send. Disable buttons while loading.

### Imports

```tsx
import React, { useState, useEffect } from 'react';
import { templatesAPI, usersAPI } from '../../services/api';
import type { Template, User } from '../../types';
import { SlidePanel } from '../ui/slide-panel';
import { Button } from '../ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select';
import { Label } from '../ui/label';
import { Textarea } from '../ui/textarea';
import { Badge } from '../ui/badge';
import { Spinner } from '../ui/spinner';
import { toast } from 'sonner';
```

### Acceptance Criteria

- [ ] User dropdown loads users from `usersAPI.list()`
- [ ] Variables textarea pre-populates from template's `variables` array
- [ ] Dynamic hint shows expected variable names
- [ ] Preview calls `templatesAPI.render()` and shows result
- [ ] HTML content rendered safely for email channel
- [ ] Send Test calls `templatesAPI.sendTest()` with selected user and parsed variables
- [ ] JSON validation with inline error message
- [ ] Both buttons show loading spinners during API calls
- [ ] Error toasts on API failures
- [ ] `tsc --noEmit` passes

---

## 6. Task 4: Template Content Controls Panel

**File:** `ui/src/components/templates/TemplateControlsPanel.tsx`  
**Action:** Create new file.

### Purpose

A dynamic form generated from a template's `controls` array. Allows non-technical team members to edit template content (colors, images, text) without touching the template body directly. Controls are grouped by the `group` field.

### API Dependencies (already in api.ts)

```ts
templatesAPI.getControls(apiKey, templateId) → TemplateControlsResponse
templatesAPI.updateControls(apiKey, templateId, { control_values }) → void
```

### Types (already in types/index.ts)

```ts
interface ContentControl {
    key: string;
    label: string;
    type: 'text' | 'textarea' | 'url' | 'color' | 'image' | 'number' | 'boolean' | 'select';
    default?: any;
    placeholder?: string;
    help_text?: string;
    group?: string;
    options?: string[];
}

interface TemplateControlsResponse {
    controls: ContentControl[];
    values: Record<string, any>;
}

interface UpdateControlsRequest {
    control_values: Record<string, any>;
}
```

### Props Interface

```tsx
interface TemplateControlsPanelProps {
    apiKey: string;
    templateId: string;
    templateName: string;
    open: boolean;
    onOpenChange: (open: boolean) => void;
}
```

### Component Layout

```
┌─ Content Controls: {templateName} ────────────────┐
│                                                    │
│  ▼ Branding (group header, collapsible)            │
│  ┌────────────────────────────────────────────────┐│
│  │ Logo URL:      [https://example.com/logo.png] ││
│  │                "URL for the header logo"       ││
│  │                                                ││
│  │ Brand Color:   [#FF5542] 🟥                    ││
│  │                "Primary brand color for CTA"   ││
│  │                                                ││
│  │ Show Footer:   [✓]                             ││
│  │                "Display footer in email"       ││
│  └────────────────────────────────────────────────┘│
│                                                    │
│  ▼ Content (group header, collapsible)             │
│  ┌────────────────────────────────────────────────┐│
│  │ Welcome Text:  [Hello {{user_name}}!]          ││
│  │                "Greeting shown at the top"     ││
│  │                                                ││
│  │ Tone:          [Friendly ▼]                    ││
│  │                "Select the email tone"         ││
│  └────────────────────────────────────────────────┘│
│                                                    │
│            [Reset to Defaults]  [Save]             │
└────────────────────────────────────────────────────┘
```

### Implementation Details

1. **Container**: `SlidePanel` with title "Content Controls: {templateName}".
2. **Data Fetch**: On open, call `templatesAPI.getControls(apiKey, templateId)`. Store `controls` and `values` in state.
3. **Grouping**: Group controls by `control.group`. Ungrouped controls go into a "General" group. Each group renders as a collapsible section using a simple `details`/`summary` or a manual toggle with `ChevronDown`/`ChevronUp`.
4. **Dynamic Form Rendering**: For each control, render the appropriate input based on `control.type`:

   | Type | Input Element |
   |------|--------------|
   | `text` | `<Input>` |
   | `textarea` | `<Textarea>` |
   | `url` | `<Input type="url">` |
   | `color` | `<Input type="color">` + text display of hex value |
   | `image` | `<Input type="url">` + small preview `<img>` if value is set |
   | `number` | `<Input type="number">` |
   | `boolean` | `<Checkbox>` |
   | `select` | `<Select>` with options from `control.options[]` |

5. **Values State**: Initialize from API response `values`. Fall back to `control.default` for missing keys.
6. **Help Text**: Display `control.help_text` below each input as `<p className="text-xs text-muted-foreground mt-1">`.
7. **Save**: Calls `templatesAPI.updateControls(apiKey, templateId, { control_values: values })`. Toast: "Controls updated."
8. **Reset to Defaults**: Resets all values to their `control.default`. Does NOT save automatically — user must click Save.
9. **Loading**: Show `Spinner` while fetching controls. Disable Save while submitting.
10. **Empty State**: If `controls` array is empty, show message: "This template has no content controls defined. Add a `controls` array to your template to enable this feature."

### Imports

```tsx
import React, { useState, useEffect } from 'react';
import { templatesAPI } from '../../services/api';
import type { ContentControl, TemplateControlsResponse } from '../../types';
import { SlidePanel } from '../ui/slide-panel';
import { Button } from '../ui/button';
import { Input } from '../ui/input';
import { Textarea } from '../ui/textarea';
import { Label } from '../ui/label';
import { Checkbox } from '../ui/checkbox';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select';
import { Spinner } from '../ui/spinner';
import { ChevronDown, ChevronUp } from 'lucide-react';
import { toast } from 'sonner';
```

### Acceptance Criteria

- [ ] Fetches controls from `templatesAPI.getControls()` on open
- [ ] Groups controls by `group` field with collapsible sections
- [ ] Renders correct input type for each control type (all 8 types)
- [ ] Help text displayed below each input
- [ ] Color input shows both picker and hex value
- [ ] Image input shows preview thumbnail when URL is set
- [ ] Save calls `templatesAPI.updateControls()` with current values
- [ ] Reset to Defaults restores all values without auto-saving
- [ ] Empty state shown when template has no controls
- [ ] Loading spinner while fetching, disabled button while saving
- [ ] `tsc --noEmit` passes

---

## 7. Task 5: AppTemplates Integration

**File:** `ui/src/components/AppTemplates.tsx`  
**Action:** Modify existing file to add buttons and wire up the 4 new template components from Tasks 1–4.

### Current State

`AppTemplates.tsx` (786 lines) has:
- Template CRUD (create, edit, delete)
- Template rendering preview (slide panel)
- Version History dialog with basic "Restore" (manual update)
- Template Library (clone from library)

### Changes Required

#### 1. New Imports

```tsx
import TemplateDiffViewer from './templates/TemplateDiffViewer';
import TemplateTestPanel from './templates/TemplateTestPanel';
import TemplateControlsPanel from './templates/TemplateControlsPanel';
import ConfirmDialog from './ConfirmDialog';
```

#### 2. New State Variables

```tsx
// Diff viewer state
const [diffTemplate, setDiffTemplate] = useState<Template | null>(null);

// Test send state
const [testTemplate, setTestTemplate] = useState<Template | null>(null);

// Controls panel state
const [controlsTemplate, setControlsTemplate] = useState<Template | null>(null);

// Rollback confirm state
const [rollbackTarget, setRollbackTarget] = useState<{ template: Template; version: number } | null>(null);
```

#### 3. New Action Buttons Per Template Row

Add to the existing per-template action area (alongside Edit, Delete, Preview, Version History):

| Button | Icon | Label | Action |
|--------|------|-------|--------|
| Compare | `GitCompare` | "Compare Versions" | `setDiffTemplate(template)` |
| Test Send | `Send` | "Test Send" | `setTestTemplate(template)` |
| Controls | `SlidersHorizontal` | "Controls" | `setControlsTemplate(template)` |

These should appear in the existing action button group for each template row/card.

#### 4. Replace handleRestoreVersion with handleRollback

Replace the current manual restore logic with rollback API call + confirm dialog (see Task 2).

#### 5. Render New Components

At the bottom of the component return, alongside existing dialogs:

```tsx
{/* Diff Viewer */}
{diffTemplate && (
    <TemplateDiffViewer
        apiKey={apiKey}
        appId={appId}
        templateId={diffTemplate.id}
        templateName={diffTemplate.name}
        versions={versions}
        open={!!diffTemplate}
        onOpenChange={(open) => { if (!open) setDiffTemplate(null); }}
    />
)}

{/* Test Send Panel */}
{testTemplate && (
    <TemplateTestPanel
        apiKey={apiKey}
        template={testTemplate}
        open={!!testTemplate}
        onOpenChange={(open) => { if (!open) setTestTemplate(null); }}
    />
)}

{/* Controls Panel */}
{controlsTemplate && (
    <TemplateControlsPanel
        apiKey={apiKey}
        templateId={controlsTemplate.id}
        templateName={controlsTemplate.name}
        open={!!controlsTemplate}
        onOpenChange={(open) => { if (!open) setControlsTemplate(null); }}
    />
)}

{/* Rollback Confirm */}
<ConfirmDialog
    open={!!rollbackTarget}
    onOpenChange={(open) => { if (!open) setRollbackTarget(null); }}
    title="Rollback Template"
    description={`This will create a new version with the content from v${rollbackTarget?.version}. The current version will not be deleted.`}
    onConfirm={() => rollbackTarget && handleRollback(rollbackTarget.template, rollbackTarget.version)}
/>
```

#### 6. Version History Dialog Changes

In the existing version row actions, change:
- "Restore" button → "Rollback to v{N}" button
- Click handler: `setRollbackTarget({ template: versionHistoryTemplate, version: v.version })`
- Add "Compare" button that opens diff viewer pre-selecting this version

### Notes

- The diff viewer needs the `versions` array. When opening diff from a template row (not from version history), fetch versions first via `templatesAPI.getVersions()` before opening.
- Load versions when the user clicks "Compare Versions" — the diff viewer receives them as a prop.

### Acceptance Criteria

- [ ] "Compare Versions" button appears per template → opens TemplateDiffViewer
- [ ] "Test Send" button appears per template → opens TemplateTestPanel
- [ ] "Controls" button appears per template → opens TemplateControlsPanel
- [ ] Version History: "Restore" replaced with "Rollback to v{N}" + confirm dialog
- [ ] Rollback uses `templatesAPI.rollback()` API instead of manual update
- [ ] All 4 new components render correctly inside AppTemplates
- [ ] No regressions to existing template CRUD or preview
- [ ] `tsc --noEmit` passes

---

## 8. Task 6: Team Management Tab

**File:** `ui/src/components/apps/AppTeam.tsx`  
**Action:** Create new file.

### Purpose

A tab component inside AppDetail for managing team members. Displays current members with roles, supports inviting new members, updating roles, and removing members. Permission-gated: requires JWT auth and `PermManageMembers`.

### API Dependencies (already in api.ts)

```ts
teamAPI.listMembers(appId) → AppMembership[]
teamAPI.inviteMember(appId, { email, role }) → AppMembership
teamAPI.updateRole(appId, membershipId, { role }) → AppMembership
teamAPI.removeMember(appId, membershipId) → void
```

### Types (already in types/index.ts)

```ts
type TeamRole = 'owner' | 'admin' | 'editor' | 'viewer';

interface AppMembership {
    membership_id: string;
    app_id: string;
    user_id: string;
    user_email: string;
    role: TeamRole;
    invited_by: string;
    created_at: string;
    updated_at: string;
}

interface InviteMemberRequest {
    email: string;
    role: Exclude<TeamRole, 'owner'>;
}

interface UpdateRoleRequest {
    role: TeamRole;
}
```

### Props Interface

```tsx
interface AppTeamProps {
    appId: string;
}
```

Note: No `apiKey` prop — team APIs use `appId` directly (JWT-authenticated, not API-key-authenticated).

### Component Layout

```
┌─────────────────────────────────────────────────────────────┐
│  Team Members                                [+ Invite]     │
│                                                             │
│  ┌─────────────────────────────────────────────────────────┐│
│  │ Email             │ Role        │ Joined      │ Actions ││
│  ├───────────────────┼─────────────┼─────────────┼─────────┤│
│  │ alice@team.com    │ ● Owner     │ Jan 15      │         ││
│  │ bob@team.com      │ ◐ Admin  ▼  │ Feb 03      │ [🗑]    ││
│  │ carol@team.com    │ ○ Editor ▼  │ Mar 22      │ [🗑]    ││
│  └─────────────────────────────────────────────────────────┘│
│                                                             │
│  ┌─ Permission Reference ─────────────────────────────────┐ │
│  │ Owner:  Full access, manage billing, delete app        │ │
│  │ Admin:  Manage members, settings, all resources        │ │
│  │ Editor: Create/edit templates, workflows, topics       │ │
│  │ Viewer: Read-only access to all resources              │ │
│  └────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

### Implementation Details

1. **Data Fetch**: `useApiQuery(() => teamAPI.listMembers(appId), [appId])`. Show `SkeletonTable` while loading.
2. **Member Table**: Standard `Table` from shadcn/ui.
   - Email column: text
   - Role column: `Badge` with role-specific styling:
     - Owner: `variant="default"` (dark)
     - Admin: `variant="secondary"`
     - Editor: `variant="outline"`
     - Viewer: muted text, no badge
   - For non-owner rows, the role cell is an inline `Select` dropdown that triggers `handleRoleUpdate` on change.
   - Joined column: formatted date
   - Actions column: Remove button (trash icon) — only shown for non-owner, non-self rows
3. **Invite Button**: Opens a `SlidePanel` with:
   - Email input (required, type="email")
   - Role dropdown: Admin, Editor, Viewer (no Owner option)
   - Submit button: "Send Invite"
   - Calls `teamAPI.inviteMember(appId, { email, role })`
   - Success toast: "Invitation sent to {email}"
4. **Role Update**: Inline `Select` on the role cell. On change:
   - If updating to Owner or if it's the last owner being changed, show a warning and cancel.
   - Call `teamAPI.updateRole(appId, membershipId, { role })`.
   - Toast: "Role updated to {role}"
   - Refresh member list.
5. **Remove**: `ConfirmDialog` with: "Remove {email} from this application? They will lose all access."
   - Can't remove self.
   - Can't remove last owner.
   - Call `teamAPI.removeMember(appId, membershipId)`.
   - Toast: "Member removed"
6. **Permission Reference**: Static info card at the bottom showing what each role can do. Uses `Card` with `CardContent`. Muted background.
7. **Empty State**: If no members (unlikely — app always has at least the owner): `EmptyState` with "No team members found."

### Imports

```tsx
import React, { useState } from 'react';
import { teamAPI } from '../../services/api';
import type { AppMembership, TeamRole } from '../../types';
import { useApiQuery } from '../../hooks/use-api-query';
import { SlidePanel } from '../ui/slide-panel';
import { Button } from '../ui/button';
import { Input } from '../ui/input';
import { Label } from '../ui/label';
import { Badge } from '../ui/badge';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../ui/table';
import { Card, CardContent } from '../ui/card';
import SkeletonTable from '../SkeletonTable';
import ConfirmDialog from '../ConfirmDialog';
import { UserPlus, Trash2 } from 'lucide-react';
import { toast } from 'sonner';
```

### Acceptance Criteria

- [ ] Member list loads from `teamAPI.listMembers()`
- [ ] Role badges with distinct styling per role
- [ ] Inline role dropdown for non-owner members
- [ ] Role update calls `teamAPI.updateRole()` with confirm
- [ ] Invite slide panel with email + role, calls `teamAPI.inviteMember()`
- [ ] Can't invite as Owner
- [ ] Remove member with confirm dialog
- [ ] Can't remove self or last owner
- [ ] Permission reference card at bottom
- [ ] SkeletonTable while loading
- [ ] Error and success toasts
- [ ] `tsc --noEmit` passes

---

## 9. Task 7: Custom Providers Tab

**File:** `ui/src/components/apps/AppProviders.tsx`  
**Action:** Create new file.

### Purpose

A tab component inside AppDetail for registering and managing custom delivery providers. Providers are external webhooks that receive notification payloads with HMAC-signed headers. The signing key is shown once after creation.

### API Dependencies (already in api.ts)

```ts
providersAPI.register(appId, { name, channel, webhook_url, headers? }) → CustomProvider
providersAPI.list(appId) → CustomProvider[]
providersAPI.remove(appId, providerId) → void
```

### Types (already in types/index.ts)

```ts
interface CustomProvider {
    provider_id: string;
    name: string;
    channel: string;
    webhook_url: string;
    headers?: Record<string, string>;
    signing_key?: string;
    active: boolean;
    created_at: string;
}

interface RegisterProviderRequest {
    name: string;
    channel: string;
    webhook_url: string;
    headers?: Record<string, string>;
}
```

### Props Interface

```tsx
interface AppProvidersProps {
    appId: string;
}
```

### Component Layout

```
┌─────────────────────────────────────────────────────────────┐
│  Custom Providers                       [+ Register]        │
│                                                             │
│  ┌─────────────────────────────────────────────────────────┐│
│  │ Name         │ Channel     │ Webhook URL    │ Status │ ⋮││
│  ├──────────────┼─────────────┼────────────────┼────────┤  ││
│  │ Slack Bot    │ slack_int   │ https://...    │ Active │ 🗑││
│  │ PagerDuty   │ pager_duty  │ https://...    │ Active │ 🗑││
│  └─────────────────────────────────────────────────────────┘│
│                                                             │
│  ┌─ One-Time Key Display (shown after create) ──────────┐   │
│  │ ⚠ Save this signing key — it won't be shown again.   │   │
│  │                                                       │   │
│  │ sk_live_abc123def456...                        [Copy]  │   │
│  │                                                       │   │
│  │ Use this key to verify webhook HMAC-SHA256 signatures.│   │
│  └───────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

### Implementation Details

1. **Data Fetch**: `useApiQuery(() => providersAPI.list(appId), [appId])`. Show `SkeletonTable` while loading.
2. **Provider Table**: Standard `Table`.
   - Name column: text
   - Channel column: `Badge variant="outline"`
   - Webhook URL column: truncated with tooltip, monospace font
   - Status column: Green "Active" badge
   - Actions column: Remove button (trash icon)
3. **Register Button**: Opens a `SlidePanel` with form:
   - **Name** (required): `Input` — "Descriptive name for this provider"
   - **Channel** (required): `Input` — "Custom channel name — e.g. 'slack_internal', 'pager_duty'"
     - Hint: "This channel name will be used when sending notifications. Must be unique within this app."
   - **Webhook URL** (required): `Input type="url"` — "The HTTP endpoint that will receive notification payloads"
   - **Headers** (optional): `Textarea` for JSON key-value pairs — e.g. `{"Authorization": "Bearer xxx"}`
     - Validate JSON on blur. Show inline error if invalid.
   - Submit button: "Register Provider"
4. **Post-Create: Signing Key Display**: After successful `providersAPI.register()`:
   - Close the form panel.
   - Open a `Dialog` (modal) showing the `signing_key` returned by the API.
   - Warning banner: "⚠ Save this signing key — it won't be shown again."
   - Key displayed in a `<code>` block with a Copy button.
   - Explanation: "Use this key to verify webhook HMAC-SHA256 signatures on incoming payloads."
   - Only closes when user clicks "I've saved it" or presses the close button.
5. **Remove**: `ConfirmDialog` — "Remove provider '{name}' (channel: {channel})? Notifications using this channel will fail."
   - Calls `providersAPI.remove(appId, providerId)`. Toast: "Provider removed."
6. **Empty State**: `EmptyState` with title "No Custom Providers" and description "Register a custom provider to deliver notifications to your own webhook endpoints." Action: `{ label: 'Register Provider', onClick: openForm }`.

### Imports

```tsx
import React, { useState } from 'react';
import { providersAPI } from '../../services/api';
import type { CustomProvider } from '../../types';
import { useApiQuery } from '../../hooks/use-api-query';
import { SlidePanel } from '../ui/slide-panel';
import { Button } from '../ui/button';
import { Input } from '../ui/input';
import { Label } from '../ui/label';
import { Textarea } from '../ui/textarea';
import { Badge } from '../ui/badge';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../ui/table';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '../ui/dialog';
import SkeletonTable from '../SkeletonTable';
import ConfirmDialog from '../ConfirmDialog';
import EmptyState from '../EmptyState';
import { Plus, Trash2, Copy, Check, AlertTriangle } from 'lucide-react';
import { toast } from 'sonner';
```

### Acceptance Criteria

- [ ] Provider list loads from `providersAPI.list()`
- [ ] Register form validates all required fields
- [ ] Headers textarea validates JSON on blur
- [ ] After registration, signing key displayed in a modal with copy button and warning
- [ ] Signing key modal requires explicit dismissal
- [ ] Remove provider with confirm dialog
- [ ] Empty state with "Register Provider" action button
- [ ] SkeletonTable while loading
- [ ] `tsc --noEmit` passes

---

## 10. Task 8: Multi-Environment Tab & Env Switcher

**File:** `ui/src/components/apps/AppEnvironments.tsx`  
**Action:** Create new file.

### Purpose

A tab component inside AppDetail for managing environments (dev/staging/prod). Each environment has its own API key. Includes a promote flow for copying resources between environments. Also integrates an env switcher into the AppDetail header that changes the active API key for all child tabs.

### API Dependencies (already in api.ts)

```ts
environmentsAPI.create(appId, { name }) → Environment
environmentsAPI.list(appId) → Environment[]
environmentsAPI.get(appId, envId) → Environment
environmentsAPI.delete(appId, envId) → void
environmentsAPI.promote(appId, { source_env_id, target_env_id, resources }) → void
```

### Types (already in types/index.ts)

```ts
type EnvironmentName = 'development' | 'staging' | 'production';

interface Environment {
    id: string;
    app_id: string;
    name: EnvironmentName;
    slug: string;
    api_key: string;
    is_default: boolean;
    created_at: string;
    updated_at: string;
}

interface CreateEnvironmentRequest {
    name: EnvironmentName;
}

interface PromoteEnvironmentRequest {
    source_env_id: string;
    target_env_id: string;
    resources: string[];
}
```

### Props Interface

```tsx
interface AppEnvironmentsProps {
    appId: string;
    currentApiKey: string;
    onApiKeyChange: (apiKey: string, envName: string) => void;
}
```

The `onApiKeyChange` callback is critical — it propagates the selected environment's API key up to `AppDetail`, which then passes it to all child tabs (Templates, Notifications, etc.).

### Component Layout: Environment Cards

```
┌─────────────────────────────────────────────────────────────────┐
│  Environments                                  [+ Create]       │
│                                                                 │
│  ┌── Development ─────────────────────┐  ┌── Production ──────┐ │
│  │                                    │  │                    │ │
│  │  Slug: development                 │  │  Slug: production  │ │
│  │  API Key: frn_dev_●●●●  [👁] [📋] │  │  API Key: frn...   │ │
│  │  Default: ✓                        │  │  Default: ✗        │ │
│  │  Created: Jan 15, 2025             │  │  Created: Feb 01   │ │
│  │                                    │  │                    │ │
│  │  [Use This Env]        [Delete]    │  │  [Use This Env]    │ │
│  └────────────────────────────────────┘  └────────────────────┘ │
│                                                                 │
│  ┌─ Promote Resources ──────────────────────────────────────┐   │
│  │  From: [Development ▼]    To: [Production ▼]            │   │
│  │                                                          │   │
│  │  Resources:                                              │   │
│  │  ☑ Templates   ☑ Workflows   ☐ Digest Rules             │   │
│  │                                                          │   │
│  │                                        [Promote]         │   │
│  └──────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

### Implementation Details

#### Part A: Environment List (Cards)

1. **Data Fetch**: `useApiQuery(() => environmentsAPI.list(appId), [appId])`.
2. **Card Grid**: `div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4"`. Each env is a `Card`.
3. **Card Content**:
   - **Name** in `CardTitle` with colored dot:
     - Production: `bg-green-500`
     - Staging: `bg-yellow-500`
     - Development: `bg-blue-500`
   - **Slug**: muted text
   - **API Key**: Masked by default (`frn_●●●●●●●●`). Toggle reveal button (eye icon). Copy button.
   - **Default badge**: If `is_default`, show `Badge variant="secondary"` with "Default".
   - **Created date**: formatted
4. **Card Actions**:
   - "Use This Env" button: Calls `onApiKeyChange(env.api_key, env.name)`. Highlighted if this env's API key matches `currentApiKey`.
   - "Delete" button: Only if not `is_default`. Confirm dialog: "All resources scoped to this environment will become inaccessible."

#### Part B: Create Environment

1. **Create Button**: Opens a `SlidePanel`.
2. **Form**: Single field — Environment Name as a `Select` dropdown.
   - Only show names NOT already created. E.g., if "development" and "production" exist, only "staging" appears.
   - If all 3 exist, show a message: "All environments have been created" and disable the form.
3. **Post-Create**: Show the new environment's API key in a `Dialog` modal.
   - Warning: "Use this API key in your application to route notifications to this environment."
   - Copy button, monospace code display.
   - Refresh env list.

#### Part C: Promote Resources

1. **Promote Section**: Below the env cards. Only shown if ≥2 environments exist.
2. **Source/Target Dropdowns**: Two `Select` components listing env names. Can't pick the same env for both.
3. **Resource Checkboxes**: `templates`, `workflows`, `digest-rules`. Use `Checkbox` from shadcn/ui.
4. **Promote Button**: Calls `environmentsAPI.promote(appId, { source_env_id, target_env_id, resources })`.
   - Toast: "Resources promoted from {source} to {target}."
   - Disable button if no resources selected or source === target.
5. **Confirm Dialog** before promote: "This will copy the selected resources from {source} to {target}. Existing resources with the same name in {target} will be overwritten."

#### Part D: Env Switcher (integrated into AppDetail)

This is handled in Task 11 (AppDetail Tab Extension). The `AppEnvironments` component already provides the `onApiKeyChange` callback. In AppDetail:
- Add an env switcher dropdown in the header area (next to the app name).
- The switcher shows env name + colored dot.
- Changing env updates `activeApiKey` state, which cascades to all child tabs.

### Imports

```tsx
import React, { useState } from 'react';
import { environmentsAPI } from '../../services/api';
import type { Environment, EnvironmentName } from '../../types';
import { useApiQuery } from '../../hooks/use-api-query';
import { SlidePanel } from '../ui/slide-panel';
import { Button } from '../ui/button';
import { Input } from '../ui/input';
import { Label } from '../ui/label';
import { Badge } from '../ui/badge';
import { Card, CardContent, CardHeader, CardTitle } from '../ui/card';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select';
import { Checkbox } from '../ui/checkbox';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '../ui/dialog';
import ConfirmDialog from '../ConfirmDialog';
import { Spinner } from '../ui/spinner';
import { Plus, Trash2, Eye, EyeOff, Copy, Check } from 'lucide-react';
import { toast } from 'sonner';
```

### Acceptance Criteria

- [ ] Environment list loads from `environmentsAPI.list()`
- [ ] Cards show name, slug, masked API key with reveal/copy, default badge, creation date
- [ ] Colored dots: green (production), yellow (staging), blue (development)
- [ ] "Use This Env" button calls `onApiKeyChange` callback
- [ ] Active env visually highlighted
- [ ] Create form only shows available (un-created) env names
- [ ] Post-create: API key shown in modal with copy button
- [ ] Delete with confirm, can't delete default env
- [ ] Promote section: source/target dropdowns, resource checkboxes, confirm dialog
- [ ] Promote calls `environmentsAPI.promote()` with validation (source ≠ target, ≥1 resource selected)
- [ ] Promote section hidden when < 2 environments exist
- [ ] `tsc --noEmit` passes

---

## 11. Task 9: Audit Logs Page

**File:** `ui/src/pages/audit/AuditLogsList.tsx`  
**Action:** Create new file.

### Purpose

A standalone page (not embedded in AppDetail) for viewing audit logs across all applications. Supports filtering by app, action, resource type, date range, and actor. Detail view in a slide panel.

### API Dependencies (already in api.ts)

```ts
auditAPI.list(filters?: AuditLogFilters) → { logs: AuditLog[], total: number, limit: number, offset: number }
auditAPI.get(id) → AuditLog
```

### Types (already in types/index.ts)

```ts
type AuditAction = 'create' | 'update' | 'delete' | 'send';
type ActorType = 'user' | 'api_key' | 'system';

interface AuditLog {
    audit_id: string;
    app_id: string;
    environment_id?: string;
    actor_id: string;
    actor_type: ActorType;
    action: AuditAction;
    resource: string;
    resource_id: string;
    changes: Record<string, { old: any; new: any }>;
    ip_address?: string;
    user_agent?: string;
    created_at: string;
}

interface AuditLogFilters {
    app_id?: string;
    actor_id?: string;
    action?: AuditAction;
    resource?: string;
    from_date?: string;
    to_date?: string;
    limit?: number;
    offset?: number;
}
```

### Component Layout

```
┌─ Audit Logs ──────────────────────────────────────────────────┐
│                                                                │
│  Filters:                                                      │
│  [App ▼] [Action ▼] [Resource ▼] [From Date] [To Date] [Clear]│
│                                                                │
│  ┌────────────────────────────────────────────────────────────┐│
│  │ Timestamp       │ Actor      │ Action │ Resource │ ID      ││
│  ├─────────────────┼────────────┼────────┼──────────┼─────────┤│
│  │ Jan 15 14:32    │ alice@...  │ create │ template │ tmpl_...││
│  │ Jan 15 14:30    │ API Key    │ send   │ notif    │ ntf_... ││
│  │ Jan 15 14:25    │ System     │ update │ workflow │ wf_...  ││
│  └────────────────────────────────────────────────────────────┘│
│                                                                │
│  [Pagination: < 1 2 3 ... 10 >]                               │
│                                                                │
│  ┌─ Detail Panel (SlidePanel) ─────────────────────────────┐   │
│  │  Audit ID: aud_abc123                                   │   │
│  │  App: My App                                            │   │
│  │  Actor: alice@team.com (user)                           │   │
│  │  Action: update                                         │   │
│  │  Resource: template / tmpl_123                          │   │
│  │  Timestamp: Jan 15, 2025 14:32:05 UTC                   │   │
│  │  IP: 192.168.1.1                                        │   │
│  │  User Agent: Mozilla/5.0...                             │   │
│  │                                                         │   │
│  │  Changes:                                               │   │
│  │  ┌──────────────────────────────────────────────────┐   │   │
│  │  │ subject:                                         │   │   │
│  │  │  − "Old Subject"                                 │   │   │
│  │  │  + "New Subject"                                 │   │   │
│  │  │ body:                                            │   │   │
│  │  │  − "<p>Old body</p>"                             │   │   │
│  │  │  + "<p>New body</p>"                             │   │   │
│  │  └──────────────────────────────────────────────────┘   │   │
│  └─────────────────────────────────────────────────────────┘   │
└────────────────────────────────────────────────────────────────┘
```

### Implementation Details

1. **Page Wrapper**: Same standalone page pattern as `WorkflowsList` — no `apiKey` prop needed (JWT-authenticated endpoint).
2. **Filters Bar**: Row of filter controls above the table:
   - **App**: `Select` dropdown. Load apps via `applicationsAPI.list()` on mount. Show app name as display, store `app_id`.
   - **Action**: `Select` with options: create, update, delete, send. Include "All" as default.
   - **Resource**: `Select` with options: template, notification, workflow, digest-rule, topic, user, application, provider, environment. Include "All" as default.
   - **Date Range**: Two `Input type="date"` for `from_date` and `to_date`.
   - **Clear Button**: Resets all filters.
3. **Data Fetch**: `useApiQuery(() => auditAPI.list(filters), [filters])`. Trigger fresh fetch when filters or page change.
4. **Table**: Standard `Table` with columns:
   - Timestamp: formatted with time
   - Actor: Display `actor_id` for 'user' type, "API Key" for 'api_key', "System" for 'system'. Use a `Badge variant="outline"` for the actor type.
   - Action: `Badge` with color per action:
     - create: green (`variant="default"` or custom green)
     - update: blue (custom)
     - delete: red (destructive)
     - send: neutral
   - Resource: resource type as text
   - Resource ID: truncated, monospace
5. **Row Click**: Opens detail `SlidePanel` for the clicked log entry.
6. **Detail Panel**: `SlidePanel` showing all `AuditLog` fields:
   - All metadata fields in a labeled grid
   - **Changes section**: Same diff display format as `TemplateDiffViewer` — iterate over `changes` keys, show old (red) vs new (green) values.
   - If changes is empty or null, show "No field changes recorded."
7. **Pagination**: `Pagination` component with `totalItems` from API response, `pageSize=20`.
8. **Empty State**: `EmptyState` with title "No Audit Logs" and description "Audit logs will appear here as actions are performed across your applications."
9. **Loading**: `SkeletonTable` while loading.

### Imports

```tsx
import React, { useState, useCallback, useMemo } from 'react';
import { auditAPI, applicationsAPI } from '../../services/api';
import type { AuditLog, AuditLogFilters, AuditAction, Application } from '../../types';
import { useApiQuery } from '../../hooks/use-api-query';
import { SlidePanel } from '../../components/ui/slide-panel';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import { Label } from '../../components/ui/label';
import { Badge } from '../../components/ui/badge';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../components/ui/select';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../../components/ui/table';
import { Pagination } from '../../components/Pagination';
import SkeletonTable from '../../components/SkeletonTable';
import EmptyState from '../../components/EmptyState';
import { toast } from 'sonner';
```

### Acceptance Criteria

- [ ] Audit log table loads from `auditAPI.list()` with filters
- [ ] App, Action, Resource, and Date filters work correctly
- [ ] Clear button resets all filters
- [ ] Row click opens detail SlidePanel
- [ ] Detail panel shows all metadata and changes diff
- [ ] Changes displayed in old/new format (red/green)
- [ ] Action badges with distinct colors per action type
- [ ] Actor type displayed as badge
- [ ] Pagination works with API-returned `total`
- [ ] Empty state when no logs
- [ ] SkeletonTable while loading
- [ ] `tsc --noEmit` passes

---

## 12. Task 10: Sidebar & Route Registration

### Sidebar Update

**File:** `ui/src/components/Sidebar.tsx`  
**Action:** Add "Audit Logs" nav item under the ADMIN section.

#### Current State

```tsx
const navItems: NavItem[] = [
    { label: 'Applications', icon: <LayoutGrid className="h-4 w-4" />, to: '/apps', section: 'MAIN' },
    { label: 'Workflows', icon: <Workflow className="h-4 w-4" />, to: '/workflows', section: 'MAIN' },
    { label: 'Digest Rules', icon: <Timer className="h-4 w-4" />, to: '/digest-rules', section: 'MAIN' },
    { label: 'Topics', icon: <Tag className="h-4 w-4" />, to: '/topics', section: 'MAIN' },
    { label: 'Dashboard', icon: <BarChart3 className="h-4 w-4" />, to: '/dashboard', section: 'ADMIN' },
];
```

#### Target State

```tsx
import { ..., ScrollText } from 'lucide-react';

const navItems: NavItem[] = [
    { label: 'Applications', icon: <LayoutGrid className="h-4 w-4" />, to: '/apps', section: 'MAIN' },
    { label: 'Workflows', icon: <Workflow className="h-4 w-4" />, to: '/workflows', section: 'MAIN' },
    { label: 'Digest Rules', icon: <Timer className="h-4 w-4" />, to: '/digest-rules', section: 'MAIN' },
    { label: 'Topics', icon: <Tag className="h-4 w-4" />, to: '/topics', section: 'MAIN' },
    { label: 'Dashboard', icon: <BarChart3 className="h-4 w-4" />, to: '/dashboard', section: 'ADMIN' },
    { label: 'Audit Logs', icon: <ScrollText className="h-4 w-4" />, to: '/audit', section: 'ADMIN' },
];
```

### Route Registration

**File:** `ui/src/App.tsx`  
**Action:** Add lazy import for `AuditLogsList` and register route.

#### New Lazy Import

```tsx
const AuditLogsList = lazy(() => import('./pages/audit/AuditLogsList'));
```

#### New Route

Inside the protected `DashboardLayout` route group:

```tsx
<Route path="/audit" element={<AuditLogsList />} />
```

### Acceptance Criteria

- [ ] "Audit Logs" appears in sidebar under ADMIN section
- [ ] Clicking it navigates to `/audit`
- [ ] Route is lazy-loaded
- [ ] `tsc --noEmit` passes

---

## 13. Task 11: AppDetail Tab Extension

**File:** `ui/src/pages/AppDetail.tsx`  
**Action:** Extend tabs from 8 to 11 — add Team, Providers, and Environments.

### Current State

```tsx
const [activeTab, setActiveTab] = useState<
    'overview' | 'users' | 'templates' | 'notifications' | 'digest-rules' | 'topics' | 'settings' | 'integration'
>('overview');

const tabLabels: Record<string, string> = {
    'digest-rules': 'Digest Rules',
    'topics': 'Topics',
};
```

8 tabs: overview, users, templates, notifications, digest-rules, topics, settings, integration.

### Target State

```tsx
const [activeTab, setActiveTab] = useState<
    'overview' | 'users' | 'templates' | 'notifications' | 'digest-rules' | 'topics' |
    'team' | 'providers' | 'environments' | 'settings' | 'integration'
>('overview');

const tabLabels: Record<string, string> = {
    'digest-rules': 'Digest Rules',
    'topics': 'Topics',
    'team': 'Team',
    'providers': 'Providers',
    'environments': 'Environments',
};
```

11 tabs total.

### Changes Required

#### 1. New Imports

```tsx
import AppTeam from '../components/apps/AppTeam';
import AppProviders from '../components/apps/AppProviders';
import AppEnvironments from '../components/apps/AppEnvironments';
```

#### 2. Active API Key State (for Env Switcher)

```tsx
// Below existing state
const [activeApiKey, setActiveApiKey] = useState<string>('');
const [activeEnvName, setActiveEnvName] = useState<string>('');

// In fetchAppDetails, after setApp(appData):
setActiveApiKey(appData.api_key);
setActiveEnvName('default');
```

Then use `activeApiKey` instead of `app.api_key` when passing `apiKey` to child tabs:
```tsx
// Before:
{activeTab === 'templates' && <AppTemplates appId={app.app_id} apiKey={app.api_key} ... />}

// After:
{activeTab === 'templates' && <AppTemplates appId={app.app_id} apiKey={activeApiKey || app.api_key} ... />}
```

#### 3. Env Switcher in Header

Add a small env switcher dropdown in the AppDetail header, next to the app name:

```tsx
{/* In the header area, next to the app name */}
<div className="flex items-center gap-2">
    <h1 className="text-2xl font-bold">{app.app_name}</h1>
    {activeEnvName && (
        <Badge variant="outline" className="text-xs">
            <span className={`inline-block w-2 h-2 rounded-full mr-1 ${
                activeEnvName === 'production' ? 'bg-green-500' :
                activeEnvName === 'staging' ? 'bg-yellow-500' :
                activeEnvName === 'development' ? 'bg-blue-500' :
                'bg-gray-400'
            }`} />
            {activeEnvName}
        </Badge>
    )}
</div>
```

The actual env switching is triggered from the Environments tab via the `onApiKeyChange` callback.

#### 4. New Tab Render Blocks

```tsx
{activeTab === 'team' && app && (
    <AppTeam appId={app.app_id} />
)}

{activeTab === 'providers' && app && (
    <AppProviders appId={app.app_id} />
)}

{activeTab === 'environments' && app && (
    <AppEnvironments
        appId={app.app_id}
        currentApiKey={activeApiKey || app.api_key}
        onApiKeyChange={(apiKey, envName) => {
            setActiveApiKey(apiKey);
            setActiveEnvName(envName);
        }}
    />
)}
```

#### 5. Tab Order

Recommended order for the tab bar:

| Position | Tab | Access |
|----------|-----|--------|
| 1 | Overview | Any |
| 2 | Users | Any |
| 3 | Templates | Any |
| 4 | Notifications | Any |
| 5 | Workflows (if present) | Any |
| 6 | Digest Rules | Any |
| 7 | Topics | Any |
| 8 | Team | JWT |
| 9 | Providers | JWT |
| 10 | Environments | JWT |
| 11 | Settings | Any |

Note: "Integration" tab is repositioned or merged into Settings depending on existing content. If Integration has distinct content, keep it at position 12.

#### 6. Tab Overflow Handling

With 11+ tabs, horizontal overflow is likely on smaller screens. Ensure the tab bar has:
```tsx
<div className="flex gap-1 overflow-x-auto border-b pb-px scrollbar-hide">
```

This allows horizontal scrolling on narrow viewports.

### Acceptance Criteria

- [ ] ActiveTab union type includes all 11 tabs
- [ ] tabLabels map includes human-readable labels for Team, Providers, Environments
- [ ] New imports for AppTeam, AppProviders, AppEnvironments
- [ ] All 3 new tab render blocks present and conditional on `app` being loaded
- [ ] `activeApiKey` state added and env switcher badge in header
- [ ] `onApiKeyChange` callback wired from AppEnvironments to update `activeApiKey`
- [ ] Child tabs (Templates, Notifications, etc.) use `activeApiKey` instead of raw `app.api_key`
- [ ] Tab bar handles overflow with horizontal scroll
- [ ] `tsc --noEmit` passes

---

## 14. Task 12: Build Verification

**Action:** Run the full build pipeline and verify zero errors.

### Steps

```powershell
cd ui
npx tsc --noEmit          # TypeScript type check — must be zero errors
npm run build             # Vite production build — must succeed
```

### Verification Checklist

- [ ] `tsc --noEmit` returns 0 errors
- [ ] `vite build` completes successfully
- [ ] New chunks appear in build output for:
  - `AuditLogsList` (lazy-loaded page)
  - Template components (bundled with `AppTemplates`)
  - App team/providers/environments (bundled with `AppDetail`)
- [ ] No TypeScript warnings about unused imports
- [ ] No console errors in browser when navigating to:
  - `/apps/:id` → Team tab
  - `/apps/:id` → Providers tab
  - `/apps/:id` → Environments tab
  - `/apps/:id` → Templates tab → Compare/Test/Controls buttons
  - `/audit`

---

## 15. Acceptance Criteria

### Template Enhancements
- [ ] Diff viewer: two-version comparison with field-level highlighting
- [ ] Rollback: confirm dialog, uses rollback API, creates new version
- [ ] Test send: user picker, variable editor, live preview, send test
- [ ] Content controls: dynamic form from controls array, grouped, all 8 input types
- [ ] All 4 features wired into AppTemplates with action buttons

### Team Management
- [ ] Member list with role badges and permissions info
- [ ] Invite form in slide panel (email + role, no owner option)
- [ ] Inline role update dropdown with validation
- [ ] Remove member with confirm (can't remove self or last owner)

### Custom Providers
- [ ] Provider list with channel badges
- [ ] Register form with name, channel, URL, optional headers
- [ ] One-time signing key display in modal after creation
- [ ] Remove with confirm dialog

### Multi-Environment
- [ ] Environment cards with masked API keys (reveal/copy)
- [ ] Create form showing only available env names
- [ ] Post-create API key display
- [ ] Promote dialog: source → target with resource checkboxes
- [ ] Env switcher in AppDetail header with colored dots
- [ ] Switching env updates API key for all child tabs

### Audit Logs
- [ ] Standalone page at `/audit` with sidebar nav
- [ ] Filterable table (app, action, resource, date range)
- [ ] Detail slide panel with metadata and changes diff
- [ ] Pagination and empty state

### Build
- [ ] `tsc --noEmit` passes with zero errors
- [ ] `vite build` succeeds
- [ ] All new routes lazy-loaded

---

## Appendix: Pattern Reference

These are the established patterns from Phase 1 and Phase 2 that MUST be followed in all Phase 3 work.

### Import Patterns

| Import | Path | Export Type |
|--------|------|-------------|
| `useApiQuery` | `../../hooks/use-api-query` | Named: `{ useApiQuery }` |
| `Pagination` | `../../components/Pagination` | Named: `{ Pagination }` |
| `SlidePanel` | `../ui/slide-panel` | Named: `{ SlidePanel }` |
| `SkeletonTable` | `../../components/SkeletonTable` | Default |
| `EmptyState` | `../../components/EmptyState` | Default |
| `ConfirmDialog` | `../../components/ConfirmDialog` | Default |
| `Spinner` | `../ui/spinner` | Named: `{ Spinner }` |

### Component API Patterns

| Component | Key Props | Notes |
|-----------|-----------|-------|
| `useApiQuery` | `(fetcher, deps, options?)` | 3 args: fetcher function, dependency array, `{ enabled?: boolean }` |
| `Pagination` | `currentPage, totalItems, pageSize, onPageChange` | NOT `totalPages` |
| `SkeletonTable` | `columns` | NOT `cols` |
| `EmptyState` | `action: { label, onClick }` | NOT a ReactNode |
| `ConfirmDialog` | `onOpenChange` | NOT `onClose` |
| `Template` | `id` field | NOT `template_id` |

### Dual-Mode Pattern (standalone + embedded)

For components that can be used both as standalone pages AND as tabs inside AppDetail:

```tsx
interface Props {
    apiKey?: string;    // If provided, skip app picker
    embedded?: boolean; // If true, skip page wrapper/title
}
```

Note: Team, Providers, and Environments do NOT use dual-mode — they are AppDetail-only tabs using `appId` (JWT-auth, not API-key-auth).

### Theme Constants

- Background: `#FAFAFA`, Foreground: `#121212`
- Accent: `#FF5542` (coral, use sparingly — CTAs only)
- Borders: `#E5E5E5`
- Border radius: `0.5rem`
- Font: Inter

---

*This plan should be treated as the implementation guide for Phase 3. Update the checklist items as tasks are completed.*
