import { HttpClient } from './client';
import type { CheckInParams } from './types';

export class PresenceClient {
    constructor(private readonly http: HttpClient) { }

    /**
     * Register a user's presence for smart delivery routing.
     * If webhookURL is provided, it overrides the user's static webhook URL.
     */
    async checkIn(params: CheckInParams): Promise<void> {
        await this.http.request('POST', '/presence/check-in', params);
    }
}
