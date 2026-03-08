# Phase 5 — Polish, Docs & SDK: Implementation Plan

> **Parent:** [UI_API_INTEGRATION_PLAN.md](UI_API_INTEGRATION_PLAN.md)  
> **Prerequisite:** [PHASE4_OBSERVABILITY_ADMIN.md](PHASE4_OBSERVABILITY_ADMIN.md) (completed)  
> **Duration:** ~1 week  
> **Goal:** Add a React error boundary, upgrade loading states to skeletons, extend the onboarding wizard, build an in-app documentation hub with API reference, add a command palette, and perform a final accessibility/performance pass.

---

## Table of Contents

1. [Task Breakdown](#1-task-breakdown)
2. [Dependency Order](#2-dependency-order)
3. [Task 1: Error Boundary](#3-task-1-error-boundary)
4. [Task 2: Loading States Upgrade](#4-task-2-loading-states-upgrade)
5. [Task 3: Onboarding Wizard Enhancement](#5-task-3-onboarding-wizard-enhancement)
6. [Task 4: Documentation Hub — Pages & Routing](#6-task-4-documentation-hub--pages--routing)
7. [Task 5: Documentation Hub — Content Files](#7-task-5-documentation-hub--content-files)
8. [Task 6: API Reference Page (Swagger/ReDoc)](#8-task-6-api-reference-page-swaggerredoc)
9. [Task 7: Command Palette (Ctrl+K)](#9-task-7-command-palette-ctrlk)
10. [Task 8: Accessibility Pass](#10-task-8-accessibility-pass)
11. [Task 9: Wire Everything + Final Routing](#11-task-9-wire-everything--final-routing)
12. [Task 10: Build Verification & Performance](#12-task-10-build-verification--performance)
13. [Acceptance Criteria](#13-acceptance-criteria)

---

## 1. Task Breakdown

| # | Task | New Files | Modified Files | Est. |
|---|------|-----------|----------------|------|
| 1 | Error Boundary | 1 | 1 | 1.5h |
| 2 | Loading States Upgrade | 0 | 3 | 1.5h |
| 3 | Onboarding Wizard Enhancement | 0 | 1 | 3h |
| 4 | Documentation Hub — Pages & Routing | 3 | 3 | 3h |
| 5 | Documentation Hub — Content Files | 7 | 0 | 4h |
| 6 | API Reference Page (Swagger/ReDoc) | 1 | 1 | 2h |
| 7 | Command Palette (Ctrl+K) | 1 | 2 | 3h |
| 8 | Accessibility Pass | 0 | 5 | 2h |
| 9 | Wire Everything + Final Routing | 0 | 2 | 1h |
| 10 | Build Verification & Performance | 0 | 0 | 1h |
| **Total** | | **13** | **18** | **~22h** |

### New Files (13)

| File | Task | Description |
|------|------|-------------|
| `components/ErrorBoundary.tsx` | 1 | React error boundary with friendly fallback UI and "Reload" button |
| `pages/docs/DocsLayout.tsx` | 4 | Docs layout with sidebar navigation between doc sections |
| `pages/docs/DocsPage.tsx` | 4 | Markdown renderer page — loads `.md` content by slug |
| `pages/docs/ApiReferencePage.tsx` | 6 | Lazy-loaded Swagger/OpenAPI viewer (lightweight custom renderer) |
| `components/CommandPalette.tsx` | 7 | Ctrl+K searchable command palette (apps, templates, workflows, topics, nav) |
| `docs/getting-started.md` | 5 | Quick start guide: create app → user → template → send |
| `docs/channels.md` | 5 | Channel setup guide (webhook, email, APNS, FCM, SMS, SSE) |
| `docs/workflows.md` | 5 | Workflow creation, step types, triggers, execution model |
| `docs/templates.md` | 5 | Template CRUD, variables, versioning, diff, rollback |
| `docs/topics.md` | 5 | Topics & subscriptions, fan-out broadcast model |
| `docs/environments.md` | 5 | Multi-environment setup, API key scoping, promotion |
| `docs/sdk.md` | 5 | Go/JS/React SDK installation and usage |
| `docs/troubleshooting.md` | 5 | Common issues, debugging, FAQ |

### Modified Files (18)

| File | Task | Changes |
|------|------|---------|
| `App.tsx` | 1, 4, 6, 7, 9 | Wrap tree in ErrorBoundary, add `/docs/*` routes, add CommandPalette |
| `components/Sidebar.tsx` | 4, 9 | Add "Documentation" nav item under ADMIN section |
| `components/SetupWizard.tsx` | 3 | Extend from 4 to 6 steps (add Create User + Send Test improvements) |
| `pages/Dashboard.tsx` | 2 | Replace `<Spinner />` loading with skeleton cards |
| `components/AppTemplates.tsx` | 2 | Replace `"Loading..."` text with SkeletonTable |
| `components/dashboard/QuickTestPanel.tsx` | 2 | Replace `"Loading..."` placeholders with spinner icons |
| `package.json` | 4 | Add `react-markdown`, `remark-gfm` dependencies |
| `components/apps/AppTeam.tsx` | 8 | Add aria-labels to interactive elements |
| `components/apps/AppProviders.tsx` | 8 | Add aria-labels to interactive elements |
| `components/apps/AppEnvironments.tsx` | 8 | Add aria-labels to interactive elements |
| `components/ActivityFeed.tsx` | 8 | Add aria-labels to filter controls |
| `components/AnalyticsDashboard.tsx` | 8 | Add aria-labels to period selector |

---

## 2. Dependency Order

```
Task 1 (Error Boundary) ───────────────┐
Task 2 (Loading States) ───────────────┤
Task 3 (Onboarding Wizard) ────────────┤
Task 4 (Docs Hub Pages) ──────┐        │
Task 5 (Docs Content) ────────┼────────┼─→ Task 9 (Wire Everything) ─→ Task 10 (Build Verify)
Task 6 (API Reference) ───────┘        │
Task 7 (Command Palette) ──────────────┤
Task 8 (Accessibility) ────────────────┘
```

Tasks 1-3 are fully independent.  
Tasks 4-6 form a dependency chain (4 creates the layout, 5 writes content, 6 adds API reference page).  
Task 7 is independent.  
Task 8 is independent.  
Task 9 wires everything into App.tsx and Sidebar.  
Task 10 runs build checks.

---

## 3. Task 1: Error Boundary

### Current State

**No React error boundary exists anywhere in the UI.** If a component throws during render, the entire app white-screens. This is a critical production gap.

### New File: `components/ErrorBoundary.tsx`

```
Type: React class component (error boundaries require class components)

State:
  - hasError: boolean
  - error: Error | null

Methods:
  - static getDerivedStateFromError(error) → { hasError: true, error }
  - componentDidCatch(error, errorInfo) → console.error for logging
  - handleReload() → window.location.reload()
  - handleGoHome() → window.location.href = '/'

Render (when hasError):
  - Full-screen centered layout (min-h-screen, bg-background)
  - AlertTriangle icon (lucide, 48px, text-destructive)
  - Heading: "Something went wrong"
  - Subtitle: "An unexpected error occurred. This has been logged."
  - Error message in a code block (text-xs, mono, bg-muted, rounded, max-h-24 overflow-auto)
  - Two buttons:
    - "Go to Dashboard" → navigates to /apps
    - "Reload Page" → full page reload
  - Below: "If this keeps happening, contact support."

Render (when no error):
  - return this.props.children
```

### App.tsx Changes

Wrap the entire `<BrowserRouter>` content (or at least `<Routes>`) with `<ErrorBoundary>`:

```tsx
import ErrorBoundary from './components/ErrorBoundary';

// Inside App:
<BrowserRouter>
  <AuthProvider>
    <ErrorBoundary>
      <Toaster />
      <Routes>...</Routes>
    </ErrorBoundary>
  </AuthProvider>
</BrowserRouter>
```

Place the boundary **inside** `AuthProvider` (so auth context is available for navigation) but **outside** `Routes` (so it catches any page-level render errors).

---

## 4. Task 2: Loading States Upgrade

### Current Gaps

| File | Current Loading | Upgrade To |
|------|----------------|------------|
| `Dashboard.tsx` | `<Spinner />` full-center | Skeleton card grid (4 cards) + skeleton table |
| `AppTemplates.tsx` | `"Loading..."` raw text | `<SkeletonTable rows={5} columns={5} />` |
| `QuickTestPanel.tsx` | `"Loading..."` in Select placeholder | `<Loader2>` spinner icon in placeholder text |

### Dashboard.tsx Changes

Replace:
```tsx
{loading && Object.keys(stats).length === 0 ? (
    <div className="flex justify-center items-center py-12">
        <Spinner />
    </div>
) : (
```

With a skeleton layout that matches the Overview tab structure:
```tsx
{loading && Object.keys(stats).length === 0 ? (
    <div className="space-y-6">
        {/* Skeleton for OverviewStats */}
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            {Array.from({ length: 4 }).map((_, i) => (
                <Card key={i}><CardContent className="pt-5 pb-4">
                    <Skeleton className="h-16 w-full" />
                </CardContent></Card>
            ))}
        </div>
        {/* Skeleton for QueueDepthCards */}
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            {Array.from({ length: 3 }).map((_, i) => (
                <Card key={i}><CardContent className="pt-5 pb-4">
                    <Skeleton className="h-20 w-full" />
                </CardContent></Card>
            ))}
        </div>
    </div>
) : (
```

### AppTemplates.tsx Changes

Find the `"Loading..."` text and replace with `<SkeletonTable />`. Import SkeletonTable from the existing component.

### QuickTestPanel.tsx Changes

The `"Loading..."` strings in Select placeholders are acceptable UX for dropdown placeholders — they indicate the dropdown is populating. However, improve them slightly by changing to a more descriptive string:
- `loadingApps ? 'Loading apps...'` → keep as-is (acceptable for Select placeholder)
- The real fix: if `loadingApps` is true, show a subtle inline spinner next to the Select instead.

**Decision:** Leave QuickTestPanel placeholders as-is — they're inside Select triggers which don't render rich content well. Changing just `Dashboard.tsx` and `AppTemplates.tsx` covers the visible gaps.

---

## 5. Task 3: Onboarding Wizard Enhancement

### Current State

`SetupWizard.tsx` has 4 steps:

| Step | Key | Description |
|------|-----|-------------|
| 1 | `channel` | Pick delivery channel (email, webhook, push, sms, sse) |
| 2 | `template` | Create template with name/subject/body |
| 3 | `recipient` | Enter recipient email (auto-creates user via quick-send) |
| 4 | `send` | Preview + fill variables + send notification |

### Enhanced 6-Step Flow

| Step | Key | New? | Description |
|------|-----|------|-------------|
| 1 | `channel` | No | Pick delivery channel — same as current |
| 2 | `environment` | **Yes** | Create first environment (optional, skip button). Explain: "Environments let you separate dev/staging/prod. Each gets its own API key." Show a simple name + description form. If skipped, uses default environment. |
| 3 | `template` | No | Create template — same as current |
| 4 | `user` | **Changed** | Currently just asks for email and auto-creates via quick-send. Upgrade: show a proper user creation form with `email` + `external_id` (optional). Use `usersAPI.create(apiKey, payload)` directly. Explain: "Every notification needs a recipient. This user represents a person in your system." Show the returned `user_id` in a copy-able badge. |
| 5 | `send` | **Changed** | Auto-fills `user_id` from step 4 and `template_id` from step 3. No more quick-send — use `notificationsAPI.send()` with proper payload. Shows channel, template name, user email in summary. Result: success card with notification_id or error card. |
| 6 | `done` | **Yes** | Completion screen with: checkmark animation, "Your first notification was sent!" heading, links to: "View All Notifications", "Create Another Template", "Explore Workflows", "Read the Docs". |

### Key Changes

1. **Insert step 2 (environment)** between channel and template — keep it optional with a "Skip" button.
2. **Step 4 (user)** — replace the simple email input with `email` + `external_id` fields. Call `usersAPI.create()` instead of relying on quick-send's auto-creation. Store the returned `user_id` in wizard state.
3. **Step 5 (send)** — use `notificationsAPI.send()` with `user_id`, `template_id`, `channel`. Remove quick-send dependency.
4. **Add step 6 (done)** — celebration + next-steps links.
5. Update progress bar from 4 steps to 6.

### API Dependencies

| Step | API | Auth |
|------|-----|------|
| 2 (env) | `environmentsAPI.create(apiKey, payload)` | API Key |
| 3 (template) | `templatesAPI.create(apiKey, payload)` | API Key |
| 4 (user) | `usersAPI.create(apiKey, payload)` | API Key |
| 5 (send) | `notificationsAPI.send(apiKey, payload)` | API Key |

Check if `environmentsAPI` exists in api.ts. If not available, the environment step should gracefully handle the missing API and just show an informational card.

---

## 6. Task 4: Documentation Hub — Pages & Routing

### Architecture

The docs hub lives under `/docs/*` and uses its own sub-layout with a sidebar for section navigation.

### New File: `pages/docs/DocsLayout.tsx`

```
Component: React functional component

Structure:
  - Left sidebar (w-56, hidden on mobile, Sheet on mobile)
    - Logo/title: "Documentation"
    - Nav sections:
      - GUIDES
        - Getting Started → /docs/getting-started
        - Templates → /docs/templates
        - Workflows → /docs/workflows
        - Topics → /docs/topics
        - Channels → /docs/channels
        - Environments → /docs/environments
      - REFERENCE
        - API Reference → /docs/api
        - SDK Guide → /docs/sdk
      - HELP
        - Troubleshooting → /docs/troubleshooting
    - Each link: NavLink with active state styling (same pattern as Sidebar.tsx)
  - Right content area (flex-1, overflow-y-auto, max-w-3xl, mx-auto, px-6 py-8)
    - Renders <Outlet /> from react-router-dom

  Responsive:
    - Mobile: hamburger toggles Sheet with doc nav
    - Desktop: persistent sidebar

  Back link: "← Back to Dashboard" at top of sidebar
```

### New File: `pages/docs/DocsPage.tsx`

```
Route param: slug (from /docs/:slug)

Behavior:
  1. On mount/slug change, dynamically import the markdown content file
  2. Render using react-markdown with remark-gfm plugin
  3. Custom renderers:
     - h1 → text-2xl font-bold mb-4
     - h2 → text-xl font-semibold mt-8 mb-3 border-b pb-2
     - h3 → text-lg font-medium mt-6 mb-2
     - p → text-sm text-foreground/80 leading-relaxed mb-4
     - code (inline) → bg-muted px-1.5 py-0.5 rounded text-xs font-mono
     - pre/code (block) → bg-muted rounded-lg p-4 overflow-x-auto text-xs font-mono
     - a → text-accent hover:underline
     - table → border-collapse, bordered cells
     - ul/ol → list-disc/list-decimal pl-6 space-y-1 text-sm
     - blockquote → border-l-4 border-accent pl-4 italic text-muted-foreground

  Loading: Skeleton with 3 text blocks
  Error: "Document not found" with link back to /docs/getting-started

  Content loading strategy:
    - Store markdown files as .ts modules exporting string constants (avoids raw file loader config)
    - OR use Vite's ?raw import suffix: import content from './docs/getting-started.md?raw'
    - Preference: ?raw import — simpler, no extra files
```

### Package.json Changes

```bash
npm install react-markdown remark-gfm
```

Add to dependencies:
- `react-markdown` — Markdown renderer
- `remark-gfm` — GitHub Flavored Markdown support (tables, strikethrough, task lists)

### Sidebar.tsx Changes

Add "Documentation" nav item:

```tsx
{ label: 'Documentation', icon: <BookOpen className="h-4 w-4" />, to: '/docs', section: 'ADMIN' },
```

Insert between "Audit Logs" and the user section. Import `BookOpen` from lucide-react.

### App.tsx Changes

Add docs routes:

```tsx
const DocsLayout = lazy(() => import('./pages/docs/DocsLayout'));
const DocsPage = lazy(() => import('./pages/docs/DocsPage'));
const ApiReferencePage = lazy(() => import('./pages/docs/ApiReferencePage'));

// Inside protected routes:
<Route path="/docs" element={<DocsLayout />}>
  <Route index element={<Navigate to="/docs/getting-started" replace />} />
  <Route path=":slug" element={<DocsPage />} />
  <Route path="api" element={<ApiReferencePage />} />
</Route>
```

---

## 7. Task 5: Documentation Hub — Content Files

### Content Strategy

Markdown files stored at `ui/src/docs/` and imported using Vite's `?raw` suffix. Each file is a standalone guide focused on one domain.

### Files to Create

#### `docs/getting-started.md` (~200 lines)

```
# Getting Started with FreeRangeNotify

## Overview
Brief intro: what FreeRangeNotify is, hub-and-spoke architecture summary.

## Quick Start (5 minutes)

### Step 1: Create an Application
- Navigate to Applications → Create
- Name your app, get API key
- Code example: curl POST /v1/apps/

### Step 2: Register a User
- Users represent notification recipients
- Code example: curl POST /v1/users/ with API key header
- Note: user_id (internal UUID) vs external_id (your system's ID)

### Step 3: Create a Template
- Templates define notification content
- Support variables: {{.user_name}}, {{.order_id}}
- Code example: curl POST /v1/templates/

### Step 4: Send Your First Notification
- Code example: curl POST /v1/notifications/
- Payload: user_id, template_id, channel, data (variables)

### Step 5: Check Delivery
- View notifications in the dashboard
- Check status: pending → processing → sent/failed

## Key Concepts
- Applications (API key scoping)
- Users (recipients)
- Templates (content)
- Channels (email, push, webhook, SMS, SSE)
- Priorities (low, normal, high, critical)

## Next Steps
Links to: Templates, Workflows, Topics, SDK docs
```

#### `docs/templates.md` (~150 lines)

```
# Templates

## Overview
Templates define notification content with Go template variables.

## Creating Templates
- Fields: name, channel, subject, body, variables
- Variable syntax: {{.variable_name}}

## Versioning
- Each edit creates a new version
- Compare versions with diff view
- Rollback to any previous version

## Template Library
- Clone pre-built templates
- Customize for your use case

## Testing
- Render preview with sample data
- Test send to a specific user

## API Reference
- POST /v1/templates/ — Create
- GET /v1/templates/ — List
- PUT /v1/templates/:id — Update
- POST /v1/templates/:id/render — Preview
```

#### `docs/workflows.md` (~150 lines)

```
# Workflows

## Overview
Multi-step notification pipelines triggered by events.

## Concepts
- Trigger: event that starts the workflow
- Steps: channel, delay, digest, condition
- Execution: a running instance

## Creating Workflows
- Visual builder in the dashboard
- Step types explained
- Condition expressions

## Triggering
- POST /v1/workflows/:id/trigger
- Payload: user_id + data

## Monitoring
- Execution timeline view
- Step-by-step status
- Pause/resume/cancel
```

#### `docs/topics.md` (~100 lines)

```
# Topics & Subscriptions

## Overview
Fan-out notifications to groups of users via topic subscriptions.

## Creating Topics
- topic_key: unique identifier
- description: human-readable

## Subscriptions
- Subscribe users to topics
- Bulk subscribe/unsubscribe

## Broadcasting
- POST /v1/notifications/broadcast
- All subscribers receive the notification
```

#### `docs/channels.md` (~200 lines)

```
# Delivery Channels

## Webhook
- HTTP POST to a URL with HMAC-SHA256 signing
- Set webhook_url per notification or per user profile

## Email (SMTP)
- Direct SMTP or SendGrid integration
- Configure in app settings

## Apple Push Notification service (APNS)
- iOS push notifications
- Requires: certificate or auth key, bundle ID

## Firebase Cloud Messaging (FCM)
- Cross-platform push (Android, iOS, Web)
- Requires: service account JSON

## SMS (Twilio)
- Programmable messaging
- Requires: Account SID, Auth Token, From number

## Server-Sent Events (SSE)
- Real-time browser notifications
- Connect: GET /v1/sse?user_id={uuid}
- No refresh needed
```

#### `docs/environments.md` (~100 lines)

```
# Multi-Environment

## Overview
Separate dev/staging/prod with independent API keys.

## Creating Environments
- Name, description, settings
- Each gets a unique API key

## Promotion
- Promote templates from dev → staging → prod
- POST /v1/apps/:id/environments/:env_id/promote

## API Key Scoping
- Each API call uses the environment's key
- Notifications are scoped per environment
```

#### `docs/sdk.md` (~200 lines)

```
# SDK Reference

## Go SDK
- Installation: go get ...
- Client setup, send notification, trigger workflow

## JavaScript SDK
- Installation: npm install @freerangenotify/js
- Client setup, headless methods (10 methods)
- SSE real-time subscription

## React SDK
- Installation: npm install @freerangenotify/react
- <FreeRangeProvider> setup
- <NotificationBell> component
- <Preferences> component
- Hooks: useNotifications, useUnreadCount, usePreferences
```

#### `docs/troubleshooting.md` (~100 lines)

```
# Troubleshooting

## Common Issues

### Notification stuck in "pending"
- Check worker is running: docker-compose logs -f notification-worker
- Check queue depth: GET /v1/admin/queues/stats

### Webhook not receiving payloads
- Verify webhook_url is accessible from the server
- Check HMAC signature verification
- Use the Webhook Playground to test

### Template variables not rendering
- Ensure variable names match: {{.user_name}} in template, "user_name" in data
- Check template render preview

### SSE not connecting
- Verify user_id is the internal UUID (not external_id)
- Check JWT token is valid
- Browser must support EventSource

## Debugging
- docker-compose logs -f
- Check /v1/admin/providers/health
- Check /v1/admin/queues/stats
- Enable debug logging: FREERANGE_LOG_LEVEL=debug

## FAQ
- Q: Can I use external_id instead of user_id? A: Yes, with quick-send API
- Q: Maximum notification payload size? A: 64KB
- Q: Rate limiting? A: Configurable per app in settings
```

---

## 8. Task 6: API Reference Page (Swagger/ReDoc)

### Approach

Instead of adding heavyweight dependencies like `swagger-ui-react` (500KB+) or `redoc` (300KB+), build a **lightweight custom API reference** that reads the OpenAPI spec and renders a clean, searchable endpoint list.

### Why Not ReDoc/SwaggerUI?

Both add 300-500KB to the bundle. Since we already have a clean design system, a custom renderer is lighter, faster, and visually consistent.

### New File: `pages/docs/ApiReferencePage.tsx`

```
Behavior:
  1. On mount, fetch /v1/swagger.json (or import from docs/swagger.json directly)
  2. Parse the OpenAPI spec
  3. Group endpoints by tag (Auth, Applications, Users, Templates, Notifications, etc.)
  4. Render each group as a collapsible section

Render per endpoint:
  - Method badge: GET (green), POST (blue), PUT (amber), DELETE (red)
  - Path: /v1/apps/{id}/settings
  - Summary (from spec)
  - Collapsible details:
    - Parameters table (name, in, type, required, description)
    - Request body (JSON schema rendered as a formatted code block)
    - Response codes + response body schema
  - "Try it" link → points to relevant Dashboard page (e.g., /apps for app endpoints)

State:
  - spec: OpenAPISpec | null
  - expandedPaths: Set<string>
  - searchQuery: string
  - loading: boolean

Search:
  - Text input at top
  - Filters endpoints by path, summary, or tag (case-insensitive)

Styling:
  - Left: endpoint list (scrollable)
  - Method badges: colored rounded pills
  - Code blocks: bg-muted, monospace
  - Clean, minimal — matches existing design system
```

### No New Dependencies Required

The OpenAPI spec is just JSON. We parse it with standard JavaScript and render with React. No swagger-ui or redoc needed.

---

## 9. Task 7: Command Palette (Ctrl+K)

### New File: `components/CommandPalette.tsx`

```
Trigger: Ctrl+K (or Cmd+K on Mac)

Architecture:
  - Uses shadcn Dialog + Command (cmdk) components
  - The Command component should already exist (check ui/components/ui/command.tsx)
  - If not, use a simple Dialog + Input + filtered list

State:
  - open: boolean
  - query: string
  - results: CommandItem[]
  - loading: boolean

Data sources (fetched lazily on first open):
  - Applications: applicationsAPI.list() → items with { label: app_name, href: /apps/{id} }
  - Recent pages: from a simple localStorage array of last 5 visited routes
  - Navigation shortcuts: static list of all pages

Command categories:
  - NAVIGATION
    - Applications → /apps
    - Workflows → /workflows
    - Digest Rules → /digest-rules
    - Topics → /topics
    - Dashboard → /dashboard
    - Audit Logs → /audit
    - Documentation → /docs
  - APPLICATIONS (dynamic, loaded on first open)
    - {app.app_name} → /apps/{app.app_id}
  - ACTIONS
    - Create Application → /apps (with ?action=create, or just navigate)
    - Change Password → triggers the ChangePasswordDialog

Render:
  - Dialog overlay with Command component
  - Search input (auto-focused, placeholder: "Search or type a command...")
  - Grouped results with section headers
  - Each item: icon + label + optional badge + keyboard shortcut hint
  - Arrow key navigation, Enter to select
  - Escape or click outside to close

Keyboard:
  - Ctrl+K / Cmd+K → toggle open
  - Escape → close
  - Arrow Up/Down → navigate items
  - Enter → select item and navigate

Performance:
  - API data cached in state, refreshed on each open
  - Filter is instant (client-side on cached data)
  - Dialog is lazy-rendered (not in DOM when closed)
```

### App.tsx Changes

Add the global keyboard listener and CommandPalette component:

```tsx
import CommandPalette from './components/CommandPalette';

// Inside the authenticated layout:
<CommandPalette />
```

The CommandPalette manages its own keyboard listener internally via `useEffect`.

### Check: Command Component

Verify `ui/src/components/ui/command.tsx` exists. If not, use a Dialog + Input + filtered div approach instead (avoiding the `cmdk` dependency).

---

## 10. Task 8: Accessibility Pass

### Scope

Audit all Phase 2-4 components and add missing `aria-label`, `aria-describedby`, and keyboard navigation attributes. Focus on interactive elements that screen readers can't interpret.

### Files to Audit and Fix

| File | Fixes Needed |
|------|-------------|
| `AppTeam.tsx` | Add aria-labels to "Invite Member" button, role selector, remove member button |
| `AppProviders.tsx` | Add aria-labels to "Register Provider" button, provider cards |
| `AppEnvironments.tsx` | Add aria-labels to "Create Environment" button, promote button, env cards |
| `ActivityFeed.tsx` | Add aria-labels to filter Select components, auto-scroll checkbox, connection indicator |
| `AnalyticsDashboard.tsx` | Add aria-label to period selector, chart regions |

### Specific Fixes

```tsx
// Example: ActivityFeed filter selects
<Select value={filterChannel} onValueChange={setFilterChannel} aria-label="Filter by channel">

// Example: AppTeam invite button
<Button onClick={...} aria-label="Invite new team member">

// Example: AppEnvironments promote button  
<Button onClick={...} aria-label={`Promote environment ${env.name}`}>
```

### General Rules

1. Every `<Button>` with only an icon (no text) must have `aria-label`
2. Every `<Select>` must have `aria-label` if no visible `<Label>` is associated
3. All images/icons used decoratively should have `aria-hidden="true"`
4. All dialogs should have descriptive titles (already handled by shadcn Dialog)
5. Tab order should flow logically through forms

---

## 11. Task 9: Wire Everything + Final Routing

### App.tsx Final Changes

Consolidate all Phase 5 additions:

1. Import and wrap with `<ErrorBoundary>`
2. Add lazy imports for `DocsLayout`, `DocsPage`, `ApiReferencePage`
3. Add `/docs/*` routes inside the protected layout
4. Import and render `<CommandPalette />` inside the authenticated layout

### Sidebar.tsx Final Changes

Add the Documentation nav item to the ADMIN section:

```tsx
{ label: 'Documentation', icon: <BookOpen className="h-4 w-4" />, to: '/docs', section: 'ADMIN' },
```

Place after "Audit Logs" in the nav items array.

---

## 12. Task 10: Build Verification & Performance

### Build Checks

```powershell
cd ui
npx tsc --noEmit          # Zero errors
npx vite build            # Clean bundle
```

### Performance Checks

1. Verify all new pages are lazy-loaded (check `React.lazy` in App.tsx)
2. Check that `react-markdown` chunk is only loaded when visiting `/docs/*`
3. Verify the API reference page doesn't import the full swagger spec at build time (use dynamic import or fetch)
4. Check total bundle size hasn't grown more than ~30KB gzipped from `react-markdown` + `remark-gfm`

### Manual Smoke Test

| Check | Route | Expected |
|-------|-------|----------|
| Error boundary | Intentionally throw in a component | Friendly error page with reload button |
| Loading skeletons | `/dashboard` (slow network) | Skeleton cards instead of spinner |
| Onboarding wizard | `/apps/:id` → open wizard | 6 steps, env step skippable |
| Docs getting started | `/docs/getting-started` | Rendered markdown with TOC |
| Docs API reference | `/docs/api` | Searchable endpoint list |
| Command palette | `Ctrl+K` | Opens search overlay, navigates on Enter |
| Accessibility | Tab through any form | All elements reachable, labels announced |

---

## 13. Acceptance Criteria

### Task 1: Error Boundary
- [ ] `ErrorBoundary` catches render errors without white-screening
- [ ] Shows AlertTriangle icon, error message, and two action buttons
- [ ] "Reload Page" calls `window.location.reload()`
- [ ] "Go to Dashboard" navigates to `/apps`

### Task 2: Loading States
- [ ] `Dashboard.tsx` shows skeleton cards during initial load
- [ ] `AppTemplates.tsx` shows `SkeletonTable` instead of "Loading..." text
- [ ] No visible raw "Loading..." text on any page

### Task 3: Onboarding Wizard
- [ ] 6 steps with progress bar
- [ ] Step 2 (environment) is skippable
- [ ] Step 4 creates user via `usersAPI.create()`
- [ ] Step 5 sends via `notificationsAPI.send()` with auto-filled user_id + template_id
- [ ] Step 6 shows completion + next-steps links

### Task 4: Documentation Hub
- [ ] `/docs` redirects to `/docs/getting-started`
- [ ] Left sidebar with section navigation
- [ ] Markdown rendered with proper styling
- [ ] Mobile-responsive (sidebar collapses to sheet)
- [ ] "Documentation" link in main sidebar

### Task 5: Documentation Content
- [ ] 7 markdown files covering all major features
- [ ] Content is accurate to current API endpoints
- [ ] Code examples use correct curl syntax
- [ ] Cross-links between docs pages work

### Task 6: API Reference
- [ ] Parses OpenAPI spec and renders grouped endpoints
- [ ] Search filters endpoints by path/summary/tag
- [ ] Collapsible endpoint details (params, request body, responses)
- [ ] Method badges color-coded (GET=green, POST=blue, PUT=amber, DELETE=red)

### Task 7: Command Palette
- [ ] Opens on Ctrl+K / Cmd+K
- [ ] Closes on Escape or click outside
- [ ] Shows navigation shortcuts, dynamic application list
- [ ] Arrow key navigation works
- [ ] Enter selects and navigates to the chosen item
- [ ] Search filters results in real-time

### Task 8: Accessibility
- [ ] All icon-only buttons have `aria-label`
- [ ] All `Select` components have `aria-label` when no visible label
- [ ] Tab navigation works through all forms without getting stuck
- [ ] Screen reader can announce all interactive elements

### Task 9: Wiring
- [ ] ErrorBoundary wraps the app tree in App.tsx
- [ ] Docs routes registered and lazy-loaded
- [ ] CommandPalette rendered in authenticated layout
- [ ] Sidebar "Documentation" link navigates to /docs

### Task 10: Build
- [ ] `tsc --noEmit` — zero errors
- [ ] `vite build` — clean bundle
- [ ] New chunks for docs are lazy-loaded (not in main bundle)

---

## Appendix: Current State Reference

### Existing Components to Reuse

| Component | Location | Notes |
|-----------|----------|-------|
| `SkeletonTable` | `components/SkeletonTable.tsx` | Already used by 6+ pages |
| `Skeleton` | `components/ui/skeleton.tsx` | shadcn primitive |
| `EmptyState` | `components/EmptyState.tsx` | Centered icon + title + optional CTA |
| `Spinner` | `components/ui/spinner.tsx` | Currently used by Dashboard loading |
| `Dialog` | `components/ui/dialog.tsx` | For CommandPalette if no Command component |
| `Sheet` | `components/ui/sheet.tsx` | For mobile docs sidebar |
| `Command` | `components/ui/command.tsx` | **Check if exists** — needed for CommandPalette |

### SetupWizard Current Step Keys

```typescript
type WizardStep = 'channel' | 'template' | 'recipient' | 'send';
// Will become:
type WizardStep = 'channel' | 'environment' | 'template' | 'user' | 'send' | 'done';
```

### Toast Pattern (already consistent)

```typescript
toast.success('Resource created successfully');
toast.error(err?.response?.data?.error || 'Failed to create resource');
```

### Route Structure After Phase 5

```
/ → LandingPage
/login → Login
/register → Register
/forgot-password → ForgotPassword
/reset-password → ResetPassword
/auth/callback → SSOCallback

(Protected - DashboardLayout)
/apps → AppsList
/apps/:id → AppDetail
/dashboard → Dashboard
/workflows → WorkflowsList
/workflows/new → WorkflowBuilder
/workflows/:id → WorkflowBuilder
/workflows/executions → WorkflowExecutions
/digest-rules → DigestRulesList
/topics → TopicsList
/audit → AuditLogsList
/docs → DocsLayout
  /docs/getting-started → DocsPage
  /docs/templates → DocsPage
  /docs/workflows → DocsPage
  /docs/topics → DocsPage
  /docs/channels → DocsPage
  /docs/environments → DocsPage
  /docs/sdk → DocsPage
  /docs/troubleshooting → DocsPage
  /docs/api → ApiReferencePage
```
