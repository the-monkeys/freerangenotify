# Phase 4 — Observability & Admin: Implementation Plan

> **Parent:** [UI_API_INTEGRATION_PLAN.md](UI_API_INTEGRATION_PLAN.md)  
> **Prerequisite:** [PHASE3_ADVANCED_FEATURES.md](PHASE3_ADVANCED_FEATURES.md) (completed)  
> **Duration:** ~1 week  
> **Goal:** Upgrade the Dashboard with richer admin visibility (stat cards, channel charts, enhanced activity feed, quick test panel), improve queue management UI, and add a Change Password dialog wired through a user dropdown menu in the sidebar.

---

## Table of Contents

1. [Task Breakdown](#1-task-breakdown)
2. [Dependency Order](#2-dependency-order)
3. [Task 1: Dashboard Overview Stat Cards](#3-task-1-dashboard-overview-stat-cards)
4. [Task 2: Analytics Charts Enhancement](#4-task-2-analytics-charts-enhancement)
5. [Task 3: Activity Feed Filters & Auto-Scroll](#5-task-3-activity-feed-filters--auto-scroll)
6. [Task 4: Quick Test Panel (Tools Tab)](#6-task-4-quick-test-panel-tools-tab)
7. [Task 5: Queue Management Enhancements](#7-task-5-queue-management-enhancements)
8. [Task 6: Provider Health Enhancement](#8-task-6-provider-health-enhancement)
9. [Task 7: User Menu Dropdown](#9-task-7-user-menu-dropdown)
10. [Task 8: Change Password Dialog](#10-task-8-change-password-dialog)
11. [Task 9: Wire Everything Together](#11-task-9-wire-everything-together)
12. [Task 10: Build Verification](#12-task-10-build-verification)
13. [Acceptance Criteria](#13-acceptance-criteria)

---

## 1. Task Breakdown

| # | Task | New Files | Modified Files | Est. |
|---|------|-----------|----------------|------|
| 1 | Dashboard Overview Stat Cards | 1 | 1 | 3h |
| 2 | Analytics Charts Enhancement | 1 | 1 | 4h |
| 3 | Activity Feed Filters & Auto-Scroll | 0 | 1 | 2h |
| 4 | Quick Test Panel (Tools Tab) | 1 | 1 | 3h |
| 5 | Queue Management Enhancements | 1 | 1 | 3h |
| 6 | Provider Health Enhancement | 0 | 1 | 2h |
| 7 | User Menu Dropdown | 1 | 1 | 2h |
| 8 | Change Password Dialog | 1 | 0 | 2h |
| 9 | Wire Everything Together | 0 | 2 | 1h |
| 10 | Build Verification | 0 | 0 | 30m |
| **Total** | | **6** | **9** | **~23h** |

### New Files (6)

| File | Task | Description |
|------|------|-------------|
| `components/dashboard/OverviewStats.tsx` | 1 | System-wide stat cards (apps, users, templates, workflows, notifications today/week) |
| `components/dashboard/ChannelBreakdownChart.tsx` | 2 | Horizontal bar chart + mini pie for per-channel analytics with delivery rate & avg latency |
| `components/dashboard/QuickTestPanel.tsx` | 4 | App → User → Template picker that fires a test notification in one click |
| `components/dashboard/QueueDepthCards.tsx` | 5 | Per-priority queue cards with DLQ checkbox-select replay |
| `components/UserMenu.tsx` | 7 | Dropdown menu component for user section with Change Password & Logout |
| `components/ChangePasswordDialog.tsx` | 8 | Dialog form: current password, new password, confirm |

### Modified Files (9)

| File | Task | Changes |
|------|------|---------|
| `pages/Dashboard.tsx` | 1, 9 | Replace inline queue stats with `OverviewStats` component, wire Quick Test in Tools tab |
| `components/AnalyticsDashboard.tsx` | 2 | Import and render `ChannelBreakdownChart`, add delivery rate & avg latency stat cards |
| `components/ActivityFeed.tsx` | 3 | Add event-type filter dropdown, auto-scroll toggle, filter state |
| `components/WebhookPlayground.tsx` | 4 | Share Tools tab with QuickTestPanel (or Dashboard wires them side by side) |
| `services/api.ts` | 5, 1 | Add `adminAPI.getSystemStats()`, extend types if needed |
| `types/index.ts` | 1, 5, 6 | Add `SystemStats`, extend `ProviderHealth` with latency/last_error |
| `components/Sidebar.tsx` | 7 | Replace logout icon with `UserMenu` dropdown component |
| `pages/Dashboard.tsx` | 5 | Wire `QueueDepthCards` into Overview tab replacing inline cards |
| `pages/Dashboard.tsx` | 6 | Enhance inline provider health table with latency + error columns |

---

## 2. Dependency Order

```
Task 7 (UserMenu) ─────────────────────────→ Task 8 (Change Password Dialog)
                                                         │
Task 1 (Overview Stats) ───────┐                         │
Task 2 (Analytics Charts) ─────┤                         │
Task 3 (Activity Filters) ─────┼─→ Task 9 (Wire Everything) ─→ Task 10 (Build Verify)
Task 4 (Quick Test Panel) ─────┤
Task 5 (Queue Enhancements) ───┤
Task 6 (Provider Health) ──────┘
```

Tasks 1-6 are independent of each other. Task 8 depends on Task 7 (UserMenu must exist before dialog can be opened from it). Task 9 wires all pieces into Dashboard.tsx and Sidebar.tsx.

---

## 3. Task 1: Dashboard Overview Stat Cards

### Current State

The Overview tab in `Dashboard.tsx` currently shows:
- **Queue stats cards** — `Object.entries(stats).map()` rendering raw Redis queue depths (e.g. `frn:queue:high`)
- **Provider Health table** — name, channel, healthy badge, breaker state badge
- **DLQ table** — notification ID, priority, reason, failed at, retries, bulk replay

There are **no system-wide aggregation cards** (total apps, total users, total templates, total workflows, notifications sent today/this week).

### Required API

The backend exposes `GET /v1/admin/analytics/summary?period=1d` which returns totals, but not app/user/template/workflow counts. We need to compose from existing APIs or add a new admin endpoint.

**Approach:** Add `adminAPI.getSystemStats()` that calls a new or composite endpoint. If the backend doesn't have a dedicated `/admin/stats` endpoint, we assemble from multiple calls (`applicationsAPI.list`, `adminAPI.getAnalyticsSummary`).

### New File: `components/dashboard/OverviewStats.tsx`

```
Props: none (fetches its own data)

State:
  - stats: { total_apps, total_users, total_templates, total_workflows,
             notifications_today, notifications_this_week, success_rate }
  - loading: boolean

Data fetching:
  - On mount, call adminAPI.getAnalyticsSummary('1d') for today's stats
  - Call adminAPI.getAnalyticsSummary('7d') for this week
  - Optionally call applicationsAPI.list(1, 0) just to read `total` count
  - Cache in state, refresh every 30s

Render:
  - 2-row grid of stat cards
  - Row 1: Total Apps | Total Users | Total Templates | Active Workflows
  - Row 2: Sent Today | Sent This Week | Overall Success Rate
  - Each card: icon (top-left), value (large text), label (muted), optional +/- trend badge
  - Cards use bg-card, border-border, rounded-lg
  - Icon mapping:
    - Apps → LayoutGrid
    - Users → Users
    - Templates → FileText
    - Workflows → Workflow
    - Sent Today → Send
    - Sent Week → TrendingUp
    - Success Rate → CheckCircle (green if >95%, amber if >80%, red otherwise)
```

### Types Addition (`types/index.ts`)

```typescript
export interface SystemStats {
    total_apps: number;
    total_users: number;
    total_templates: number;
    total_workflows: number;
    notifications_today: number;
    notifications_this_week: number;
    success_rate: number;
}
```

### API Addition (`services/api.ts`)

```typescript
// Inside adminAPI:
getSystemStats: async (): Promise<SystemStats> => {
    // Compose from available endpoints
    const [summary1d, summary7d] = await Promise.all([
        adminAPI.getAnalyticsSummary('1d'),
        adminAPI.getAnalyticsSummary('7d'),
    ]);
    return {
        total_apps: 0,        // will be patched by caller from applicationsAPI
        total_users: 0,       // placeholder — no aggregate endpoint
        total_templates: 0,   // placeholder
        total_workflows: 0,   // placeholder
        notifications_today: summary1d.total_sent,
        notifications_this_week: summary7d.total_sent,
        success_rate: summary7d.success_rate,
    };
},
```

If the backend `/admin/analytics/summary` already includes `total_apps`, `total_users`, etc. in its response, use those directly and skip the composite approach.

### Dashboard.tsx Changes

Replace the inline "Message Queues" header + `Object.entries(stats).map()` card grid at the top of the Overview tab with:

```tsx
<OverviewStats />
```

Move the raw queue depth cards down below the overview stats, or replace them with the enhanced `QueueDepthCards` component (Task 5).

---

## 4. Task 2: Analytics Charts Enhancement

### Current State

`AnalyticsDashboard.tsx` renders:
- 4 stat cards: Total Sent, Delivered, Failed, Success Rate
- CSS-based bar chart for `daily_breakdown` (custom divs, no charting library)
- Channel Breakdown table (per-channel rows: sent/delivered/failed/total/success rate)
- 3 secondary stat cards: Pending, Read, Total All Time
- Period selector: 1d, 7d, 30d, 90d

**Missing:** Pie/bar visualization for channel breakdown, delivery rate %, average latency metric.

### New File: `components/dashboard/ChannelBreakdownChart.tsx`

```
Props:
  - channels: ChannelAnalytics[]
  - totalSent: number

Render:
  - Left side: Horizontal stacked bar chart (CSS-based, no library needed)
    - Each channel gets a color-coded bar segment proportional to its `total` / totalSent
    - Color mapping: email=#3B82F6, push=#8B5CF6, sms=#10B981, webhook=#F59E0B, sse=#EC4899, in_app=#6366F1
    - Below bars: legend row with colored dots + channel name + count
  - Right side: Mini donut / ring indicator (CSS conic-gradient)
    - Shows delivery rate (delivered/sent * 100) as a percentage arc
    - Center text: delivery rate %
    - Color: green >95%, amber >80%, red otherwise

  Responsive:
    - On mobile (<768px): stack vertically
    - On desktop: side-by-side in a 2-column grid
```

### AnalyticsDashboard.tsx Changes

1. Import `ChannelBreakdownChart`
2. Replace the plain Channel Breakdown `<Table>` with a card containing:
   - `<ChannelBreakdownChart>` visual at top
   - Existing table below (kept for detail, but made collapsible)
3. Add two new stat cards after the existing 4:
   - **Delivery Rate** — `summary.total_delivered / summary.total_sent * 100` (formatted to 1 decimal)
   - **Avg Latency** — if available from backend (`summary.avg_latency_ms`), else show "—"
4. Stat card row becomes a 6-card grid (3 per row on desktop)

### Types Addition

If the backend returns latency data in the analytics summary, add:

```typescript
// Extend AnalyticsSummary
avg_latency_ms?: number;
```

---

## 5. Task 3: Activity Feed Filters & Auto-Scroll

### Current State

`ActivityFeed.tsx`:
- Uses `EventSource` with JWT token query param
- Buffers last 100 events in state
- Renders events as a list: notification_id (truncated), channel badge, status badge, timestamp
- No filtering, no auto-scroll control

### Changes to `ActivityFeed.tsx`

**New state variables:**
```typescript
const [filterChannel, setFilterChannel] = useState<string>('');
const [filterStatus, setFilterStatus] = useState<string>('');
const [autoScroll, setAutoScroll] = useState(true);
const feedEndRef = useRef<HTMLDivElement>(null);
```

**Filter bar:** Add above the event list:
```
Row:
  - Channel filter: Select (All | email | push | sms | webhook | sse | in_app)
  - Status filter: Select (All | sent | delivered | failed | pending | queued)
  - Auto-scroll toggle: Switch with label "Auto-scroll"
  - Clear filters button (ghost, X icon) — shown when any filter active
```

**Filtering logic:** Applied client-side on the buffered events array:
```typescript
const filteredEvents = events.filter(e =>
    (!filterChannel || e.channel === filterChannel) &&
    (!filterStatus || e.status === filterStatus)
);
```

**Auto-scroll:** When `autoScroll` is true and a new event arrives, scroll `feedEndRef` into view:
```typescript
useEffect(() => {
    if (autoScroll && feedEndRef.current) {
        feedEndRef.current.scrollIntoView({ behavior: 'smooth' });
    }
}, [events.length, autoScroll]);
```

**Render:** At the bottom of the event list, add `<div ref={feedEndRef} />`.

---

## 6. Task 4: Quick Test Panel (Tools Tab)

### Current State

The Tools tab currently only contains `<WebhookPlayground />`.

### New File: `components/dashboard/QuickTestPanel.tsx`

```
Props: none

State:
  - apps: Application[]
  - selectedAppId: string
  - selectedApiKey: string  (derived from selected app)
  - users: User[]
  - selectedUserId: string
  - templates: Template[]
  - selectedTemplateId: string
  - variablesJson: string
  - sending: boolean
  - result: { success: boolean; message: string } | null

Data flow (cascading pickers):
  1. On mount → fetch applicationsAPI.list() to populate app dropdown
  2. On app select → store api_key, fetch usersAPI.list(apiKey) + templatesAPI.list(apiKey)
  3. On template select → pre-populate variablesJson from template.variables
  4. On Send → call notificationsAPI.send(apiKey, { user_id, template_id, data: parsedVars })
  5. Show toast + result card (success green, failure red)

Render:
  - Card with title "Quick Test" and subtitle "Send a test notification end-to-end"
  - Step 1: App selector (Select dropdown, shows app_name)
  - Step 2: User selector (Select dropdown, shows email, disabled until app selected)
  - Step 3: Template selector (Select dropdown, shows name + channel badge, disabled until app selected)
  - Step 4: Variables editor (Textarea, pre-populated, disabled until template selected)
  - Send button (full-width, disabled until user + template selected)
  - Result card below: green border + "Sent successfully" OR red border + error message
  - Link below: "View API Documentation →" pointing to /docs or external URL
```

### API Dependencies

Uses existing APIs — no new endpoints required:
- `applicationsAPI.list()` — JWT auth
- `usersAPI.list(apiKey)` — API key auth
- `templatesAPI.list(apiKey)` — API key auth
- `notificationsAPI.send(apiKey, payload)` — API key auth

### Dashboard.tsx Changes

In the `{activeTab === 'tools'}` block, render both side by side:
```tsx
{activeTab === 'tools' && (
    <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <QuickTestPanel />
        <WebhookPlayground />
    </div>
)}
```

---

## 7. Task 5: Queue Management Enhancements

### Current State

Queue stats are rendered inline in `Dashboard.tsx` Overview tab:
- Simple cards showing queue name → message count
- DLQ table with bulk "Replay All" button

### New File: `components/dashboard/QueueDepthCards.tsx`

```
Props:
  - stats: Record<string, number>
  - dlqItems: DLQItem[]
  - onReplay: (ids?: string[]) => void
  - replaying: boolean

Render:
  Priority Queue Section:
    - 3 cards for high / normal / low queues
    - Each card: priority label (uppercase), count (large number), color accent
      - high: red-tinted border
      - normal: neutral border
      - low: blue-tinted border
    - Mini sparkline placeholder (a thin CSS bar showing relative depth vs max)
    - Total combined count badge

  DLQ Section:
    - Table with checkboxes per row
    - New state: selectedDlqIds: Set<string>
    - "Select All" checkbox in header
    - "Replay Selected" button (disabled when none selected)
    - "Replay All" button (existing behavior)
    - Each row: checkbox, notification_id, priority badge, reason, timestamp, retry_count
    - When "Replay Selected" clicked → call onReplay(Array.from(selectedDlqIds))
```

### API Consideration

The current `adminAPI.replayDLQ(limit)` replays by limit count, not by specific IDs. If the backend supports replaying specific items, add:
```typescript
replayDLQByIds: async (ids: string[]) => {
    const { data } = await api.post<{ replayed_count: number }>('/admin/queues/dlq/replay', { ids });
    return data;
},
```

If the backend doesn't support selective replay, fall back to `replayDLQ(selectedCount)` and note the limitation. The checkbox UI still improves UX by letting users count how many they want to replay.

### Dashboard.tsx Changes

Replace the inline queue stats cards + DLQ section in Overview tab with:
```tsx
<QueueDepthCards
    stats={stats}
    dlqItems={dlqItems}
    onReplay={handleReplayDLQ}
    replaying={replaying}
/>
```

---

## 8. Task 6: Provider Health Enhancement

### Current State

Provider health is rendered inline in `Dashboard.tsx` Overview tab as a table with 4 columns: Provider, Channel, Status, Circuit Breaker.

### Changes to Dashboard.tsx (inline — no new file)

Enhance the existing provider health table with 2 additional columns:

| Column | Source | Rendering |
|--------|--------|-----------|
| **Latency** | `ProviderHealth.latency_ms` (if available) | `{latency}ms` with color: green <100ms, amber <500ms, red ≥500ms |
| **Last Error** | `ProviderHealth.last_error` (if available) | Truncated to 60 chars, tooltip with full text, red text |

### Types Extension (`types/index.ts`)

```typescript
// Extend existing ProviderHealth
export interface ProviderHealth {
    name: string;
    channel: string;
    healthy: boolean;
    breaker_state: string;
    latency_ms?: number;     // NEW — average latency in ms
    last_error?: string;     // NEW — most recent error message
    last_error_at?: string;  // NEW — timestamp of last error
}
```

If the backend doesn't return these fields yet, the columns should gracefully show "—". The UI extension is forward-compatible.

---

## 9. Task 7: User Menu Dropdown

### Current State

`Sidebar.tsx` has a bottom user section (lines 83–98):
- Shows `user.full_name` or `user.email`
- Shows email as secondary text
- Single `<LogOut>` icon button

**No dropdown menu exists.** Phase 4 requires one to hold "Change Password" and potentially other account actions.

### New File: `components/UserMenu.tsx`

```
Props:
  - user: { full_name?: string; email?: string }
  - onChangePassword: () => void
  - onLogout: () => void

Render:
  Uses shadcn DropdownMenu:
  - Trigger: the entire user info area (name + email) + ChevronUp icon
  - Content (aligned bottom-start for sidebar placement):
    - Header: user.full_name + user.email (non-interactive)
    - Separator
    - Item: "Change Password" (Lock icon) → calls onChangePassword
    - Item: "Logout" (LogOut icon) → calls onLogout
    - Separator
    - Footer: "v{VERSION}" in muted text (optional)

  Styling:
    - w-56, bg-popover, border-border
    - Items: text-sm, hover:bg-accent
    - Destructive styling on Logout (text-destructive)
```

### Import Check

Verify that `DropdownMenu` components exist in the project:
- `components/ui/dropdown-menu.tsx` — should already exist from shadcn/ui setup
- If not, generate with: `npx shadcn@latest add dropdown-menu`

---

## 10. Task 8: Change Password Dialog

### Current State

`authExtendedAPI.changePassword({ old_password, new_password })` exists in `api.ts` (line 916) but is **not wired into any UI**. The backend route `POST /v1/admin/change-password` exists in `routes.go` (line 201), JWT-protected.

### New File: `components/ChangePasswordDialog.tsx`

```
Props:
  - open: boolean
  - onOpenChange: (open: boolean) => void

State:
  - currentPassword: string
  - newPassword: string
  - confirmPassword: string
  - loading: boolean
  - errors: { current?: string; new?: string; confirm?: string }

Validation (client-side, on submit):
  - currentPassword: required
  - newPassword: required, min 8 chars
  - confirmPassword: must match newPassword
  - If validation fails, show per-field error messages in red text

On Submit:
  1. Validate fields
  2. Call authExtendedAPI.changePassword({ old_password: currentPassword, new_password: newPassword })
  3. On success: toast.success('Password changed'), close dialog, reset form
  4. On error: toast.error(err.response.data.error || 'Failed to change password')

Render:
  Dialog (shadcn):
    - Title: "Change Password"
    - Description: "Enter your current password and choose a new one."
    - Form (3 fields):
      - Current Password: Input type="password", eye toggle
      - New Password: Input type="password", eye toggle, 8-char minimum hint
      - Confirm Password: Input type="password"
      - Per-field error text in text-destructive text-xs
    - Footer: Cancel + Save buttons
    - Save disabled when loading, shows Loader2 spinner
```

---

## 11. Task 9: Wire Everything Together

### Sidebar.tsx Changes

Replace the bottom user section with `UserMenu`:

```tsx
// Before:
<div className="border-t border-sidebar-border px-4 py-3">
    <div className="flex items-center justify-between">
        <div>...</div>
        <button onClick={handleLogout}><LogOut /></button>
    </div>
</div>

// After:
<div className="border-t border-sidebar-border px-4 py-3">
    <UserMenu
        user={{ full_name: user?.full_name, email: user?.email }}
        onChangePassword={() => setChangePasswordOpen(true)}
        onLogout={handleLogout}
    />
    <ChangePasswordDialog
        open={changePasswordOpen}
        onOpenChange={setChangePasswordOpen}
    />
</div>
```

Add state `changePasswordOpen` to `SidebarNav` component. Import `UserMenu` and `ChangePasswordDialog`.

### Dashboard.tsx Changes Summary

Consolidate all Dashboard modifications from Tasks 1, 4, 5, 6:

```tsx
// Overview tab — new structure:
{activeTab === 'overview' && (
    <>
        <OverviewStats />

        <QueueDepthCards
            stats={stats}
            dlqItems={dlqItems}
            onReplay={handleReplayDLQ}
            replaying={replaying}
        />

        {/* Enhanced Provider Health (inline, extra columns) */}
        <Card className="mt-6">
            <CardHeader><CardTitle>Provider Health</CardTitle></CardHeader>
            <CardContent>
                <Table>
                    {/* 6 columns: Provider, Channel, Status, Breaker, Latency, Last Error */}
                </Table>
            </CardContent>
        </Card>
    </>
)}

// Tools tab — side by side:
{activeTab === 'tools' && (
    <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        <QuickTestPanel />
        <WebhookPlayground />
    </div>
)}
```

---

## 12. Task 10: Build Verification

Run both checks from `ui/`:

```powershell
npx tsc --noEmit          # Zero errors expected
npx vite build            # Clean bundle expected
```

Fix any type errors, unused imports, or missing dependencies before marking complete.

---

## 13. Acceptance Criteria

### Task 1: Overview Stat Cards
- [ ] `OverviewStats` renders 6-7 stat cards in a grid
- [ ] Cards show real numbers from API (or "—" if endpoint unavailable)
- [ ] Auto-refreshes every 30 seconds
- [ ] Responsive: 2 columns on mobile, 4 on desktop

### Task 2: Analytics Charts Enhancement
- [ ] `ChannelBreakdownChart` renders horizontal bar + delivery rate donut
- [ ] Color-coded per channel, legend shown
- [ ] Delivery rate + avg latency stat cards added to AnalyticsDashboard
- [ ] Existing channel table kept but made collapsible

### Task 3: Activity Feed Filters
- [ ] Channel filter dropdown filters events client-side
- [ ] Status filter dropdown filters events client-side
- [ ] Auto-scroll toggle defaults to ON, pauses scrolling when OFF
- [ ] Clear filters button visible when filters active

### Task 4: Quick Test Panel
- [ ] App → User → Template cascade works (each dropdown populates on parent change)
- [ ] Variables textarea pre-populates from template.variables
- [ ] Send button calls notificationsAPI.send() and shows toast result
- [ ] Error state shown clearly with red border card
- [ ] API docs link present

### Task 5: Queue Enhancements
- [ ] Priority queue cards with color-coded borders
- [ ] DLQ table has per-row checkboxes
- [ ] "Replay Selected" button replays checked items
- [ ] "Select All" checkbox in header
- [ ] Graceful fallback if backend doesn't support selective replay

### Task 6: Provider Health Enhancement
- [ ] Latency column shows colored ms value (green/amber/red thresholds)
- [ ] Last Error column shows truncated error with tooltip
- [ ] Columns gracefully show "—" if fields not present

### Task 7: User Menu
- [ ] Dropdown trigger replaces raw logout icon in sidebar
- [ ] Shows user name + email in header
- [ ] "Change Password" item present
- [ ] "Logout" item with destructive styling

### Task 8: Change Password
- [ ] Dialog opens from User Menu
- [ ] 3-field form with client-side validation (min 8 chars, match check)
- [ ] Per-field error messages shown
- [ ] Calls `authExtendedAPI.changePassword()` on submit
- [ ] Success toast + dialog close on success
- [ ] Error toast on failure

### Task 9: Wiring
- [ ] All new components imported and rendered in correct locations
- [ ] No orphaned imports or unused state

### Task 10: Build
- [ ] `tsc --noEmit` — zero errors
- [ ] `vite build` — clean bundle, all chunks generated

---

## Appendix: Current State Reference

### Existing API Methods Available

| Method | Auth | Returns |
|--------|------|---------|
| `adminAPI.getQueueStats()` | Public | `Record<string, number>` |
| `adminAPI.listDLQ()` | Public | `DLQItem[]` |
| `adminAPI.replayDLQ(limit)` | Public | `{ replayed_count }` |
| `adminAPI.getProviderHealth()` | Public | `Record<string, ProviderHealth>` |
| `adminAPI.createPlayground()` | JWT | `{ id, url, expires_in }` |
| `adminAPI.getPlaygroundPayloads(id)` | Public | `{ id, payloads[], count }` |
| `adminAPI.getAnalyticsSummary(period)` | JWT | `AnalyticsSummary` |
| `authExtendedAPI.changePassword(payload)` | JWT | void |
| `applicationsAPI.list()` | JWT | `{ applications[], total }` |
| `usersAPI.list(apiKey, page, size)` | API Key | `{ users[] }` |
| `templatesAPI.list(apiKey, limit, offset)` | API Key | `{ templates[], total }` |
| `notificationsAPI.send(apiKey, payload)` | API Key | `Notification` |

### Backend Routes (Admin)

```
GET  /v1/admin/queues/stats          — public
GET  /v1/admin/queues/dlq            — public
POST /v1/admin/queues/dlq/replay     — public
GET  /v1/admin/providers/health      — public
POST /v1/admin/playground/webhook    — JWT
GET  /v1/admin/analytics/summary     — JWT
GET  /v1/admin/activity-feed         — JWT (SSE)
POST /v1/admin/change-password       — JWT
```

### Existing Types

```typescript
interface ProviderHealth { name, channel, healthy, breaker_state }
interface DLQItem { notification_id, priority, reason, timestamp, retry_count }
interface ChannelAnalytics { channel, sent, delivered, failed, total, success_rate }
interface DailyStat { date, count }
interface AnalyticsSummary { period, total_sent, total_delivered, total_failed,
    total_pending, total_read, total_all, success_rate, by_channel, daily_breakdown }
```

### Dashboard.tsx Current Structure

```
Overview tab:
  → Queue stats cards (inline, from stats map)
  → Provider Health table (inline, 4 columns)
  → DLQ table (inline, bulk replay)
Analytics tab:
  → <AnalyticsDashboard />
Activity tab:
  → <ActivityFeed />
Tools tab:
  → <WebhookPlayground />
```
