import React, { useState, useEffect, useRef, useCallback } from 'react';
import { useFreeRange } from './FreeRangeProvider';
import { useNotifications, useSSE } from './hooks';
import type { NotificationResponse, SSENotification } from '../../js/src/types';

// ── Types ──

export interface NotificationBellTab {
    label: string;
    category: string;
}

export interface NotificationBellProps {
    /** Maximum notifications to keep in the dropdown. Default: 50. */
    maxItems?: number;
    /** Custom className applied to the root container. */
    className?: string;
    /** Custom bell icon. Defaults to a bell emoji. */
    bellIcon?: React.ReactNode;
    /** Tabs for category filtering. Defaults to All / Alerts / Updates. */
    tabs?: NotificationBellTab[];
    /** Called whenever a new notification arrives via SSE. */
    onNotification?: (notification: SSENotification) => void;
    /** Visual theme. Default: 'light'. */
    theme?: 'light' | 'dark';
    /** Page size for notification list pagination. Default: 20. */
    pageSize?: number;
}

const DEFAULT_TABS: NotificationBellTab[] = [
    { label: 'All', category: '' },
    { label: 'Alerts', category: 'alert' },
    { label: 'Updates', category: 'update' },
    { label: 'Social', category: 'social' },
];

// ── Styles ──

function getBellStyles(dark: boolean): Record<string, React.CSSProperties> {
    return {
        root: {
            position: 'relative',
            display: 'inline-block',
            fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
        },
        button: {
            position: 'relative',
            background: 'none',
            border: `1px solid ${dark ? '#475569' : '#e2e8f0'}`,
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
            width: 360,
            maxHeight: 480,
            overflowY: 'auto' as const,
            background: dark ? '#1e293b' : '#fff',
            border: `1px solid ${dark ? '#334155' : '#e2e8f0'}`,
            borderRadius: 10,
            boxShadow: '0 4px 16px rgba(0,0,0,0.12)',
            zIndex: 9999,
        },
        header: {
            padding: '12px 16px',
            borderBottom: `1px solid ${dark ? '#334155' : '#f1f5f9'}`,
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'center',
        },
        headerTitle: {
            fontWeight: 600,
            fontSize: 14,
            color: dark ? '#f1f5f9' : '#1e293b',
        },
        headerActions: {
            display: 'flex',
            gap: 8,
        },
        actionBtn: {
            fontSize: 12,
            color: '#3b82f6',
            background: 'none',
            border: 'none',
            cursor: 'pointer',
            padding: 0,
        },
        tabs: {
            display: 'flex',
            borderBottom: `1px solid ${dark ? '#334155' : '#f1f5f9'}`,
            padding: '0 12px',
        },
        tab: {
            padding: '8px 12px',
            fontSize: 12,
            fontWeight: 500,
            cursor: 'pointer',
            background: 'none',
            border: 'none',
            borderBottom: '2px solid transparent',
            color: dark ? '#94a3b8' : '#64748b',
            transition: 'color 0.15s, border-color 0.15s',
        },
        tabActive: {
            color: '#3b82f6',
            borderBottomColor: '#3b82f6',
        },
        empty: {
            padding: 24,
            textAlign: 'center' as const,
            color: dark ? '#94a3b8' : '#94a3b8',
            fontSize: 13,
        },
        item: {
            padding: '12px 16px',
            borderBottom: `1px solid ${dark ? '#1e293b' : '#f8fafc'}`,
            cursor: 'default',
            display: 'flex',
            justifyContent: 'space-between',
            alignItems: 'flex-start',
        },
        itemContent: {
            flex: 1,
            minWidth: 0,
        },
        itemTitle: {
            fontWeight: 600,
            fontSize: 13,
            color: dark ? '#f1f5f9' : '#1e293b',
            marginBottom: 2,
        },
        itemBody: {
            fontSize: 12,
            color: dark ? '#94a3b8' : '#64748b',
            margin: '4px 0 0',
            lineHeight: 1.4,
        },
        itemTime: {
            fontSize: 11,
            color: '#94a3b8',
            marginTop: 4,
        },
        itemActions: {
            display: 'flex',
            gap: 4,
            marginLeft: 8,
            flexShrink: 0,
        },
        itemActionBtn: {
            fontSize: 11,
            color: '#94a3b8',
            background: 'none',
            border: 'none',
            cursor: 'pointer',
            padding: '2px 4px',
        },
        connectionDot: {
            width: 6,
            height: 6,
            borderRadius: '50%',
            display: 'inline-block',
            marginRight: 4,
        },
        loadMore: {
            padding: '10px 16px',
            textAlign: 'center' as const,
            fontSize: 12,
            color: '#3b82f6',
            cursor: 'pointer',
            background: 'none',
            border: 'none',
            width: '100%',
        },
    };
}

// ── Helpers ──

function formatTime(ts?: string): string {
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
}

// ── Component ──

/**
 * NotificationBell — Drop-in notification bell with real-time SSE updates,
 * category tabs, mark-read, archive, and snooze actions.
 *
 * Must be used within a <FreeRangeProvider>.
 *
 * ```tsx
 * <FreeRangeProvider apiKey="frn_xxx" userId="user-uuid">
 *   <NotificationBell />
 * </FreeRangeProvider>
 * ```
 */
export function NotificationBell({
    maxItems = 50,
    className,
    bellIcon,
    tabs = DEFAULT_TABS,
    onNotification,
    theme = 'light',
    pageSize = 20,
}: NotificationBellProps) {
    const dark = theme === 'dark';
    const styles = getBellStyles(dark);

    const [open, setOpen] = useState(false);
    const [activeTab, setActiveTab] = useState(0);
    const dropdownRef = useRef<HTMLDivElement>(null);

    const activeCategory = tabs[activeTab]?.category || undefined;

    const {
        notifications,
        loading,
        unreadCount,
        markRead,
        markAllRead,
        archive,
        snooze,
        loadMore,
        hasMore,
        refresh,
    } = useNotifications({
        category: activeCategory,
        pageSize,
        unreadOnly: false,
    });

    // SSE connection for real-time updates
    const { connected } = useSSE({
        onNotification: (n) => {
            onNotification?.(n);
            // Refresh list to pick up the new notification
            refresh();
        },
    });

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

    const handleTabChange = useCallback((idx: number) => {
        setActiveTab(idx);
    }, []);

    // Filter displayed notifications by maxItems
    const displayed = notifications.slice(0, maxItems);

    // Filter by category tab (if backend filtering isn't exact)
    const filtered = activeCategory
        ? displayed.filter((n) => n.category === activeCategory)
        : displayed;

    return (
        <div ref={dropdownRef} style={styles.root} className={className}>
            <button onClick={handleToggle} style={styles.button} aria-label="Notifications">
                {bellIcon ?? '🔔'}
                {unreadCount > 0 && (
                    <span style={styles.badge}>
                        {unreadCount > 99 ? '99+' : unreadCount}
                    </span>
                )}
            </button>

            {open && (
                <div style={styles.dropdown}>
                    {/* Header */}
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
                        <div style={styles.headerActions}>
                            {unreadCount > 0 && (
                                <button
                                    style={styles.actionBtn}
                                    onClick={() => markAllRead()}
                                >
                                    Mark all read
                                </button>
                            )}
                        </div>
                    </div>

                    {/* Tabs */}
                    {tabs.length > 1 && (
                        <div style={styles.tabs}>
                            {tabs.map((tab, idx) => (
                                <button
                                    key={tab.label}
                                    style={{
                                        ...styles.tab,
                                        ...(idx === activeTab ? styles.tabActive : {}),
                                    }}
                                    onClick={() => handleTabChange(idx)}
                                >
                                    {tab.label}
                                </button>
                            ))}
                        </div>
                    )}

                    {/* Notification list */}
                    {loading && filtered.length === 0 ? (
                        <p style={styles.empty}>Loading…</p>
                    ) : filtered.length === 0 ? (
                        <p style={styles.empty}>No notifications yet</p>
                    ) : (
                        <>
                            {filtered.map((n) => (
                                <NotificationItem
                                    key={n.notification_id}
                                    notification={n}
                                    styles={styles}
                                    onMarkRead={() => markRead([n.notification_id])}
                                    onArchive={() => archive([n.notification_id])}
                                    onSnooze={() => snooze(n.notification_id, '1h')}
                                />
                            ))}
                            {hasMore && (
                                <button style={styles.loadMore} onClick={loadMore}>
                                    Load more
                                </button>
                            )}
                        </>
                    )}
                </div>
            )}
        </div>
    );
}

// ── Notification Item Sub-component ──

interface NotificationItemProps {
    notification: NotificationResponse;
    styles: Record<string, React.CSSProperties>;
    onMarkRead: () => void;
    onArchive: () => void;
    onSnooze: () => void;
}

function NotificationItem({
    notification: n,
    styles,
    onMarkRead,
    onArchive,
    onSnooze,
}: NotificationItemProps) {
    const isUnread = !n.read_at;
    return (
        <div
            style={{
                ...styles.item,
                background: isUnread ? 'rgba(59, 130, 246, 0.04)' : undefined,
            }}
        >
            <div style={styles.itemContent}>
                <div style={styles.itemTitle}>
                    {n.content?.title || 'Notification'}
                </div>
                <p style={styles.itemBody}>{n.content?.body}</p>
                <div style={styles.itemTime}>{formatTime(n.created_at)}</div>
            </div>
            <div style={styles.itemActions}>
                {isUnread && (
                    <button
                        style={styles.itemActionBtn}
                        onClick={onMarkRead}
                        title="Mark read"
                    >
                        ✓
                    </button>
                )}
                <button
                    style={styles.itemActionBtn}
                    onClick={onSnooze}
                    title="Snooze 1h"
                >
                    ⏰
                </button>
                <button
                    style={styles.itemActionBtn}
                    onClick={onArchive}
                    title="Archive"
                >
                    📥
                </button>
            </div>
        </div>
    );
}
