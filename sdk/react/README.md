# @freerangenotify/react

Drop-in React notification bell component for **FreeRangeNotify** using Server-Sent Events (SSE).

## Installation

```bash
npm install @freerangenotify/react
```

## Quick Start

```tsx
import { NotificationBell } from '@freerangenotify/react';

function App() {
  return (
    <NotificationBell
      userId="user-uuid-from-creation"     // Internal UUID (NOT external_id)
      apiBaseURL="http://localhost:8080"     // Optional — defaults to window.location.origin
      onNotification={(n) => console.log('New notification:', n)}
    />
  );
}
```

## Props

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `userId` | `string` | **required** | Internal UUID of the user (returned from `POST /v1/users/`). |
| `apiBaseURL` | `string` | `window.location.origin` | Base URL of the FreeRangeNotify API server. |
| `onNotification` | `(notification) => void` | — | Callback fired on each incoming SSE notification. |
| `maxItems` | `number` | `50` | Maximum notifications to keep in the dropdown list. |
| `className` | `string` | — | Custom CSS class applied to the root container. |
| `bellIcon` | `ReactNode` | `🔔` | Custom icon/element to replace the default bell emoji. |

## How It Works

1. The component opens an `EventSource` connection to `/v1/sse?user_id={userId}`.
2. Incoming `notification` events are parsed and displayed in a dropdown.
3. An unread badge appears on the bell icon.
4. The connection indicator (green/red dot) shows real-time connectivity status.

## Important Notes

- The `userId` must be the **internal UUID** returned by the FreeRangeNotify API when creating a user, NOT an `external_id` or email.
- Send notifications using `channel: "sse"` for them to appear via this component.
- The component **does not** require any CSS framework — all styles are inline.

## License

MIT
