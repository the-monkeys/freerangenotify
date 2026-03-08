# @freerangenotify/react

Drop-in React components and headless hooks for **FreeRangeNotify**.

Provides a context-based architecture with `FreeRangeProvider`, pre-built UI components (`NotificationBell`, `Preferences`), and headless hooks for custom UIs.

## Installation

```bash
npm install @freerangenotify/react @freerangenotify/sdk
```

## Quick Start

```tsx
import { FreeRangeProvider, NotificationBell, Preferences } from '@freerangenotify/react';

function App() {
  return (
    <FreeRangeProvider apiKey="frn_xxx" userId="user-uuid" apiBaseURL="http://localhost:8080/v1">
      <NotificationBell />
      <Preferences />
    </FreeRangeProvider>
  );
}
```

## FreeRangeProvider

Wraps children with a React context containing the initialized JS SDK client.

```tsx
<FreeRangeProvider
  apiKey="frn_xxx"              // Required
  userId="user-internal-uuid"   // Required — internal UUID from user creation
  apiBaseURL="http://localhost:8080/v1"  // Optional
  subscriberHash="hmac-hash"    // Optional — for authenticated SSE
>
  {children}
</FreeRangeProvider>
```

| Prop | Type | Required | Description |
|------|------|----------|-------------|
| `apiKey` | `string` | yes | Application API key (frn_xxx). |
| `userId` | `string` | yes | Internal UUID of the authenticated user. |
| `apiBaseURL` | `string` | no | API base URL. Default: `http://localhost:8080/v1`. |
| `subscriberHash` | `string` | no | HMAC hash for authenticated SSE. |

## Components

### NotificationBell

Notification bell with real-time SSE, category tabs, mark-read, archive, and snooze.

```tsx
<NotificationBell
  tabs={[
    { label: 'All', category: '' },
    { label: 'Alerts', category: 'alert' },
    { label: 'Updates', category: 'update' },
  ]}
  theme="light"
  onNotification={(n) => console.log(n)}
/>
```

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `maxItems` | `number` | `50` | Max notifications in the dropdown. |
| `className` | `string` | — | Custom CSS class for root container. |
| `bellIcon` | `ReactNode` | `🔔` | Custom bell icon element. |
| `tabs` | `NotificationBellTab[]` | All/Alerts/Updates | Category filter tabs. |
| `onNotification` | `(SSENotification) => void` | — | Callback on new SSE notification. |
| `theme` | `'light' \| 'dark'` | `'light'` | Visual theme. |
| `pageSize` | `number` | `20` | Page size for pagination. |

### Preferences

Channel toggles, quiet hours, and Do Not Disturb management.

```tsx
<Preferences
  theme="dark"
  onSave={(prefs) => console.log('Saved:', prefs)}
/>
```

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `theme` | `'light' \| 'dark'` | `'light'` | Visual theme. |
| `onSave` | `(Preferences) => void` | — | Callback after save. |
| `className` | `string` | — | Custom CSS class. |

### ChannelToggle

Individual channel toggle switch (used internally by Preferences, also exported).

```tsx
<ChannelToggle label="Email" enabled={true} onChange={(v) => setEnabled(v)} />
```

### QuietHoursEditor

Time range picker for quiet hours configuration.

```tsx
<QuietHoursEditor value={{ start: '22:00', end: '08:00' }} onChange={setQuietHours} />
```

## Headless Hooks

For building custom UIs without the pre-built components.

### useNotifications

```tsx
const {
  notifications,   // NotificationResponse[]
  loading,          // boolean
  unreadCount,      // number
  markRead,         // (ids: string[]) => Promise<void>
  markAllRead,      // () => Promise<void>
  archive,          // (ids: string[]) => Promise<void>
  snooze,           // (id: string, duration: string) => Promise<void>
  loadMore,         // () => Promise<void>
  hasMore,          // boolean
  refresh,          // () => Promise<void>
} = useNotifications({ category: 'alert', pageSize: 20, unreadOnly: false });
```

### usePreferences

```tsx
const {
  preferences,  // Preferences | null
  loading,      // boolean
  update,       // (prefs: Partial<Preferences>) => Promise<void>
} = usePreferences();
```

### useSSE

```tsx
const {
  connected,          // boolean
  lastNotification,   // SSENotification | null
} = useSSE({
  onNotification: (n) => console.log(n),
});
```

### useUnreadCount

```tsx
const { count, loading, refresh } = useUnreadCount();
```

### useFreeRange

Low-level context access — returns the SDK client and user info.

```tsx
const { client, userId, subscriberHash } = useFreeRange();
await client.notifications.send({ ... });
```

## File Structure

```
sdk/react/src/
├── index.tsx                         # Re-exports all components and hooks
├── FreeRangeProvider.tsx              # Context provider
├── NotificationBell.tsx              # Bell with tabs, actions, SSE
├── Preferences.tsx                   # Channel toggles, quiet hours
├── hooks.ts                          # Headless hooks
└── components/
    ├── ChannelToggle.tsx             # Toggle switch
    └── QuietHoursEditor.tsx          # Quiet hours time picker
```

## Important Notes

- The `userId` must be the **internal UUID** returned by the FreeRangeNotify API when creating a user, NOT an `external_id` or email.
- All components must be wrapped in a `<FreeRangeProvider>`.
- Send notifications using `channel: "sse"` for real-time delivery to the bell.
- Components use inline styles — no CSS framework required.

## License

MIT
