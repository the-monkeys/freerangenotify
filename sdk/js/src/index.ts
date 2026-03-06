/**
 * FreeRangeNotify JavaScript/TypeScript SDK
 *
 * Usage:
 *   import { FreeRangeNotify } from '@freerangenotify/sdk';
 *
 *   const client = new FreeRangeNotify('your-api-key', {
 *     baseURL: 'https://notify.example.com/v1',
 *   });
 *
 *   await client.send({ to: 'user@example.com', template: 'welcome_email', data: { name: 'Alice' } });
 */

export interface SendParams {
  to: string;
  template?: string;
  subject?: string;
  body?: string;
  data?: Record<string, unknown>;
  channel?: string;
  priority?: 'low' | 'normal' | 'high' | 'critical';
  scheduledAt?: Date;
}

export interface SendResult {
  notification_id: string;
  status: string;
  user_id: string;
  channel: string;
}

export interface BroadcastParams {
  template: string;
  data?: Record<string, unknown>;
  channel?: string;
  priority?: string;
}

export interface BroadcastResult {
  total_sent: number;
  notifications: SendResult[];
}

export interface CreateUserParams {
  email?: string;
  phone?: string;
  timezone?: string;
  language?: string;
  external_id?: string;
}

export interface UpdateUserParams {
  external_id?: string;
  email?: string;
  phone?: string;
  timezone?: string;
  language?: string;
  webhook_url?: string;
}

export interface User {
  user_id: string;
  external_id: string;
  email: string;
  phone: string;
  timezone: string;
  language: string;
  created_at: string;
  updated_at: string;
}

export interface SSENotification {
  notification_id: string;
  title: string;
  body: string;
  channel?: string;
  category?: string;
  status: string;
  data?: Record<string, unknown>;
  created_at: string;
}

export interface SSEConnectionOptions {
  /** Called when a notification arrives via SSE. */
  onNotification: (notification: SSENotification) => void;
  /** Called when the SSE connection is established. */
  onConnected?: () => void;
  /** Called when the SSE connection encounters an error. */
  onError?: (event: Event) => void;
}

export interface SSEConnection {
  /** Close the SSE connection. */
  close: () => void;
}

export interface FreeRangeNotifyOptions {
  baseURL?: string;
}

export class FreeRangeNotifyError extends Error {
  status: number;
  body: string;

  constructor(status: number, body: string) {
    super(`FreeRangeNotify API error (${status}): ${body}`);
    this.name = 'FreeRangeNotifyError';
    this.status = status;
    this.body = body;
  }
}

export class FreeRangeNotify {
  private readonly apiKey: string;
  private readonly baseURL: string;

  constructor(apiKey: string, options?: FreeRangeNotifyOptions) {
    if (!apiKey) throw new Error('API key is required');
    this.apiKey = apiKey;
    this.baseURL = (options?.baseURL || 'http://localhost:8080/v1').replace(/\/+$/, '');
  }

  /**
   * Send a notification to a single recipient using Quick-Send.
   */
  async send(params: SendParams): Promise<SendResult> {
    return this.request<SendResult>('POST', '/quick-send', {
      to: params.to,
      template: params.template,
      subject: params.subject,
      body: params.body,
      data: params.data,
      channel: params.channel,
      priority: params.priority,
      scheduled_at: params.scheduledAt?.toISOString(),
    });
  }

  /**
   * Broadcast a notification to all users in the application.
   */
  async broadcast(params: BroadcastParams): Promise<BroadcastResult> {
    return this.request<BroadcastResult>('POST', '/notifications/broadcast', params);
  }

  /**
   * Create a user profile for targeting notifications.
   */
  async createUser(params: CreateUserParams): Promise<User> {
    return this.request<User>('POST', '/users/', params);
  }

  /**
   * Update an existing user (e.g. to change external_id after a username change).
   */
  async updateUser(userId: string, params: UpdateUserParams): Promise<User> {
    return this.request<User>('PUT', `/users/${userId}`, params);
  }

  /**
   * List users in the application.
   */
  async listUsers(page = 1, pageSize = 20): Promise<{ users: User[]; total_count: number }> {
    return this.request('GET', `/users/?page=${page}&page_size=${pageSize}`);
  }

  /**
   * Open a real-time SSE connection for a user.
   *
   * Uses named SSE events — the server sends `event: notification` with a flat JSON payload.
   *
   * `userId` can be the internal UUID or an `external_id`. When a token is provided,
   * the server resolves `external_id` to the internal UUID automatically.
   *
   * ```ts
   * const conn = client.connectSSE('user-uuid', {
   *   onNotification: (n) => console.log(n.title, n.body),
   *   onConnected: () => console.log('SSE connected'),
   * });
   * // Using external_id:
   * const conn = client.connectSSE('my_platform_username', { ... });
   * // later: conn.close();
   * ```
   */
  connectSSE(userId: string, options: SSEConnectionOptions): SSEConnection {
    if (!userId) throw new Error('userId is required for SSE');
    if (!options.onNotification) throw new Error('onNotification callback is required');

    // Build URL with user_id + optional token for auth. Strip /v1 suffix to get origin.
    const origin = this.baseURL.replace(/\/v1$/, '');
    let url = `${origin}/v1/sse?user_id=${encodeURIComponent(userId)}`;
    url += `&token=${encodeURIComponent(this.apiKey)}`;

    const es = new EventSource(url);

    es.addEventListener('connected', () => {
      options.onConnected?.();
    });

    es.addEventListener('notification', (event) => {
      try {
        const notif: SSENotification = JSON.parse((event as MessageEvent).data);
        options.onNotification(notif);
      } catch {
        // ignore malformed
      }
    });

    if (options.onError) {
      es.onerror = options.onError;
    }

    return {
      close: () => es.close(),
    };
  }

  // ── Internal ──────────────────────────────────────────────

  private async request<T>(method: string, path: string, body?: unknown): Promise<T> {
    const url = this.baseURL + path;
    const headers: Record<string, string> = {
      'Authorization': `Bearer ${this.apiKey}`,
      'Content-Type': 'application/json',
    };

    const init: RequestInit = { method, headers };
    if (body && method !== 'GET') {
      init.body = JSON.stringify(body);
    }

    const res = await fetch(url, init);
    if (!res.ok) {
      const text = await res.text().catch(() => '');
      throw new FreeRangeNotifyError(res.status, text);
    }

    return res.json() as Promise<T>;
  }
}

export default FreeRangeNotify;
