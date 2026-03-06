import type { SSENotification, SSEConnectionOptions, SSEConnection } from './types';

/**
 * Open a real-time SSE connection for a user.
 *
 * Uses named SSE events — the server sends `event: notification` with JSON payloads.
 * Supports auto-reconnect with configurable interval.
 *
 * @param baseURL - API base URL (e.g. "http://localhost:8080/v1")
 * @param apiKey  - The application API key
 * @param userId  - Internal UUID or external_id of the user
 * @param options - Connection callbacks and configuration
 */
export function connectSSE(
    baseURL: string,
    apiKey: string,
    userId: string,
    options: SSEConnectionOptions,
): SSEConnection {
    if (!userId) throw new Error('userId is required for SSE');
    if (!options.onNotification) throw new Error('onNotification callback is required');

    const reconnectInterval = options.reconnectInterval ?? 3000;
    const autoReconnect = options.autoReconnect ?? true;
    let closed = false;
    let es: EventSource | null = null;

    function connect() {
        // Strip /v1 suffix to get origin, then build SSE URL
        const origin = baseURL.replace(/\/v1$/, '');
        let url = `${origin}/v1/sse?user_id=${encodeURIComponent(userId)}`;
        url += `&token=${encodeURIComponent(apiKey)}`;
        if (options.subscriberHash) {
            url += `&subscriber_hash=${encodeURIComponent(options.subscriberHash)}`;
        }

        es = new EventSource(url);

        es.addEventListener('connected', () => {
            options.onConnected?.();
            options.onConnectionChange?.(true);
        });

        es.addEventListener('notification', (event) => {
            try {
                const notif: SSENotification = JSON.parse((event as MessageEvent).data);
                options.onNotification(notif);
            } catch {
                // ignore malformed events
            }
        });

        es.addEventListener('unread_count', (event) => {
            try {
                const data = JSON.parse((event as MessageEvent).data);
                options.onUnreadCountChange?.(data.count);
            } catch {
                // ignore
            }
        });

        es.onerror = (event) => {
            options.onError?.(event);
            options.onConnectionChange?.(false);

            if (autoReconnect && !closed) {
                es?.close();
                setTimeout(() => {
                    if (!closed) connect();
                }, reconnectInterval);
            }
        };

        es.onopen = () => {
            options.onConnectionChange?.(true);
        };
    }

    connect();

    return {
        close: () => {
            closed = true;
            es?.close();
            options.onConnectionChange?.(false);
        },
    };
}
