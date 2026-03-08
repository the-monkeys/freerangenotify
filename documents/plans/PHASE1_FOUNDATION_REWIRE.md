# Phase 1 — Foundation Rewire: Implementation Plan

> **Parent:** [UI_API_INTEGRATION_PLAN.md](UI_API_INTEGRATION_PLAN.md)  
> **Duration:** ~1 week  
> **Goal:** Apply the monkeys.com.co-inspired theme, restructure the layout (sidebar navigation), fix existing bugs, extend `api.ts` with all 56 missing endpoint functions, and add shared utility components.

---

## Table of Contents

1. [Task Breakdown](#1-task-breakdown)
2. [Task 1: Bug Fixes & Cleanup](#2-task-1-bug-fixes--cleanup)
3. [Task 2: Theme Application](#3-task-2-theme-application)
4. [Task 3: Layout Architecture](#4-task-3-layout-architecture)
5. [Task 4: New Shared Components](#5-task-4-new-shared-components)
6. [Task 5: Custom Hooks](#6-task-5-custom-hooks)
7. [Task 6: Types Extension](#7-task-6-types-extension)
8. [Task 7: API Layer Extension](#8-task-7-api-layer-extension)
9. [Task 8: Route Restructure & Lazy Loading](#9-task-8-route-restructure--lazy-loading)
10. [Task 9: Page Retheme](#10-task-9-page-retheme)
11. [Task 10: Build Verification](#11-task-10-build-verification)
12. [Dependency Order](#12-dependency-order)
13. [Acceptance Criteria](#13-acceptance-criteria)

---

## 1. Task Breakdown

| # | Task | Files | New | Modified | Deleted | Est. |
|---|------|-------|-----|----------|---------|------|
| 1 | Bug Fixes & Cleanup | 3 | 0 | 1 | 1 | 30m |
| 2 | Theme Application | 2 | 0 | 2 | 0 | 1h |
| 3 | Layout Architecture | 4 | 4 | 0 | 0 | 3h |
| 4 | New Shared Components | 5 | 5 | 0 | 0 | 3h |
| 5 | Custom Hooks | 2 | 2 | 0 | 0 | 1h |
| 6 | Types Extension | 1 | 0 | 1 | 0 | 1.5h |
| 7 | API Layer Extension | 1 | 0 | 1 | 0 | 2h |
| 8 | Route Restructure | 1 | 0 | 1 | 0 | 1.5h |
| 9 | Page Retheme | 5 | 0 | 5 | 0 | 3h |
| 10 | Build Verification | 0 | 0 | 0 | 0 | 30m |
| **Total** | | **24** | **11** | **11** | **1** | **~17h** |

---

## 2. Task 1: Bug Fixes & Cleanup

Fix known defects before any feature work. These are blocking or dead-code issues.

### 1.1 — Fix ActivityFeed SSE token key

**File:** `ui/src/components/ActivityFeed.tsx`  
**Line:** 49  
**Problem:** `localStorage.getItem('token')` — the app stores JWT as `access_token`, not `token`. This causes SSE to never authenticate.

```diff
- const token = localStorage.getItem('token');
+ const token = localStorage.getItem('access_token');
```

### 1.2 — Delete dead code: AppCard.tsx

**File:** `ui/src/components/AppCard.tsx` (15 lines)  
**Action:** Delete the file entirely.  
**Reason:** Not imported anywhere. Dead code. `AppsList.tsx` renders app cards inline.

### 1.3 — Note: AppForm.tsx does not exist

The earlier audit flagged `AppForm.tsx` as dead code, but it doesn't exist on disk. No action needed.

### 1.4 — Fix ForgotPassword.tsx raw axios usage

**File:** `ui/src/pages/ForgotPassword.tsx`  
**Problem:** Uses `axios.post('/v1/auth/forgot-password', ...)` directly instead of the configured `api` instance. This bypasses interceptors and the `VITE_API_BASE_URL` environment variable, causing it to fail in production (Vercel).

```diff
- import axios from 'axios';
+ import api from '../services/api';

- await axios.post('/v1/auth/forgot-password', { email });
+ await api.post('/auth/forgot-password', { email });
```

Note: The api instance already has `/v1` as baseURL, so use `/auth/forgot-password` (not `/v1/auth/...`).

### 1.5 — Evaluate `next-themes` dependency

**File:** `ui/package.json`  
**Current state:** `next-themes` v0.4.6 is installed as a dependency but never used anywhere (no `ThemeProvider` in `App.tsx` or any component).  
**Decision:** Keep it — we'll wire it up in Task 3 (DashboardLayout) for the dark mode toggle. No action now.

---

## 3. Task 2: Theme Application

Replace the Azure/corporate blue with the monkeys.com.co-inspired neutral palette. Coral (`#FF5542`) is used sparingly — only on primary CTA buttons and active indicators.

### 2.1 — Add Inter font to index.html

**File:** `ui/index.html`

Add font preconnect and stylesheet links in `<head>`:

```html
<!DOCTYPE html>
<html lang="en">

<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&family=JetBrains+Mono:wght@400;500&display=swap" rel="stylesheet">
    <title>FreeRangeNotify - Dashboard</title>
</head>

<body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
</body>

</html>
```

### 2.2 — Replace CSS variables in index.css

**File:** `ui/src/index.css`

This is the biggest single change. Replace the entire file. Key changes:

1. **Remove** Ubuntu font import and all `--azure-*` custom properties.
2. **Add** Inter-based font stack.
3. **Replace** all shadcn oklch values with the Monkeys Neutral palette.
4. **Add** `.dark` class variant for dark mode support.
5. **Keep** the `@theme inline` block updated with the new variable names.
6. **Keep** the `@layer base` styles but switch body font to Inter.

**New `index.css` (full replacement):**

```css
@import "tailwindcss";
@import "tw-animate-css";

/* ─── Monkeys Neutral Theme ─── */
/* Primary UI: neutral grayscale. Accent (coral #FF5542) used ONLY on: 
   - Primary <Button> fill
   - Active sidebar indicator
   - Destructive badge hover
   Never on backgrounds, cards, headers, or borders. */

:root {
  /* Core palette */
  --background: #FAFAFA;
  --foreground: #121212;
  --card: #FFFFFF;
  --card-foreground: #121212;
  --popover: #FFFFFF;
  --popover-foreground: #121212;

  /* Buttons & interactive */
  --primary: #121212;
  --primary-foreground: #FAFAFA;
  --secondary: #F0F0F0;
  --secondary-foreground: #121212;
  --muted: #F0F0F0;
  --muted-foreground: #6B7280;
  --accent: #FF5542;
  --accent-foreground: #FFFFFF;
  --destructive: #DC2626;
  --destructive-foreground: #FFFFFF;

  /* Borders & inputs */
  --border: #E5E5E5;
  --input: #E5E5E5;
  --ring: #121212;

  /* Semantic */
  --success: #16A34A;
  --warning: #F59E0B;

  /* Chart colors */
  --chart-1: #121212;
  --chart-2: #6B7280;
  --chart-3: #9CA3AF;
  --chart-4: #D1D5DB;
  --chart-5: #FF5542;

  /* Sidebar */
  --sidebar: #FFFFFF;
  --sidebar-foreground: #121212;
  --sidebar-primary: #121212;
  --sidebar-primary-foreground: #FAFAFA;
  --sidebar-accent: #F0F0F0;
  --sidebar-accent-foreground: #121212;
  --sidebar-border: #E5E5E5;
  --sidebar-ring: #121212;

  /* Layout */
  --radius: 0.5rem; /* 8px */

  /* Typography */
  --font-sans: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Roboto', 'Helvetica Neue', sans-serif;
  --font-mono: 'JetBrains Mono', 'Fira Code', 'Consolas', monospace;
}

/* Dark mode */
.dark {
  --background: #121212;
  --foreground: #F5F5F5;
  --card: #1E1E1E;
  --card-foreground: #F5F5F5;
  --popover: #1E1E1E;
  --popover-foreground: #F5F5F5;

  --primary: #F5F5F5;
  --primary-foreground: #121212;
  --secondary: #2C2C2C;
  --secondary-foreground: #F5F5F5;
  --muted: #2C2C2C;
  --muted-foreground: #9CA3AF;
  --accent: #FF5542;
  --accent-foreground: #FFFFFF;
  --destructive: #EF4444;
  --destructive-foreground: #FFFFFF;

  --border: #3F3F3F;
  --input: #3F3F3F;
  --ring: #F5F5F5;

  --success: #22C55E;
  --warning: #FBBF24;

  --chart-1: #F5F5F5;
  --chart-2: #9CA3AF;
  --chart-3: #6B7280;
  --chart-4: #4B5563;
  --chart-5: #FF5542;

  --sidebar: #1E1E1E;
  --sidebar-foreground: #F5F5F5;
  --sidebar-primary: #F5F5F5;
  --sidebar-primary-foreground: #121212;
  --sidebar-accent: #2C2C2C;
  --sidebar-accent-foreground: #F5F5F5;
  --sidebar-border: #3F3F3F;
  --sidebar-ring: #F5F5F5;
}

@theme inline {
  --radius-sm: calc(var(--radius) - 4px);
  --radius-md: calc(var(--radius) - 2px);
  --radius-lg: var(--radius);
  --radius-xl: calc(var(--radius) + 4px);
  --radius-2xl: calc(var(--radius) + 8px);
  --radius-3xl: calc(var(--radius) + 12px);
  --radius-4xl: calc(var(--radius) + 16px);
  --color-background: var(--background);
  --color-foreground: var(--foreground);
  --color-card: var(--card);
  --color-card-foreground: var(--card-foreground);
  --color-popover: var(--popover);
  --color-popover-foreground: var(--popover-foreground);
  --color-primary: var(--primary);
  --color-primary-foreground: var(--primary-foreground);
  --color-secondary: var(--secondary);
  --color-secondary-foreground: var(--secondary-foreground);
  --color-muted: var(--muted);
  --color-muted-foreground: var(--muted-foreground);
  --color-accent: var(--accent);
  --color-accent-foreground: var(--accent-foreground);
  --color-destructive: var(--destructive);
  --color-destructive-foreground: var(--destructive-foreground);
  --color-border: var(--border);
  --color-input: var(--input);
  --color-ring: var(--ring);
  --color-success: var(--success);
  --color-warning: var(--warning);
  --color-chart-1: var(--chart-1);
  --color-chart-2: var(--chart-2);
  --color-chart-3: var(--chart-3);
  --color-chart-4: var(--chart-4);
  --color-chart-5: var(--chart-5);
  --color-sidebar: var(--sidebar);
  --color-sidebar-foreground: var(--sidebar-foreground);
  --color-sidebar-primary: var(--sidebar-primary);
  --color-sidebar-primary-foreground: var(--sidebar-primary-foreground);
  --color-sidebar-accent: var(--sidebar-accent);
  --color-sidebar-accent-foreground: var(--sidebar-accent-foreground);
  --color-sidebar-border: var(--sidebar-border);
  --color-sidebar-ring: var(--sidebar-ring);
}

@layer base {
  * {
    @apply border-border outline-ring/50;
  }

  body {
    @apply bg-background text-foreground antialiased;
    font-family: var(--font-sans);
  }

  #root {
    @apply min-h-screen;
  }

  h1, h2, h3, h4, h5, h6 {
    @apply m-0 font-semibold text-foreground;
  }

  /* Heading scale */
  h1 { @apply text-[1.75rem] font-semibold tracking-tight; }       /* 28px */
  h2 { @apply text-[1.25rem] font-semibold tracking-[-0.01em]; }   /* 20px */
  h3 { @apply text-base font-medium; }                              /* 16px */

  a {
    @apply no-underline text-foreground transition-colors duration-100;
  }

  a:hover {
    @apply text-foreground/80;
  }

  ul {
    @apply list-none p-0;
  }

  button {
    @apply cursor-pointer font-[inherit] border-none outline-none transition-all duration-100;
  }

  button:disabled {
    @apply opacity-60 cursor-not-allowed;
  }

  /* Utility: monospace text (API keys, IDs, code) */
  .font-mono {
    font-family: var(--font-mono);
  }
}

/* Hide scrollbar for tab bars on mobile */
.scrollbar-hide {
  -ms-overflow-style: none;
  scrollbar-width: none;
}

.scrollbar-hide::-webkit-scrollbar {
  display: none;
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}
```

**What changed vs the old file:**
- Removed `@import url(...)` for Ubuntu font (Inter loaded from `<link>` in `index.html` instead — avoids render-blocking CSS import).
- Removed all `--azure-*` custom properties.
- Replaced oklch values with hex values for clarity and straightforward theming.
- Added `--accent` / `--accent-foreground` as the coral color (`#FF5542` / `#FFFFFF`).
- Added `--destructive-foreground`, `--success`, `--warning` tokens.
- Added `.dark` class with inverted palette.
- Added `--font-sans` and `--font-mono` CSS variables.
- Updated `@theme inline` to include new tokens (`--color-destructive-foreground`, `--color-success`, `--color-warning`).
- Body uses `bg-background text-foreground` instead of hardcoded `bg-gray-50 text-gray-900`.
- Links use `text-foreground` instead of `text-blue-600` (no more blue links — neutral theme).
- Added heading scale (`h1`/`h2`/`h3`) in the base layer.

---

## 4. Task 3: Layout Architecture

Create the DashboardLayout (sidebar + topbar) and AuthLayout (centered card) to replace the current flat header/footer structure.

### 3.1 — AuthLayout

**New file:** `ui/src/layouts/AuthLayout.tsx`

Purpose: Centered card layout used by Login, Register, ForgotPassword, ResetPassword pages.

```
┌──────────────────────────────────────────────┐
│                                              │
│                                              │
│              ┌──────────────┐                │
│              │  FRN Logo    │                │
│              ├──────────────┤                │
│              │              │                │
│              │  Form Card   │                │
│              │              │                │
│              └──────────────┘                │
│                                              │
│                                              │
└──────────────────────────────────────────────┘
```

**Props:**
```tsx
interface AuthLayoutProps {
  children: React.ReactNode;
}
```

**Structure:**
- Full-viewport centered flex container.
- Background: `bg-background` (resolves to `#FAFAFA`).
- Children rendered inside a `max-w-md w-full` card with border, padding, rounded corners.
- Logo (text-based "FreeRangeNotify" or icon) above the card.

### 3.2 — Sidebar

**New file:** `ui/src/components/Sidebar.tsx`

The primary navigation for authenticated users. Fixed left column on desktop, collapsible sheet on mobile.

**Navigation structure:**

```
┌─── Sidebar (240px) ──────────────┐
│                                   │
│  🔔 FreeRangeNotify               │
│                                   │
│  ─────────────────────────────    │
│                                   │
│  MAIN                             │
│  ○ Applications                   │
│                                   │
│  ADMIN                            │
│  ○ Dashboard                      │
│                                   │
│  ─────────────────────────────    │
│                                   │
│  USER                             │
│  ○ Settings (future)              │
│  ○ Docs (future)                  │
│                                   │
│  ─────────────────────────────    │
│                                   │
│  user@email.com          [Logout] │
│                                   │
└───────────────────────────────────┘
```

**Phase 1 nav items** (only items with existing pages):

| Section   | Label          | Icon              | Route          | Notes                       |
|-----------|----------------|-------------------|----------------|-----------------------------|
| MAIN      | Applications   | `LayoutGrid`      | `/apps`        | Active when `/apps*`        |
| ADMIN     | Dashboard      | `BarChart3`       | `/dashboard`   | Active when `/dashboard*`   |

The sidebar is intentionally sparse in Phase 1. Items for Workflows, Topics, Digest Rules, Audit Logs will be added in Phases 2-3 with their pages.

**Active state:** Active nav item gets a 3px left border in `border-accent` (coral) + `bg-muted` background. Text stays `text-foreground`. This is the *only* use of the coral accent in the sidebar.

**Mobile:** On screens < 768px, the sidebar is hidden. A hamburger button in the Topbar opens it as a `Sheet` (slide-in from left). This reuses the shadcn `sheet.tsx` component.

**Auth display:** Bottom section shows current user's email (from `useAuth()` context) and a logout button.

### 3.3 — Topbar

**New file:** `ui/src/components/Topbar.tsx`

Replaces the current `Header.tsx` for authenticated (dashboard) pages. The existing `Header.tsx` stays as-is for the landing page.

```
┌──────────────────────────────────────────────────────────────┐
│  [☰]  Applications > My App > Templates          [User ▾]   │
└──────────────────────────────────────────────────────────────┘
```

**Elements:**
- **Left:** Hamburger button (mobile only, toggles sidebar sheet) + breadcrumb (text-based, derived from current route path).
- **Right:** User dropdown menu — email, "Change Password" (wired later), "Logout".
- Height: `h-14` (56px). Border bottom: `border-border`.
- Background: `bg-card` (white).

**Breadcrumb logic:**
- `/apps` → "Applications"
- `/apps/:id` → "Applications > {app_name}" (app name from context or just "App Detail")
- `/dashboard` → "Dashboard"
- Chevron separator: `ChevronRight` from lucide.

### 3.4 — DashboardLayout

**New file:** `ui/src/layouts/DashboardLayout.tsx`

Wraps all authenticated routes. Composes Sidebar + Topbar + scrollable main content area.

```
┌─── Sidebar ──┬──── Topbar ────────────────────┐
│              │                                 │
│  [Nav]       ├─────────────────────────────────┤
│              │                                 │
│              │     Main Content (scrollable)   │
│              │                                 │
│              │                                 │
│              │                                 │
│              │                                 │
│  [User]      │                                 │
└──────────────┴─────────────────────────────────┘
```

**Structure (desktop):**
```tsx
<div className="flex h-screen overflow-hidden">
  <Sidebar />
  <div className="flex flex-col flex-1 overflow-hidden">
    <Topbar />
    <main className="flex-1 overflow-y-auto p-6">
      <Outlet /> {/* or children */}
    </main>
  </div>
</div>
```

**Responsive:**
- Desktop (≥768px): Sidebar visible (w-60), main content fills remaining width.
- Mobile (<768px): Sidebar hidden, toggled via Sheet. Main content full width.

**Props:**
```tsx
interface DashboardLayoutProps {
  children: React.ReactNode;
}
```

### Required shadcn components for Task 3

These should be added via `npx shadcn@latest add`:

| Component | Used by |
|-----------|---------|
| `sheet` | Sidebar (mobile slide-in) |
| `dropdown-menu` | Topbar user menu (already installed as `@radix-ui/react-dropdown-menu`) |
| `tooltip` | Sidebar item tooltips on collapsed state (future) |

---

## 5. Task 4: New Shared Components

Reusable components used across all feature pages in Phases 2-5.

### 4.1 — SkeletonTable

**New file:** `ui/src/components/SkeletonTable.tsx`

A loading placeholder for table-based pages.

**Props:**
```tsx
interface SkeletonTableProps {
  rows?: number;      // default: 5
  columns?: number;   // default: 4
}
```

**Render:**
- A table structure with `rows` rows and `columns` columns.
- Each cell renders a `div` with `bg-muted animate-pulse rounded h-4` at varying widths (header row: `h-3 w-20`, data rows: random widths 40-80%).
- Header row uses `bg-muted/50` darker placeholders.

### 4.2 — EmptyState

**New file:** `ui/src/components/EmptyState.tsx`

Shown when a list has zero items.

**Props:**
```tsx
interface EmptyStateProps {
  icon?: React.ReactNode;   // lucide icon, defaults to InboxIcon
  title: string;            // e.g. "No templates yet"
  description?: string;     // e.g. "Create your first template to start sending notifications."
  action?: {
    label: string;          // e.g. "Create Template"
    onClick: () => void;
  };
}
```

**Render:**
- Centered vertically in parent container.
- Icon in `text-muted-foreground` at 48px.
- Title as `text-lg font-medium text-foreground`.
- Description as `text-sm text-muted-foreground max-w-sm text-center`.
- Optional action button: `variant="default"` (primary dark button, NOT accent — we reserve accent for the most important CTA on a page).

### 4.3 — ConfirmDialog

**New file:** `ui/src/components/ConfirmDialog.tsx`

Reusable confirmation dialog for destructive actions (delete, remove, etc.).

**Props:**
```tsx
interface ConfirmDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;                          // e.g. "Delete Template"
  description: string;                    // e.g. "This action cannot be undone. The template and all its versions will be permanently deleted."
  confirmLabel?: string;                  // default: "Delete"
  cancelLabel?: string;                   // default: "Cancel"
  variant?: 'destructive' | 'default';   // default: 'destructive'
  loading?: boolean;
  onConfirm: () => void;
}
```

**Render:**
- Uses shadcn `Dialog` (already installed: `@radix-ui/react-dialog`).
- `DialogTitle`: title text.
- `DialogDescription`: description text.
- Two buttons: Cancel (outline variant) and Confirm (destructive variant by default: `bg-destructive text-destructive-foreground`).
- `loading` state disables the Confirm button and shows a spinner.

### 4.4 — ResourcePicker

**New file:** `ui/src/components/ResourcePicker.tsx`

The most important shared component. A searchable combobox that queries a cross-API endpoint and returns a selected item's ID.

**Props:**
```tsx
interface ResourcePickerProps<T> {
  label: string;                                 // e.g. "Template"
  value: string | null;                          // current selected ID
  onChange: (id: string | null) => void;
  fetcher: () => Promise<T[]>;                   // API call to list resources
  labelKey: keyof T;                             // field to display (e.g. 'name')
  valueKey: keyof T;                             // field to use as value (e.g. 'id')
  renderItem?: (item: T) => React.ReactNode;     // optional custom row renderer
  hint?: string;                                 // help text below the picker
  placeholder?: string;                          // default: "Select..."
  disabled?: boolean;
  required?: boolean;
  filterFn?: (item: T, search: string) => boolean; // custom client-side filter
}
```

**Behavior:**
1. On mount or first focus: call `fetcher()`, cache results in local state.
2. Show a `Popover` containing a search input + scrollable list.
3. Items filtered client-side by search text matching `item[labelKey]`.
4. Selecting an item calls `onChange(item[valueKey])`, closes the popover, shows the item's label in the trigger button.
5. Clear button (X icon) to reset to null.
6. If `fetcher` rejects: show inline error "Failed to load options".

**Dependencies:**
- `@radix-ui/react-popover` — install via `npx shadcn@latest add popover`.
- No `cmdk` needed at this stage — a simple Popover + input + list is sufficient and avoids adding a dependency. Can upgrade to cmdk later if needed.

**Usage example (Workflow step → template picker):**
```tsx
<ResourcePicker<Template>
  label="Template"
  value={step.config.template_id}
  onChange={(id) => updateStep({ ...step, config: { ...step.config, template_id: id } })}
  fetcher={() => templatesAPI.list(apiKey).then(r => r.templates)}
  labelKey="name"
  valueKey="id"
  renderItem={(t) => (
    <div className="flex items-center gap-2">
      <span>{t.name}</span>
      <span className="text-xs text-muted-foreground bg-muted px-1.5 py-0.5 rounded">{t.channel}</span>
    </div>
  )}
  hint="Select the template that defines this notification's content."
  required
/>
```

### 4.5 — JsonEditor

**New file:** `ui/src/components/JsonEditor.tsx`

A textarea with JSON syntax validation for payload/variables fields.

**Props:**
```tsx
interface JsonEditorProps {
  label?: string;
  value: string;
  onChange: (value: string) => void;
  hint?: string;
  placeholder?: string;
  rows?: number;           // default: 6
  required?: boolean;
  disabled?: boolean;
}
```

**Behavior:**
- Renders a `textarea` with `font-mono` styling.
- On blur: attempt `JSON.parse(value)`. If invalid, show a red error message below: "Invalid JSON: {error.message}".
- If valid: format the JSON with `JSON.stringify(JSON.parse(value), null, 2)` (auto-prettify on blur).
- Error state: `border-destructive` ring on the textarea.

---

## 6. Task 5: Custom Hooks

### 5.1 — useApiQuery

**New file:** `ui/src/hooks/use-api-query.ts`

A lightweight data-fetching hook. Not a full React Query replacement — just eliminates the `useState` + `useEffect` + `loading` + `error` boilerplate from every page.

```tsx
import { useState, useEffect, useCallback } from 'react';

interface UseApiQueryResult<T> {
  data: T | null;
  loading: boolean;
  error: string | null;
  refetch: () => Promise<void>;
}

export function useApiQuery<T>(
  fetcher: () => Promise<T>,
  deps: React.DependencyList = [],
  options?: { enabled?: boolean }
): UseApiQueryResult<T> {
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refetch = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const result = await fetcher();
      setData(result);
    } catch (err: any) {
      const message = err?.response?.data?.error
        || err?.response?.data?.message
        || err?.message
        || 'An unexpected error occurred';
      setError(message);
    } finally {
      setLoading(false);
    }
  }, deps); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (options?.enabled === false) {
      setLoading(false);
      return;
    }
    refetch();
  }, [refetch, options?.enabled]);

  return { data, loading, error, refetch };
}
```

**Usage:**
```tsx
const { data: templates, loading, error, refetch } = useApiQuery(
  () => templatesAPI.list(apiKey),
  [apiKey]
);
```

**Why not React Query?**
- The project already has 17 dependencies. Adding tanstack/react-query brings in 3+ more packages.
- This hook covers 90% of the use cases (fetch on mount, loading state, error, refetch).
- We don't need cache invalidation, infinite queries, or prefetching at this stage.
- If the project grows to need those features, swapping this hook's internals for React Query is straightforward — the call-site API is similar.

### 5.2 — useDebounce

**New file:** `ui/src/hooks/use-debounce.ts`

For debouncing search inputs in ResourcePicker and list filters.

```tsx
import { useState, useEffect } from 'react';

export function useDebounce<T>(value: T, delay = 300): T {
  const [debounced, setDebounced] = useState(value);

  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delay);
    return () => clearTimeout(timer);
  }, [value, delay]);

  return debounced;
}
```

---

## 7. Task 6: Types Extension

**File:** `ui/src/types/index.ts`  
**Action:** Append the following interfaces and types at the end of the file.

These cover every backend API not yet typed. Each type maps exactly to the backend's JSON response structure (as documented in the handler files).

```typescript
// ============= Workflow Types =============
export type WorkflowStepType = 'channel' | 'delay' | 'digest' | 'condition';
export type WorkflowStatus = 'draft' | 'active' | 'inactive';
export type ExecutionStatus = 'running' | 'paused' | 'completed' | 'failed' | 'cancelled';
export type StepResultStatus = 'pending' | 'running' | 'completed' | 'failed' | 'skipped';
export type ConditionOperator = 'eq' | 'neq' | 'contains' | 'gt' | 'lt' | 'exists' | 'not_read';

export interface StepCondition {
  field: string;
  operator: ConditionOperator;
  value: any;
}

export interface StepConfig {
  channel?: string;
  template_id?: string;
  provider?: string;
  duration?: string;
  digest_key?: string;
  window?: string;
  max_batch?: number;
  condition?: StepCondition;
}

export interface WorkflowStep {
  id: string;
  name: string;
  type: WorkflowStepType;
  order: number;
  config: StepConfig;
  on_success?: string;
  on_failure?: string;
  skip_if?: StepCondition;
}

export interface Workflow {
  id: string;
  app_id: string;
  environment_id?: string;
  name: string;
  description: string;
  trigger_id: string;
  steps: WorkflowStep[];
  status: WorkflowStatus;
  version: number;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface CreateWorkflowRequest {
  name: string;
  description?: string;
  trigger_id: string;
  steps: Omit<WorkflowStep, 'id'>[];
}

export interface UpdateWorkflowRequest {
  name?: string;
  description?: string;
  trigger_id?: string;
  steps?: Omit<WorkflowStep, 'id'>[];
  status?: WorkflowStatus;
}

export interface TriggerWorkflowRequest {
  trigger_id: string;
  user_id: string;
  payload?: Record<string, any>;
  transaction_id?: string;
}

export interface StepResult {
  step_id: string;
  status: StepResultStatus;
  notification_id?: string;
  digest_count?: number;
  started_at?: string;
  completed_at?: string;
  error?: string;
}

export interface WorkflowExecution {
  id: string;
  workflow_id: string;
  app_id: string;
  user_id: string;
  transaction_id?: string;
  status: ExecutionStatus;
  payload: Record<string, any>;
  step_results: Record<string, StepResult>;
  started_at: string;
  completed_at?: string;
}

// ============= Digest Rule Types =============
export type DigestRuleStatus = 'active' | 'inactive';

export interface DigestRule {
  id: string;
  app_id: string;
  environment_id?: string;
  name: string;
  digest_key: string;
  window: string;
  channel: string;
  template_id: string;
  max_batch: number;
  status: DigestRuleStatus;
  created_at: string;
  updated_at: string;
}

export interface CreateDigestRuleRequest {
  name: string;
  digest_key: string;
  window: string;
  channel: string;
  template_id: string;
  max_batch?: number;
}

export interface UpdateDigestRuleRequest {
  name?: string;
  digest_key?: string;
  window?: string;
  channel?: string;
  template_id?: string;
  max_batch?: number;
  status?: DigestRuleStatus;
}

// ============= Topic Types =============
export interface Topic {
  id: string;
  app_id: string;
  environment_id?: string;
  name: string;
  key: string;
  description?: string;
  subscriber_count?: number;
  created_at: string;
  updated_at: string;
}

export interface CreateTopicRequest {
  name: string;
  key: string;
  description?: string;
}

export interface UpdateTopicRequest {
  name?: string;
  key?: string;
  description?: string;
}

export interface TopicSubscription {
  id: string;
  topic_id: string;
  app_id: string;
  user_id: string;
  created_at: string;
}

export interface TopicSubscribersRequest {
  user_ids: string[];
}

// ============= Team / RBAC Types =============
export type TeamRole = 'owner' | 'admin' | 'editor' | 'viewer';

export interface AppMembership {
  membership_id: string;
  app_id: string;
  user_id: string;
  user_email: string;
  role: TeamRole;
  invited_by: string;
  created_at: string;
  updated_at: string;
}

export interface InviteMemberRequest {
  email: string;
  role: Exclude<TeamRole, 'owner'>; // can't invite as owner
}

export interface UpdateRoleRequest {
  role: TeamRole;
}

// ============= Audit Log Types =============
export type AuditAction = 'create' | 'update' | 'delete' | 'send';
export type ActorType = 'user' | 'api_key' | 'system';

export interface AuditLog {
  audit_id: string;
  app_id: string;
  environment_id?: string;
  actor_id: string;
  actor_type: ActorType;
  action: AuditAction;
  resource: string;
  resource_id: string;
  changes: Record<string, any>;
  ip_address?: string;
  user_agent?: string;
  created_at: string;
}

export interface AuditLogFilters {
  app_id?: string;
  actor_id?: string;
  action?: AuditAction;
  resource?: string;
  from_date?: string;
  to_date?: string;
  limit?: number;
  offset?: number;
}

// ============= Environment Types =============
export type EnvironmentName = 'development' | 'staging' | 'production';

export interface Environment {
  id: string;
  app_id: string;
  name: string;
  slug: string;
  api_key: string;
  is_default: boolean;
  created_at: string;
  updated_at: string;
}

export interface CreateEnvironmentRequest {
  name: EnvironmentName;
}

export interface PromoteEnvironmentRequest {
  source_env_id: string;
  target_env_id: string;
  resources: string[]; // e.g. ['templates', 'workflows']
}

// ============= Custom Provider Types =============
export interface CustomProvider {
  provider_id: string;
  name: string;
  channel: string;
  webhook_url: string;
  headers?: Record<string, string>;
  signing_key?: string; // only present in create response
  active: boolean;
  created_at: string;
}

export interface RegisterProviderRequest {
  name: string;
  channel: string;
  webhook_url: string;
  headers?: Record<string, string>;
}

// ============= Presence Types =============
export interface PresenceCheckInRequest {
  user_id: string;
  url: string;
}

// ============= Batch Notification Types =============
export interface BatchNotificationRequest {
  notifications: NotificationRequest[];
}

export interface CancelBatchRequest {
  batch_id: string;
}

// ============= Notification Inbox Types =============
export interface MarkReadRequest {
  notification_ids: string[];
}

export interface MarkAllReadRequest {
  user_id: string;
}

export interface BulkArchiveRequest {
  notification_ids: string[];
}

export interface SnoozeRequest {
  until: string; // ISO 8601
}

export interface UnreadCountResponse {
  user_id: string;
  count: number;
}

// ============= Template Advanced Types =============
export interface TemplateRollbackRequest {
  target_version: number;
}

export interface TemplateDiffResponse {
  from_version: number;
  to_version: number;
  changes: Record<string, { old: any; new: any }>;
}

export interface TemplateTestRequest {
  user_id: string;
  variables?: Record<string, any>;
}

export interface ContentControl {
  key: string;
  label: string;
  type: 'text' | 'textarea' | 'url' | 'color' | 'image' | 'number' | 'boolean' | 'select';
  default?: any;
  placeholder?: string;
  help_text?: string;
  group?: string;
  options?: string[]; // for 'select' type
}

export interface TemplateControlsResponse {
  controls: ContentControl[];
  values: Record<string, any>;
}

export interface UpdateControlsRequest {
  control_values: Record<string, any>;
}

// ============= User Advanced Types =============
export interface BulkCreateUsersRequest {
  users: CreateUserRequest[];
}

export interface SubscriberHashResponse {
  user_id: string;
  subscriber_hash: string;
}
```

---

## 8. Task 7: API Layer Extension

**File:** `ui/src/services/api.ts`  
**Action:** Add all 56 missing endpoint functions. Keep the existing functions untouched.

First, add the new type imports at the top of the file:

```typescript
import type {
  // ... existing imports stay ...
  Workflow, CreateWorkflowRequest, UpdateWorkflowRequest, TriggerWorkflowRequest, WorkflowExecution,
  DigestRule, CreateDigestRuleRequest, UpdateDigestRuleRequest,
  Topic, CreateTopicRequest, UpdateTopicRequest, TopicSubscription, TopicSubscribersRequest,
  AppMembership, InviteMemberRequest, UpdateRoleRequest,
  AuditLog, AuditLogFilters,
  Environment, CreateEnvironmentRequest, PromoteEnvironmentRequest,
  CustomProvider, RegisterProviderRequest,
  PresenceCheckInRequest,
  BatchNotificationRequest, CancelBatchRequest,
  MarkReadRequest, MarkAllReadRequest, BulkArchiveRequest, SnoozeRequest, UnreadCountResponse,
  TemplateRollbackRequest, TemplateDiffResponse, TemplateTestRequest,
  TemplateControlsResponse, UpdateControlsRequest,
  BulkCreateUsersRequest, SubscriberHashResponse,
} from '../types';
```

Then add the following API objects after the existing exports (before `export default api;`):

### 7.1 — Workflow APIs (`workflowsAPI`)

```typescript
// ============= Workflow APIs =============
interface WorkflowListResponse {
  workflows: Workflow[];
  total: number;
  limit: number;
  offset: number;
}

interface ExecutionListResponse {
  executions: WorkflowExecution[];
  total: number;
  limit: number;
  offset: number;
}

export const workflowsAPI = {
  create: async (apiKey: string, payload: CreateWorkflowRequest) => {
    const { data } = await api.post<Workflow>('/workflows/', payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  list: async (apiKey: string, limit = 20, offset = 0) => {
    const { data } = await api.get<WorkflowListResponse>(`/workflows/?limit=${limit}&offset=${offset}`, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  get: async (apiKey: string, id: string) => {
    const { data } = await api.get<Workflow>(`/workflows/${id}`, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  update: async (apiKey: string, id: string, payload: UpdateWorkflowRequest) => {
    const { data } = await api.put<Workflow>(`/workflows/${id}`, payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  delete: async (apiKey: string, id: string) => {
    await api.delete(`/workflows/${id}`, {
      headers: getAuthHeaders(apiKey)
    });
  },

  trigger: async (apiKey: string, payload: TriggerWorkflowRequest) => {
    const { data } = await api.post('/workflows/trigger', payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  listExecutions: async (apiKey: string, limit = 20, offset = 0, workflowId?: string) => {
    const params = new URLSearchParams({ limit: String(limit), offset: String(offset) });
    if (workflowId) params.set('workflow_id', workflowId);
    const { data } = await api.get<ExecutionListResponse>(`/workflows/executions?${params}`, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  getExecution: async (apiKey: string, id: string) => {
    const { data } = await api.get<WorkflowExecution>(`/workflows/executions/${id}`, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  cancelExecution: async (apiKey: string, id: string) => {
    await api.post(`/workflows/executions/${id}/cancel`, {}, {
      headers: getAuthHeaders(apiKey)
    });
  },
};
```

### 7.2 — Digest Rule APIs (`digestRulesAPI`)

```typescript
// ============= Digest Rule APIs =============
interface DigestRuleListResponse {
  rules: DigestRule[];
  total: number;
  limit: number;
  offset: number;
}

export const digestRulesAPI = {
  create: async (apiKey: string, payload: CreateDigestRuleRequest) => {
    const { data } = await api.post<DigestRule>('/digest-rules/', payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  list: async (apiKey: string, limit = 20, offset = 0) => {
    const { data } = await api.get<DigestRuleListResponse>(`/digest-rules/?limit=${limit}&offset=${offset}`, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  get: async (apiKey: string, id: string) => {
    const { data } = await api.get<DigestRule>(`/digest-rules/${id}`, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  update: async (apiKey: string, id: string, payload: UpdateDigestRuleRequest) => {
    const { data } = await api.put<DigestRule>(`/digest-rules/${id}`, payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  delete: async (apiKey: string, id: string) => {
    await api.delete(`/digest-rules/${id}`, {
      headers: getAuthHeaders(apiKey)
    });
  },
};
```

### 7.3 — Topic APIs (`topicsAPI`)

```typescript
// ============= Topic APIs =============
interface TopicListResponse {
  topics: Topic[];
  total: number;
  limit: number;
  offset: number;
}

interface SubscriberListResponse {
  subscribers: TopicSubscription[];
  total: number;
  limit: number;
  offset: number;
}

export const topicsAPI = {
  create: async (apiKey: string, payload: CreateTopicRequest) => {
    const { data } = await api.post<Topic>('/topics/', payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  list: async (apiKey: string, limit = 20, offset = 0) => {
    const { data } = await api.get<TopicListResponse>(`/topics/?limit=${limit}&offset=${offset}`, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  get: async (apiKey: string, id: string) => {
    const { data } = await api.get<Topic>(`/topics/${id}`, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  getByKey: async (apiKey: string, key: string) => {
    const { data } = await api.get<Topic>(`/topics/key/${key}`, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  update: async (apiKey: string, id: string, payload: UpdateTopicRequest) => {
    const { data } = await api.put<Topic>(`/topics/${id}`, payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  delete: async (apiKey: string, id: string) => {
    await api.delete(`/topics/${id}`, {
      headers: getAuthHeaders(apiKey)
    });
  },

  addSubscribers: async (apiKey: string, topicId: string, payload: TopicSubscribersRequest) => {
    const { data } = await api.post(`/topics/${topicId}/subscribers`, payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  removeSubscribers: async (apiKey: string, topicId: string, payload: TopicSubscribersRequest) => {
    await api.delete(`/topics/${topicId}/subscribers`, {
      headers: getAuthHeaders(apiKey),
      data: payload,
    });
  },

  getSubscribers: async (apiKey: string, topicId: string, limit = 20, offset = 0) => {
    const { data } = await api.get<SubscriberListResponse>(
      `/topics/${topicId}/subscribers?limit=${limit}&offset=${offset}`,
      { headers: getAuthHeaders(apiKey) }
    );
    return data;
  },
};
```

### 7.4 — Team APIs (`teamAPI`)

```typescript
// ============= Team / RBAC APIs =============
export const teamAPI = {
  inviteMember: async (appId: string, payload: InviteMemberRequest) => {
    const { data } = await api.post<AppMembership>(`/apps/${appId}/team/`, payload);
    return data;
  },

  listMembers: async (appId: string) => {
    const { data } = await api.get<{ members: AppMembership[] }>(`/apps/${appId}/team/`);
    return data.members;
  },

  updateRole: async (appId: string, membershipId: string, payload: UpdateRoleRequest) => {
    const { data } = await api.put<AppMembership>(`/apps/${appId}/team/${membershipId}`, payload);
    return data;
  },

  removeMember: async (appId: string, membershipId: string) => {
    await api.delete(`/apps/${appId}/team/${membershipId}`);
  },
};
```

Note: Team APIs use JWT auth (not API key), so no `getAuthHeaders(apiKey)` — the interceptor adds the JWT automatically.

### 7.5 — Audit Log APIs (`auditAPI`)

```typescript
// ============= Audit Log APIs =============
interface AuditLogListResponse {
  logs: AuditLog[];
  total: number;
  limit: number;
  offset: number;
}

export const auditAPI = {
  list: async (filters?: AuditLogFilters) => {
    const params = new URLSearchParams();
    if (filters?.app_id) params.set('app_id', filters.app_id);
    if (filters?.actor_id) params.set('actor_id', filters.actor_id);
    if (filters?.action) params.set('action', filters.action);
    if (filters?.resource) params.set('resource', filters.resource);
    if (filters?.from_date) params.set('from_date', filters.from_date);
    if (filters?.to_date) params.set('to_date', filters.to_date);
    if (filters?.limit) params.set('limit', String(filters.limit));
    if (filters?.offset) params.set('offset', String(filters.offset));

    const { data } = await api.get<AuditLogListResponse>(`/admin/audit/?${params}`);
    return data;
  },

  get: async (id: string) => {
    const { data } = await api.get<AuditLog>(`/admin/audit/${id}`);
    return data;
  },
};
```

### 7.6 — Environment APIs (`environmentsAPI`)

```typescript
// ============= Environment APIs =============
export const environmentsAPI = {
  create: async (appId: string, payload: CreateEnvironmentRequest) => {
    const { data } = await api.post<Environment>(`/apps/${appId}/environments`, payload);
    return data;
  },

  list: async (appId: string) => {
    const { data } = await api.get<{ environments: Environment[] }>(`/apps/${appId}/environments`);
    return data.environments;
  },

  get: async (appId: string, envId: string) => {
    const { data } = await api.get<Environment>(`/apps/${appId}/environments/${envId}`);
    return data;
  },

  delete: async (appId: string, envId: string) => {
    await api.delete(`/apps/${appId}/environments/${envId}`);
  },

  promote: async (appId: string, payload: PromoteEnvironmentRequest) => {
    const { data } = await api.post(`/apps/${appId}/environments/promote`, payload);
    return data;
  },
};
```

### 7.7 — Custom Provider APIs (`providersAPI`)

```typescript
// ============= Custom Provider APIs =============
export const providersAPI = {
  register: async (appId: string, payload: RegisterProviderRequest) => {
    const { data } = await api.post<CustomProvider>(`/apps/${appId}/providers`, payload);
    return data;
  },

  list: async (appId: string) => {
    const { data } = await api.get<{ providers: CustomProvider[] }>(`/apps/${appId}/providers`);
    return data.providers;
  },

  remove: async (appId: string, providerId: string) => {
    await api.delete(`/apps/${appId}/providers/${providerId}`);
  },
};
```

### 7.8 — Presence API (`presenceAPI`)

```typescript
// ============= Presence API =============
export const presenceAPI = {
  checkIn: async (apiKey: string, payload: PresenceCheckInRequest) => {
    const { data } = await api.post('/presence/check-in', payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },
};
```

### 7.9 — Extended Notification APIs

Add these methods to the existing `notificationsAPI` object:

```typescript
  // Batch
  sendBatch: async (apiKey: string, payload: BatchNotificationRequest) => {
    const { data } = await api.post('/notifications/batch', payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  cancelBatch: async (apiKey: string, payload: CancelBatchRequest) => {
    await api.delete('/notifications/batch', {
      headers: getAuthHeaders(apiKey),
      data: payload,
    });
  },

  // Inbox operations
  getUnreadCount: async (apiKey: string, userId: string) => {
    const { data } = await api.get<UnreadCountResponse>(
      `/notifications/unread/count?user_id=${userId}`,
      { headers: getAuthHeaders(apiKey) }
    );
    return data;
  },

  listUnread: async (apiKey: string, userId: string, limit = 20, offset = 0) => {
    const { data } = await api.get<NotificationListResponse>(
      `/notifications/unread?user_id=${userId}&limit=${limit}&offset=${offset}`,
      { headers: getAuthHeaders(apiKey) }
    );
    return data;
  },

  markRead: async (apiKey: string, payload: MarkReadRequest) => {
    await api.post('/notifications/read', payload, {
      headers: getAuthHeaders(apiKey)
    });
  },

  markAllRead: async (apiKey: string, payload: MarkAllReadRequest) => {
    await api.post('/notifications/read-all', payload, {
      headers: getAuthHeaders(apiKey)
    });
  },

  bulkArchive: async (apiKey: string, payload: BulkArchiveRequest) => {
    await api.patch('/notifications/bulk/archive', payload, {
      headers: getAuthHeaders(apiKey)
    });
  },

  snooze: async (apiKey: string, id: string, payload: SnoozeRequest) => {
    await api.post(`/notifications/${id}/snooze`, payload, {
      headers: getAuthHeaders(apiKey)
    });
  },

  unsnooze: async (apiKey: string, id: string) => {
    await api.post(`/notifications/${id}/unsnooze`, {}, {
      headers: getAuthHeaders(apiKey)
    });
  },
```

### 7.10 — Extended Template APIs

Add these methods to the existing `templatesAPI` object:

```typescript
  rollback: async (apiKey: string, id: string, payload: TemplateRollbackRequest) => {
    const { data } = await api.post(`/templates/${id}/rollback`, payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  diff: async (apiKey: string, id: string, fromVersion: number, toVersion: number) => {
    const { data } = await api.get<TemplateDiffResponse>(
      `/templates/${id}/diff?from=${fromVersion}&to=${toVersion}`,
      { headers: getAuthHeaders(apiKey) }
    );
    return data;
  },

  sendTest: async (apiKey: string, id: string, payload: TemplateTestRequest) => {
    const { data } = await api.post(`/templates/${id}/test`, payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  getControls: async (apiKey: string, id: string) => {
    const { data } = await api.get<TemplateControlsResponse>(`/templates/${id}/controls`, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  updateControls: async (apiKey: string, id: string, payload: UpdateControlsRequest) => {
    const { data } = await api.put(`/templates/${id}/controls`, payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  getVersion: async (apiKey: string, appId: string, templateName: string, version: number) => {
    const { data } = await api.get<TemplateVersion>(
      `/templates/${appId}/${templateName}/versions/${version}`,
      { headers: getAuthHeaders(apiKey) }
    );
    return data;
  },
```

### 7.11 — Extended User APIs

Add these to the existing `usersAPI` object:

```typescript
  bulkCreate: async (apiKey: string, payload: BulkCreateUsersRequest) => {
    const { data } = await api.post('/users/bulk', payload, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },

  getSubscriberHash: async (apiKey: string, userId: string) => {
    const { data } = await api.get<SubscriberHashResponse>(`/users/${userId}/subscriber-hash`, {
      headers: getAuthHeaders(apiKey)
    });
    return data;
  },
```

### 7.12 — Extended Auth API (change-password)

Add to the existing admin section or create a new `authAPI` export. Since the current `api.ts` has auth in `AuthContext.tsx`, add a standalone export:

```typescript
// ============= Auth Extended APIs =============
export const authExtendedAPI = {
  changePassword: async (payload: { old_password: string; new_password: string }) => {
    await api.post('/admin/change-password', payload);
  },
};
```

### Summary of additions to `api.ts`

| Export | Methods | Count |
|--------|---------|-------|
| `workflowsAPI` | create, list, get, update, delete, trigger, listExecutions, getExecution, cancelExecution | 9 |
| `digestRulesAPI` | create, list, get, update, delete | 5 |
| `topicsAPI` | create, list, get, getByKey, update, delete, addSubscribers, removeSubscribers, getSubscribers | 9 |
| `teamAPI` | inviteMember, listMembers, updateRole, removeMember | 4 |
| `auditAPI` *(extend existing)* | list, get | 2 |
| `environmentsAPI` | create, list, get, delete, promote | 5 |
| `providersAPI` | register, list, remove | 3 |
| `presenceAPI` | checkIn | 1 |
| `authExtendedAPI` | changePassword | 1 |
| `notificationsAPI` *(extend)* | sendBatch, cancelBatch, getUnreadCount, listUnread, markRead, markAllRead, bulkArchive, snooze, unsnooze | 9 |
| `templatesAPI` *(extend)* | rollback, diff, sendTest, getControls, updateControls, getVersion | 6 |
| `usersAPI` *(extend)* | bulkCreate, getSubscriberHash | 2 |
| **Total** | | **56** |

---

## 9. Task 8: Route Restructure & Lazy Loading

**File:** `ui/src/App.tsx`

Replace the current eager-import router with lazy-loaded routes and the new layout wrappers.

### Current problems:
1. All page components are eagerly imported — every page is in the initial bundle.
2. The `Header` and `Footer` are rendered inside a `Routes` wrapper, which is awkward and causes nested `<Routes>`.
3. No layout abstraction — layout logic mixed with routing.

### New structure:

```tsx
import React, { Suspense, lazy } from 'react';
import { BrowserRouter as Router, Route, Routes, Navigate } from 'react-router-dom';
import { AuthProvider } from './contexts/AuthContext';
import ProtectedRoute from './components/ProtectedRoute';
import AuthLayout from './layouts/AuthLayout';
import DashboardLayout from './layouts/DashboardLayout';
import { Toaster } from './components/ui/sonner';
import './index.css';

// Lazy-loaded pages
const LandingPage = lazy(() => import('./pages/LandingPage'));
const Login = lazy(() => import('./pages/Login'));
const Register = lazy(() => import('./pages/Register'));
const ForgotPassword = lazy(() => import('./pages/ForgotPassword'));
const ResetPassword = lazy(() => import('./pages/ResetPassword'));
const SSOCallback = lazy(() => import('./pages/SSOCallback'));
const AppsList = lazy(() => import('./pages/AppsList'));
const AppDetail = lazy(() => import('./pages/AppDetail'));
const Dashboard = lazy(() => import('./pages/Dashboard'));

// Loading fallback (shown while lazy chunk loads)
const PageLoader = () => (
  <div className="flex items-center justify-center h-screen">
    <div className="h-8 w-8 animate-spin rounded-full border-4 border-muted border-t-foreground" />
  </div>
);

const App: React.FC = () => {
  return (
    <Router>
      <AuthProvider>
        <Suspense fallback={<PageLoader />}>
          <Routes>
            {/* Landing page — standalone, no layout wrapper */}
            <Route path="/" element={<LandingPage />} />

            {/* Auth routes — centered card layout */}
            <Route element={<AuthLayout />}>
              <Route path="/login" element={<Login />} />
              <Route path="/register" element={<Register />} />
              <Route path="/forgot-password" element={<ForgotPassword />} />
              <Route path="/reset-password" element={<ResetPassword />} />
            </Route>

            {/* SSO callback — no layout */}
            <Route path="/auth/callback" element={<SSOCallback />} />

            {/* Protected dashboard routes — sidebar layout */}
            <Route element={<ProtectedRoute><DashboardLayout /></ProtectedRoute>}>
              <Route path="/apps" element={<AppsList />} />
              <Route path="/apps/:id" element={<AppDetail />} />
              <Route path="/dashboard" element={<Dashboard />} />
            </Route>

            {/* Catch-all redirect */}
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </Suspense>
        <Toaster />
      </AuthProvider>
    </Router>
  );
};

export default App;
```

### Key changes:
1. **Lazy loading:** All pages wrapped in `React.lazy()` + `Suspense`. Each page is a separate chunk.
2. **Layout routes:** `AuthLayout` and `DashboardLayout` are *layout routes* (they use `<Outlet />`).
3. **ProtectedRoute wraps DashboardLayout:** One wrapper instead of per-route.
4. **No nested `<Routes>`:** Flat route structure, cleaner.
5. **LandingPage:** Standalone — has its own Header/Footer (imported inside the component, not from a layout).
6. **PageLoader:** Minimal spinner while chunks load.

### Impact on existing components:

- **`Header.tsx`:** Stays as-is. Only used inside `LandingPage.tsx` now (imported there directly). The dashboard uses `Topbar.tsx` instead.
- **`Footer.tsx`:** Stays as-is. Only used inside `LandingPage.tsx`.
- **`ProtectedRoute.tsx`:** May need a minor tweak. Currently it wraps `children`. It needs to also work as a layout route (render `<Outlet />` when no children passed, or wrap `children` when given). Check current implementation and adjust.
- **`DashboardLayout.tsx`:** Uses `<Outlet />` from react-router-dom to render child routes.
- **`AuthLayout.tsx`:** Uses `<Outlet />` to render the auth form.

### ProtectedRoute adjustment:

Current implementation likely looks like:
```tsx
const ProtectedRoute = ({ children }) => {
  const { isAuthenticated } = useAuth();
  if (!isAuthenticated) return <Navigate to="/login" />;
  return <>{children}</>;
};
```

Update to support both patterns:
```tsx
import { Outlet } from 'react-router-dom';

const ProtectedRoute = ({ children }: { children?: React.ReactNode }) => {
  const { isAuthenticated, loading } = useAuth();

  if (loading) return <PageLoader />;
  if (!isAuthenticated) return <Navigate to="/login" replace />;

  return children ? <>{children}</> : <Outlet />;
};
```

---

## 10. Task 9: Page Retheme

Update existing pages to use the new design tokens instead of hardcoded colors.

### 9.1 — LandingPage.tsx

| What | Before | After |
|------|--------|-------|
| Hero gradient | `from-blue-600 to-blue-800` | `bg-foreground` (solid dark, clean) |
| Hero text | `text-white` | `text-background` (inverts to white on dark bg) |
| CTA button | `bg-white text-blue-600` | `bg-accent text-accent-foreground` (coral — this is the ONE use of coral on the page) |
| Secondary button | `border-white text-white` | `border-background/30 text-background` |
| Feature cards | `bg-white shadow-md` | `bg-card border border-border` (no shadow by default) |
| Feature card icons | `text-blue-600` | `text-foreground` |
| Section backgrounds | `bg-gray-50` | `bg-background` |
| Showcase cards | `border-blue-200` | `border-border` |
| Footer | Inline in component | Import existing `Footer.tsx` component |
| Header | Comes from layout wrapper | Import `Header.tsx` directly inside LandingPage (since it's no longer in a layout) |

### 9.2 — Login.tsx

| What | Before | After |
|------|--------|-------|
| Container | Full page with custom bg | Just the form card (AuthLayout handles centering + bg) |
| Card | `bg-white shadow-lg rounded-lg` | `bg-card border border-border rounded-lg` |
| Submit button | `bg-blue-600 hover:bg-blue-700` | `bg-primary text-primary-foreground` (dark button, NOT coral — login is not a primary sales CTA) |
| SSO button | `border-blue-600 text-blue-600` | `border-border text-foreground` (outline variant) |
| Links | `text-blue-600` | `text-muted-foreground hover:text-foreground` |
| Input focus ring | `focus:ring-blue-500` | `focus:ring-ring` (neutral) |

### 9.3 — Register.tsx

Same changes as Login.tsx. Plus:
- The submit button can use `bg-accent text-accent-foreground` here since "Create Account" is a conversion CTA. This is a judgment call — keep it `bg-primary` if we want consistency with Login. **Recommendation: keep `bg-primary`** for all auth forms — save coral for the main app.

### 9.4 — ForgotPassword.tsx

Same card retheme as Login. Additionally fix the raw axios import (Task 1.4).

### 9.5 — ResetPassword.tsx

Same card retheme as Login.

### 9.6 — Header.tsx (for landing page)

| What | Before | After |
|------|--------|-------|
| Background | `bg-blue-600` | `bg-foreground` (dark neutral) |
| Text | `text-white` | `text-background` |
| Active link | `bg-white/20` | `bg-background/10` |
| Logo text | White | `text-background` |
| Mobile menu | Blue variants | Dark neutral variants |
| CTA buttons (Login/Signup) | `bg-white text-blue-600` | Login: `text-background` (ghost), Signup: `bg-accent text-accent-foreground` (coral — CTA) |

### General replacements across all pages:

| Pattern | Replace with |
|---------|-------------|
| `bg-blue-600` | `bg-primary` or `bg-accent` (depending on intent) |
| `text-blue-600` | `text-foreground` or `text-accent` |
| `hover:bg-blue-700` | `hover:bg-primary/90` or `hover:bg-accent/90` |
| `border-blue-*` | `border-border` |
| `bg-gray-50` | `bg-background` |
| `bg-gray-100` | `bg-muted` |
| `text-gray-900` | `text-foreground` |
| `text-gray-500` / `text-gray-600` | `text-muted-foreground` |
| `bg-white` | `bg-card` |
| `shadow-md` / `shadow-lg` | Remove or use `shadow-sm` on hover only |

---

## 11. Task 10: Build Verification

After all changes, verify the build passes clean.

```bash
cd ui
npm install          # if any new deps added (popover, sheet, skeleton, command)
npm run build        # must complete with 0 errors, 0 warnings
npm run preview      # manual visual check
```

### Checklist:

- [ ] `npm run build` exits with code 0
- [ ] No TypeScript errors
- [ ] No unused import warnings
- [ ] Landing page renders with new theme
- [ ] Login page renders centered in AuthLayout
- [ ] Protected routes redirect to login when unauthenticated
- [ ] After login, sidebar + topbar layout renders
- [ ] Apps list page accessible at `/apps`
- [ ] App detail page accessible at `/apps/:id`
- [ ] Dashboard page accessible at `/dashboard`
- [ ] Mobile view: sidebar hidden, hamburger opens sheet
- [ ] No console errors

---

## 12. Dependency Order

Tasks must be done in this order due to file dependencies:

```
Task 1 (Bug Fixes)          ─── independent, do first
   │
   ▼
Task 2 (Theme: index.css + index.html)  ─── independent of layout
   │
   ▼
Task 3 (Layout: AuthLayout + DashboardLayout + Sidebar + Topbar)
   │    └── requires: shadcn components (sheet, etc.) installed via npm
   │
   ├── Task 4 (Shared Components) ─── can run in parallel with Task 3
   │    └── requires: popover installed
   │
   ├── Task 5 (Hooks) ─── can run in parallel with Task 3
   │
   ├── Task 6 (Types) ─── can run in parallel with Task 3
   │
   └── Task 7 (API Layer) ─── depends on Task 6 (types must exist first)
          │
          ▼
       Task 8 (Route Restructure) ─── depends on Task 3 (layouts must exist)
          │
          ▼
       Task 9 (Page Retheme) ─── depends on Task 2, 3, 8
          │
          ▼
       Task 10 (Build Verification) ─── depends on everything
```

**Parallelizable groups:**
- Group A (parallel): Tasks 4, 5, 6 — after Task 1+2 are done
- Group B (parallel): Tasks 3, 7 — Task 7 after Task 6

---

## 13. Acceptance Criteria

Phase 1 is complete when ALL of the following are true:

### Theme
- [ ] Inter font loads on all pages (no Ubuntu or system fallback visible)
- [ ] Background is `#FAFAFA` (not `#f3f3f3` or white)
- [ ] No blue (`#0078d4`, `bg-blue-*`, `text-blue-*`) appears anywhere in the UI except as chart color or link visited state
- [ ] Coral (`#FF5542`) appears ONLY on: primary CTA buttons, primary action buttons, active sidebar indicator. Nowhere else.
- [ ] Cards have border, no drop shadow (except `shadow-sm` on hover)
- [ ] Dark mode class (`.dark`) CSS variables are defined (toggle UI wired in Phase 4)

### Layout
- [ ] Authenticated pages show sidebar (left) + topbar (top) + content (center)
- [ ] Landing page does NOT show sidebar — has its own header/footer
- [ ] Auth pages (login/register/forgot/reset) show centered card layout
- [ ] On mobile (<768px), sidebar is hidden; hamburger in topbar opens a slide-in sheet
- [ ] Sidebar shows: Applications, Dashboard. Active item has coral left border indicator.
- [ ] Topbar shows: breadcrumb (left), user dropdown (right)

### API & Types
- [ ] `api.ts` exports 12 API objects with 56 new methods (total ~91 methods across all exports)
- [ ] All 56 methods have correct route paths matching `routes.go`
- [ ] All 56 methods pass the API key or JWT correctly (API-key endpoints use `getAuthHeaders`, JWT endpoints use the interceptor)
- [ ] `types/index.ts` has interfaces for every request/response shape used by the new methods
- [ ] No `any` types except where the backend genuinely returns `Record<string, any>` (like `payload`, `changes`, `data`)

### Shared Components
- [ ] `SkeletonTable` renders a pulsing table placeholder
- [ ] `EmptyState` renders icon + title + description + optional CTA button
- [ ] `ConfirmDialog` opens a modal with cancel/confirm buttons, supports loading state
- [ ] `ResourcePicker` fetches items, shows searchable list, returns selected ID
- [ ] `JsonEditor` validates JSON on blur, auto-formats, shows error for invalid JSON

### Hooks
- [ ] `useApiQuery` returns `{ data, loading, error, refetch }` and calls fetcher on mount
- [ ] `useDebounce` returns debounced value after configurable delay

### Bugs Fixed
- [ ] `ActivityFeed.tsx` SSE connects with correct token key
- [ ] `AppCard.tsx` deleted
- [ ] `ForgotPassword.tsx` uses configured api instance, not raw axios

### Build
- [ ] `npm run build` succeeds with 0 errors, 0 warnings
- [ ] No console errors on any page at runtime

---

*Once this Phase 1 is reviewed and approved, we proceed to Phase 2 (Core API Surfaces) which builds the Workflow, Digest Rule, Topic pages and extends the notification/template pages with their remaining features.*
