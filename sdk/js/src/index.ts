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
   * List users in the application.
   */
  async listUsers(page = 1, pageSize = 20): Promise<{ users: User[]; total_count: number }> {
    return this.request('GET', `/users/?page=${page}&page_size=${pageSize}`);
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
