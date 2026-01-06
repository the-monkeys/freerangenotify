"use client";

import { useState, useEffect, useRef, useCallback, useMemo } from 'react';
import { Bell } from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
    Popover,
    PopoverContent,
    PopoverTrigger,
} from '@/components/ui/popover';
import { Badge } from '@/components/ui/badge';
import { ScrollArea } from '@/components/ui/scroll-area';

type Notification = {
    notification_id: string;
    app_id: string;
    user_id: string;
    channel: string;
    priority: string;
    status: string;
    content: {
        title: string;
        body: string;
        data: Record<string, string | number | boolean>;
        notification?: string;
    };
    template_id: string;
    sent_at: string;
    read_at: string | null;
    retry_count: number;
    created_at: string;
    updated_at: string;
};

export default function Notifications() {
    const [notifications, setNotifications] = useState<Notification[]>([]);
    const [isLoading, setIsLoading] = useState(true);
    const [sseStatus, setSseStatus] = useState<'connecting' | 'connected' | 'disconnected'>('connecting');
    const eventSourceRef = useRef<EventSource | null>(null);
    const hasInitialized = useRef(false);

    const apiUrl = process.env.NEXT_PUBLIC_FREERANGENOTIFY_API_URL;
    const appId = process.env.NEXT_PUBLIC_FREERANGENOTIFY_APP_ID;
    const apiKey = process.env.NEXT_PUBLIC_FREERANGENOTIFY_API_KEY;
    const userId = 'admin';

    const SSE_CONNECTION_URL = `${apiUrl}/sse?user_id=${userId}&app_id=${appId}`;
    const FETCH_UNREAD = `${apiUrl}/notifications/unread?user_id=${userId}&app_id=${appId}`;
    const READ_NOTIFICATION = `${apiUrl}/notifications/read`;

    const HEADERS = useMemo(() => ({
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${apiKey}`,
    }), [apiKey]);

    const readNotifications = useCallback(async (notification_ids: string[]) => {
        try {
            await fetch(READ_NOTIFICATION, {
                method: 'POST',
                headers: HEADERS,
                body: JSON.stringify({
                    user_id: userId,
                    notification_ids: notification_ids,
                }),
            });
        } catch (error) {
            console.error('Error marking notifications as read:', error);
        }
    }, [HEADERS, READ_NOTIFICATION, userId]);

    useEffect(() => {
        const fetchUnreadNotifications = async () => {
            try {
                setIsLoading(true);
                const response = await fetch(FETCH_UNREAD, {
                    method: 'GET',
                    headers: HEADERS,
                });
                const data = await response.json();
                console.log('Fetched unread notifications:', data);
                setNotifications(data?.data || []);
            } catch (error) {
                console.error('Error fetching notifications:', error);
            } finally {
                setIsLoading(false);
            }
        };

        const setupSSEConnection = () => {
            // Close existing connection if any
            if (eventSourceRef.current) {
                eventSourceRef.current.close();
            }

            try {
                console.log('Setting up SSE connection to:', SSE_CONNECTION_URL);
                setSseStatus('connecting');

                const eventSource = new EventSource(SSE_CONNECTION_URL);
                eventSourceRef.current = eventSource;

                eventSource.onopen = () => {
                    console.log('SSE connection established');
                    setSseStatus('connected');
                };

                eventSource.onmessage = (event) => {
                    console.log('SSE message received (raw):', event.data);

                    try {
                        // Parse the incoming data
                        const parsedData = JSON.parse(event.data);
                        console.log('Parsed SSE data:', parsedData);

                        // Handle different possible data structures
                        let notification: Notification;

                        if (parsedData.notification) {
                            // If notification is a string, parse it; otherwise use it directly
                            if (typeof parsedData.notification === 'string') {
                                try {
                                    notification = JSON.parse(parsedData.notification);
                                } catch (e) {
                                    console.error('Failed to parse notification string:', e);
                                    // If parsing fails, treat the whole parsedData as notification
                                    notification = parsedData;
                                }
                            } else {
                                // notification is already an object
                                notification = parsedData.notification;
                            }
                        } else {
                            // The data itself is the notification
                            notification = parsedData;
                        }

                        console.log('Final notification object:', notification);

                        setNotifications((prevNotifications) => {
                            // Check if notification already exists to avoid duplicates
                            const exists = prevNotifications.some(
                                n => n.notification_id === notification.notification_id
                            );

                            if (exists) {
                                console.log('Notification already exists, skipping');
                                return prevNotifications;
                            }

                            console.log('Adding new notification to state');
                            return [notification, ...prevNotifications];
                        });
                    } catch (error) {
                        console.error('Error parsing SSE message:', error, 'Raw data:', event.data);
                    }
                };

                eventSource.onerror = (error) => {
                    console.error('SSE error:', error);
                    setSseStatus('disconnected');
                    eventSource.close();

                    // Attempt to reconnect after 5 seconds
                    console.log('Reconnecting in 5 seconds...');
                    setTimeout(() => {
                        setupSSEConnection();
                    }, 5000);
                };
            } catch (error) {
                console.error('Error setting up SSE connection:', error);
                setSseStatus('disconnected');
            }
        };

        // Initial fetch of unread notifications and setup SSE
        if (!hasInitialized.current) {
            hasInitialized.current = true;
            fetchUnreadNotifications();
            setupSSEConnection();
        }

        // Cleanup function
        return () => {
            if (eventSourceRef.current) {
                console.log('Closing SSE connection');
                eventSourceRef.current.close();
            }
        };
    }, [SSE_CONNECTION_URL, FETCH_UNREAD, HEADERS]);

    const unreadCount = notifications.filter(n => n.status !== 'read').length;

    const markAsRead = (notification_id: string) => {
        setNotifications(notifications.map(n =>
            n.notification_id === notification_id ? { ...n, status: 'read' } : n
        ));
        readNotifications([notification_id]);
    };

    const markAllAsRead = () => {
        const unreadIds = notifications
            .filter(n => n.status !== 'read')
            .map(n => n.notification_id);

        if (unreadIds.length > 0) {
            setNotifications(notifications.map(n => ({ ...n, status: 'read' })));
            readNotifications(unreadIds);
        }
    };

    const formatTime = (dateString: string) => {
        try {
            const date = new Date(dateString);
            const now = new Date();
            const diffInMs = now.getTime() - date.getTime();
            const diffInMinutes = Math.floor(diffInMs / 60000);
            const diffInHours = Math.floor(diffInMs / 3600000);
            const diffInDays = Math.floor(diffInMs / 86400000);

            if (diffInMinutes < 1) return 'Just now';
            if (diffInMinutes < 60) return `${diffInMinutes}m ago`;
            if (diffInHours < 24) return `${diffInHours}h ago`;
            if (diffInDays < 7) return `${diffInDays}d ago`;
            return date.toLocaleDateString();
        } catch {
            return '';
        }
    };

    return (
        <Popover>
            <PopoverTrigger asChild>
                <Button variant="outline" size="icon" className="relative text-primary">
                    <Bell className="size-5" />
                    {unreadCount > 0 && (
                        <Badge
                            variant="default"
                            className="absolute -top-2 -right-2 size-5 flex items-center justify-center p-0 text-xs tabular-nums"
                        >
                            {unreadCount > 99 ? '99+' : unreadCount}
                        </Badge>
                    )}
                    {/* Debug indicator for SSE connection status */}
                    <div className={`absolute -bottom-1 -right-1 size-2 rounded-full ${sseStatus === 'connected' ? 'bg-green-500' :
                            sseStatus === 'connecting' ? 'bg-yellow-500' :
                                'bg-red-500'
                        }`} title={`SSE: ${sseStatus}`} />
                </Button>
            </PopoverTrigger>
            <PopoverContent className="w-80 p-0" align="end">
                <div className="flex items-center justify-between p-4 border-b">
                    <div className="flex items-center gap-2">
                        <h3 className="font-semibold text-sm">Notifications</h3>
                        <span className="text-xs text-muted-foreground">
                            ({sseStatus})
                        </span>
                    </div>
                    {unreadCount > 0 && (
                        <Button
                            variant="ghost"
                            size="sm"
                            className="h-auto p-0 text-xs text-blue-600 hover:text-blue-700"
                            onClick={markAllAsRead}
                        >
                            Mark all as read
                        </Button>
                    )}
                </div>
                <ScrollArea className="h-100">
                    {isLoading ? (
                        <div className="p-8 text-center text-sm text-muted-foreground">
                            Loading notifications...
                        </div>
                    ) : notifications.length === 0 ? (
                        <div className="p-8 text-center text-sm text-muted-foreground">
                            No notifications
                        </div>
                    ) : (
                        <div className="divide-y">
                            {notifications.map((notification) => (
                                <div
                                    key={notification.notification_id}
                                    className={`p-4 hover:bg-accent cursor-pointer transition-colors ${notification.status !== 'read' ? 'bg-blue-50/50' : ''
                                        }`}
                                    onClick={() => markAsRead(notification.notification_id)}
                                >
                                    <div className="flex items-start justify-between gap-2">
                                        <div className="flex-1 space-y-1">
                                            <p className="text-sm font-medium leading-none">
                                                {notification.content.title}
                                            </p>
                                            <p className="text-sm text-muted-foreground">
                                                {notification.content.notification || notification.content.body}
                                            </p>
                                            <p className="text-xs text-muted-foreground">
                                                {formatTime(notification.sent_at || notification.created_at)}
                                            </p>
                                        </div>
                                        {notification.status !== 'read' && (
                                            <div className="h-2 w-2 rounded-full bg-blue-600 mt-1 shrink-0" />
                                        )}
                                    </div>
                                </div>
                            ))}
                        </div>
                    )}
                </ScrollArea>
            </PopoverContent>
        </Popover>
    );
}