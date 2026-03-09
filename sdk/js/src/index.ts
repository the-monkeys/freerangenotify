/**
 * FreeRangeNotify JavaScript/TypeScript SDK
 *
 * Sub-client pattern for resource-oriented access:
 *
 *   import { FreeRangeNotify } from '@freerangenotify/sdk';
 *
 *   const client = new FreeRangeNotify('frn_xxx', { baseURL: 'http://localhost:8080/v1' });
 *
 *   await client.notifications.send({ user_id: '...', channel: 'email', template_id: 'welcome' });
 *   await client.users.create({ email: 'alice@example.com' });
 *   await client.templates.list({ channel: 'email' });
 *
 *   // Backward-compatible convenience methods
 *   await client.send({ to: 'user@example.com', template: 'welcome_email' });
 */

import { HttpClient } from './client';
import { NotificationsClient } from './notifications';
import { UsersClient } from './users';
import { TemplatesClient } from './templates';
import { WorkflowsClient } from './workflows';
import { TopicsClient } from './topics';
import { PresenceClient } from './presence';
import { connectSSE } from './sse';
import type {
  QuickSendParams,
  SendResult,
  BroadcastParams,
  BroadcastResult,
  CreateUserParams,
  UpdateUserParams,
  User,
  SSEConnectionOptions,
  SSEConnection,
} from './types';

export interface FreeRangeNotifyOptions {
  baseURL?: string;
  /**
   * Environment label (informational only).
   * The actual environment is determined server-side by the API key used.
   * Use a per-environment API key (e.g., `frn_dev_xxx`, `frn_stg_xxx`, `frn_prod_xxx`)
   * to scope all operations to that environment.
   */
  environment?: string;
}

export class FreeRangeNotify {
  private readonly apiKey: string;
  private readonly baseURL: string;
  private readonly http: HttpClient;

  /** Notification operations: send, list, mark-read, snooze, archive, etc. */
  readonly notifications: NotificationsClient;
  /** User management: CRUD, devices, preferences, subscriber hash. */
  readonly users: UsersClient;
  /** Template management: CRUD, versioning, rollback, diff, render, test. */
  readonly templates: TemplatesClient;
  /** Workflow management: CRUD, trigger, executions. */
  readonly workflows: WorkflowsClient;
  /** Topic management: CRUD, subscribers. */
  readonly topics: TopicsClient;
  /** Presence: smart delivery check-in. */
  readonly presence: PresenceClient;

  constructor(apiKey: string, options?: FreeRangeNotifyOptions) {
    if (!apiKey) throw new Error('API key is required');
    this.apiKey = apiKey;
    this.baseURL = (options?.baseURL || 'http://localhost:8080/v1').replace(/\/+$/, '');
    this.http = new HttpClient(this.apiKey, this.baseURL);

    this.notifications = new NotificationsClient(this.http);
    this.users = new UsersClient(this.http);
    this.templates = new TemplatesClient(this.http);
    this.workflows = new WorkflowsClient(this.http);
    this.topics = new TopicsClient(this.http);
    this.presence = new PresenceClient(this.http);
  }

  // ── Backward-compatible convenience methods ──

  /** Send a notification via Quick-Send (delegates to notifications.quickSend). */
  async send(params: QuickSendParams): Promise<SendResult> {
    return this.notifications.quickSend(params);
  }

  /**
   * Trigger a notification by template name (code-first style).
   * Equivalent to: send({ template, to, data, ...opts })
   *
   * @example
   * await client.trigger('welcome-email', { to: 'user@example.com', name: 'Alice' });
   * await client.trigger('order-shipped', { to: userId, orderId: '123', trackingUrl: '...' });
   * await client.trigger('welcome', { to: 'u@x.com', idempotencyKey: 'req-123' }); // safe retry
   */
  async trigger(
    template: string,
    params: { to: string; data?: Record<string, unknown> } & Omit<QuickSendParams, 'to' | 'template' | 'data'>,
  ): Promise<SendResult> {
    const { to, data, ...rest } = params;
    return this.notifications.quickSend({
      template,
      to,
      data,
      ...rest,
    });
  }

  /** Broadcast to all users (delegates to notifications.broadcast). */
  async broadcast(params: BroadcastParams): Promise<BroadcastResult> {
    return this.notifications.broadcast(params);
  }

  /** Create a user (delegates to users.create). */
  async createUser(params: CreateUserParams): Promise<User> {
    return this.users.create(params);
  }

  /** Update a user (delegates to users.update). */
  async updateUser(userId: string, params: UpdateUserParams): Promise<User> {
    return this.users.update(userId, params);
  }

  /** List users (delegates to users.list). */
  async listUsers(page = 1, pageSize = 20): Promise<{ users: User[]; total_count: number }> {
    const result = await this.users.list(page, pageSize);
    return { users: result.users, total_count: result.total_count };
  }

  // ── SSE ──

  /**
   * Open a real-time SSE connection for a user.
   *
   * ```ts
   * const conn = client.connectSSE('user-uuid', {
   *   onNotification: (n) => console.log(n.title, n.body),
   *   onConnected: () => console.log('SSE connected'),
   * });
   * // later: conn.close();
   * ```
   */
  connectSSE(userId: string, options: SSEConnectionOptions): SSEConnection {
    return connectSSE(this.baseURL, this.apiKey, userId, options);
  }
}

// Re-export everything
export { FreeRangeNotifyError } from './errors';
export { HttpClient } from './client';
export { NotificationsClient } from './notifications';
export { UsersClient } from './users';
export { TemplatesClient } from './templates';
export { WorkflowsClient } from './workflows';
export { TopicsClient } from './topics';
export { PresenceClient } from './presence';
export { connectSSE } from './sse';
export {
  workflow,
  emailStep, smsStep, pushStep, inAppStep, webhookStep, slackStep, discordStep,
  delayStep, digestStep, condition, noop,
  channelStep,
  WorkflowBuilder, ChannelStepBuilder, DelayStepBuilder, DigestStepBuilder,
  ConditionStepBuilder, NoopStepBuilder,
} from './workflow_builder';
export type { StepBuilder, ConditionOperator } from './workflow_builder';
export * from './types';

export default FreeRangeNotify;
