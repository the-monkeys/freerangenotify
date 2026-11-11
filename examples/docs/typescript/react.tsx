import { FreeRangeProvider, NotificationBell, useNotifications } from '@freerangenotify/react';

// Legacy SDK usage is shown only for a trusted environment. Public browsers
// should use backend-issued SSE tokens and authorized backend CRUD requests.
export function TrustedInbox() {
 return <FreeRangeProvider apiKey="YOUR_API_KEY" userId="YOUR_USER_ID" apiBaseURL="https://notify.example.test/v1"><Inbox /></FreeRangeProvider>;
}
function Inbox() {
 const { unreadCount } = useNotifications({pageSize:20});
 return <><NotificationBell /><span>{unreadCount}</span></>;
}
