export class FreeRangeNotifyError extends Error {
    status: number;
    body: string;

    constructor(status: number, body: string) {
        super(`FreeRangeNotify API error (${status}): ${body}`);
        this.name = 'FreeRangeNotifyError';
        this.status = status;
        this.body = body;
    }

    get isNotFound(): boolean { return this.status === 404; }
    get isUnauthorized(): boolean { return this.status === 401; }
    get isRateLimited(): boolean { return this.status === 429; }
    get isValidationError(): boolean { return this.status === 400 || this.status === 422; }
}
