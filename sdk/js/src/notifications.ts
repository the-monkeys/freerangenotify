import { HttpClient } from './client';
import type {
    QuickSendParams,
    SendResult,
    NotificationSendParams,
    NotificationResponse,
    BulkSendParams,
    BulkSendResult,
    BroadcastParams,
    BroadcastResult,
    ListNotificationsOptions,
    NotificationListResponse,
} from './types';

export class NotificationsClient {
    constructor(private readonly http: HttpClient) { }

    // ── Quick-Send ──

    async quickSend(params: QuickSendParams): Promise<SendResult> {
        const headers: Record<string, string> = {};
        if (params.idempotencyKey) headers['Idempotency-Key'] = params.idempotencyKey;
        return this.http.request<SendResult>('POST', '/quick-send', {
            to: params.to,
            template: params.template,
            subject: params.subject,
            body: params.body,
            data: params.data,
            channel: params.channel,
            priority: params.priority,
            scheduled_at: params.scheduledAt?.toISOString(),
        }, undefined, { headers });
    }

    // ── Standard Send ──

    async send(params: NotificationSendParams): Promise<NotificationResponse> {
        const headers: Record<string, string> = {};
        if (params.idempotency_key) headers['Idempotency-Key'] = params.idempotency_key;
        const { idempotency_key: _, ...body } = params;
        return this.http.request<NotificationResponse>('POST', '/notifications/', body, undefined, { headers });
    }

    // ── Bulk Send ──

    async sendBulk(params: BulkSendParams): Promise<BulkSendResult> {
        const headers: Record<string, string> = {};
        if (params.idempotency_key) headers['Idempotency-Key'] = params.idempotency_key;
        const { idempotency_key: _, ...body } = params;
        return this.http.request<BulkSendResult>('POST', '/notifications/bulk', body, undefined, { headers });
    }

    // ── Batch Send ──

    async sendBatch(notifications: NotificationSendParams[]): Promise<BulkSendResult> {
        return this.http.request<BulkSendResult>('POST', '/notifications/batch', { notifications });
    }

    // ── Broadcast ──

    async broadcast(params: BroadcastParams): Promise<BroadcastResult> {
        const headers: Record<string, string> = {};
        if (params.idempotency_key) headers['Idempotency-Key'] = params.idempotency_key;
        const { idempotency_key: _, ...body } = params;
        return this.http.request<BroadcastResult>('POST', '/notifications/broadcast', body, undefined, { headers });
    }

    // ── List ──

    async list(opts?: ListNotificationsOptions): Promise<NotificationListResponse> {
        const query: Record<string, string | undefined> = {};
        if (opts?.userId) query.user_id = opts.userId;
        if (opts?.appId) query.app_id = opts.appId;
        if (opts?.channel) query.channel = opts.channel;
        if (opts?.status) query.status = opts.status;
        if (opts?.category) query.category = opts.category;
        if (opts?.priority) query.priority = opts.priority;
        if (opts?.page) query.page = String(opts.page);
        if (opts?.pageSize) query.page_size = String(opts.pageSize);
        if (opts?.unreadOnly) query.unread_only = 'true';

        return this.http.request<NotificationListResponse>('GET', '/notifications/', undefined, query);
    }

    // ── Get ──

    async get(notificationId: string): Promise<NotificationResponse> {
        return this.http.request<NotificationResponse>('GET', `/notifications/${notificationId}`);
    }

    // ── Unread Count ──

    async getUnreadCount(userId: string): Promise<number> {
        const result = await this.http.request<{ count: number }>(
            'GET',
            '/notifications/unread/count',
            undefined,
            { user_id: userId },
        );
        return result.count;
    }

    // ── List Unread ──

    async listUnread(
        userId: string,
        page?: number,
        pageSize?: number,
    ): Promise<NotificationListResponse> {
        const query: Record<string, string | undefined> = { user_id: userId };
        if (page) query.page = String(page);
        if (pageSize) query.page_size = String(pageSize);

        return this.http.request<NotificationListResponse>(
            'GET',
            '/notifications/unread',
            undefined,
            query,
        );
    }

    // ── Mark Read ──

    async markRead(userId: string, notificationIds: string[]): Promise<void> {
        await this.http.request('POST', '/notifications/read', {
            user_id: userId,
            notification_ids: notificationIds,
        });
    }

    // ── Mark All Read ──

    async markAllRead(userId: string, category?: string): Promise<void> {
        const payload: Record<string, unknown> = { user_id: userId };
        if (category) payload.category = category;
        await this.http.request('POST', '/notifications/read-all', payload);
    }

    // ── Update Status ──

    async updateStatus(
        notificationId: string,
        status: string,
        errorMessage?: string,
    ): Promise<void> {
        const payload: Record<string, unknown> = { status };
        if (errorMessage) payload.error_message = errorMessage;
        await this.http.request('PUT', `/notifications/${notificationId}/status`, payload);
    }

    // ── Cancel ──

    async cancel(notificationId: string): Promise<void> {
        await this.http.request('DELETE', `/notifications/${notificationId}`);
    }

    // ── Cancel Batch ──

    async cancelBatch(notificationIds: string[]): Promise<void> {
        await this.http.request('DELETE', '/notifications/batch', {
            notification_ids: notificationIds,
        });
    }

    // ── Retry ──

    async retry(notificationId: string): Promise<NotificationResponse> {
        return this.http.request<NotificationResponse>(
            'POST',
            `/notifications/${notificationId}/retry`,
        );
    }

    // ── Snooze ──

    async snooze(notificationId: string, duration: string): Promise<void> {
        await this.http.request('POST', `/notifications/${notificationId}/snooze`, { duration });
    }

    // ── Unsnooze ──

    async unsnooze(notificationId: string): Promise<void> {
        await this.http.request('POST', `/notifications/${notificationId}/unsnooze`);
    }

    // ── Archive ──

    async archive(userId: string, notificationIds: string[]): Promise<void> {
        await this.http.request('PATCH', '/notifications/bulk/archive', {
            notification_ids: notificationIds,
            user_id: userId,
        });
    }
}
