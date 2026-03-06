# FreeRangeNotify — UI ↔ API Full Integration Plan

> **Date:** March 7, 2026  
> **Scope:** Wire every backend API end-to-end into the admin dashboard (`ui/`), apply the monkeys.com.co brand, and provide documentation + SDK scope.  
> **Prerequisite:** All backend APIs are implemented and pass `go build ./...` / `go vet ./...`.

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Current State Audit](#2-current-state-audit)
3. [Design System & Theme](#3-design-system--theme)
4. [Architecture & Patterns](#4-architecture--patterns)
5. [API Dependency Graph](#5-api-dependency-graph)
6. [Phase 1 — Foundation Rewire (Week 1)](#6-phase-1--foundation-rewire)
7. [Phase 2 — Core API Surfaces (Weeks 2-3)](#7-phase-2--core-api-surfaces)
8. [Phase 3 — Advanced Features (Weeks 4-5)](#8-phase-3--advanced-features)
9. [Phase 4 — Observability & Admin (Week 6)](#9-phase-4--observability--admin)
10. [Phase 5 — Polish, Docs & SDK (Week 7)](#10-phase-5--polish-docs--sdk)
11. [File Inventory (New & Modified)](#11-file-inventory)
12. [Testing Strategy](#12-testing-strategy)
13. [Documentation & SDK Scope](#13-documentation--sdk-scope)

---

## 1. Executive Summary

### What We Have

- **19 / 20 backend features** are fully implemented (95%) — see `IMPLEMENTATION_AUDIT.md`.
- The UI currently integrates **~35 of ~85 API endpoints** (41%). Entire feature domains have no UI: **Workflows, Digest Rules, Topics, Team/RBAC, Audit Logs, Custom Providers, Multi-Environment**, and most Phase 5 inbox operations (snooze, unread count, mark-all-read, bulk archive).
- The theme is an Azure/corporate blue with Ubuntu font — it needs to shift to the **monkeys.com.co** editorial aesthetic: Inter font, near-white backgrounds (`#FAFAFA`), warm coral accent (`#FF5542`) used very sparingly, and high-contrast neutral typography.

### What We Need

1. **~50 unintegrated API endpoints** wired into the UI with proper forms, field validation, contextual help, and dependency-aware UX (e.g. the notification form's template picker pre-populates from the Templates API).
2. **Monkeys brand** applied globally — subtle, eye-friendly, fast-loading.
3. **Documentation hub** in the dashboard + scope for SDK reference.
4. **Zero full-page reloads** — SPA architecture with code-splitting per route.

### Guiding Principles

| Principle | Detail |
|-----------|--------|
| **API-first** | Every UI action maps to exactly one API call. No client-side business logic. |
| **Dependency-aware UX** | When a field needs data from another API (e.g. `template_id`), show a searchable dropdown that queries the dependency API lazily — never make the user copy-paste UUIDs. |
| **No eye strain** | Light, neutral palette. The coral accent (`#FF5542`) is used only on primary CTAs, active nav items, and destructive confirmations — nowhere else. Background: `#FAFAFA`, card: `#FFFFFF`, text: `#121212`. |
| **Fast by default** | Code-split every route. Lazy-load heavy components. No render-blocking API calls — show skeleton loaders. |
| **Toast, don&apos;t redirect** | After create/update/delete, show a Sonner toast and optimistically update the list — don&apos;t navigate away. |

---

## 2. Current State Audit

### Integrated vs Missing APIs

| Domain | Integrated | Missing | Coverage |
|--------|-----------|---------|----------|
| Auth | 8/8 | 1 (change-password) | 89% |
| Applications | 8/8 | 0 | 100% |
| Users | 10/10 | 2 (bulk create, subscriber-hash) | 83% |
| Notifications | 8/15 | 7 (batch, unread, read, read-all, archive, snooze, unsnooze) | 53% |
| Templates | 10/15 | 5 (rollback, diff, test, controls, single version) | 67% |
| Quick-Send | 1/1 | 0 | 100% |
| Admin/System | 7/7 | 0 | 100% |
| **Workflows** | 0/9 | 9 | 0% |
| **Digest Rules** | 0/5 | 5 | 0% |
| **Topics** | 0/9 | 9 | 0% |
| **Team/RBAC** | 0/4 | 4 | 0% |
| **Audit Logs** | 0/2 | 2 | 0% |
| **Custom Providers** | 0/3 | 3 | 0% |
| **Multi-Environment** | 0/5 | 5 | 0% |
| **Presence** | 0/1 | 1 | 0% |
| **Total** | **~35 / 85** | **~50** | **41%** |

### Current UI Bugs to Fix (P0)

| # | Bug | File | Fix |
|---|-----|------|-----|
| 1 | `ActivityFeed.tsx` reads `localStorage.getItem('token')` but app stores as `access_token` | `ActivityFeed.tsx` | Change to `access_token` |
| 2 | Dead imports: `AppCard.tsx`, `AppForm.tsx` not used anywhere | `components/` | Delete files |
| 3 | `next-themes` installed but no dark mode toggle | `package.json` | Remove or wire up in Phase 1 |

---

## 3. Design System & Theme

### 3.1 Color Palette — "Monkeys Neutral"

The goal is a **clean, editorial feel** with minimal use of the orangish accent. The coral is reserved for primary action buttons, active navigation, and critical alerts. Everything else is neutral grayscale.

```
┌─────────────────────────────────────────────────────────┐
│ Token               │ Light Mode   │ Dark Mode          │
├─────────────────────┼──────────────┼────────────────────┤
│ --background        │ #FAFAFA      │ #121212            │
│ --foreground        │ #121212      │ #F5F5F5            │
│ --card              │ #FFFFFF      │ #1E1E1E            │
│ --card-foreground   │ #121212      │ #F5F5F5            │
│ --primary           │ #121212      │ #F5F5F5            │
│ --primary-foreground│ #FAFAFA      │ #121212            │
│ --accent            │ #FF5542      │ #FF5542            │
│ --accent-foreground │ #FFFFFF      │ #FFFFFF            │
│ --muted             │ #F0F0F0      │ #2C2C2C            │
│ --muted-foreground  │ #6B7280      │ #9CA3AF            │
│ --border            │ #E5E5E5      │ #3F3F3F            │
│ --input             │ #E5E5E5      │ #3F3F3F            │
│ --destructive       │ #DC2626      │ #EF4444            │
│ --ring              │ #121212      │ #F5F5F5            │
│ --success           │ #16A34A      │ #22C55E            │
│ --warning           │ #F59E0B      │ #FBBF24            │
│ --radius            │ 8px          │ 8px                │
└─────────────────────────────────────────────────────────┘
```

**Usage rules:**
- `--accent` (#FF5542): Only on → primary `<Button>` fill, active sidebar item indicator, error/destructive badges, and the logo mark. **Never on backgrounds, cards, headers, or borders.**
- `--primary` (#121212): All standard buttons (outline variant), text, icons.
- `--muted`: Secondary surfaces, hover states, code blocks.
- `--border`: Table borders, card outlines, dividers — always 1px.

### 3.2 Typography

```css
--font-sans: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Roboto', sans-serif;
--font-mono: 'JetBrains Mono', 'Fira Code', 'Consolas', monospace;
```

| Element | Size | Weight | Tracking |
|---------|------|--------|----------|
| Page title (h1) | 1.75rem (28px) | 600 | -0.02em |
| Section heading (h2) | 1.25rem (20px) | 600 | -0.01em |
| Card title (h3) | 1rem (16px) | 500 | normal |
| Body | 0.875rem (14px) | 400 | normal |
| Caption / hint | 0.75rem (12px) | 400 | normal |
| Code / API key | 0.8125rem (13px) | 400 (mono) | normal |

### 3.3 Component Conventions

- **Cards**: `bg-card border rounded-lg p-6`. No shadows except on hover (`shadow-sm`).
- **Buttons**: 
  - Primary: `bg-accent text-white hover:bg-accent/90` (used sparingly — 1 per visible area).
  - Secondary: `bg-primary text-primary-foreground hover:bg-primary/90` (dark, neutral).
  - Outline: `border border-border bg-transparent hover:bg-muted` (most common).
  - Ghost: `hover:bg-muted` (icon buttons, nav items).
- **Tables**: Alternate row striping via `even:bg-muted/30`. Header row: `bg-muted text-muted-foreground uppercase text-xs tracking-wider`.
- **Forms**: Labels above inputs. Hint text below in `text-muted-foreground text-xs`. Required fields marked with `*` not asterisk — use `aria-required`.
- **Loading**: Skeleton pulse animations. Never a full-screen spinner.

### 3.4 Responsive Breakpoints

| Breakpoint | Width | Layout |
|------------|-------|--------|
| Mobile | < 768px | Single column, collapsible sidebar → hamburger, stacked cards |
| Tablet | 768px–1024px | Sidebar hidden, top nav, 2-column grid |
| Desktop | > 1024px | Fixed sidebar (240px) + main content area |

### 3.5 Files to Create/Modify

| File | Action |
|------|--------|
| `ui/src/index.css` | Replace CSS variables with Monkeys palette, switch font to Inter |
| `ui/tailwind.config.ts` | Not needed — Tailwind v4 uses CSS variables directly |
| `ui/index.html` | Add Inter font preload `<link>` |
| `ui/src/components/ui/sidebar.tsx` | New — collapsible sidebar component |

---

## 4. Architecture & Patterns

### 4.1 Project Structure (Target)

```
ui/src/
├── main.tsx
├── App.tsx                             # Root router with lazy routes
├── index.css                           # Theme variables + Tailwind
├── contexts/
│   └── AuthContext.tsx                  # Existing — minor fixes
├── hooks/
│   ├── use-mobile.ts                   # Existing
│   ├── use-debounce.ts                 # New — debounced search inputs
│   └── use-api-query.ts               # New — lightweight fetch + cache + loading state
├── lib/
│   ├── utils.ts                        # Existing
│   └── api-keys.ts                     # New — per-app API key context helper
├── services/
│   └── api.ts                          # Existing — extend with all missing endpoints
├── types/
│   └── index.ts                        # Existing — extend with all missing types
├── layouts/
│   ├── DashboardLayout.tsx             # New — sidebar + topbar + main content
│   └── AuthLayout.tsx                  # New — centered card layout for login/register
├── pages/
│   ├── LandingPage.tsx                 # Existing — retheme
│   ├── Login.tsx                       # Existing — retheme
│   ├── Register.tsx                    # Existing — retheme
│   ├── ForgotPassword.tsx              # Existing — retheme
│   ├── ResetPassword.tsx               # Existing — retheme
│   ├── SSOCallback.tsx                 # Existing — no changes
│   ├── apps/
│   │   ├── AppsList.tsx                # Existing — refactor with new layout
│   │   └── AppDetail.tsx               # Existing — extend with new tabs
│   ├── dashboard/
│   │   └── Dashboard.tsx               # Existing — extend
│   ├── workflows/                      # NEW
│   │   ├── WorkflowsList.tsx
│   │   ├── WorkflowBuilder.tsx
│   │   └── WorkflowExecutions.tsx
│   ├── digest/                         # NEW
│   │   └── DigestRulesList.tsx
│   ├── topics/                         # NEW
│   │   └── TopicsList.tsx
│   └── audit/                          # NEW
│       └── AuditLogsList.tsx
├── components/
│   ├── Header.tsx                      # Existing → refactor to Topbar.tsx
│   ├── Sidebar.tsx                     # NEW — main navigation
│   ├── Footer.tsx                      # Existing — keep for landing page only
│   ├── ProtectedRoute.tsx              # Existing
│   ├── Pagination.tsx                  # Existing
│   ├── SkeletonTable.tsx               # NEW — loading state for tables
│   ├── EmptyState.tsx                  # NEW — zero-data illustrations
│   ├── ConfirmDialog.tsx               # NEW — reusable delete/destructive confirm
│   ├── ResourcePicker.tsx              # NEW — searchable dropdown for cross-API refs
│   ├── apps/
│   │   ├── AppUsers.tsx                # Existing — extend
│   │   ├── AppTemplates.tsx            # Existing — extend with diff/rollback/controls
│   │   ├── AppNotifications.tsx        # Existing — extend with snooze/archive/unread
│   │   ├── AppSettings.tsx             # Existing (currently in AppDetail)
│   │   ├── AppTeam.tsx                 # NEW — Team/RBAC management
│   │   ├── AppProviders.tsx            # NEW — Custom provider management
│   │   └── AppEnvironments.tsx         # NEW — Multi-environment management
│   ├── workflows/
│   │   ├── WorkflowStepEditor.tsx      # NEW — visual step editor
│   │   ├── WorkflowStepCard.tsx        # NEW — single step display
│   │   ├── ExecutionTimeline.tsx        # NEW — execution step progress
│   │   └── TriggerTestPanel.tsx         # NEW — test trigger with payload
│   ├── templates/
│   │   ├── TemplateEditor.tsx          # Existing — extend
│   │   ├── TemplateDiffViewer.tsx      # NEW — side-by-side version diff
│   │   ├── TemplateControlsPanel.tsx   # NEW — content controls form
│   │   └── TemplateTestPanel.tsx       # NEW — send test notification
│   ├── notifications/
│   │   ├── NotificationDetail.tsx       # NEW — full notification view
│   │   └── BulkActionsBar.tsx          # NEW — mark read/archive selected
│   ├── topics/
│   │   ├── TopicSubscribers.tsx        # NEW — subscriber management
│   │   └── TopicForm.tsx               # NEW — create/edit topic
│   ├── digest/
│   │   └── DigestRuleForm.tsx          # NEW — create/edit digest rule
│   ├── audit/
│   │   └── AuditLogDetail.tsx          # NEW — single audit entry view
│   └── ui/                             # Existing shadcn primitives + additions
│       ├── sidebar.tsx                 # NEW
│       ├── skeleton.tsx                # NEW
│       ├── tooltip.tsx                 # NEW
│       ├── popover.tsx                 # NEW
│       ├── command.tsx                 # NEW — for ResourcePicker (cmdk)
│       ├── sheet.tsx                   # NEW — mobile sidebar
│       └── ... (existing primitives)
```

### 4.2 Data Fetching Pattern

**No React Query / SWR.** Keep it simple with a custom `useApiQuery` hook:

```tsx
// hooks/use-api-query.ts
function useApiQuery<T>(fetcher: () => Promise<T>, deps: any[]) {
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refetch = useCallback(async () => {
    setLoading(true);
    try {
      const result = await fetcher();
      setData(result);
      setError(null);
    } catch (err) { setError(err.message); }
    finally { setLoading(false); }
  }, deps);

  useEffect(() => { refetch(); }, [refetch]);
  return { data, loading, error, refetch };
}
```

This keeps the bundle tiny while providing loading/error/refetch everywhere.

### 4.3 Cross-API Dependency Pattern — `ResourcePicker`

The most critical UX problem: fields like `template_id`, `user_id`, `workflow trigger_id` require IDs from other APIs. Users should never have to copy-paste UUIDs.

**`ResourcePicker<T>` component:**
- Takes `fetcher`, `labelKey`, `valueKey`, and `renderItem` props.
- On focus → fetches the resource list lazily.
- Shows a searchable combobox (built on cmdk `<Command>`).
- Displays the selected item's human-readable name, stores the UUID.
- Used across: workflow step config (`template_id`), digest rule creation (`template_id`), topic subscriber add (`user_ids`), notification send (`user_id`, `template_id`).

```tsx
// Example usage in WorkflowStepEditor:
<ResourcePicker
  label="Template"
  fetcher={() => templatesAPI.list(apiKey)}
  labelKey="name"
  valueKey="id"
  value={step.config.template_id}
  onChange={(id) => updateStep({ ...step, config: { ...step.config, template_id: id } })}
  hint="The template used to render this step's notification content"
/>
```

### 4.4 API Key Context Flow

Dashboard users authenticate with JWT. But most resource APIs (users, templates, notifications, etc.) require an **API key**. The current flow:

1. User selects an app from the Apps list.
2. `AppDetail` fetches the app and extracts `app.api_key`.
3. All child components receive `apiKey` as a prop.

**Enhancement:** When Multi-Environment is enabled, the user can switch environments. Each environment has its own API key. The flow becomes:

1. User selects an app → fetches environments → shows env switcher in the app header.
2. Selected environment's `api_key` becomes the active key.
3. All resource API calls use the selected env's key.

This is wired through a simple `useState` in `AppDetail` — no global context needed.

---

## 5. API Dependency Graph

Understanding which APIs depend on which is critical for both implementation order and UX design. The user needs to create resources in the right order, and the UI must guide them.

```
┌─────────────────────────────────────────────────────────────────┐
│                      DEPENDENCY GRAPH                           │
│                                                                 │
│  Auth (Login/Register)                                          │
│    └── Applications (Create App → get API key)                  │
│          ├── Environments (Create dev/staging/prod)             │
│          │     └── All resource APIs use env-scoped API key     │
│          ├── Team/RBAC (Invite members to app)                  │
│          ├── Custom Providers (Register webhook endpoints)      │
│          ├── Users (Create notification recipients)             │
│          │     ├── Devices (Register push tokens)               │
│          │     ├── Preferences (Set channel preferences)        │
│          │     └── Presence (Check-in for Smart Delivery)       │
│          ├── Templates (Create notification templates)          │
│          │     ├── Versions (Create/diff/rollback versions)     │
│          │     ├── Controls (Configure content controls)        │
│          │     └── Library (Clone pre-built templates)          │
│          ├── Topics (Create subscriber groups)                  │
│          │     └── Subscribers (Add users to topics)            │
│          │           └── depends on: Users                      │
│          ├── Digest Rules (Batch notification rules)            │
│          │     └── depends on: Templates                        │
│          ├── Workflows (Multi-step notification flows)          │
│          │     ├── Steps reference: Templates, Digest config    │
│          │     ├── Trigger requires: Users (user_id), Payload   │
│          │     └── Executions (monitor running instances)       │
│          └── Notifications (Send notifications)                 │
│                ├── depends on: Users (user_id)                  │
│                ├── optionally: Templates (template_id)          │
│                ├── optionally: Topics (topic_key for fanout)    │
│                ├── optionally: Workflows (trigger_id)           │
│                └── Inbox ops: unread, read, snooze, archive     │
│                                                                 │
│  Admin (Dashboard-level)                                        │
│    ├── Audit Logs (read-only, filterable)                       │
│    ├── Queue Stats / DLQ (system health)                        │
│    ├── Provider Health (delivery channel status)                │
│    ├── Analytics (notification metrics)                         │
│    ├── Activity Feed (real-time SSE)                            │
│    └── Webhook Playground (testing tool)                        │
└─────────────────────────────────────────────────────────────────┘
```

### UX Implications

1. **Onboarding wizard must create in order:** App → (optional: Environment) → First User → First Template → Send Test Notification. The existing `SetupWizard.tsx` only does App + Template. Extend it.
2. **Notification Send form** needs inline pickers for `user_id` (from Users API), `template_id` (from Templates API), and an optional `topic_key` (from Topics API). Each picker explains what it is and why it's needed.
3. **Workflow Builder** is the most dependency-heavy page: each step can reference a template (channel step), a digest config, or a condition. The builder must have inline template selection and a "test trigger" panel that takes `user_id` + custom payload.
4. **Empty states** must guide the user: "No templates yet — create one to start sending notifications" with a CTA button.

---

## 6. Phase 1 — Foundation Rewire (Week 1)

### Goal
Apply the Monkeys theme, fix the layout architecture (sidebar navigation), fix existing bugs, and extend `api.ts` + `types/index.ts` with all missing API functions and types.

### 6.1 Theme Application

| Task | File | Detail |
|------|------|--------|
| 6.1.1 | `index.html` | Add `<link rel="preconnect" href="https://fonts.googleapis.com">` and Inter font `<link>` |
| 6.1.2 | `index.css` | Replace all CSS custom properties with Monkeys Neutral palette (see §3.1). Replace Ubuntu with Inter. Add dark mode `.dark` variant. |
| 6.1.3 | `LandingPage.tsx` | Retheme hero, feature cards, footer to neutral palette. Replace any blue accents with neutral. Use coral only on CTA button. |
| 6.1.4 | `Login.tsx` / `Register.tsx` / `ForgotPassword.tsx` / `ResetPassword.tsx` | Retheme to use `AuthLayout` (centered card on `#FAFAFA` background). |

### 6.2 Layout Architecture

| Task | File | Detail |
|------|------|--------|
| 6.2.1 | `layouts/DashboardLayout.tsx` | **New.** Fixed sidebar (240px) + top bar + main content scrollable area. Sidebar collapses to icon-only on mobile. |
| 6.2.2 | `layouts/AuthLayout.tsx` | **New.** Centered card with logo above. Used by login/register/reset pages. |
| 6.2.3 | `components/Sidebar.tsx` | **New.** Navigation sections: Apps, Workflows, Digest Rules, Topics, Audit, Dashboard (admin). Shows active indicator with coral left border. |
| 6.2.4 | `components/Header.tsx` → `components/Topbar.tsx` | Rename. Convert to dashboard top bar: breadcrumb + user menu dropdown + optional env switcher. Keep Footer only for landing page. |
| 6.2.5 | `App.tsx` | Restructure routes: auth routes use `AuthLayout`, protected routes use `DashboardLayout`. Lazy-load all route components. |

### 6.3 API Layer Extension

Extend `services/api.ts` with all missing endpoints:

| API Group | Functions to Add | Count |
|-----------|-----------------|-------|
| `authAPI` | `changePassword` | 1 |
| `usersAPI` | `bulkCreate`, `getSubscriberHash` | 2 |
| `notificationsAPI` | `sendBatch`, `getUnreadCount`, `listUnread`, `markRead`, `markAllRead`, `bulkArchive`, `cancelBatch`, `snooze`, `unsnooze` | 9 |
| `templatesAPI` | `rollback`, `diff`, `sendTest`, `getControls`, `updateControls`, `getVersion` | 6 |
| `workflowsAPI` | `create`, `list`, `get`, `update`, `delete`, `trigger`, `listExecutions`, `getExecution`, `cancelExecution` | 9 |
| `digestRulesAPI` | `create`, `list`, `get`, `update`, `delete` | 5 |
| `topicsAPI` | `create`, `list`, `get`, `getByKey`, `update`, `delete`, `addSubscribers`, `removeSubscribers`, `getSubscribers` | 9 |
| `teamAPI` | `inviteMember`, `listMembers`, `updateRole`, `removeMember` | 4 |
| `auditAPI` | `list`, `get` | 2 |
| `environmentsAPI` | `create`, `list`, `get`, `delete`, `promote` | 5 |
| `providersAPI` | `register`, `list`, `remove` | 3 |
| `presenceAPI` | `checkIn` | 1 |
| **Total** | | **56** |

### 6.4 Types Extension

Add to `types/index.ts`:

```typescript
// ── Workflows ──
interface WorkflowStep { id: string; name: string; type: 'channel'|'delay'|'digest'|'condition'; order: number; config: StepConfig; on_success?: string; on_failure?: string; skip_if?: StepCondition; }
interface StepConfig { channel?: string; template_id?: string; provider?: string; duration?: string; digest_key?: string; window?: string; max_batch?: number; condition?: StepCondition; }
interface StepCondition { field: string; operator: 'eq'|'neq'|'contains'|'gt'|'lt'|'exists'|'not_read'; value: any; }
interface Workflow { id: string; app_id: string; environment_id?: string; name: string; description: string; trigger_id: string; steps: WorkflowStep[]; status: 'draft'|'active'|'inactive'; version: number; created_by: string; created_at: string; updated_at: string; }
interface WorkflowExecution { id: string; workflow_id: string; app_id: string; user_id: string; transaction_id?: string; status: 'running'|'paused'|'completed'|'failed'|'cancelled'; payload: Record<string,any>; step_results: Record<string,StepResult>; started_at: string; completed_at?: string; }
interface StepResult { step_id: string; status: 'pending'|'running'|'completed'|'failed'|'skipped'; notification_id?: string; digest_count?: number; started_at?: string; completed_at?: string; error?: string; }

// ── Digest Rules ──
interface DigestRule { id: string; app_id: string; environment_id?: string; name: string; digest_key: string; window: string; channel: string; template_id: string; max_batch: number; status: 'active'|'inactive'; created_at: string; updated_at: string; }

// ── Topics ──
interface Topic { id: string; app_id: string; environment_id?: string; name: string; key: string; description?: string; created_at: string; updated_at: string; }
interface TopicSubscription { id: string; topic_id: string; app_id: string; user_id: string; created_at: string; }

// ── Team/RBAC ──
interface AppMembership { membership_id: string; app_id: string; user_id: string; user_email: string; role: 'owner'|'admin'|'editor'|'viewer'; invited_by: string; created_at: string; updated_at: string; }

// ── Audit ──
interface AuditLog { audit_id: string; app_id: string; environment_id?: string; actor_id: string; actor_type: 'user'|'api_key'|'system'; action: 'create'|'update'|'delete'|'send'; resource: string; resource_id: string; changes: Record<string,any>; ip_address?: string; user_agent?: string; created_at: string; }

// ── Environments ──
interface Environment { id: string; app_id: string; name: string; slug: string; api_key: string; is_default: boolean; created_at: string; updated_at: string; }

// ── Custom Providers ──
interface CustomProvider { provider_id: string; name: string; channel: string; webhook_url: string; headers?: Record<string,string>; signing_key?: string; active: boolean; created_at: string; }
```

### 6.5 Bug Fixes

| # | Fix | File | Time |
|---|-----|------|------|
| 1 | `ActivityFeed.tsx`: `localStorage.getItem('token')` → `localStorage.getItem('access_token')` | `ActivityFeed.tsx` | 5 min |
| 2 | Delete unused `AppCard.tsx` and `AppForm.tsx` | `components/` | 5 min |
| 3 | Remove or defer `next-themes` (add dark mode toggle later) | `package.json` | 10 min |

### 6.6 Shared Components

| Component | Purpose |
|-----------|---------|
| `SkeletonTable.tsx` | N-row skeleton loader for table pages |
| `EmptyState.tsx` | Icon + title + description + CTA for zero-data states |
| `ConfirmDialog.tsx` | "Are you sure?" dialog for delete/destructive actions |
| `ResourcePicker.tsx` | Searchable combobox for cross-API ID references (see §4.3) |
| `JsonEditor.tsx` | Simple JSON textarea with syntax validation for payload fields |

### Phase 1 Deliverables Checklist

- [ ] Monkeys theme applied (Inter font, neutral palette, coral accent minimal)
- [ ] `DashboardLayout` + `Sidebar` + `Topbar` working
- [ ] All routes using lazy loading with `React.lazy` + `Suspense`
- [ ] `api.ts` extended with all 56 missing endpoint functions
- [ ] `types/index.ts` extended with all missing interfaces
- [ ] 5 shared components created
- [ ] 3 bugs fixed
- [ ] Build passes (`npm run build` — zero errors, zero warnings)

---

## 7. Phase 2 — Core API Surfaces (Weeks 2-3)

### Goal
Build the 4 major feature pages from scratch (Workflows, Digest Rules, Topics, Notifications enhancement) and extend AppDetail with Team, Providers, and Environments tabs.

### 7.1 Workflows Page (Highest Complexity)

**Route:** `/workflows` (list) → `/workflows/new` (builder) → `/workflows/:id` (detail/edit)

#### 7.1.1 Workflows List (`pages/workflows/WorkflowsList.tsx`)

| Element | Detail |
|---------|--------|
| Table columns | Name, Trigger ID, Status (badge), Steps count, Version, Updated At |
| Actions | Create (button → WorkflowBuilder), Edit, Delete (with confirm), duplicate |
| Filters | Status dropdown (draft/active/inactive), search by name |
| Empty state | "No workflows yet — create your first multi-step notification flow" |
| API calls | `workflowsAPI.list(apiKey, limit, offset)` |

#### 7.1.2 Workflow Builder (`pages/workflows/WorkflowBuilder.tsx`)

This is the most complex page in the entire UI. It's a **vertical step editor** (not a drag-and-drop canvas — keep it simple and fast).

```
┌───────────────────────────────────────────────────────────┐
│ ← Back to Workflows           [Save Draft] [Activate]     │
├───────────────────────────────────────────────────────────┤
│ Name: [________________]  Trigger ID: [________________]   │
│ Description: [________________________________]            │
├───────────────────────────────────────────────────────────┤
│ STEPS                                                      │
│                                                            │
│ ┌─ Step 1 ─────────────────────────────────────────────┐  │
│ │ ● Channel Step                          [Edit] [✕]   │  │
│ │ Channel: email    Template: "Welcome Email" (v3)     │  │
│ └──────────────────────────────────────────────────────┘  │
│         │                                                  │
│         ▼                                                  │
│ ┌─ Step 2 ─────────────────────────────────────────────┐  │
│ │ ◷ Delay Step                            [Edit] [✕]   │  │
│ │ Wait: 24h                                            │  │
│ └──────────────────────────────────────────────────────┘  │
│         │                                                  │
│         ▼                                                  │
│ ┌─ Step 3 ─────────────────────────────────────────────┐  │
│ │ ⁇ Condition Step                        [Edit] [✕]   │  │
│ │ IF payload.opened == false → continue                │  │
│ │ ELSE → skip remaining                               │  │
│ └──────────────────────────────────────────────────────┘  │
│         │                                                  │
│         ▼                                                  │
│       [ + Add Step ]                                       │
│                                                            │
├───────────────────────────────────────────────────────────┤
│ TEST TRIGGER                                               │
│ User: [picker → Users API]                                 │
│ Payload (JSON): [__________]                               │
│ [Trigger Workflow]                                         │
└───────────────────────────────────────────────────────────┘
```

**Key dependency-aware UX elements:**
- **Template picker** in Channel step: `ResourcePicker` backed by `templatesAPI.list()`. Shows template name, channel badge, and version. Hint text: *"Select the template that defines this notification's content. The template's channel must match the step's channel."*
- **User picker** in Test Trigger: `ResourcePicker` backed by `usersAPI.list()`. Shows user email/external_id. Hint text: *"Select the user who will receive this test notification. This is the internal user_id, not the external_id."*
- **Payload editor**: JSON textarea with `JsonEditor` component. Hint text: *"Pass variables that your template expects — e.g. `{\"user_name\": \"Alice\", \"order_id\": \"12345\"}`. These map to `{{user_name}}` in your template body."*

| Step Type | Config Fields | Dependency |
|-----------|--------------|------------|
| `channel` | Channel (dropdown: email, push, sms, webhook, slack, discord, etc.), Template (ResourcePicker → Templates), Provider (optional text) | Templates API |
| `delay` | Duration (input: "1h", "30m", "7d") | None |
| `digest` | Digest Key (text), Window (input: "1h"), Max Batch (number) | None |
| `condition` | Field path (text), Operator (dropdown), Value (text/number) | None |

#### 7.1.3 Workflow Executions (`pages/workflows/WorkflowExecutions.tsx`)

| Element | Detail |
|---------|--------|
| Table columns | Execution ID (truncated), Workflow Name, User ID, Status (badge: running/completed/failed/cancelled), Started At, Duration |
| Row expand | Click row → `ExecutionTimeline` component showing step-by-step progress with status badges, durations, and error messages |
| Actions | Cancel (for running), Retry (for failed — via re-trigger) |
| Filters | Workflow dropdown (from workflows list), Status dropdown, date range |
| API calls | `workflowsAPI.listExecutions(apiKey, { workflow_id, limit, offset })` |

#### 7.1.4 Execution Timeline (`components/workflows/ExecutionTimeline.tsx`)

A vertical timeline showing each step:

```
● Step 1: Send Email (channel)        ✅ completed (1.2s)
│  → notification_id: abc-123
│
● Step 2: Wait 24h (delay)            ✅ completed
│
● Step 3: Check if opened (condition) ⬜ skipped
│  → User already opened the email
│
● Step 4: Send reminder (channel)     ⬜ skipped
```

### 7.2 Digest Rules Page

**Route:** Sub-tab in AppDetail or standalone at `/digest-rules` (depends on nav decision).

**Recommendation:** Keep it as an AppDetail sub-tab since digest rules are per-app. Add a "Digest Rules" tab to the existing app detail tabs.

#### 7.2.1 Digest Rules List + Form

| Element | Detail |
|---------|--------|
| Table columns | Name, Digest Key, Window, Channel (badge), Template Name, Status (badge), Updated At |
| Create form (slide panel) | Name*, Digest Key* (hint: "Events with the same key are grouped together"), Window* (hint: "How long to accumulate — e.g. '1h', '30m'"), Channel* (dropdown), Template* (`ResourcePicker` → Templates API), Max Batch (number, hint: "Max events per digest — 0 = unlimited") |
| Actions | Edit (same form pre-filled), Delete (confirm), Toggle status |
| API calls | `digestRulesAPI.create/list/get/update/delete(apiKey, ...)` |

**Dependency UX:** The Template picker only shows templates whose channel matches the selected channel. If the user picks `email` as the channel, only email templates appear in the picker.

### 7.3 Topics Page

**Route:** Sub-tab in AppDetail.

#### 7.3.1 Topics List + Create Form

| Element | Detail |
|---------|--------|
| Table columns | Name, Key, Description (truncated), Subscriber Count, Created At |
| Create form (slide panel) | Name*, Key* (auto-slug from name, editable, hint: "Machine-readable key — e.g. 'project-123-watchers'. Used in API calls."), Description |
| Actions | Edit, Delete (confirm), View Subscribers |
| API calls | `topicsAPI.create/list/get/update/delete(apiKey, ...)` |

#### 7.3.2 Topic Subscribers (`components/topics/TopicSubscribers.tsx`)

Shown when clicking a topic row or "Manage Subscribers" action.

| Element | Detail |
|---------|--------|
| Current subscribers | Table: User ID, Email/External ID, Added At. With remove button per row. |
| Add subscribers | Multi-select `ResourcePicker` backed by `usersAPI.list()`. Shows user email/external_id. Button: "Add Selected". |
| Hint text | "Users added to this topic will receive all notifications sent to this topic's key via the broadcast or topic notification API." |
| API calls | `topicsAPI.addSubscribers(apiKey, topicId, { user_ids })`, `topicsAPI.removeSubscribers(apiKey, topicId, { user_ids })`, `topicsAPI.getSubscribers(apiKey, topicId, limit, offset)` |

### 7.4 Notifications Enhancements

Extend existing `AppNotifications.tsx` with missing inbox operations.

| Feature | UI Element | API |
|---------|-----------|-----|
| Unread count | Badge on "Notifications" tab header | `notificationsAPI.getUnreadCount(apiKey, { user_id })` |
| Mark as read | Checkbox per row + "Mark Read" bulk action button | `notificationsAPI.markRead(apiKey, { notification_ids })` |
| Mark all read | "Mark All Read" button in toolbar | `notificationsAPI.markAllRead(apiKey, { user_id })` |
| Bulk archive | Multi-select checkboxes + "Archive" bulk action | `notificationsAPI.bulkArchive(apiKey, { notification_ids })` |
| Snooze | "Snooze" button on notification row → duration picker (15m, 1h, 4h, 24h, custom) | `notificationsAPI.snooze(apiKey, notificationId, { until: ISO8601 })` |
| Unsnooze | "Unsnooze" button on snoozed notifications | `notificationsAPI.unsnooze(apiKey, notificationId)` |
| Send batch | "Send Batch" button on toolbar → batch ID input | `notificationsAPI.sendBatch(apiKey, { ... })` |
| Cancel batch | "Cancel Batch" in actions menu → batch ID confirm | `notificationsAPI.cancelBatch(apiKey, { batch_id })` |

**Bulk Actions Bar** (`components/notifications/BulkActionsBar.tsx`):
- Appears at the top when ≥1 notification is selected (checkbox).
- Shows: "N selected" | [Mark Read] | [Archive] | [Cancel]
- Sticky bar, same pattern as Gmail/Novu.

### 7.5 AppDetail New Tabs

Extend the existing `AppDetail.tsx` from 6 tabs to 10:

| Tab | Component | Auth Level |
|-----|-----------|------------|
| Overview | Existing | Any |
| Users | Existing `AppUsers.tsx` | Any |
| Templates | Existing `AppTemplates.tsx` (extended) | Any |
| Notifications | Existing `AppNotifications.tsx` (extended) | Any |
| Workflows | New — embedded workflow list filtered by app | Any |
| Digest Rules | New `DigestRulesList` scoped to app | Any |
| Topics | New `TopicsList` scoped to app | Any |
| Team | New `AppTeam.tsx` | JWT (PermManageMembers) |
| Providers | New `AppProviders.tsx` | JWT (owner only) |
| Environments | New `AppEnvironments.tsx` | JWT (PermManageApp) |
| Settings | Existing (refactored from AppDetail inline) | JWT |

**Tab overflow on mobile:** Horizontal scrollable tab bar with overflow indicators (chevrons on edges).

### Phase 2 Deliverables Checklist

- [ ] Workflows list, builder (vertical step editor), execution timeline
- [ ] Digest rules list with template-filtered ResourcePicker
- [ ] Topics list with subscriber management
- [ ] Notifications: unread count, mark read/all, bulk archive, snooze/unsnooze
- [ ] AppDetail extended with 4 new tabs (Team, Providers, Environments, Digest/Topics)
- [ ] All forms use ResourcePicker for cross-API dependencies
- [ ] Empty states on all new pages
- [ ] All new pages use SkeletonTable loading states

---

## 8. Phase 3 — Advanced Features (Weeks 4-5)

### 8.1 Template Enhancements

#### 8.1.1 Template Diff Viewer (`TemplateDiffViewer.tsx`)

A side-by-side comparison of two template versions.

| Element | Detail |
|---------|--------|
| Version selectors | Two dropdowns: "From version" and "To version" (populated by `templatesAPI.getVersions`) |
| Diff display | Two-column layout. Changed fields highlighted: green for additions, red for removals. Shows: body, subject, variables, metadata, controls. |
| API call | `templatesAPI.diff(apiKey, templateId, { from_version, to_version })` |
| Access | Button on template detail: "Compare Versions" |

#### 8.1.2 Template Rollback

| Element | Detail |
|---------|--------|
| Trigger | Button on version list: "Rollback to this version" |
| Confirm dialog | "This will create a new version (v{N+1}) with the content from v{target}. The current version will not be deleted." |
| API call | `templatesAPI.rollback(apiKey, templateId, { target_version })` |
| Post-action | Refresh version list, show toast with new version number |

#### 8.1.3 Template Test Send (`TemplateTestPanel.tsx`)

A slide-out panel for sending a test notification from a template.

```
┌─ Send Test Notification ──────────────────────────┐
│                                                    │
│ User:     [ResourcePicker → Users API]             │
│                                                    │
│ Channel:  [auto-filled from template]              │
│                                                    │
│ Variables (JSON):                                  │
│ ┌────────────────────────────────────────────────┐ │
│ │ {                                              │ │
│ │   "user_name": "Alice",                        │ │
│ │   "order_id": "ORD-12345"                      │ │
│ │ }                                              │ │
│ └────────────────────────────────────────────────┘ │
│ Hint: These variables fill {{user_name}} etc.      │
│ in the template body.                              │
│                                                    │
│ Preview:                                           │
│ ┌──────────────────────────────────────────────┐   │
│ │ (rendered HTML/text output here)             │   │
│ └──────────────────────────────────────────────┘   │
│                                                    │
│            [Preview]  [Send Test]                  │
└────────────────────────────────────────────────────┘
```

- **Preview** calls `templatesAPI.render(apiKey, templateId, { variables })` and shows the result.
- **Send Test** calls `templatesAPI.sendTest(apiKey, templateId, { user_id, variables })`.
- The variables hint dynamically shows the template's `variables` list: *"This template expects: user_name, order_id, tracking_url"*.

#### 8.1.4 Content Controls Panel (`TemplateControlsPanel.tsx`)

For non-technical team members to edit template content without touching the template body.

| Element | Detail |
|---------|--------|
| Display | Dynamic form generated from template's `controls` array. Each control has: label, type (text/textarea/url/color/image/number/boolean/select), default, placeholder, help_text, group. |
| Grouping | Controls are grouped by `group` field. Each group is a collapsible section. |
| Save | `templatesAPI.updateControls(apiKey, templateId, { control_values: { ... } })` |
| Read | `templatesAPI.getControls(apiKey, templateId)` |

### 8.2 Team Management (`AppTeam.tsx`)

| Element | Detail |
|---------|--------|
| Member list | Table: Email, Role (badge with color), Joined At, Actions |
| Role badges | Owner: neutral/dark, Admin: neutral, Editor: outline, Viewer: muted |
| Invite form | Slide panel: Email*, Role* (dropdown: admin/editor/viewer — can't invite as owner). |
| Update role | Inline dropdown on the role cell. Confirm on change. Can't change own role. Can't demote last owner. |
| Remove | Confirm dialog. Can't remove self. Can't remove last owner. |
| Permissions table | Static info box showing: "Owners can X, Admins can Y, Editors can Z, Viewers can W" |
| API calls | `teamAPI.inviteMember/listMembers/updateRole/removeMember(appId, ...)` |

### 8.3 Custom Providers (`AppProviders.tsx`)

| Element | Detail |
|---------|--------|
| Provider list | Table: Name, Channel, Webhook URL, Status (active badge), Created At |
| Register form | Slide panel: Name*, Channel* (text, hint: "Custom channel name — e.g. 'slack_internal', 'pager_duty'"), Webhook URL* (url input), Headers (JSON key-value editor) |
| Post-create | Show the `signing_key` in a one-time-visible code block with copy button. Warning: "Save this key — it won't be shown again. Use it to verify webhook HMAC signatures." |
| Remove | Confirm dialog with channel name. |
| API calls | `providersAPI.register/list/remove(apiKey, appId, ...)` |

### 8.4 Multi-Environment (`AppEnvironments.tsx`)

| Element | Detail |
|---------|--------|
| Environment list | Cards (not table): Name, Slug, API Key (masked, with copy/reveal buttons), Is Default (badge), Created At |
| Create form | Slide panel: Name* (dropdown: development/staging/production). Only names not already created are shown. |
| Post-create | Show the environment API key in a one-time code block. Explain: "Use this API key in your application to route notifications to this environment." |
| Promote | Button: "Promote Resources". Dialog: Source env (dropdown) → Target env (dropdown) → Resources (checkboxes: templates, workflows). Shows count preview. |
| Delete | Confirm dialog. Can't delete default env. Warning: "All resources scoped to this environment will become inaccessible." |
| Env switcher | In the AppDetail header: dropdown showing current env name + colored dot (green=production, yellow=staging, blue=development). Changing env updates the `apiKey` used by all child tabs. |

### 8.5 Audit Logs Page (`pages/audit/AuditLogsList.tsx`)

| Element | Detail |
|---------|--------|
| Table columns | Timestamp, Actor (email or "API Key" or "System"), Action (badge: create/update/delete/send), Resource Type, Resource ID (truncated, link if applicable) |
| Filters | App (dropdown), Action (dropdown), Resource type (dropdown), Date range, Actor search |
| Detail view | Slide panel: Full audit entry with `changes` diff displayed as a before/after list |
| API calls | `auditAPI.list(filters)`, `auditAPI.get(id)` |
| Auth | JWT-only. Requires `PermViewAudit`. Show "Access Denied" if user lacks permission. |

### Phase 3 Deliverables Checklist

- [ ] Template diff viewer, rollback, test send panel, content controls panel
- [ ] Team management with invitation, role management, permission info
- [ ] Custom provider registration with signing key reveal
- [ ] Multi-environment with env switcher and promote dialog
- [ ] Audit logs with filtering and detail view
- [ ] All panels use slide-out pattern (consistent with existing UX)

---

## 9. Phase 4 — Observability & Admin (Week 6)

### 9.1 Dashboard Enhancements

Extend `Dashboard.tsx` with richer admin visibility:

| Sub-tab | Enhancements |
|---------|-------------|
| Overview | Add: total apps, total users, total templates, total active workflows cards. Add: notifications sent today/this week chart. |
| Analytics | Already partially done. Add: per-channel breakdown (pie chart or horizontal bar), delivery rate %, average latency. |
| Activity | Fix SSE token bug (done in P1). Add: filter by event type. Add: auto-scroll toggle. |
| Tools | Enhance: Webhook Playground already works. Add: "Quick Test" panel — select an app, user, template → send a test notification. Link to API docs. |

### 9.2 Queue Management Enhancements

Currently shows stats and DLQ. Enhance:

| Feature | Detail |
|---------|--------|
| Queue depth chart | Simple sparkline for each priority queue (high/normal/low) over last hour |
| DLQ replay | Existing. Add: select individual messages to replay (checkbox) |
| Provider health | Existing. Add: latency sparklines per provider, last error message |

### 9.3 Change Password

| Element | Detail |
|---------|--------|
| Location | User menu dropdown → "Change Password" option |
| Dialog | Current Password*, New Password*, Confirm Password*. Validation: min 8 chars, must match. |
| API call | `authAPI.changePassword({ current_password, new_password })` |

### Phase 4 Deliverables Checklist

- [ ] Dashboard overview cards with real-time stats
- [ ] Analytics charts enhanced with per-channel breakdown
- [ ] Activity feed SSE bug fixed + event type filter
- [ ] Quick test panel in Tools tab
- [ ] Change password dialog in user menu
- [ ] Queue depth sparklines

---

## 10. Phase 5 — Polish, Docs & SDK (Week 7)

### 10.1 UX Polish

| Task | Detail |
|------|--------|
| Loading states | Audit all pages: replace raw `loading ? "Loading..." : content` with `SkeletonTable`/`SkeletonCard` |
| Error boundaries | Add a React error boundary at the layout level. Show a friendly error page with "Reload" button. |
| Keyboard shortcuts | `Ctrl+K` → quick search (command palette style) searching across apps, templates, workflows, topics by name |
| Mobile responsive | Test all table pages. Convert to card layout on `< 768px`. Test sidebar collapse. |
| Toast consistency | Audit all create/update/delete actions use Sonner toast with consistent messaging |
| Performance | Run Lighthouse on `/apps`, `/apps/:id`, `/workflows`. Target: >90 performance score. Lazy-load all heavy components. |
| Accessibility | All interactive elements have `aria-label`. All images have `alt`. Tab navigation works on all forms. |

### 10.2 Onboarding Wizard Enhancement

Extend `SetupWizard.tsx` to cover the full end-to-end flow:

| Step | Current | Enhanced |
|------|---------|----------|
| 1 | Create App ✅ | Same |
| 2 | — | **Create First Environment** (optional — skip to use default) |
| 3 | — | **Create First User** (email/external_id, explain it's the notification recipient) |
| 4 | Create/Clone Template ✅ | Same, but filtered to app |
| 5 | — | **Send Test Notification** — auto-fills user_id and template_id from steps 3+4. Shows result. |

Each step explains *why* in plain language: "Every notification needs a recipient. Create a user who represents a person in your system."

### 10.3 Contextual Help & Field Hints

Every form field that involves a cross-API reference or a non-obvious concept gets a hint. These are critical for end-to-end testing.

| Field | Hint Text |
|-------|-----------|
| `user_id` (notification send) | "The internal UUID of the notification recipient. Select from your registered users above, or pass an external_id with the lookup option." |
| `template_id` (notification send) | "Optional. If provided, the notification content is rendered from this template. Otherwise, provide title and body directly." |
| `trigger_id` (workflow trigger) | "The workflow's trigger identifier. This must match the trigger_id of an existing active workflow." |
| `topic_key` (notification broadcast) | "The topic key to fan out this notification to. All users subscribed to this topic will receive it." |
| `payload` (workflow trigger / template render) | "A JSON object with values for your template variables. Example: `{\"user_name\": \"Alice\"}` maps to `{{user_name}}`." |
| `digest_key` (digest rule) | "Events with the same digest_key are grouped into a single batched notification." |
| `window` (digest rule / workflow step) | "The time window for accumulation. Examples: '30m', '1h', '4h', '24h'." |
| `api_key` (environment) | "Each environment has its own API key. Use it in your backend to control which environment receives notifications." |
| `channel` (any send/template) | "The delivery channel: email, push (APNS/FCM), sms, webhook, sse, slack, discord, whatsapp." |
| `external_id` (user) | "Your system's user identifier (e.g. 'usr_12345'). Used to map between your system and FreeRangeNotify's internal UUIDs." |
| `signing_key` (custom provider) | "HMAC-SHA256 signing key for verifying webhook payloads. Save this — it won't be shown again." |

---

## 11. File Inventory (New & Modified)

### New Files (~40)

| File | Phase | Category |
|------|-------|----------|
| `layouts/DashboardLayout.tsx` | P1 | Layout |
| `layouts/AuthLayout.tsx` | P1 | Layout |
| `components/Sidebar.tsx` | P1 | Navigation |
| `components/Topbar.tsx` | P1 | Navigation |
| `components/SkeletonTable.tsx` | P1 | Shared |
| `components/EmptyState.tsx` | P1 | Shared |
| `components/ConfirmDialog.tsx` | P1 | Shared |
| `components/ResourcePicker.tsx` | P1 | Shared |
| `components/JsonEditor.tsx` | P1 | Shared |
| `hooks/use-debounce.ts` | P1 | Hook |
| `hooks/use-api-query.ts` | P1 | Hook |
| `lib/api-keys.ts` | P1 | Utility |
| `pages/workflows/WorkflowsList.tsx` | P2 | Page |
| `pages/workflows/WorkflowBuilder.tsx` | P2 | Page |
| `pages/workflows/WorkflowExecutions.tsx` | P2 | Page |
| `pages/digest/DigestRulesList.tsx` | P2 | Page |
| `pages/topics/TopicsList.tsx` | P2 | Page |
| `pages/audit/AuditLogsList.tsx` | P3 | Page |
| `components/workflows/WorkflowStepEditor.tsx` | P2 | Feature |
| `components/workflows/WorkflowStepCard.tsx` | P2 | Feature |
| `components/workflows/ExecutionTimeline.tsx` | P2 | Feature |
| `components/workflows/TriggerTestPanel.tsx` | P2 | Feature |
| `components/topics/TopicSubscribers.tsx` | P2 | Feature |
| `components/topics/TopicForm.tsx` | P2 | Feature |
| `components/digest/DigestRuleForm.tsx` | P2 | Feature |
| `components/notifications/NotificationDetail.tsx` | P2 | Feature |
| `components/notifications/BulkActionsBar.tsx` | P2 | Feature |
| `components/templates/TemplateDiffViewer.tsx` | P3 | Feature |
| `components/templates/TemplateControlsPanel.tsx` | P3 | Feature |
| `components/templates/TemplateTestPanel.tsx` | P3 | Feature |
| `components/apps/AppTeam.tsx` | P3 | Feature |
| `components/apps/AppProviders.tsx` | P3 | Feature |
| `components/apps/AppEnvironments.tsx` | P3 | Feature |
| `components/audit/AuditLogDetail.tsx` | P3 | Feature |
| `components/ui/sidebar.tsx` | P1 | shadcn |
| `components/ui/skeleton.tsx` | P1 | shadcn |
| `components/ui/tooltip.tsx` | P1 | shadcn |
| `components/ui/popover.tsx` | P1 | shadcn |
| `components/ui/command.tsx` | P1 | shadcn |
| `components/ui/sheet.tsx` | P1 | shadcn |

### Modified Files (~15)

| File | Phase | Changes |
|------|-------|---------|
| `index.html` | P1 | Add Inter font preload |
| `index.css` | P1 | Replace CSS variables with Monkeys palette |
| `App.tsx` | P1 | New route structure, lazy loading, layout wrappers |
| `services/api.ts` | P1 | Add 56 missing API functions |
| `types/index.ts` | P1 | Add all missing type interfaces |
| `contexts/AuthContext.tsx` | P1 | Minor cleanup |
| `pages/LandingPage.tsx` | P1 | Retheme |
| `pages/Login.tsx` | P1 | Retheme + AuthLayout |
| `pages/Register.tsx` | P1 | Retheme + AuthLayout |
| `pages/AppDetail.tsx` | P2 | Add 4 new tabs, env switcher |
| `components/AppNotifications.tsx` | P2 | Add unread/read/snooze/archive features |
| `components/AppTemplates.tsx` | P3 | Add diff/rollback/test/controls buttons |
| `components/ActivityFeed.tsx` | P1 | Fix SSE token bug |
| `components/SetupWizard.tsx` | P5 | Extended onboarding flow |
| `pages/Dashboard.tsx` | P4 | Enhanced overview + quick test panel |

### Deleted Files (2)

| File | Reason |
|------|--------|
| `components/AppCard.tsx` | Dead code — never imported |
| `components/AppForm.tsx` | Dead code — never imported |

---

## 12. Testing Strategy

### 12.1 Unit Tests (Optional — Not Primary Focus)

Use Vitest (already available via Vite). Test:
- `useApiQuery` hook behavior (loading, error, refetch)
- `ResourcePicker` selection logic
- `JsonEditor` validation
- Type-level sanity (TypeScript compilation)

### 12.2 End-to-End Manual Testing Matrix

This is the critical testing path. Each row is a complete user journey that tests API dependencies end-to-end.

| Journey | Steps | APIs Tested |
|---------|-------|-------------|
| **First-run onboarding** | Register → Create App → Create User → Create Template → Send Test | auth, apps, users, templates, notifications |
| **Workflow lifecycle** | Create Template → Create Workflow (channel step referencing template) → Trigger Workflow (select user, pass payload) → View Execution | templates, workflows, users |
| **Digest lifecycle** | Create Template → Create Digest Rule (select template) → Send events → Wait for flush → Check notification | templates, digest-rules, notifications |
| **Topic broadcast** | Create Topic → Add Subscribers (pick users) → Send Broadcast (use topic key) → Each subscriber gets notification | topics, users, notifications |
| **Team management** | Invite Member → Log in as member → Verify access based on role → Update role → Verify changed access | team, auth, apps |
| **Environment promotion** | Create 2 envs → Create template in dev env → Promote to prod → Verify template exists in prod | environments, templates |
| **Template versioning** | Create Template → Edit → Create Version → Compare Diff → Rollback → Verify content | templates |
| **Custom provider** | Register Provider → Send notification to custom channel → Verify webhook received at playground URL | providers, notifications, playground |
| **Audit trail** | Perform various actions → Check audit logs → Filter by action/resource → Verify entries | audit, various |
| **Inbox operations** | Send notifications → View unread count → Mark read → Snooze → Unsnooze → Bulk archive | notifications |

### 12.3 Visual Build Check

```bash
cd ui
npm run build          # Zero errors, zero warnings
npm run preview        # Manual visual check — all pages render
```

### 12.4 Performance Targets

| Metric | Target |
|--------|--------|
| Initial bundle (gzipped) | < 200KB |
| Largest Contentful Paint | < 1.5s |
| Time to Interactive | < 2.0s |
| Lighthouse Performance | > 90 |
| Route code-split | Every page lazy-loaded |

---

## 13. Documentation & SDK Scope

### 13.1 In-App Documentation Hub

Add a `/docs` route in the dashboard (accessible from sidebar) with:

| Section | Content | Source |
|---------|---------|--------|
| Getting Started | Quick start guide: create app → create user → send notification | Static markdown rendered as React |
| API Reference | Interactive endpoint listing per domain (auth, apps, users, templates, etc.). Each endpoint shows: method, path, request body, response, example curl. | Generated from `docs/openapi/swagger.yaml` |
| SDK Guide | Installation, setup, and usage for Go SDK, JS SDK, React SDK | Static markdown |
| Channels | Setup guide for each channel: webhook, email/SMTP, Apple Push Notification service, Firebase Cloud Messaging, Slack, Discord, WhatsApp, SSE | Static markdown |
| Workflows | How to create multi-step flows, step types, trigger payloads | Static markdown |
| Environments | Dev/staging/prod setup, API key scoping, promotion | Static markdown |

**Implementation:** Use `react-markdown` + `remark-gfm` for rendering. Store docs as `.md` files in `ui/src/docs/`. Sidebar sub-nav for doc sections. Syntax highlighting with `react-syntax-highlighter` (lazy-loaded).

### 13.2 SDK Reference Scope

The SDKs already exist at `sdk/go/`, `sdk/js/`, `sdk/react/`. Documentation scope:

#### Go SDK (`sdk/go/`)

| Topic | Content |
|-------|---------|
| Installation | `go get github.com/the-monkeys/freerangenotify/sdk/go` |
| Client setup | `client := freerange.New("api-key", freerange.WithBaseURL("..."))` |
| Send notification | Code example with template + variables |
| Trigger workflow | Code example with trigger_id + user_id + payload |
| Topic publish | Code example with topic key |
| Error handling | Custom error types |

#### JS SDK (`sdk/js/`)

| Topic | Content |
|-------|---------|
| Installation | `npm install @freerangenotify/js` |
| Client setup | `const client = new FreeRangeNotify({ apiKey, baseURL })` |
| All 10 headless methods | `fetchNotifications`, `markAsRead`, `markAllAsRead`, `archive`, `snooze`, `unsnooze`, `fetchUnreadCount`, `fetchPreferences`, `updatePreferences`, `connect` (SSE) |
| SSE real-time | Subscribing to events with callbacks |

#### React SDK (`sdk/react/`)

| Topic | Content |
|-------|---------|
| Installation | `npm install @freerangenotify/react` |
| Provider setup | `<FreeRangeProvider apiKey="..." userId="..." baseURL="...">` |
| `<NotificationBell>` | Props: position, tabs, unreadBadge, dark mode |
| `<Preferences>` | Channel-level opt-in/opt-out controls |
| Hooks | `useNotifications`, `useUnreadCount`, `usePreferences` |
| Theming | CSS variables for customization |

### 13.3 Documentation Files to Create

```
ui/src/docs/
├── getting-started.md
├── api-reference.md         # Or auto-generated from swagger
├── channels/
│   ├── webhook.md
│   ├── email-smtp.md
│   ├── apns.md
│   ├── fcm.md
│   ├── slack.md
│   ├── discord.md
│   ├── whatsapp.md
│   └── sse.md
├── workflows.md
├── environments.md
├── topics.md
├── templates.md
├── sdk/
│   ├── go.md
│   ├── javascript.md
│   └── react.md
└── troubleshooting.md
```

### 13.4 Swagger/OpenAPI Integration

The backend already generates `docs/swagger.json` via Swag. Plan:

1. Add `swagger-ui-react` or `redoc` as an optional dependency (lazy-loaded).
2. Route: `/docs/api` renders the Swagger UI pointing at `/v1/swagger.json`.
3. Alternative: Render a custom API reference page from the OpenAPI spec using a lightweight renderer.

**Recommendation:** Use `redoc` — it's read-only, lighter than swagger-ui, and produces a clean, searchable API reference.

---

## Summary — Effort Estimate

| Phase | Scope | Duration | Key Deliverables |
|-------|-------|----------|------------------|
| **P1** | Foundation Rewire | Week 1 | Theme, layout, api.ts + types, shared components, bug fixes |
| **P2** | Core API Surfaces | Weeks 2-3 | Workflows (builder + executions), Digest Rules, Topics, Notification enhancements, AppDetail new tabs |
| **P3** | Advanced Features | Weeks 4-5 | Template diff/rollback/test/controls, Team RBAC, Custom Providers, Multi-Environment, Audit Logs |
| **P4** | Observability & Admin | Week 6 | Dashboard enhancements, queue charts, change password |
| **P5** | Polish, Docs & SDK | Week 7 | Onboarding wizard, docs hub, SDK reference, UX polish, performance audit |
| **Total** | | **~7 weeks** | 85/85 API endpoints integrated, ~40 new files, ~15 modified files |

---

*This plan should be treated as the source of truth for all UI work. Update the checklist items as they are completed.*
