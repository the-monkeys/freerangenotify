import { HttpClient } from './client';
import type {
    CreateTopicParams,
    Topic,
    TopicListResponse,
    SubscriberListResponse,
} from './types';

export class TopicsClient {
    constructor(private readonly http: HttpClient) { }

    async create(params: CreateTopicParams): Promise<Topic> {
        return this.http.request<Topic>('POST', '/topics/', params);
    }

    async get(topicId: string): Promise<Topic> {
        return this.http.request<Topic>('GET', `/topics/${topicId}`);
    }

    async getByKey(key: string): Promise<Topic> {
        return this.http.request<Topic>('GET', `/topics/key/${key}`);
    }

    async delete(topicId: string): Promise<void> {
        await this.http.request('DELETE', `/topics/${topicId}`);
    }

    async list(page?: number, pageSize?: number): Promise<TopicListResponse> {
        const query: Record<string, string | undefined> = {};
        if (page) query.page = String(page);
        if (pageSize) query.page_size = String(pageSize);
        return this.http.request<TopicListResponse>('GET', '/topics/', undefined, query);
    }

    async addSubscribers(topicId: string, userIds: string[]): Promise<void> {
        await this.http.request('POST', `/topics/${topicId}/subscribers`, {
            user_ids: userIds,
        });
    }

    async removeSubscribers(topicId: string, userIds: string[]): Promise<void> {
        await this.http.request('DELETE', `/topics/${topicId}/subscribers`, {
            user_ids: userIds,
        });
    }

    async getSubscribers(
        topicId: string,
        page?: number,
        pageSize?: number,
    ): Promise<SubscriberListResponse> {
        const query: Record<string, string | undefined> = {};
        if (page) query.page = String(page);
        if (pageSize) query.page_size = String(pageSize);
        return this.http.request<SubscriberListResponse>(
            'GET',
            `/topics/${topicId}/subscribers`,
            undefined,
            query,
        );
    }
}
