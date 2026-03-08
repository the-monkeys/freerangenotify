# Phase 6 — Remaining Gaps Implementation Plan

> **Date:** March 7, 2026  
> **Prerequisite:** Phases 1-5 complete. `tsc --noEmit` and `vite build` both pass cleanly.  
> **Scope:** Wire 5 unwired notification inbox APIs, improve mobile responsiveness, add code syntax highlighting, and fill documentation gaps.

---

## Overview

The Phase 1-5 audit found **10 remaining items** split into two tiers. This plan covers all of them.

| Tier | Item Count | Effort | Character |
|------|-----------|--------|-----------|
| **Tier 1** — Functional Gaps | 5 | ~1.5 hours | Missing API-to-UI wiring for notification inbox ops |
| **Tier 2** — Polish | 5 | ~2 hours | Mobile layout, syntax highlighting, docs, field hints |

---

## Tier 1 — Functional Gaps

### Task 1: Unread Count Badge on Notifications Tab

**File:** `pages/AppDetail.tsx`  
**Lines:** ~179-191 (tab rendering loop)

**What:** Add a badge showing unread notification count next to the "Notifications" tab label.

**Implementation:**
1. Add state: `const [unreadCount, setUnreadCount] = useState<number>(0)`
2. Add effect: When `app` loads and `apiKey` is available, call `notificationsAPI.getUnreadCount(apiKey, '')` (empty user_id returns total). Set `unreadCount` from the response. Re-fetch when `activeTab` changes away from `notifications`.
3. In the tab rendering loop, when `tab === 'notifications'` and `unreadCount > 0`, render a small `<Badge variant="destructive" className="ml-1.5 text-[10px] px-1.5 py-0 h-4 min-w-4">{unreadCount}</Badge>` after the label text.

**Note:** The `getUnreadCount` API requires a `userId`. If the endpoint supports app-wide counts without a user filter, use that. Otherwise, skip the badge or show it only when a user filter is active in AppNotifications.

**Alternative approach (simpler):** Instead of calling the API from AppDetail, have `AppNotifications.tsx` expose the count via a callback. Add `onUnreadCount?: (count: number) => void` prop to `AppNotifications`. Call `getUnreadCount` inside `useNotificationData` and invoke the callback. AppDetail receives it and renders the badge.

---

### Task 2: Mark All Read Button

**File:** `components/AppNotifications.tsx`  
**Lines:** ~858-873 (bulk actions bar) and ~296-307 (toolbar)

**What:** Add a "Mark All Read" button that marks all notifications for a given user as read in one API call.

**Implementation:**
1. Add handler:
   ```ts
   const handleMarkAllRead = async () => {
     if (!selectedUserId) {
       toast.error('Select a user to mark all notifications as read');
       return;
     }
     try {
       await notificationsAPI.markAllRead(apiKey, { user_id: selectedUserId });
       toast.success('All notifications marked as read');
       refetch();
       setSelectedIds(new Set());
     } catch (err: any) {
       toast.error(err.message || 'Failed to mark all as read');
     }
   };
   ```
2. Add a "Mark All Read" button in the toolbar area (lines ~296-307), next to the existing "Send Notification" button. Use outline variant with `CheckSquare` icon.
3. The button should be disabled when no user filter is active (since `markAllRead` requires a `user_id`).
4. Show a brief confirm toast or use `ConfirmDialog` before executing — this is a destructive bulk action.

**Dependency:** The user filter in the notifications list. If there's no user filter UI, add a user selector dropdown in the filter bar that sets `selectedUserId`.

---

### Task 3: Send Batch & Cancel Batch Dialogs

**File:** `components/AppNotifications.tsx`

**What:** Add UI for sending notifications in batch and canceling a batch by ID.

**Implementation — Send Batch:**
1. Add a "Send Batch" button to the toolbar (dropdown menu or secondary button alongside "Send Notification").
2. On click, open a `Dialog` with:
   - A JSON editor (`JsonEditor` component) pre-filled with an array template:
     ```json
     {
       "notifications": [
         { "user_id": "", "template_id": "", "payload": {} }
       ]
     }
     ```
   - Hint text: "Send multiple notifications in a single request. Each entry needs at minimum a user_id."
   - Submit calls `notificationsAPI.sendBatch(apiKey, parsedPayload)`.
   - On success: toast with count, refetch list.

**Implementation — Cancel Batch:**
1. Add a "Cancel Batch" option in the toolbar dropdown menu.
2. On click, open a `Dialog` with:
   - A single `Input` for `batch_id` with label "Batch ID" and hint "The batch ID returned when you sent a batch notification."
   - Submit calls `notificationsAPI.cancelBatch(apiKey, { batch_id })`.
   - On success: toast confirmation, refetch list.

**Both dialogs** use the existing `Dialog` + `DialogContent` pattern from shadcn/ui.

---

### Task 4: Unsnooze Button on Snoozed Notifications

**File:** `components/AppNotifications.tsx`  
**Lines:** ~282-290 (status badge), ~924 (snooze action), ~1037-1044 (detail panel snooze buttons)

**What:** Show a visual indicator when a notification is snoozed, and provide an "Unsnooze" button.

**Implementation:**
1. **Status badge:** Add `snoozed` case to `getStatusBadgeClass`:
   ```ts
   case 'snoozed': return 'bg-purple-100 text-purple-700 border-purple-300';
   ```
2. **Table row indicator:** In the table row rendering, if `notification.status === 'snoozed'` or `notification.snoozed_until`, show a small `BellOff` icon with a tooltip showing the snooze-until time.
3. **Unsnooze handler:**
   ```ts
   const handleUnsnooze = async (notifId: string) => {
     try {
       await notificationsAPI.unsnooze(apiKey, notifId);
       toast.success('Notification unsnoozed');
       refetch();
     } catch (err: any) {
       toast.error(err.message || 'Failed to unsnooze');
     }
   };
   ```
4. **Table row action:** Next to the existing snooze `BellOff` button, conditionally show an "Unsnooze" button (e.g., `Bell` icon) when `notification.status === 'snoozed'`. Replace the snooze button with unsnooze when already snoozed.
5. **Detail panel:** In the notification detail slide panel (lines ~1037-1044), show the snoozed-until time if present, and add an "Unsnooze" button that calls `handleUnsnooze`.

---

### Task 5: Additional Status Badges

**File:** `components/AppNotifications.tsx`  
**Lines:** ~282-290

**What:** The `getStatusBadgeClass` function only handles 5 statuses. Add missing ones.

**Implementation:** Add cases:
```ts
case 'snoozed': return 'bg-purple-100 text-purple-700 border-purple-300';
case 'archived': return 'bg-gray-100 text-gray-500 border-gray-300';
case 'read': return 'bg-sky-100 text-sky-700 border-sky-300';
case 'dead_letter': return 'bg-red-200 text-red-800 border-red-400';
```

This is a small change bundled with Task 4 but covers all states the system can produce.

---

## Tier 2 — Polish

### Task 6: Code Syntax Highlighting in Docs

**File:** `pages/docs/DocsPage.tsx` (lines ~103-116)  
**New dependency:** `react-syntax-highlighter`

**What:** Replace plain monospace code blocks with language-aware syntax highlighting.

**Implementation:**
1. Install: `npm install react-syntax-highlighter @types/react-syntax-highlighter`
2. In `DocsPage.tsx`, lazy-import the highlighter:
   ```ts
   import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
   import { oneLight } from 'react-syntax-highlighter/dist/esm/styles/prism';
   ```
3. Replace the `code` component in the ReactMarkdown `components` prop:
   ```tsx
   code: ({ className, children, ...props }) => {
     const match = /language-(\w+)/.exec(className || '');
     if (match) {
       return (
         <SyntaxHighlighter
           style={oneLight}
           language={match[1]}
           PreTag="div"
           className="rounded-lg text-xs my-4 border border-border"
         >
           {String(children).replace(/\n$/, '')}
         </SyntaxHighlighter>
       );
     }
     return (
       <code className="bg-muted px-1.5 py-0.5 rounded text-xs font-mono" {...props}>
         {children}
       </code>
     );
   },
   ```
4. Remove the custom `pre` component (SyntaxHighlighter handles its own wrapping).
5. Use `oneLight` theme to match the Monkeys Neutral aesthetic (light background, no jarring colors).

---

### Task 7: Mobile Table → Card Conversion

**Files:** All major table pages — `AppNotifications.tsx`, `WorkflowsList.tsx`, `DigestRulesList.tsx`, `TopicsList.tsx`, `AuditLogsList.tsx`, `WorkflowExecutions.tsx`

**What:** On screens below 768px, convert table rows into stacked cards.

**Implementation strategy — responsive table wrapper pattern:**

1. Create `components/ResponsiveTable.tsx`:
   ```tsx
   interface Column<T> {
     key: string;
     label: string;
     render: (item: T) => React.ReactNode;
     hideOnMobile?: boolean;  // columns hidden on mobile cards
     priority?: number;       // 1 = always shown on mobile card, 2 = optional
   }
   
   // Desktop: renders a standard <Table>
   // Mobile (<768px): renders cards with label: value pairs
   ```
2. This is a rendering wrapper, not a data change. Each page wraps its existing table in `<ResponsiveTable>` and provides column definitions.
3. **Mobile card layout:**
   - Each card: `bg-card rounded-lg border p-4 mb-2`
   - Primary info (name/title, status badge) on top line
   - Secondary info (created_at, channel, etc.) in `text-xs text-muted-foreground` below
   - Actions row at bottom (same buttons, icon-only on mobile)
4. Use Tailwind `hidden md:table-cell` on desktop-only columns in the table head/body, and `md:hidden` on the card view.

**Scope decision:** This is significant layout work. Recommend applying to the 3 most-used pages first:
- `AppNotifications.tsx` (highest traffic)
- `WorkflowsList.tsx`
- `TopicsList.tsx`

The other pages can use `overflow-x-auto` with horizontal scroll as a fallback (add `<div className="overflow-x-auto">` around tables that don't have it).

---

### Task 8: Channels Documentation Gaps

**File:** `ui/src/docs/channels.md`

**What:** The plan notes Slack, Discord, WhatsApp channels. However, the backend currently supports: webhook, email/SMTP, SendGrid, APNS, FCM, SMS/Twilio, SSE. Slack/Discord/WhatsApp are not implemented as native providers — they're delivered via the **webhook** provider with custom URLs.

**Implementation:**
1. Add a section at the end of `channels.md` titled "## Custom Channels via Webhook" explaining:
   - Slack: use webhook channel with a Slack Incoming Webhook URL
   - Discord: use webhook channel with a Discord Webhook URL  
   - WhatsApp: use webhook channel pointed at a WhatsApp Business API endpoint
   - Microsoft Teams: use webhook channel with a Teams Incoming Webhook connector
2. Each entry gets a brief example showing the webhook URL format and any required headers.
3. Add an `in_app` section documenting the in-app notification channel (stored in Elasticsearch, queried via inbox APIs).

---

### Task 9: Field Hints on Notification Send Form

**File:** `components/AppNotifications.tsx`  
**Lines:** ~311-788 (send form tabs)

**What:** Add contextual help text to form fields that reference cross-API resources.

**Implementation:** Add `<p className="text-xs text-muted-foreground mt-1">` beneath each field:

| Field | Tab | Hint Text |
|-------|-----|-----------|
| "To" input (Quick Send) | Quick Send | "Enter an email address, external_id, or the internal UUID from Users." |
| User select (Advanced) | Advanced | "Select notification recipients. These are users created in the Users tab." |
| Template select | Quick Send + Advanced | "Optional. If selected, the notification content is rendered from this template's body. Otherwise, provide title and body manually below." |
| Channel select | Advanced | "The delivery channel: email, push (APNS/FCM), sms, webhook, sse, in_app." |
| Topic Key (Broadcast) | Broadcast | "All users subscribed to this topic will receive the notification." |
| Payload JSON | Advanced | "Variables for your template — e.g. {\"user_name\": \"Alice\"} maps to {{user_name}} in the template body." |

---

### Task 10: Queue Depth Sparklines (Best-Effort)

**File:** `components/dashboard/QueueDepthCards.tsx`

**What:** The plan calls for sparkline charts showing queue depth over the last hour. However, the backend API (`GET /admin/queues/stats`) only returns current snapshot counts — **no historical time-series data**.

**Implementation (client-side accumulation):**
1. Store a rolling window of snapshots in component state:
   ```ts
   const [history, setHistory] = useState<Array<{ time: number; stats: Record<string, number> }>>([]);
   ```
2. On each data refresh (poll every 30s via `setInterval`), push the current stats onto the history array, keeping only the last 60 entries (30 minutes at 30s intervals).
3. Render a simple inline SVG sparkline for each queue:
   ```tsx
   const Sparkline: React.FC<{ data: number[]; color: string }> = ({ data, color }) => {
     const max = Math.max(...data, 1);
     const width = 120;
     const height = 24;
     const points = data.map((v, i) => `${(i / (data.length - 1)) * width},${height - (v / max) * height}`).join(' ');
     return <svg width={width} height={height}><polyline points={points} fill="none" stroke={color} strokeWidth="1.5" /></svg>;
   };
   ```
4. Place each sparkline inside its queue card, below the count number.

**Limitation:** Data resets on page reload since there's no backend history. The sparkline builds up over time as the user stays on the dashboard. This is acceptable for real-time monitoring.

**Alternative:** Skip sparklines entirely and add a simple trend indicator (↑ / ↓ / →) comparing current count to the previous poll. This is simpler and arguably more useful.

---

## Build Verification

After all 10 tasks:
1. `npx tsc --noEmit` — zero errors
2. `npx vite build` — clean build, verify new chunks (syntax highlighter lazy-loaded)
3. `docker-compose build ui && docker-compose up -d --force-recreate ui` — container runs cleanly

---

## Task Dependency Order

```
Independent (can be done in any order):
├── Task 1  (Unread badge)
├── Task 2  (Mark All Read)
├── Task 3  (Batch send/cancel dialogs)
├── Task 4+5 (Unsnooze + status badges — do together)
├── Task 6  (Syntax highlighting)
├── Task 8  (Channels docs)
├── Task 9  (Field hints)
└── Task 10 (Sparklines)

Depends on Task 6:
└── (none — independent)

Layout work (do last):
└── Task 7  (Mobile responsive tables)
```

All tasks are independent. Recommended order: 4+5 → 1 → 2 → 3 → 9 → 6 → 8 → 10 → 7 → build.

---

## Summary

| # | Task | Tier | File(s) | Effort |
|---|------|------|---------|--------|
| 1 | Unread count badge on tab | T1 | AppDetail.tsx, AppNotifications.tsx | 15 min |
| 2 | Mark All Read button | T1 | AppNotifications.tsx | 15 min |
| 3 | Send Batch + Cancel Batch dialogs | T1 | AppNotifications.tsx | 25 min |
| 4 | Unsnooze button | T1 | AppNotifications.tsx | 15 min |
| 5 | Additional status badges | T1 | AppNotifications.tsx | 5 min |
| 6 | Code syntax highlighting | T2 | DocsPage.tsx, package.json | 20 min |
| 7 | Mobile table → card layout | T2 | 3-6 table pages | 45 min |
| 8 | Channels docs gaps | T2 | channels.md | 10 min |
| 9 | Field hints on send form | T2 | AppNotifications.tsx | 15 min |
| 10 | Queue depth sparklines | T2 | QueueDepthCards.tsx | 20 min |
| **Total** | | | | **~3 hours** |
