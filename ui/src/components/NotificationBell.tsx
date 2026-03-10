import React, { useEffect, useRef, useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { Bell } from 'lucide-react';
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuTrigger,
} from './ui/dropdown-menu';
import { adminAPI } from '../services/api';
import type { DashboardNotification } from '../types';

const SSE_BASE = import.meta.env.VITE_API_BASE_URL
    ? `${import.meta.env.VITE_API_BASE_URL.replace(/\/$/, '')}/v1`
    : '/v1';

interface NotificationBellProps {
    isAuthenticated: boolean;
}

export const NotificationBell: React.FC<NotificationBellProps> = ({ isAuthenticated }) => {
    const navigate = useNavigate();
    const [unreadCount, setUnreadCount] = useState(0);
    const [notifications, setNotifications] = useState<DashboardNotification[]>([]);
    const [open, setOpen] = useState(false);
    const eventSourceRef = useRef<EventSource | null>(null);
    const tokenRefreshRef = useRef<NodeJS.Timeout | null>(null);

    const fetchUnreadCount = useCallback(async () => {
        if (!isAuthenticated) return;
        try {
            const count = await adminAPI.getUnreadCount();
            setUnreadCount(count);
        } catch {
            // Ignore - user may not be fully authenticated
        }
    }, [isAuthenticated]);

    const fetchNotifications = useCallback(async () => {
        if (!isAuthenticated) return;
        try {
            const { notifications: list } = await adminAPI.listNotifications(20, 0);
            setNotifications(list);
        } catch {
            // Ignore
        }
    }, [isAuthenticated]);

    // Poll unread count when tab becomes visible
    useEffect(() => {
        if (!isAuthenticated) return;
        fetchUnreadCount();
        const onVisibilityChange = () => {
            if (document.visibilityState === 'visible') fetchUnreadCount();
        };
        document.addEventListener('visibilitychange', onVisibilityChange);
        return () => document.removeEventListener('visibilitychange', onVisibilityChange);
    }, [isAuthenticated, fetchUnreadCount]);

    // SSE connection for real-time notifications
    useEffect(() => {
        if (!isAuthenticated) return;

        let mounted = true;
        const connectSSE = async () => {
            try {
                const { sse_token } = await adminAPI.createDashboardSSEToken();
                const url = `${SSE_BASE}/sse?sse_token=${encodeURIComponent(sse_token)}`;
                const es = new EventSource(url);
                eventSourceRef.current = es;

                es.addEventListener('connected', () => {
                    if (mounted) fetchUnreadCount();
                });

                es.addEventListener('notification', () => {
                    if (mounted) {
                        fetchUnreadCount();
                        fetchNotifications();
                    }
                });

                es.onerror = () => {
                    es.close();
                    eventSourceRef.current = null;
                    // Reconnect after a delay
                    if (mounted) {
                        setTimeout(connectSSE, 5000);
                    }
                };
            } catch {
                // Token creation failed (e.g. not logged in) - retry later
                if (mounted) {
                    tokenRefreshRef.current = setTimeout(connectSSE, 10000);
                }
            }
        };

        connectSSE();

        return () => {
            mounted = false;
            if (tokenRefreshRef.current) {
                clearTimeout(tokenRefreshRef.current);
            }
            if (eventSourceRef.current) {
                eventSourceRef.current.close();
                eventSourceRef.current = null;
            }
        };
    }, [isAuthenticated, fetchUnreadCount, fetchNotifications]);

    // Fetch notifications when dropdown opens
    useEffect(() => {
        if (open) fetchNotifications();
    }, [open, fetchNotifications]);

    const handleMarkRead = async (ids: string[]) => {
        try {
            await adminAPI.markNotificationsRead(ids);
            setUnreadCount((c) => Math.max(0, c - ids.length));
            setNotifications((prev) =>
                prev.map((n) => (ids.includes(n.id) ? { ...n, read_at: new Date().toISOString() } : n))
            );
        } catch {
            // Ignore
        }
    };

    const handleNotificationClick = (n: DashboardNotification) => {
        if (!n.read_at) handleMarkRead([n.id]);
        if (n.category === 'org_invite' && n.data?.tenant_id) {
            navigate(`/tenants/${n.data.tenant_id}`);
        }
        setOpen(false);
    };

    if (!isAuthenticated) return null;

    return (
        <DropdownMenu open={open} onOpenChange={setOpen}>
            <DropdownMenuTrigger asChild>
                <button
                    className="relative p-1.5 rounded-md text-muted-foreground hover:text-foreground hover:bg-muted transition-colors"
                    aria-label="Notifications"
                >
                    <Bell className="h-5 w-5" />
                    {unreadCount > 0 && (
                        <span className="absolute -top-0.5 -right-0.5 h-4 min-w-4 flex items-center justify-center rounded-full bg-primary text-primary-foreground text-xs px-1">
                            {unreadCount > 99 ? '99+' : unreadCount}
                        </span>
                    )}
                </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-80 max-h-[min(24rem,80vh)] overflow-y-auto">
                <div className="px-2 py-2 border-b border-border">
                    <h3 className="text-sm font-medium">Notifications</h3>
                </div>
                {notifications.length === 0 ? (
                    <div className="px-4 py-8 text-center text-sm text-muted-foreground">
                        No notifications yet
                    </div>
                ) : (
                    <div className="py-1">
                        {notifications.map((n) => (
                            <button
                                key={n.id}
                                onClick={() => handleNotificationClick(n)}
                                className={`w-full text-left px-3 py-2 hover:bg-muted transition-colors ${!n.read_at ? 'bg-muted/50' : ''}`}
                            >
                                <p className="text-sm font-medium truncate">{n.title}</p>
                                <p className="text-xs text-muted-foreground line-clamp-2 mt-0.5">{n.body}</p>
                                <p className="text-xs text-muted-foreground mt-1">
                                    {new Date(n.created_at).toLocaleDateString(undefined, {
                                        month: 'short',
                                        day: 'numeric',
                                        hour: '2-digit',
                                        minute: '2-digit',
                                    })}
                                </p>
                            </button>
                        ))}
                    </div>
                )}
            </DropdownMenuContent>
        </DropdownMenu>
    );
};
