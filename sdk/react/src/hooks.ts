import { useState, useEffect, useCallback, useRef } from 'react';
import { useFreeRange } from './FreeRangeProvider';
import type {
    NotificationResponse,
    Preferences,
    SSENotification,
} from '../../js/src/types';

// ── useNotifications ──

export interface UseNotificationsOptions {
    category?: string;
    pageSize?: number;
    unreadOnly?: boolean;
}

export interface UseNotificationsResult {
    notifications: NotificationResponse[];
    loading: boolean;
    unreadCount: number;
    markRead: (ids: string[]) => Promise<void>;
    markAllRead: () => Promise<void>;
    archive: (ids: string[]) => Promise<void>;
    snooze: (id: string, duration: string) => Promise<void>;
    loadMore: () => Promise<void>;
    hasMore: boolean;
    refresh: () => Promise<void>;
}

/**
 * Headless hook for fetching and managing a notification list.
 * Handles pagination, mark-read, archive, and snooze via the JS SDK client.
 */
export function useNotifications(
    options?: UseNotificationsOptions,
): UseNotificationsResult {
    const { client, userId } = useFreeRange();
    const pageSize = options?.pageSize ?? 20;

    const [notifications, setNotifications] = useState<NotificationResponse[]>([]);
    const [loading, setLoading] = useState(true);
    const [unreadCount, setUnreadCount] = useState(0);
    const [page, setPage] = useState(1);
    const [hasMore, setHasMore] = useState(true);
    const initialFetched = useRef(false);

    const fetchPage = useCallback(
        async (p: number, replace: boolean) => {
            setLoading(true);
            try {
                const res = await client.notifications.list({
                    userId,
                    category: options?.category,
                    unreadOnly: options?.unreadOnly,
                    page: p,
                    pageSize,
                });

                setNotifications((prev) =>
                    replace ? res.notifications : [...prev, ...res.notifications],
                );
                setHasMore(res.notifications.length === pageSize);

                const countRes = await client.notifications.getUnreadCount(userId);
                setUnreadCount(
                    typeof countRes === 'number'
                        ? countRes
                        : (countRes as { unread_count: number }).unread_count ?? 0,
                );
            } catch {
                // Silently handle fetch errors — loading stops, list preserves state
            } finally {
                setLoading(false);
            }
        },
        [client, userId, options?.category, options?.unreadOnly, pageSize],
    );

    useEffect(() => {
        if (initialFetched.current) return;
        initialFetched.current = true;
        fetchPage(1, true);
    }, [fetchPage]);

    const loadMore = useCallback(async () => {
        if (!hasMore || loading) return;
        const next = page + 1;
        setPage(next);
        await fetchPage(next, false);
    }, [fetchPage, hasMore, loading, page]);

    const refresh = useCallback(async () => {
        setPage(1);
        await fetchPage(1, true);
    }, [fetchPage]);

    const markRead = useCallback(
        async (ids: string[]) => {
            await client.notifications.markRead(userId, ids);
            setNotifications((prev) =>
                prev.map((n) =>
                    ids.includes(n.notification_id)
                        ? { ...n, status: 'read', read_at: new Date().toISOString() }
                        : n,
                ),
            );
            setUnreadCount((c) => Math.max(0, c - ids.length));
        },
        [client, userId],
    );

    const markAllRead = useCallback(async () => {
        await client.notifications.markAllRead(userId);
        setNotifications((prev) =>
            prev.map((n) => ({
                ...n,
                status: 'read',
                read_at: n.read_at || new Date().toISOString(),
            })),
        );
        setUnreadCount(0);
    }, [client, userId]);

    const archive = useCallback(
        async (ids: string[]) => {
            await client.notifications.archive(userId, ids);
            setNotifications((prev) =>
                prev.filter((n) => !ids.includes(n.notification_id)),
            );
            setUnreadCount((c) => {
                const unreadArchived = notifications.filter(
                    (n) => ids.includes(n.notification_id) && !n.read_at,
                ).length;
                return Math.max(0, c - unreadArchived);
            });
        },
        [client, userId, notifications],
    );

    const snooze = useCallback(
        async (id: string, duration: string) => {
            await client.notifications.snooze(id, duration);
            setNotifications((prev) =>
                prev.filter((n) => n.notification_id !== id),
            );
        },
        [client],
    );

    return {
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
    };
}

// ── usePreferences ──

export interface UsePreferencesResult {
    preferences: Preferences | null;
    loading: boolean;
    update: (prefs: Partial<Preferences>) => Promise<void>;
}

/**
 * Headless hook for fetching and updating user notification preferences.
 */
export function usePreferences(): UsePreferencesResult {
    const { client, userId } = useFreeRange();
    const [preferences, setPreferences] = useState<Preferences | null>(null);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        let cancelled = false;
        client.users
            .getPreferences(userId)
            .then((p) => {
                if (!cancelled) {
                    setPreferences(p);
                    setLoading(false);
                }
            })
            .catch(() => {
                if (!cancelled) setLoading(false);
            });
        return () => {
            cancelled = true;
        };
    }, [client, userId]);

    const update = useCallback(
        async (prefs: Partial<Preferences>) => {
            const merged = { ...preferences, ...prefs } as Preferences;
            await client.users.updatePreferences(userId, merged);
            setPreferences(merged);
        },
        [client, userId, preferences],
    );

    return { preferences, loading, update };
}

// ── useSSE ──

export interface UseSSEOptions {
    onNotification?: (notification: SSENotification) => void;
}

export interface UseSSEResult {
    connected: boolean;
    lastNotification: SSENotification | null;
}

/**
 * Hook that opens an SSE connection and delivers real-time notifications.
 * Automatically reconnects and cleans up on unmount.
 */
export function useSSE(options?: UseSSEOptions): UseSSEResult {
    const { client, userId, subscriberHash } = useFreeRange();
    const [connected, setConnected] = useState(false);
    const [lastNotification, setLastNotification] =
        useState<SSENotification | null>(null);
    const callbackRef = useRef(options?.onNotification);
    callbackRef.current = options?.onNotification;

    useEffect(() => {
        const conn = client.connectSSE(userId, {
            onNotification: (n) => {
                setLastNotification(n);
                callbackRef.current?.(n);
            },
            onConnected: () => setConnected(true),
            onConnectionChange: (c) => setConnected(c),
            subscriberHash,
            autoReconnect: true,
        });

        return () => {
            conn.close();
            setConnected(false);
        };
    }, [client, userId, subscriberHash]);

    return { connected, lastNotification };
}

// ── useUnreadCount ──

export interface UseUnreadCountResult {
    count: number;
    loading: boolean;
    refresh: () => Promise<void>;
}

/**
 * Lightweight hook that fetches and exposes the unread notification count.
 */
export function useUnreadCount(): UseUnreadCountResult {
    const { client, userId } = useFreeRange();
    const [count, setCount] = useState(0);
    const [loading, setLoading] = useState(true);

    const refresh = useCallback(async () => {
        setLoading(true);
        try {
            const res = await client.notifications.getUnreadCount(userId);
            setCount(
                typeof res === 'number'
                    ? res
                    : (res as { unread_count: number }).unread_count ?? 0,
            );
        } catch {
            // keep previous count
        } finally {
            setLoading(false);
        }
    }, [client, userId]);

    useEffect(() => {
        refresh();
    }, [refresh]);

    return { count, loading, refresh };
}
