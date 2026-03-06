import { FreeRangeNotifyError } from './errors';

export class HttpClient {
    constructor(
        private readonly apiKey: string,
        private readonly baseURL: string,
    ) { }

    async request<T>(
        method: string,
        path: string,
        body?: unknown,
        query?: Record<string, string | undefined>,
    ): Promise<T> {
        let url = this.baseURL + path;

        if (query) {
            const params = new URLSearchParams(
                Object.fromEntries(
                    Object.entries(query).filter(
                        (entry): entry is [string, string] =>
                            entry[1] !== undefined && entry[1] !== '',
                    ),
                ),
            );
            const qs = params.toString();
            if (qs) url += '?' + qs;
        }

        const headers: Record<string, string> = {
            Authorization: `Bearer ${this.apiKey}`,
            'Content-Type': 'application/json',
        };

        const init: RequestInit = { method, headers };
        if (body && method !== 'GET' && method !== 'DELETE') {
            init.body = JSON.stringify(body);
        }

        const res = await fetch(url, init);
        if (!res.ok) {
            const text = await res.text().catch(() => '');
            throw new FreeRangeNotifyError(res.status, text);
        }

        const contentLength = res.headers.get('content-length');
        if (contentLength === '0' || res.status === 204) {
            return undefined as unknown as T;
        }

        return res.json() as Promise<T>;
    }
}
