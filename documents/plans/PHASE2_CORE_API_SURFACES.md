# Phase 2 — Core API Surfaces: Implementation Plan

> **Parent:** [UI_API_INTEGRATION_PLAN.md](UI_API_INTEGRATION_PLAN.md)  
> **Prerequisite:** [PHASE1_FOUNDATION_REWIRE.md](PHASE1_FOUNDATION_REWIRE.md) (completed)  
> **Duration:** ~2 weeks  
> **Goal:** Build the 5 major feature areas from scratch — Workflows (list + builder + executions), Digest Rules, Topics (with subscriber management), Notification inbox enhancements — and extend the sidebar, AppDetail tabs, and routing to surface them.

---

## Table of Contents

1. [Task Breakdown](#1-task-breakdown)
2. [Dependency Order](#2-dependency-order)
3. [Task 1: Sidebar Navigation Extension](#3-task-1-sidebar-navigation-extension)
4. [Task 2: Route Registration & Lazy Loading](#4-task-2-route-registration--lazy-loading)
5. [Task 3: Workflows List Page](#5-task-3-workflows-list-page)
6. [Task 4: Workflow Step Components](#6-task-4-workflow-step-components)
7. [Task 5: Workflow Builder Page](#7-task-5-workflow-builder-page)
8. [Task 6: Workflow Executions & Timeline](#8-task-6-workflow-executions--timeline)
9. [Task 7: Digest Rules (List + Form)](#9-task-7-digest-rules-list--form)
10. [Task 8: Topics (List + Form + Subscribers)](#10-task-8-topics-list--form--subscribers)
11. [Task 9: Notification Inbox Enhancements](#11-task-9-notification-inbox-enhancements)
12. [Task 10: AppDetail Tab Extension](#12-task-10-appdetail-tab-extension)
13. [Task 11: Build Verification](#13-task-11-build-verification)
14. [Acceptance Criteria](#14-acceptance-criteria)

---

## 1. Task Breakdown

| # | Task | New Files | Modified Files | Est. |
|---|------|-----------|----------------|------|
| 1 | Sidebar Navigation Extension | 0 | 1 | 30m |
| 2 | Route Registration & Lazy Loading | 0 | 1 | 30m |
| 3 | Workflows List Page | 1 | 0 | 3h |
| 4 | Workflow Step Components | 2 | 0 | 4h |
| 5 | Workflow Builder Page | 1 | 0 | 6h |
| 6 | Workflow Executions & Timeline | 2 | 0 | 3h |
| 7 | Digest Rules (List + Form) | 1 | 0 | 3h |
| 8 | Topics (List + Form + Subscribers) | 2 | 0 | 4h |
| 9 | Notification Inbox Enhancements | 2 | 1 | 5h |
| 10 | AppDetail Tab Extension | 0 | 1 | 2h |
| 11 | Build Verification | 0 | 0 | 30m |
| **Total** | | **11** | **4** | **~31h** |

### New Files (11)

| File | Task | Description |
|------|------|-------------|
| `pages/workflows/WorkflowsList.tsx` | 3 | Workflow listing with filters, create/edit/delete |
| `components/workflows/WorkflowStepCard.tsx` | 4 | Read-only step card in the vertical step list |
| `components/workflows/WorkflowStepEditor.tsx` | 4 | Step editing form (channel/delay/digest/condition configs) |
| `pages/workflows/WorkflowBuilder.tsx` | 5 | Full workflow create/edit page with vertical step editor |
| `pages/workflows/WorkflowExecutions.tsx` | 6 | Execution listing with filters |
| `components/workflows/ExecutionTimeline.tsx` | 6 | Vertical timeline for step-by-step execution progress |
| `pages/digest/DigestRulesList.tsx` | 7 | Digest rule list + slide-panel create/edit form |
| `pages/topics/TopicsList.tsx` | 8 | Topic list + slide-panel create/edit form |
| `components/topics/TopicSubscribers.tsx` | 8 | Subscriber management panel (add/remove users) |
| `components/notifications/BulkActionsBar.tsx` | 9 | Sticky bar for multi-select notification actions |
| `components/notifications/NotificationDetail.tsx` | 9 | Slide-panel notification detail view |

### Modified Files (4)

| File | Task | Changes |
|------|------|---------|
| `components/Sidebar.tsx` | 1 | Add Workflows, Digest Rules, Topics nav items |
| `App.tsx` | 2 | Add 5 new lazy-loaded routes |
| `components/AppNotifications.tsx` | 9 | Add checkbox selection, unread count, mark read/all, snooze, archive, bulk bar |
| `pages/AppDetail.tsx` | 10 | Extend tabs from 6 to 8 (add Digest Rules, Topics) |

---

## 2. Dependency Order

```
Task 1 (Sidebar) ──────────────────────────────────┐
Task 2 (Routes) ────────────────────────────────────┤
                                                    ├─→ Task 11 (Build Verify)
Task 4 (Step Components) ──→ Task 5 (Builder) ─────┤
Task 3 (Workflows List) ───→ Task 5 (Builder) ─────┤
Task 6 (Executions + Timeline) ────────────────────┤
Task 7 (Digest Rules) ────────────────────────────┤
Task 8 (Topics) ───────────────────────────────────┤
Task 9 (Notification Enhancements) ────────────────┤
Task 10 (AppDetail Tabs) ──────────────────────────┘
```

**Critical path:** Task 4 → Task 5 (step components must exist before the builder page).  
**Parallelizable:** Tasks 1+2, Tasks 3+4, Tasks 6+7+8+9, Tasks 10 alone.

---

## 3. Task 1: Sidebar Navigation Extension

**File:** `ui/src/components/Sidebar.tsx`  
**Action:** Modify `navItems` array to add 3 new navigation items.

### Current State

```tsx
const navItems: NavItem[] = [
    { label: 'Applications', icon: <LayoutGrid className="h-4 w-4" />, to: '/apps', section: 'MAIN' },
    { label: 'Dashboard', icon: <BarChart3 className="h-4 w-4" />, to: '/dashboard', section: 'ADMIN' },
];
```

### Target State

```tsx
import { Bell, LayoutGrid, BarChart3, LogOut, Workflow, Timer, Tag } from 'lucide-react';

const navItems: NavItem[] = [
    { label: 'Applications', icon: <LayoutGrid className="h-4 w-4" />, to: '/apps', section: 'MAIN' },
    { label: 'Workflows', icon: <Workflow className="h-4 w-4" />, to: '/workflows', section: 'MAIN' },
    { label: 'Digest Rules', icon: <Timer className="h-4 w-4" />, to: '/digest-rules', section: 'MAIN' },
    { label: 'Topics', icon: <Tag className="h-4 w-4" />, to: '/topics', section: 'MAIN' },
    { label: 'Dashboard', icon: <BarChart3 className="h-4 w-4" />, to: '/dashboard', section: 'ADMIN' },
];
```

**Icon choices:**
- `Workflow` (lucide) — flowchart icon for workflows
- `Timer` (lucide) — time-based grouping icon for digest rules
- `Tag` (lucide) — labeling icon for topics

### Changes

1. Add `Workflow`, `Timer`, `Tag` to the lucide-react import.
2. Insert 3 new entries into `navItems` between Applications and Dashboard.

---

## 4. Task 2: Route Registration & Lazy Loading

**File:** `ui/src/App.tsx`  
**Action:** Add 5 new lazy-loaded page imports and their routes under the protected `DashboardLayout`.

### New Imports

```tsx
const WorkflowsList = lazy(() => import('./pages/workflows/WorkflowsList'));
const WorkflowBuilder = lazy(() => import('./pages/workflows/WorkflowBuilder'));
const WorkflowExecutions = lazy(() => import('./pages/workflows/WorkflowExecutions'));
const DigestRulesList = lazy(() => import('./pages/digest/DigestRulesList'));
const TopicsList = lazy(() => import('./pages/topics/TopicsList'));
```

### New Routes

Add inside the `<Route element={<ProtectedRoute><DashboardLayout /></ProtectedRoute>}>` block:

```tsx
{/* Existing */}
<Route path="/apps" element={<AppsList />} />
<Route path="/apps/:id" element={<AppDetail />} />
<Route path="/dashboard" element={<Dashboard />} />

{/* Phase 2 — Workflows */}
<Route path="/workflows" element={<WorkflowsList />} />
<Route path="/workflows/new" element={<WorkflowBuilder />} />
<Route path="/workflows/:id" element={<WorkflowBuilder />} />
<Route path="/workflows/executions" element={<WorkflowExecutions />} />

{/* Phase 2 — Digest Rules */}
<Route path="/digest-rules" element={<DigestRulesList />} />

{/* Phase 2 — Topics */}
<Route path="/topics" element={<TopicsList />} />
```

**Note:** `/workflows/new` and `/workflows/:id` both use `WorkflowBuilder`. The builder detects mode from the presence of `id` param — `useParams<{ id: string }>()`. If `id` is undefined → create mode. If `id` is present → edit mode (fetches existing workflow).

---

## 5. Task 3: Workflows List Page

**File:** `ui/src/pages/workflows/WorkflowsList.tsx` (new)  
**Dependencies:** `workflowsAPI`, `Workflow` type, `useApiQuery`, `SkeletonTable`, `EmptyState`, `ConfirmDialog`, `Badge`, `Table`

### Specification

This page shows all workflows across all apps. The user needs an API key context to call the workflows API. Phase 2 uses the same pattern as existing pages — the user must have created an app first. In the absence of a global app context, the page should present an app picker at the top (using `ResourcePicker` backed by `applicationsAPI.list`). Selecting an app sets the API key context for the page.

**Alternative (simpler, recommended):** Skip the app picker. Workflows are scoped to the app, so this page is only reachable from the sidebar. Require the user to have at least one app. Use `localStorage.getItem('lastAppApiKey')` or prompt them to pick an app. Since `AppDetail.tsx` already stores the last-used API key context, the simplest approach is to store the selected app's API key in `localStorage` whenever `AppDetail` loads, and read it from all standalone pages.

**Pragmatic approach for Phase 2:** Add a state variable `apiKey` initialized from `localStorage.getItem('last_api_key')`. If no key, show an `EmptyState` with "Select an application first" and a link to `/apps`. At the top of the page, show a small app switcher using `ResourcePicker` that updates the API key. When an app is selected, store its API key in localStorage and refetch.

### UI Layout

```
┌─────────────────────────────────────────────────────────────┐
│ Workflows                                                    │
│                                                              │
│ App: [ResourcePicker → Applications]        [+ New Workflow] │
│                                                              │
│ Filters: [Status ▼ draft|active|inactive]  [🔍 Search name] │
│                                                              │
│ ┌──────────────────────────────────────────────────────────┐ │
│ │ Name         │ Trigger ID  │ Status │ Steps │ Ver │ Updated│
│ ├──────────────┼─────────────┼────────┼───────┼─────┼───────│
│ │ Welcome Flow │ user_signup │ active │ 3     │ 2   │ 2h ago│
│ │ Cart Abandon │ cart_left   │ draft  │ 5     │ 1   │ 1d ago│
│ └──────────────────────────────────────────────────────────┘ │
│                                                              │
│ [← 1 2 3 →]                                                 │
└─────────────────────────────────────────────────────────────┘
```

### Table Columns

| Column | Source | Render |
|--------|--------|--------|
| Name | `workflow.name` | Text, clickable → navigate to `/workflows/${id}` |
| Trigger ID | `workflow.trigger_id` | Monospace badge |
| Status | `workflow.status` | Badge: `active` → green, `draft` → yellow, `inactive` → gray |
| Steps | `workflow.steps.length` | Number |
| Version | `workflow.version` | `v{n}` |
| Updated | `workflow.updated_at` | Relative time (e.g., "2h ago") |
| Actions | — | Dropdown: Edit, Duplicate, Delete |

### Actions

| Action | Behavior |
|--------|----------|
| **+ New Workflow** | `navigate('/workflows/new')` |
| **Edit** | `navigate('/workflows/${id}')` |
| **Duplicate** | Fetch workflow → `workflowsAPI.create(apiKey, { ...workflow, name: name + ' (copy)', status: 'draft' })` → refetch list → toast |
| **Delete** | `ConfirmDialog` → `workflowsAPI.delete(apiKey, id)` → refetch → toast |
| **Row click** | Same as Edit |

### Status Filter

Dropdown with options: All, Draft, Active, Inactive. Filters client-side from the fetched list (backend doesn't have filter params yet — acceptable for Phase 2).

### Search

Client-side filter on `workflow.name` using `useDebounce(searchTerm, 300)`.

### Loading & Empty States

- Loading: `<SkeletonTable rows={5} columns={6} />`
- Empty (no workflows): `<EmptyState title="No workflows yet" description="Create your first multi-step notification flow" action={{ label: 'Create Workflow', onClick: () => navigate('/workflows/new') }} />`
- Empty (no app selected): `<EmptyState title="Select an application" description="Choose an app to view its workflows" action={{ label: 'Go to Applications', onClick: () => navigate('/apps') }} />`

### Full Component Structure

```tsx
// pages/workflows/WorkflowsList.tsx
import React, { useState, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { workflowsAPI, applicationsAPI } from '../../services/api';
import type { Workflow, Application } from '../../types';
import { useApiQuery } from '../../hooks/use-api-query';
import { useDebounce } from '../../hooks/use-debounce';
import ResourcePicker from '../../components/ResourcePicker';
import SkeletonTable from '../../components/SkeletonTable';
import EmptyState from '../../components/EmptyState';
import ConfirmDialog from '../../components/ConfirmDialog';
import { Button } from '../../components/ui/button';
import { Badge } from '../../components/ui/badge';
import { Input } from '../../components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../../components/ui/select';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../../components/ui/table';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '../../components/ui/dropdown-menu';
import { MoreHorizontal, Plus, Workflow as WorkflowIcon } from 'lucide-react';
import { toast } from 'sonner';

const WorkflowsList: React.FC = () => {
    // App context
    const [apiKey, setApiKey] = useState<string | null>(
        localStorage.getItem('last_api_key')
    );
    const navigate = useNavigate();

    // Filters
    const [statusFilter, setStatusFilter] = useState<string>('all');
    const [search, setSearch] = useState('');
    const debouncedSearch = useDebounce(search, 300);

    // Data fetching
    const { data, loading, refetch } = useApiQuery(
        () => workflowsAPI.list(apiKey!, 100, 0),
        [apiKey],
        { enabled: !!apiKey }
    );

    // Delete confirm
    const [deleteTarget, setDeleteTarget] = useState<Workflow | null>(null);
    const [deleting, setDeleting] = useState(false);

    // Filtered workflows
    const workflows = useMemo(() => {
        let items = data?.workflows || [];
        if (statusFilter !== 'all') {
            items = items.filter(w => w.status === statusFilter);
        }
        if (debouncedSearch) {
            const q = debouncedSearch.toLowerCase();
            items = items.filter(w => w.name.toLowerCase().includes(q));
        }
        return items;
    }, [data, statusFilter, debouncedSearch]);

    const handleAppSelect = (appId: string | null) => { /* ... fetch app, store api_key */ };
    const handleDelete = async () => { /* ... workflowsAPI.delete → refetch → toast */ };
    const handleDuplicate = async (w: Workflow) => { /* ... workflowsAPI.create copy → refetch → toast */ };

    // Render: ResourcePicker for app, filters, table, empty/loading states
    // ...
};

export default WorkflowsList;
```

### Time Helper

Add a small helper for relative time formatting. Either inline or as a utility:

```tsx
function timeAgo(dateStr: string): string {
    const seconds = Math.floor((Date.now() - new Date(dateStr).getTime()) / 1000);
    if (seconds < 60) return 'just now';
    const minutes = Math.floor(seconds / 60);
    if (minutes < 60) return `${minutes}m ago`;
    const hours = Math.floor(minutes / 60);
    if (hours < 24) return `${hours}h ago`;
    const days = Math.floor(hours / 24);
    return `${days}d ago`;
}
```

This helper will be used across multiple Phase 2 pages. Place it in `ui/src/lib/utils.ts` or colocate it.

### Key Decisions

- **App context pattern:** `localStorage.getItem('last_api_key')` + `ResourcePicker` app switcher at the top. When `AppDetail` loads an app, it should set this key. This pattern will be used consistently across Workflows, Digest Rules, and Topics pages.
- **Pagination:** Use offset-based pagination from `workflowsAPI.list`. Show `Pagination` component from existing `components/Pagination.tsx`.

---

## 6. Task 4: Workflow Step Components

Two new components that the `WorkflowBuilder` (Task 5) will compose.

### 4.1 — WorkflowStepCard

**File:** `ui/src/components/workflows/WorkflowStepCard.tsx` (new)

A read-only summary card for a single step in the vertical step list. Shows step type icon, name, brief config summary, and action buttons (Edit, Remove).

```
┌─ Step {order} ──────────────────────────────────────────┐
│ {icon} {type} Step — {name}             [Edit] [✕]      │
│ {summary line: e.g., "Channel: email, Template: Welcome"}│
└──────────────────────────────────────────────────────────┘
```

#### Props

```tsx
interface WorkflowStepCardProps {
    step: WorkflowStep;
    index: number;
    onEdit: () => void;
    onRemove: () => void;
}
```

#### Step Type Icons & Colors

| Type | Icon (lucide) | Summary |
|------|---------------|---------|
| `channel` | `Send` | `Channel: {config.channel}, Template: {templateName or config.template_id}` |
| `delay` | `Clock` | `Wait: {config.duration}` |
| `digest` | `Layers` | `Key: {config.digest_key}, Window: {config.window}` |
| `condition` | `GitBranch` | `IF {config.condition?.field} {config.condition?.operator} {config.condition?.value}` |

#### Card Styling

- Border: `border border-border rounded-lg`
- Background: White
- Left accent stripe: 3px solid colored by step type (channel: blue-500, delay: amber-500, digest: purple-500, condition: green-500)
- Actions: Ghost buttons, only visible on hover (`opacity-0 group-hover:opacity-100 transition-opacity`)

#### Full Component

```tsx
import React from 'react';
import type { WorkflowStep } from '../../types';
import { Button } from '../ui/button';
import { Send, Clock, Layers, GitBranch, Pencil, X } from 'lucide-react';

const stepMeta: Record<string, { icon: React.ReactNode; color: string; label: string }> = {
    channel: { icon: <Send className="h-4 w-4" />, color: 'border-l-blue-500', label: 'Channel' },
    delay: { icon: <Clock className="h-4 w-4" />, color: 'border-l-amber-500', label: 'Delay' },
    digest: { icon: <Layers className="h-4 w-4" />, color: 'border-l-purple-500', label: 'Digest' },
    condition: { icon: <GitBranch className="h-4 w-4" />, color: 'border-l-green-500', label: 'Condition' },
};

function getStepSummary(step: WorkflowStep): string {
    const c = step.config;
    switch (step.type) {
        case 'channel':
            return `Channel: ${c.channel || '—'}, Template: ${c.template_id || 'none'}`;
        case 'delay':
            return `Wait: ${c.duration || '—'}`;
        case 'digest':
            return `Key: ${c.digest_key || '—'}, Window: ${c.window || '—'}`;
        case 'condition':
            return c.condition
                ? `IF ${c.condition.field} ${c.condition.operator} ${c.condition.value}`
                : 'No condition configured';
        default:
            return '';
    }
}

interface WorkflowStepCardProps {
    step: WorkflowStep;
    index: number;
    onEdit: () => void;
    onRemove: () => void;
}

const WorkflowStepCard: React.FC<WorkflowStepCardProps> = ({ step, index, onEdit, onRemove }) => {
    const meta = stepMeta[step.type] || stepMeta.channel;

    return (
        <div className={`group border border-border rounded-lg ${meta.color} border-l-[3px] bg-card`}>
            <div className="flex items-center justify-between px-4 py-3">
                <div className="flex items-center gap-3 min-w-0">
                    <span className="text-muted-foreground">{meta.icon}</span>
                    <div className="min-w-0">
                        <p className="text-sm font-medium text-foreground">
                            Step {index + 1}: {step.name || meta.label}
                        </p>
                        <p className="text-xs text-muted-foreground truncate">
                            {getStepSummary(step)}
                        </p>
                    </div>
                </div>
                <div className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                    <Button variant="ghost" size="sm" onClick={onEdit}>
                        <Pencil className="h-3.5 w-3.5" />
                    </Button>
                    <Button variant="ghost" size="sm" onClick={onRemove}>
                        <X className="h-3.5 w-3.5" />
                    </Button>
                </div>
            </div>
        </div>
    );
};

export default WorkflowStepCard;
```

### 4.2 — WorkflowStepEditor

**File:** `ui/src/components/workflows/WorkflowStepEditor.tsx` (new)

A form for creating or editing a single step. Displayed in a slide panel (`SlidePanel`) when the user clicks "Edit" on a step card or "+ Add Step".

#### Props

```tsx
interface WorkflowStepEditorProps {
    step: Partial<WorkflowStep> | null;   // null = creating new step
    apiKey: string;                        // For ResourcePicker fetchers
    onSave: (step: Omit<WorkflowStep, 'id'>) => void;
    onCancel: () => void;
}
```

#### Form Fields by Step Type

The form starts with a **Step Type** selector (radio group or select) and a **Name** field. The remaining fields change based on the selected type:

| Type | Fields |
|------|--------|
| `channel` | **Channel** (select: email, push, sms, webhook, sse, slack, discord, whatsapp), **Template** (ResourcePicker → `templatesAPI.list`), **Provider** (optional text input) |
| `delay` | **Duration** (text input, hint: "e.g., '30m', '1h', '24h', '7d'") |
| `digest` | **Digest Key** (text, hint: "Events with the same key are grouped together"), **Window** (text, hint: "e.g., '1h', '30m'"), **Max Batch** (number, hint: "Max events per digest — 0 = unlimited") |
| `condition` | **Field** (text, hint: "JSON path in payload — e.g., 'payload.opened'"), **Operator** (select: eq, neq, contains, gt, lt, exists, not_read), **Value** (text input) |

#### Template ResourcePicker (Channel Type Only)

```tsx
<ResourcePicker<Template>
    label="Template"
    value={config.template_id || null}
    onChange={(id) => setConfig({ ...config, template_id: id || '' })}
    fetcher={async () => {
        const res = await templatesAPI.list(apiKey, 100, 0);
        // Filter templates to match selected channel
        return (res.templates || []).filter(
            t => !config.channel || t.channel === config.channel
        );
    }}
    labelKey="name"
    valueKey="template_id"
    renderItem={(t) => (
        <div className="flex items-center justify-between w-full">
            <span>{t.name}</span>
            <Badge variant="outline" className="text-xs">{t.channel}</Badge>
        </div>
    )}
    hint="Select the template that defines this notification's content."
    placeholder="Search templates..."
    required
/>
```

**Dependency UX:** When the user changes the `channel` dropdown, the template picker should re-fetch (clear `hasFetched` ref) to show only templates for that channel. This requires the ResourcePicker's `fetcher` to be regenerated — achieved by including `config.channel` as a key prop on the ResourcePicker so React remounts it when channel changes.

#### Full Component Structure

```tsx
import React, { useState } from 'react';
import type { WorkflowStep, WorkflowStepType, StepConfig, StepCondition, Template } from '../../types';
import { templatesAPI } from '../../services/api';
import ResourcePicker from '../ResourcePicker';
import { SlidePanel } from '../ui/slide-panel';
import { Button } from '../ui/button';
import { Input } from '../ui/input';
import { Label } from '../ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '../ui/select';
import { Badge } from '../ui/badge';

const STEP_TYPES: { value: WorkflowStepType; label: string }[] = [
    { value: 'channel', label: 'Channel — Send via a delivery channel' },
    { value: 'delay', label: 'Delay — Wait before continuing' },
    { value: 'digest', label: 'Digest — Batch events together' },
    { value: 'condition', label: 'Condition — Branch on a condition' },
];

const CHANNELS = ['email', 'push', 'sms', 'webhook', 'sse', 'slack', 'discord', 'whatsapp'];

const OPERATORS: { value: string; label: string }[] = [
    { value: 'eq', label: 'equals' },
    { value: 'neq', label: 'not equals' },
    { value: 'contains', label: 'contains' },
    { value: 'gt', label: 'greater than' },
    { value: 'lt', label: 'less than' },
    { value: 'exists', label: 'exists' },
    { value: 'not_read', label: 'not read' },
];

interface WorkflowStepEditorProps {
    step: Partial<WorkflowStep> | null;
    apiKey: string;
    onSave: (step: Omit<WorkflowStep, 'id'>) => void;
    onCancel: () => void;
}

const WorkflowStepEditor: React.FC<WorkflowStepEditorProps> = ({
    step,
    apiKey,
    onSave,
    onCancel,
}) => {
    const [type, setType] = useState<WorkflowStepType>(step?.type || 'channel');
    const [name, setName] = useState(step?.name || '');
    const [config, setConfig] = useState<StepConfig>(step?.config || {});
    const [condition, setCondition] = useState<StepCondition | undefined>(step?.skip_if);

    const handleSave = () => {
        onSave({
            name: name || STEP_TYPES.find(t => t.value === type)!.label.split(' — ')[0],
            type,
            order: step?.order ?? 0,
            config,
            skip_if: condition,
        });
    };

    return (
        <div className="space-y-6 p-1">
            {/* Step Type selector */}
            {/* Name field */}
            {/* Dynamic config fields based on type */}
            {/* Channel: channel select → template ResourcePicker → provider input */}
            {/* Delay: duration input */}
            {/* Digest: digest_key, window, max_batch */}
            {/* Condition: field, operator, value */}
            {/* Footer: Cancel + Save buttons */}
        </div>
    );
};

export default WorkflowStepEditor;
```

**Key implementation details:**
- When type changes, reset `config` to `{}` to avoid stale fields.
- Validate required fields before enabling Save: channel type needs channel + template_id, delay needs duration, digest needs digest_key + window.
- Use hint text (`<p className="text-xs text-muted-foreground">`) below each field.

---

## 7. Task 5: Workflow Builder Page

**File:** `ui/src/pages/workflows/WorkflowBuilder.tsx` (new)  
**Dependencies:** Task 3 (WorkflowsList for navigation), Task 4 (WorkflowStepCard + WorkflowStepEditor)

### Description

The most complex page in the UI. A vertical step editor for creating and editing workflows. NOT a drag-and-drop canvas — keep it simple and fast.

### Mode Detection

```tsx
const { id } = useParams<{ id: string }>();
const isEditMode = !!id;
// If isEditMode → fetch existing workflow with workflowsAPI.get(apiKey, id)
// If create mode → start with empty form
```

### UI Layout (Wireframe from Master Plan)

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
│       [ + Add Step ]                                       │
│                                                            │
├───────────────────────────────────────────────────────────┤
│ TEST TRIGGER (collapsible)                                 │
│ User: [ResourcePicker → Users API]                         │
│ Payload (JSON): [JsonEditor]                               │
│ [Trigger Workflow]                                         │
└───────────────────────────────────────────────────────────┘
```

### State

```tsx
interface BuilderState {
    name: string;
    description: string;
    trigger_id: string;
    steps: Omit<WorkflowStep, 'id'>[];
    status: WorkflowStatus;
}

const [form, setForm] = useState<BuilderState>({
    name: '',
    description: '',
    trigger_id: '',
    steps: [],
    status: 'draft',
});

const [editingStepIndex, setEditingStepIndex] = useState<number | null>(null);
const [showStepEditor, setShowStepEditor] = useState(false);
const [saving, setSaving] = useState(false);
```

### Step Management Functions

```tsx
const addStep = (step: Omit<WorkflowStep, 'id'>) => {
    setForm(prev => ({
        ...prev,
        steps: [...prev.steps, { ...step, order: prev.steps.length + 1 }],
    }));
    setShowStepEditor(false);
};

const updateStep = (index: number, step: Omit<WorkflowStep, 'id'>) => {
    setForm(prev => ({
        ...prev,
        steps: prev.steps.map((s, i) => i === index ? { ...step, order: i + 1 } : s),
    }));
    setEditingStepIndex(null);
    setShowStepEditor(false);
};

const removeStep = (index: number) => {
    setForm(prev => ({
        ...prev,
        steps: prev.steps
            .filter((_, i) => i !== index)
            .map((s, i) => ({ ...s, order: i + 1 })),
    }));
};
```

### Save & Activate

| Button | Action | Validation |
|--------|--------|------------|
| **Save Draft** | `workflowsAPI.create(apiKey, { ...form, status: 'draft' })` or `.update(apiKey, id, { ...form })` | Name + Trigger ID required. Steps ≥ 1. |
| **Activate** | Same as save but with `status: 'active'` | All step configs must be complete (channel steps need template_id). |

Post-save: Toast success, navigate to `/workflows` or update URL to `/workflows/${newId}`.

### Test Trigger Section

A collapsible section at the bottom (using `<details>` or state toggle):

```tsx
<Card>
    <CardHeader><CardTitle>Test Trigger</CardTitle></CardHeader>
    <CardContent className="space-y-4">
        <ResourcePicker<User>
            label="User"
            value={testUserId}
            onChange={setTestUserId}
            fetcher={async () => {
                const res = await usersAPI.list(apiKey!, 100, 0);
                return res.users || [];
            }}
            labelKey="email"
            valueKey="user_id"
            hint="Select the user who will receive this test notification."
        />
        <JsonEditor
            label="Payload"
            value={testPayload}
            onChange={setTestPayload}
            hint="Variables for your template — e.g. {\"user_name\": \"Alice\"}"
        />
        <Button
            onClick={handleTrigger}
            disabled={!testUserId || triggering}
        >
            {triggering ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}
            Trigger Workflow
        </Button>
    </CardContent>
</Card>
```

API call: `workflowsAPI.trigger(apiKey, { trigger_id: form.trigger_id, user_id: testUserId, payload: JSON.parse(testPayload) })`

### Vertical Connector Between Steps

Between each `WorkflowStepCard`, render a connector:

```tsx
<div className="flex justify-center py-1">
    <div className="w-px h-6 bg-border" />
</div>
<div className="flex justify-center pb-2">
    <ChevronDown className="h-4 w-4 text-muted-foreground" />
</div>
```

### Step Editor Panel

When `showStepEditor` is true, render `WorkflowStepEditor` inside a `SlidePanel`:

```tsx
<SlidePanel
    open={showStepEditor}
    onClose={() => { setShowStepEditor(false); setEditingStepIndex(null); }}
    title={editingStepIndex !== null ? `Edit Step ${editingStepIndex + 1}` : 'Add Step'}
>
    <WorkflowStepEditor
        step={editingStepIndex !== null ? form.steps[editingStepIndex] : null}
        apiKey={apiKey!}
        onSave={(step) => {
            if (editingStepIndex !== null) {
                updateStep(editingStepIndex, step);
            } else {
                addStep(step);
            }
        }}
        onCancel={() => { setShowStepEditor(false); setEditingStepIndex(null); }}
    />
</SlidePanel>
```

---

## 8. Task 6: Workflow Executions & Timeline

### 6.1 — Workflow Executions Page

**File:** `ui/src/pages/workflows/WorkflowExecutions.tsx` (new)

Lists workflow executions across all workflows for the selected app.

#### UI Layout

```
┌─────────────────────────────────────────────────────────────┐
│ Workflow Executions                                          │
│                                                              │
│ App: [same picker as WorkflowsList]                          │
│                                                              │
│ Filters: [Workflow ▼]  [Status ▼]                           │
│                                                              │
│ ┌──────────────────────────────────────────────────────────┐ │
│ │ Exec ID  │ Workflow    │ User ID  │ Status    │ Started  │ │
│ ├──────────┼─────────────┼──────────┼───────────┼─────────│ │
│ │ abc-12.. │ Welcome     │ user-45  │ completed │ 2h ago  │ │
│ │ def-34.. │ Cart Aband. │ user-78  │ running   │ 5m ago  │ │
│ └──────────────────────────────────────────────────────────┘ │
│                                                              │
│ [← 1 2 →]                                                   │
│                                                              │
│ ┌ Expanded: Execution abc-123... ─────────────────────────┐ │
│ │ (ExecutionTimeline component)                            │ │
│ └──────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

#### Table Columns

| Column | Source | Render |
|--------|--------|--------|
| Execution ID | `execution.id` | Truncated (first 8 chars), monospace |
| Workflow | `workflowName` (cross-ref from workflows list or execution data) | Text |
| User ID | `execution.user_id` | Truncated |
| Status | `execution.status` | Badge: running → blue, completed → green, failed → red, cancelled → gray, paused → yellow |
| Started | `execution.started_at` | Relative time |

#### Row Expansion

Clicking a row toggles an expanded section below it showing `<ExecutionTimeline execution={execution} />`.

#### Actions

| Action | Condition | API |
|--------|-----------|-----|
| Cancel | status === 'running' | `workflowsAPI.cancelExecution(apiKey, id)` |

#### Filters

- **Workflow** dropdown: Populated from `workflowsAPI.list`. Sends `workflow_id` to `workflowsAPI.listExecutions`.
- **Status** dropdown: running, completed, failed, cancelled, paused.

### 6.2 — Execution Timeline Component

**File:** `ui/src/components/workflows/ExecutionTimeline.tsx` (new)

A vertical timeline showing each step's execution result. Used as the expanded row content in WorkflowExecutions.

#### Props

```tsx
interface ExecutionTimelineProps {
    execution: WorkflowExecution;
}
```

#### Layout

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

#### Step Result Rendering

Each step in `execution.step_results` maps to a timeline node:

```tsx
const statusIcon: Record<string, { icon: string; color: string }> = {
    completed: { icon: '✅', color: 'text-green-600' },
    running:   { icon: '🔄', color: 'text-blue-600' },
    failed:    { icon: '❌', color: 'text-red-600' },
    skipped:   { icon: '⬜', color: 'text-muted-foreground' },
    pending:   { icon: '⏳', color: 'text-muted-foreground' },
};
```

#### Duration Calculation

```tsx
function formatDuration(start?: string, end?: string): string {
    if (!start || !end) return '';
    const ms = new Date(end).getTime() - new Date(start).getTime();
    if (ms < 1000) return `${ms}ms`;
    return `${(ms / 1000).toFixed(1)}s`;
}
```

#### Full Component

```tsx
import React from 'react';
import type { WorkflowExecution, StepResult } from '../../types';

interface ExecutionTimelineProps {
    execution: WorkflowExecution;
}

const ExecutionTimeline: React.FC<ExecutionTimelineProps> = ({ execution }) => {
    const stepEntries = Object.entries(execution.step_results || {})
        .sort(([, a], [, b]) => {
            if (!a.started_at || !b.started_at) return 0;
            return new Date(a.started_at).getTime() - new Date(b.started_at).getTime();
        });

    if (stepEntries.length === 0) {
        return <p className="text-sm text-muted-foreground py-4 px-6">No step results available.</p>;
    }

    return (
        <div className="py-4 px-6 space-y-0">
            {stepEntries.map(([stepId, result], idx) => (
                <div key={stepId} className="flex gap-3">
                    {/* Timeline line + dot */}
                    <div className="flex flex-col items-center">
                        <div className={`h-3 w-3 rounded-full mt-1 ${getStatusColor(result.status)}`} />
                        {idx < stepEntries.length - 1 && (
                            <div className="w-px flex-1 bg-border min-h-[24px]" />
                        )}
                    </div>
                    {/* Content */}
                    <div className="pb-4 min-w-0">
                        <p className="text-sm font-medium text-foreground">
                            {result.step_id} — {result.status}
                            {result.started_at && result.completed_at && (
                                <span className="text-xs text-muted-foreground ml-2">
                                    ({formatDuration(result.started_at, result.completed_at)})
                                </span>
                            )}
                        </p>
                        {result.notification_id && (
                            <p className="text-xs text-muted-foreground">→ notification: {result.notification_id}</p>
                        )}
                        {result.error && (
                            <p className="text-xs text-red-600">Error: {result.error}</p>
                        )}
                    </div>
                </div>
            ))}
        </div>
    );
};

function getStatusColor(status: string): string {
    switch (status) {
        case 'completed': return 'bg-green-500';
        case 'running': return 'bg-blue-500';
        case 'failed': return 'bg-red-500';
        case 'skipped': return 'bg-muted-foreground/30';
        default: return 'bg-muted-foreground/30';
    }
}

function formatDuration(start: string, end: string): string {
    const ms = new Date(end).getTime() - new Date(start).getTime();
    if (ms < 1000) return `${ms}ms`;
    return `${(ms / 1000).toFixed(1)}s`;
}

export default ExecutionTimeline;
```

---

## 9. Task 7: Digest Rules (List + Form)

**File:** `ui/src/pages/digest/DigestRulesList.tsx` (new)  
**Dependencies:** `digestRulesAPI`, `templatesAPI`, `DigestRule` type, `ResourcePicker`, `SlidePanel`, `ConfirmDialog`, `EmptyState`, `SkeletonTable`

### Description

A standalone page at `/digest-rules` AND also used as an embedded tab in `AppDetail` (Task 10). The page manages digest rules — CRUD operations with a slide-panel form.

### Same App Context Pattern

Same as WorkflowsList — `localStorage.getItem('last_api_key')` + app picker at top. When embedded in AppDetail, the `apiKey` is passed as a prop instead.

### Props (Dual-Mode)

```tsx
interface DigestRulesListProps {
    apiKey?: string;         // If provided, skip app picker (embedded in AppDetail)
    embedded?: boolean;      // If true, hide page title + app picker
}
```

### UI Layout

```
┌─────────────────────────────────────────────────────────────┐
│ Digest Rules                          [+ Create Digest Rule]│
│                                                              │
│ ┌──────────────────────────────────────────────────────────┐ │
│ │ Name     │ Digest Key │ Window │ Channel │ Template │ Status│
│ ├──────────┼────────────┼────────┼─────────┼──────────┼──────│
│ │ Hourly   │ user_acts  │ 1h     │ email   │ Digest   │ active│
│ │ Daily    │ sys_events │ 24h    │ webhook │ Summary  │ inact.│
│ └──────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

### Table Columns

| Column | Render |
|--------|--------|
| Name | Text |
| Digest Key | Monospace badge |
| Window | Text (e.g., "1h") |
| Channel | Badge (color by channel) |
| Template | Text (template name via lookup or template_id) |
| Status | Badge (active → green, inactive → gray) |
| Actions | Dropdown: Edit, Toggle Status, Delete |

### Slide Panel Form (Create/Edit)

```
┌─ Create Digest Rule ──────────────────────────────┐
│                                                    │
│ Name *:      [________________]                    │
│                                                    │
│ Digest Key *: [________________]                   │
│ Hint: Events with the same key are grouped         │
│                                                    │
│ Window *:    [________________]                     │
│ Hint: How long to accumulate — e.g. '1h', '30m'   │
│                                                    │
│ Channel *:   [email ▼]                             │
│                                                    │
│ Template *:  [ResourcePicker → filtered templates] │
│ Hint: Only templates matching the selected channel │
│                                                    │
│ Max Batch:   [100]                                 │
│ Hint: Max events per digest — 0 = unlimited        │
│                                                    │
│              [Cancel]  [Save]                      │
└────────────────────────────────────────────────────┘
```

### Dependency UX: Channel-Filtered Template Picker

The template `ResourcePicker` only shows templates whose `channel` matches the currently selected channel. When the channel changes, the picker re-mounts (via `key={channel}`) to reset its cache.

```tsx
<ResourcePicker<Template>
    key={`tpl-${form.channel}`}  // Force remount when channel changes
    label="Template"
    value={form.template_id || null}
    onChange={(id) => setForm({ ...form, template_id: id || '' })}
    fetcher={async () => {
        const res = await templatesAPI.list(apiKey!, 100, 0);
        return (res.templates || []).filter(t => t.channel === form.channel);
    }}
    labelKey="name"
    valueKey="template_id"
    renderItem={(t) => (
        <div className="flex items-center justify-between w-full">
            <span>{t.name}</span>
            <Badge variant="outline" className="text-xs">v{t.version}</Badge>
        </div>
    )}
    hint="Only templates matching the selected channel are shown."
    required
/>
```

### Actions

| Action | API |
|--------|-----|
| Create | `digestRulesAPI.create(apiKey, form)` |
| Edit | `digestRulesAPI.update(apiKey, id, form)` |
| Toggle Status | `digestRulesAPI.update(apiKey, id, { status: current === 'active' ? 'inactive' : 'active' })` |
| Delete | `ConfirmDialog` → `digestRulesAPI.delete(apiKey, id)` |

---

## 10. Task 8: Topics (List + Form + Subscribers)

### 8.1 — Topics List Page

**File:** `ui/src/pages/topics/TopicsList.tsx` (new)  
**Dependencies:** `topicsAPI`, `Topic` type, `SlidePanel`, `ConfirmDialog`, `EmptyState`, `SkeletonTable`

### Same Dual-Mode Pattern as Digest Rules

```tsx
interface TopicsListProps {
    apiKey?: string;
    embedded?: boolean;
}
```

### UI Layout

```
┌─────────────────────────────────────────────────────────────┐
│ Topics                                      [+ Create Topic]│
│                                                              │
│ ┌──────────────────────────────────────────────────────────┐ │
│ │ Name     │ Key           │ Description   │ Subs │ Created│ │
│ ├──────────┼───────────────┼───────────────┼──────┼───────│ │
│ │ Watchers │ proj-watchers │ Project watch  │ 12   │ 3d ago│ │
│ │ Alerts   │ sys-alerts    │ System alerts  │ 5    │ 1w ago│ │
│ └──────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

### Table Columns

| Column | Render |
|--------|--------|
| Name | Text |
| Key | Monospace badge |
| Description | Truncated text |
| Subscribers | Number (from separate API call or stored field — depends on backend response) |
| Created | Relative time |
| Actions | Dropdown: Edit, Manage Subscribers, Delete |

### Slide Panel Form (Create/Edit)

```
┌─ Create Topic ─────────────────────────────────────┐
│                                                     │
│ Name *:        [________________]                   │
│                                                     │
│ Key *:         [auto-slugified-from-name]           │
│ Hint: Machine-readable key — e.g. 'project-123-    │
│       watchers'. Used in API calls.                 │
│                                                     │
│ Description:   [________________________________]   │
│                                                     │
│                [Cancel]  [Save]                     │
└─────────────────────────────────────────────────────┘
```

**Auto-slug:** When typing the Name field, auto-generate the Key by lowercasing, replacing spaces with hyphens, and stripping non-alphanumeric characters. The Key field is editable so the user can override.

```tsx
const autoSlug = (name: string) =>
    name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '');
```

### Actions

| Action | API |
|--------|-----|
| Create | `topicsAPI.create(apiKey, form)` |
| Edit | `topicsAPI.update(apiKey, id, form)` |
| Manage Subscribers | Open `TopicSubscribers` in slide panel |
| Delete | `ConfirmDialog` → `topicsAPI.delete(apiKey, id)` |

### 8.2 — Topic Subscribers Component

**File:** `ui/src/components/topics/TopicSubscribers.tsx` (new)

A slide-panel component showing current subscribers with the ability to add/remove users.

#### Props

```tsx
interface TopicSubscribersProps {
    topicId: string;
    apiKey: string;
    onClose: () => void;
}
```

#### UI Layout

```
┌─ Subscribers: {topic name} ───────────────────────┐
│                                                    │
│ Add Subscribers:                                   │
│ [ResourcePicker → multi-select users]   [Add]      │
│ Hint: Users added to this topic will receive all   │
│ notifications sent to this topic's key.            │
│                                                    │
│ Current Subscribers ({count}):                     │
│ ┌────────────────────────────────────────────────┐ │
│ │ User Email / External ID │ Added At │ [Remove] │ │
│ │ alice@example.com        │ 3d ago   │    ✕     │ │
│ │ bob@example.com          │ 1w ago   │    ✕     │ │
│ └────────────────────────────────────────────────┘ │
└────────────────────────────────────────────────────┘
```

#### Add Subscribers Flow

The "Add" flow uses `ResourcePicker` backed by `usersAPI.list()`. Since `ResourcePicker` is currently single-select, two options:

1. **Option A (Recommended for Phase 2):** Use single-select `ResourcePicker`. Add one user at a time. The "Add" button calls `topicsAPI.addSubscribers(apiKey, topicId, { user_ids: [selectedUserId] })`.
2. **Option B (Phase 3+):** Extend `ResourcePicker` with multi-select support.

For now, use Option A — single-select add, one at a time.

#### Remove Subscriber

Each subscriber row has a remove button. On click: `topicsAPI.removeSubscribers(apiKey, topicId, { user_ids: [userId] })` → refetch subscribers → toast.

#### Data Fetching

```tsx
const { data: subscriberData, loading, refetch } = useApiQuery(
    () => topicsAPI.getSubscribers(apiKey, topicId, 100, 0),
    [apiKey, topicId]
);
```

#### User Display

The subscriber response returns `TopicSubscription` which has `user_id` but no email/name. To display a meaningful label, either:

1. Cross-reference against `usersAPI.list()` to map `user_id` → `email`. Fetch once on mount.
2. Display `user_id` truncated if user list is unavailable.

**Recommended:** Fetch users list on mount, build a `Map<string, User>` for lookups. Show email if found, user_id otherwise.

---

## 11. Task 9: Notification Inbox Enhancements

**Modified File:** `ui/src/components/AppNotifications.tsx`  
**New Files:** `components/notifications/BulkActionsBar.tsx`, `components/notifications/NotificationDetail.tsx`

### 9.1 — BulkActionsBar Component

**File:** `ui/src/components/notifications/BulkActionsBar.tsx` (new)

Appears at the top of the notifications table when ≥1 notification is selected via checkbox.

#### Props

```tsx
interface BulkActionsBarProps {
    selectedCount: number;
    onMarkRead: () => void;
    onArchive: () => void;
    onClear: () => void;
    loading?: boolean;
}
```

#### Layout

```
┌───────────────────────────────────────────────────────────┐
│ ✓ 3 selected    [Mark Read] [Archive] [Clear Selection]   │
└───────────────────────────────────────────────────────────┘
```

#### Styling

- Sticky top bar: `sticky top-0 z-10 bg-muted/80 backdrop-blur-sm border-b border-border`
- Visible only when `selectedCount > 0`
- Buttons use `variant="outline" size="sm"`

#### Full Component

```tsx
import React from 'react';
import { Button } from '../ui/button';
import { CheckSquare, Archive, X } from 'lucide-react';

interface BulkActionsBarProps {
    selectedCount: number;
    onMarkRead: () => void;
    onArchive: () => void;
    onClear: () => void;
    loading?: boolean;
}

const BulkActionsBar: React.FC<BulkActionsBarProps> = ({
    selectedCount,
    onMarkRead,
    onArchive,
    onClear,
    loading = false,
}) => {
    if (selectedCount === 0) return null;

    return (
        <div className="sticky top-0 z-10 bg-muted/80 backdrop-blur-sm border-b border-border px-4 py-2 flex items-center gap-3">
            <span className="text-sm font-medium text-foreground">
                {selectedCount} selected
            </span>
            <div className="flex items-center gap-2">
                <Button variant="outline" size="sm" onClick={onMarkRead} disabled={loading}>
                    <CheckSquare className="h-3.5 w-3.5 mr-1.5" />
                    Mark Read
                </Button>
                <Button variant="outline" size="sm" onClick={onArchive} disabled={loading}>
                    <Archive className="h-3.5 w-3.5 mr-1.5" />
                    Archive
                </Button>
                <Button variant="ghost" size="sm" onClick={onClear}>
                    <X className="h-3.5 w-3.5 mr-1.5" />
                    Clear
                </Button>
            </div>
        </div>
    );
};

export default BulkActionsBar;
```

### 9.2 — NotificationDetail Component

**File:** `ui/src/components/notifications/NotificationDetail.tsx` (new)

A slide-panel showing full notification details with action buttons (snooze, mark read, archive).

#### Props

```tsx
interface NotificationDetailProps {
    notification: Notification;
    apiKey: string;
    onClose: () => void;
    onAction: () => void;   // Callback to refetch list after any mutation
}
```

#### Layout

```
┌─ Notification Detail ─────────────────────────────┐
│                                                    │
│ Title: {title}                                     │
│ Status: [badge]  Channel: [badge]  Priority: [badge] │
│                                                    │
│ Body:                                              │
│ ┌────────────────────────────────────────────────┐ │
│ │ {body content — rendered}                      │ │
│ └────────────────────────────────────────────────┘ │
│                                                    │
│ Recipient: {user_id}                               │
│ Template: {template_id}                            │
│ Created: {created_at}                              │
│ Delivered: {delivered_at}                           │
│ Read: {read_at or 'Unread'}                        │
│                                                    │
│ Metadata:                                          │
│ ┌────────────────────────────────────────────────┐ │
│ │ { ...data as JSON }                            │ │
│ └────────────────────────────────────────────────┘ │
│                                                    │
│ Actions:                                           │
│ [Mark Read] [Snooze ▼] [Archive]                  │
└────────────────────────────────────────────────────┘
```

#### Snooze Duration Picker

When the user clicks "Snooze", show a dropdown with preset durations:

```tsx
const SNOOZE_DURATIONS = [
    { label: '15 minutes', value: 15 },
    { label: '1 hour', value: 60 },
    { label: '4 hours', value: 240 },
    { label: '24 hours', value: 1440 },
];

// onClick → compute ISO8601 until time → notificationsAPI.snooze(apiKey, id, { until })
const handleSnooze = async (minutes: number) => {
    const until = new Date(Date.now() + minutes * 60 * 1000).toISOString();
    await notificationsAPI.snooze(apiKey, notification.notification_id, { until });
    toast.success(`Snoozed for ${minutes >= 60 ? `${minutes / 60}h` : `${minutes}m`}`);
    onAction();
};
```

### 9.3 — AppNotifications.tsx Modifications

**File:** `ui/src/components/AppNotifications.tsx`  
**Action:** Extend existing component with checkbox selection, bulk actions, individual actions, and unread count.

#### Changes Summary

| Feature | Change |
|---------|--------|
| **Checkbox column** | Add `<Checkbox>` as first column in table. Track `selectedIds: Set<string>` state. |
| **Select all header** | Checkbox in `<TableHead>` that toggles all visible notifications. |
| **BulkActionsBar** | Import and render `<BulkActionsBar>` above the table when selections exist. |
| **Unread count badge** | Fetch `notificationsAPI.getUnreadCount(apiKey, userId)` on mount. Show badge next to tab header (handled by parent AppDetail — see Task 10). |
| **Mark Read** (individual) | Add "Mark Read" option in row action dropdown → `notificationsAPI.markRead(apiKey, { notification_ids: [id] })`. |
| **Mark All Read** | Button in toolbar → `notificationsAPI.markAllRead(apiKey, { user_id: selectedUserId })`. |
| **Snooze** (individual) | Add "Snooze" dropdown in row actions → calls snooze API with duration picker. |
| **Unsnooze** | Show "Unsnooze" button on snoozed notifications → `notificationsAPI.unsnooze(apiKey, id)`. |
| **Row click** | Click row → open `NotificationDetail` in slide panel. |
| **Bulk Mark Read** | BulkActionsBar "Mark Read" → `notificationsAPI.markRead(apiKey, { notification_ids: [...selectedIds] })`. |
| **Bulk Archive** | BulkActionsBar "Archive" → `notificationsAPI.bulkArchive(apiKey, { notification_ids: [...selectedIds] })`. |

#### New State Variables

```tsx
const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
const [detailNotification, setDetailNotification] = useState<Notification | null>(null);
const [bulkLoading, setBulkLoading] = useState(false);
```

#### Select All Logic

```tsx
const toggleSelectAll = () => {
    if (selectedIds.size === notifications.length) {
        setSelectedIds(new Set());
    } else {
        setSelectedIds(new Set(notifications.map(n => n.notification_id)));
    }
};

const toggleSelect = (id: string) => {
    setSelectedIds(prev => {
        const next = new Set(prev);
        if (next.has(id)) next.delete(id);
        else next.add(id);
        return next;
    });
};
```

#### Toolbar Enhancement

Add "Mark All Read" button next to existing toolbar buttons:

```tsx
<Button variant="outline" size="sm" onClick={handleMarkAllRead}>
    Mark All Read
</Button>
```

#### Table Row Enhancement

```tsx
<TableRow
    key={n.notification_id}
    className="cursor-pointer hover:bg-muted/50"
    onClick={() => setDetailNotification(n)}
>
    <TableCell onClick={(e) => e.stopPropagation()}>
        <Checkbox
            checked={selectedIds.has(n.notification_id)}
            onCheckedChange={() => toggleSelect(n.notification_id)}
        />
    </TableCell>
    {/* ...existing columns... */}
    <TableCell onClick={(e) => e.stopPropagation()}>
        <DropdownMenu>
            {/* ...existing actions + Mark Read + Snooze... */}
        </DropdownMenu>
    </TableCell>
</TableRow>
```

---

## 12. Task 10: AppDetail Tab Extension

**File:** `ui/src/pages/AppDetail.tsx`  
**Action:** Extend the tab bar from 6 tabs to 8 (add Digest Rules and Topics as embedded sub-tabs).

### Current Tab List

```tsx
const [activeTab, setActiveTab] = useState<
    'overview' | 'users' | 'templates' | 'notifications' | 'settings' | 'integration'
>('overview');
```

### Target Tab List

```tsx
const [activeTab, setActiveTab] = useState<
    'overview' | 'users' | 'templates' | 'notifications' | 'digest-rules' | 'topics' | 'settings' | 'integration'
>('overview');
```

### New Tab Content

```tsx
{/* Digest Rules Tab */}
{activeTab === 'digest-rules' && app && (
    <DigestRulesList apiKey={app.api_key} embedded />
)}

{/* Topics Tab */}
{activeTab === 'topics' && app && (
    <TopicsList apiKey={app.api_key} embedded />
)}
```

### New Imports

```tsx
import DigestRulesList from './digest/DigestRulesList';   // relative path from pages/
import TopicsList from './topics/TopicsList';
```

**Note on import paths:** Since `AppDetail.tsx` is in `pages/` and the new pages are in `pages/digest/` and `pages/topics/`, the imports use `./digest/DigestRulesList` and `./topics/TopicsList` respectively.

### Tab Label Formatting

The existing tab rendering uses `capitalize()` on the tab string. For `'digest-rules'` and `'topics'`, add a display label mapping:

```tsx
const tabLabels: Record<string, string> = {
    'overview': 'Overview',
    'users': 'Users',
    'templates': 'Templates',
    'notifications': 'Notifications',
    'digest-rules': 'Digest Rules',
    'topics': 'Topics',
    'settings': 'Settings',
    'integration': 'Integration',
};

// In the tab rendering:
{tabLabels[tab]}
```

### Tab Overflow on Mobile

The tab bar currently uses a horizontal flex layout. With 8 tabs, it needs horizontal scrolling on mobile:

```tsx
<div className="border-b border-border overflow-x-auto">
    <div className="flex min-w-max">
        {(Object.keys(tabLabels) as Array<keyof typeof tabLabels>).map((tab) => (
            <button
                key={tab}
                onClick={() => setActiveTab(tab)}
                className={`px-3 sm:px-5 py-2.5 sm:py-3 border-b-2 whitespace-nowrap ${
                    activeTab === tab
                        ? 'border-foreground text-foreground font-medium'
                        : 'border-transparent text-muted-foreground hover:text-foreground'
                } text-sm capitalize transition-colors`}
            >
                {tabLabels[tab]}
            </button>
        ))}
    </div>
</div>
```

Key change: Add `overflow-x-auto` on the container and `min-w-max whitespace-nowrap` on the inner flex to enable horizontal scrolling.

### Store Last API Key

Add a side effect in `AppDetail` to persist the API key for standalone pages:

```tsx
useEffect(() => {
    if (app?.api_key) {
        localStorage.setItem('last_api_key', app.api_key);
    }
}, [app?.api_key]);
```

---

## 13. Task 11: Build Verification

**Action:** Run `tsc && vite build` from the `ui/` directory. Must produce zero errors.

### Steps

```powershell
cd ui
npx tsc --noEmit
npx vite build
```

### Expected Output

- TypeScript: 0 errors
- Vite build: Success with chunk output (each new page lazy-loaded as a separate chunk)
- No warnings about unused imports or missing types

### If Errors

Fix any errors before marking Phase 2 complete. Common issues:
- Missing imports (forgotten to import a type or component)
- Incorrect prop types passed to shared components
- `ResourcePicker` generic type parameter mismatches
- Missing `key` prop on list items

---

## 14. Acceptance Criteria

### Functional Requirements

- [ ] **Sidebar** shows 4 nav items: Applications, Workflows, Digest Rules, Topics + Admin section
- [ ] **Workflows List** loads and displays workflows from API, with search + status filter
- [ ] **Workflow Builder** creates/edits workflows with the vertical step editor
- [ ] **Workflow Builder** supports all 4 step types: Channel, Delay, Digest, Condition
- [ ] **Channel step** uses `ResourcePicker` for template selection, filtered by channel
- [ ] **Workflow Builder** test trigger section sends a workflow trigger with user + payload
- [ ] **Workflow Executions** lists execution history with status badges
- [ ] **Execution Timeline** shows step-by-step progress with durations and errors
- [ ] **Digest Rules List** CRUD operations with channel-filtered template picker
- [ ] **Topics List** CRUD operations with auto-slug key generation
- [ ] **Topic Subscribers** add/remove users via `ResourcePicker`
- [ ] **Notifications** have checkbox selection with bulk actions bar (Mark Read, Archive)
- [ ] **Notifications** have snooze/unsnooze with duration picker
- [ ] **Notifications** have Mark All Read button
- [ ] **Notification Detail** panel shows full notification info with actions
- [ ] **AppDetail** has 8 tabs including Digest Rules and Topics (embedded)
- [ ] **AppDetail** tab bar scrolls horizontally on mobile
- [ ] **AppDetail** stores `last_api_key` in localStorage for standalone pages

### Non-Functional Requirements

- [ ] All new pages use `SkeletonTable` for loading states
- [ ] All new pages use `EmptyState` for zero-data states
- [ ] All delete actions use `ConfirmDialog`
- [ ] All mutations show `toast` success/error feedback via Sonner
- [ ] All new pages are lazy-loaded via `React.lazy` in `App.tsx`
- [ ] Build passes: `tsc --noEmit && vite build` → zero errors

### Testing Journeys

| Journey | Validates |
|---------|-----------|
| Create app → Go to Workflows → Create workflow with 2 channel steps + 1 delay → Save → Activate | Tasks 3, 4, 5, Sidebar, Routes |
| Edit workflow → Change step → Save | Task 5 (edit mode) |
| Trigger workflow → View executions → Expand timeline | Tasks 5, 6 |
| AppDetail → Digest Rules tab → Create rule → Select template filtered by channel | Tasks 7, 10 |
| AppDetail → Topics tab → Create topic → Add subscribers → Remove subscriber | Tasks 8, 10 |
| AppDetail → Notifications tab → Select 3 → Mark Read → Archive → Snooze one → Unsnooze | Task 9 |
| AppDetail → Notifications tab → Click row → View detail panel | Task 9 |

---

## Appendix A: Files Quick Reference

### New Files (11)

```
ui/src/
├── pages/
│   ├── workflows/
│   │   ├── WorkflowsList.tsx         (Task 3)
│   │   ├── WorkflowBuilder.tsx       (Task 5)
│   │   └── WorkflowExecutions.tsx    (Task 6)
│   ├── digest/
│   │   └── DigestRulesList.tsx       (Task 7)
│   └── topics/
│       └── TopicsList.tsx            (Task 8)
├── components/
│   ├── workflows/
│   │   ├── WorkflowStepCard.tsx      (Task 4)
│   │   ├── WorkflowStepEditor.tsx    (Task 4)
│   │   └── ExecutionTimeline.tsx     (Task 6)
│   ├── topics/
│   │   └── TopicSubscribers.tsx      (Task 8)
│   └── notifications/
│       ├── BulkActionsBar.tsx        (Task 9)
│       └── NotificationDetail.tsx    (Task 9)
```

### Modified Files (4)

```
ui/src/
├── components/Sidebar.tsx            (Task 1)
├── App.tsx                           (Task 2)
├── components/AppNotifications.tsx   (Task 9)
└── pages/AppDetail.tsx               (Task 10)
```

## Appendix B: Existing Infrastructure Used

All of the following were created in Phase 1 and are ready for use:

| Component/Hook | Location | Usage in Phase 2 |
|----------------|----------|-------------------|
| `ResourcePicker<T>` | `components/ResourcePicker.tsx` | Template pickers (workflows, digest), User pickers (trigger, subscribers, topic add) |
| `ConfirmDialog` | `components/ConfirmDialog.tsx` | All delete confirmations |
| `EmptyState` | `components/EmptyState.tsx` | Zero-data states on all new pages |
| `SkeletonTable` | `components/SkeletonTable.tsx` | Loading states on all new tables |
| `JsonEditor` | `components/JsonEditor.tsx` | Workflow trigger payload, notification data |
| `SlidePanel` | `components/ui/slide-panel.tsx` | Step editor, digest form, topic form, subscriber panel, notification detail |
| `useApiQuery` | `hooks/use-api-query.ts` | Data fetching on all new pages |
| `useDebounce` | `hooks/use-debounce.ts` | Search filtering on workflow and topic lists |
| `workflowsAPI` | `services/api.ts` | All workflow CRUD + trigger + execution APIs |
| `digestRulesAPI` | `services/api.ts` | All digest rule CRUD APIs |
| `topicsAPI` | `services/api.ts` | All topic CRUD + subscriber management APIs |
| `notificationsAPI` (inbox) | `services/api.ts` | `getUnreadCount`, `markRead`, `markAllRead`, `bulkArchive`, `snooze`, `unsnooze` |

All types (`Workflow`, `WorkflowStep`, `StepConfig`, `WorkflowExecution`, `StepResult`, `DigestRule`, `Topic`, `TopicSubscription`, etc.) are already defined in `types/index.ts`.

---

## Appendix C: Phase 2 → Phase 3 Boundary

The following features are explicitly **NOT** in Phase 2 and are deferred to Phase 3:

| Feature | Phase | Reason |
|---------|-------|--------|
| Template Diff Viewer | P3 | Complex side-by-side comparison UI |
| Template Rollback | P3 | Depends on diff viewer |
| Template Test Send Panel | P3 | Not blocking workflows |
| Template Controls Panel | P3 | Non-technical user feature |
| AppTeam.tsx (Team Management) | P3 | RBAC is complex; needs permission checks |
| AppProviders.tsx (Custom Providers) | P3 | Signing key reveal UX |
| AppEnvironments.tsx (Multi-Environment) | P3 | Env switcher + promotion flow |
| Audit Logs Page | P3 | Admin-only, not blocking core features |

Phase 2 focuses on features that **exercise the core notification pipeline** — workflows, digest, topics, and inbox operations. Phase 3 adds management, admin, and advanced template features.

---

*This plan should be treated as the source of truth for Phase 2 implementation. Update the checklist items as they are completed.*
