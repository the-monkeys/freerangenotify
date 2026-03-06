import React, { useState, useEffect, useRef, useCallback } from 'react';

// ============= Types =============

export interface NotificationItem {
  notification_id: string;
  title: string;
  body: string;
  channel?: string;
  status?: string;
  created_at?: string;
  data?: Record<string, unknown>;
}

export interface NotificationBellProps {
  /** Internal UUID or external_id of the user. When a token is provided, the server resolves external_id to the internal UUID automatically. */
  userId: string;
  /** Base URL of the FreeRangeNotify API. Defaults to window.location.origin. */
  apiBaseURL?: string;
  /** API key (frn_xxx) for authenticated SSE. Passed as ?token= query param. */
  apiKey?: string;
  /** Called whenever a new notification arrives via SSE. */
  onNotification?: (notification: NotificationItem) => void;
  /** Max notifications to keep in the dropdown. Default: 50. */
  maxItems?: number;
  /** Custom className applied to the root container. */
  className?: string;
  /** Custom bell icon. Defaults to a bell emoji. */
  bellIcon?: React.ReactNode;
}

// ============= Styles =============

const styles: Record<string, React.CSSProperties> = {
  root: {
    position: 'relative',
    display: 'inline-block',
    fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
  },
  button: {
    position: 'relative',
    background: 'none',
    border: '1px solid #e2e8f0',
    borderRadius: 8,
    padding: '8px 10px',
    cursor: 'pointer',
    fontSize: 20,
    lineHeight: 1,
    transition: 'background 0.15s',
  },
  badge: {
    position: 'absolute' as const,
    top: -4,
    right: -4,
    background: '#ef4444',
    color: '#fff',
    borderRadius: '50%',
    width: 18,
    height: 18,
    fontSize: 11,
    fontWeight: 700,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    lineHeight: 1,
  },
  dropdown: {
    position: 'absolute' as const,
    right: 0,
    top: 'calc(100% + 6px)',
    width: 340,
    maxHeight: 420,
    overflowY: 'auto' as const,
    background: '#fff',
    border: '1px solid #e2e8f0',
    borderRadius: 10,
    boxShadow: '0 4px 16px rgba(0,0,0,0.12)',
    zIndex: 9999,
  },
  header: {
    padding: '12px 16px',
    borderBottom: '1px solid #f1f5f9',
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
  },
  headerTitle: {
    fontWeight: 600,
    fontSize: 14,
    color: '#1e293b',
  },
  markRead: {
    fontSize: 12,
    color: '#3b82f6',
    background: 'none',
    border: 'none',
    cursor: 'pointer',
  },
  empty: {
    padding: 24,
    textAlign: 'center' as const,
    color: '#94a3b8',
    fontSize: 13,
  },
  item: {
    padding: '12px 16px',
    borderBottom: '1px solid #f8fafc',
    cursor: 'default',
  },
  itemTitle: {
    fontWeight: 600,
    fontSize: 13,
    color: '#1e293b',
    marginBottom: 2,
  },
  itemBody: {
    fontSize: 12,
    color: '#64748b',
    margin: '4px 0 0',
    lineHeight: 1.4,
  },
  itemTime: {
    fontSize: 11,
    color: '#94a3b8',
    marginTop: 4,
  },
  connectionDot: {
    width: 6,
    height: 6,
    borderRadius: '50%',
    display: 'inline-block',
    marginRight: 4,
  },
};

// ============= Component =============

/**
 * NotificationBell — Drop-in notification bell that connects to FreeRangeNotify via SSE.
 *
 * Usage:
 * ```tsx
 * <NotificationBell userId="uuid-from-user-creation" />
 * <NotificationBell userId="uuid" apiKey="frn_xxx" apiBaseURL="https://notify.example.com" />
 * ```
 */
export function NotificationBell({
  userId,
  apiBaseURL,
  apiKey,
  onNotification,
  maxItems = 50,
  className,
  bellIcon,
}: NotificationBellProps) {
  const [unreadCount, setUnreadCount] = useState(0);
  const [notifications, setNotifications] = useState<NotificationItem[]>([]);
  const [open, setOpen] = useState(false);
  const [connected, setConnected] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);

  const baseURL = apiBaseURL || (typeof window !== 'undefined' ? window.location.origin : '');

  // Connect to SSE
  useEffect(() => {
    if (!userId || !baseURL) return;

    const url = `${baseURL}/v1/sse?user_id=${encodeURIComponent(userId)}${apiKey ? `&token=${encodeURIComponent(apiKey)}` : ''}`;
    const es = new EventSource(url);

    es.addEventListener('connected', () => {
      setConnected(true);
    });

    es.addEventListener('notification', (e) => {
      try {
        const notif: NotificationItem = JSON.parse(e.data);
        setNotifications((prev) => [notif, ...prev].slice(0, maxItems));
        setUnreadCount((prev) => prev + 1);
        onNotification?.(notif);
      } catch {
        // ignore malformed
      }
    });

    es.onerror = () => {
      setConnected(false);
    };

    es.onopen = () => {
      setConnected(true);
    };

    return () => {
      es.close();
      setConnected(false);
    };
  }, [userId, baseURL, apiKey, maxItems, onNotification]);

  // Close dropdown on outside click
  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, []);

  const handleToggle = useCallback(() => {
    setOpen((prev) => !prev);
  }, []);

  const handleMarkAllRead = useCallback(() => {
    setUnreadCount(0);
  }, []);

  const formatTime = (ts?: string) => {
    if (!ts) return '';
    const d = new Date(ts);
    const now = new Date();
    const diffMs = now.getTime() - d.getTime();
    const diffMin = Math.floor(diffMs / 60000);
    if (diffMin < 1) return 'Just now';
    if (diffMin < 60) return `${diffMin}m ago`;
    const diffHr = Math.floor(diffMin / 60);
    if (diffHr < 24) return `${diffHr}h ago`;
    return d.toLocaleDateString();
  };

  return (
    <div ref={dropdownRef} style={styles.root} className={className}>
      <button onClick={handleToggle} style={styles.button} aria-label="Notifications">
        {bellIcon ?? '🔔'}
        {unreadCount > 0 && (
          <span style={styles.badge}>
            {unreadCount > 9 ? '9+' : unreadCount}
          </span>
        )}
      </button>

      {open && (
        <div style={styles.dropdown}>
          <div style={styles.header}>
            <span style={styles.headerTitle}>
              <span
                style={{
                  ...styles.connectionDot,
                  background: connected ? '#22c55e' : '#ef4444',
                }}
              />
              Notifications
            </span>
            {unreadCount > 0 && (
              <button style={styles.markRead} onClick={handleMarkAllRead}>
                Mark all read
              </button>
            )}
          </div>

          {notifications.length === 0 ? (
            <p style={styles.empty}>No notifications yet</p>
          ) : (
            notifications.map((n, i) => (
              <div key={n.notification_id || i} style={styles.item}>
                <div style={styles.itemTitle}>{n.title || 'Notification'}</div>
                <p style={styles.itemBody}>{n.body}</p>
                <div style={styles.itemTime}>{formatTime(n.created_at)}</div>
              </div>
            ))
          )}
        </div>
      )}
    </div>
  );
}

export default NotificationBell;
