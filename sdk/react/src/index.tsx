/**
 * FreeRangeNotify React SDK
 *
 * Drop-in React components and headless hooks for the FreeRangeNotify notification service.
 *
 * Usage:
 * ```tsx
 * import { FreeRangeProvider, NotificationBell, Preferences } from '@freerangenotify/react';
 *
 * function App() {
 *   return (
 *     <FreeRangeProvider apiKey="frn_xxx" userId="user-uuid">
 *       <NotificationBell />
 *       <Preferences />
 *     </FreeRangeProvider>
 *   );
 * }
 * ```
 */

// Provider & context
export { FreeRangeProvider, useFreeRange } from './FreeRangeProvider';
export type { FreeRangeProviderProps, FreeRangeContextValue } from './FreeRangeProvider';

// Components
export { NotificationBell } from './NotificationBell';
export type { NotificationBellProps, NotificationBellTab } from './NotificationBell';

export { NotificationCenter } from './NotificationCenter';
export type { NotificationCenterProps, NotificationCenterTab } from './NotificationCenter';

export { Preferences } from './Preferences';
export type { PreferencesProps } from './Preferences';

export { ChannelToggle } from './components/ChannelToggle';
export type { ChannelToggleProps } from './components/ChannelToggle';

export { QuietHoursEditor } from './components/QuietHoursEditor';
export type { QuietHoursEditorProps } from './components/QuietHoursEditor';

// Headless hooks
export {
  useNotifications,
  usePreferences,
  useSSE,
  useUnreadCount,
} from './hooks';
export type {
  UseNotificationsOptions,
  UseNotificationsResult,
  UsePreferencesResult,
  UseSSEOptions,
  UseSSEResult,
  UseUnreadCountResult,
} from './hooks';
