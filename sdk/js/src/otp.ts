import { HttpClient } from './client';
import { FreeRangeNotifyError } from './errors';

// ── Types ─────────────────────────────────────────────────────────────────

export type OTPChannel = 'sms' | 'whatsapp' | 'email';

export interface OTPSendParams {
    /** Delivery channel. */
    channel: OTPChannel;
    /**
     * Raw E.164 phone (`sms`/`whatsapp`) or RFC 5322 email (`email`). Mutually
     * exclusive with {@link userId} and {@link externalId}.
     */
    recipient?: string;
    /**
     * Internal FRN `user_id` (UUID). When supplied, the channel-appropriate
     * contact field on the user record is used as the recipient. Mutually
     * exclusive with {@link recipient} and {@link externalId}.
     */
    userId?: string;
    /**
     * Caller-owned external identifier looked up via the users index.
     * Mutually exclusive with {@link recipient} and {@link userId}.
     */
    externalId?: string;

    /** Code length (4–10). Omit / 0 → server default of 6. */
    length?: number;
    /** Draw codes from a lookalike-free alphanumeric alphabet instead of digits. */
    alphanumeric?: boolean;
    /** Code lifetime in seconds (30–900). Omit → server default of 300. */
    ttlSeconds?: number;
    /** Verify-attempt budget (1–10). Omit → server default of 5. */
    maxAttempts?: number;

    /** Optional bring-your-own message body containing `{{code}}`. */
    templateBody?: string;
    /** Extra variables (e.g. `{ app_name: 'Acme' }`) for `templateBody`. */
    templateData?: Record<string, unknown>;
}

export interface OTPSendResult {
    request_id: string;
    notification_id: string;
    channel: OTPChannel;
    expires_at: string;
    ttl_seconds: number;
    max_attempts: number;
}

export interface OTPVerifyParams {
    requestId: string;
    code: string;
}

export interface OTPVerifyResult {
    verified: boolean;
    request_id: string;
    verified_at?: string;
    attempts_remaining?: number;
    error?: OTPVerifyErrorCode;
}

export type OTPVerifyErrorCode =
    | 'invalid_code'
    | 'attempts_exhausted'
    | 'expired'
    | 'not_found'
    | 'internal_error';

// ── Typed error ───────────────────────────────────────────────────────────

/**
 * Thrown by {@link OTPClient} for known OTP failure modes. Inspect `code` to
 * branch — it carries the same `error` discriminator the API returns, plus a
 * synthetic `rate_limited` / `already_verified` / `resend_cooldown` for the
 * send/resend paths (which never reach this exception type for the verify
 * endpoint).
 */
export class OTPError extends FreeRangeNotifyError {
    code: OTPErrorCode;
    attemptsRemaining?: number;

    constructor(status: number, body: string, code: OTPErrorCode, attemptsRemaining?: number) {
        super(status, body);
        this.name = 'OTPError';
        this.code = code;
        this.attemptsRemaining = attemptsRemaining;
    }
}

export type OTPErrorCode =
    | OTPVerifyErrorCode
    | 'already_verified'
    | 'resend_cooldown'
    | 'rate_limited';

// ── Client ────────────────────────────────────────────────────────────────

export class OTPClient {
    constructor(private readonly http: HttpClient) { }

    /**
     * Generate an OTP, hash it, and dispatch it via the requested channel.
     * Throws {@link OTPError} with `code='rate_limited'` on 429.
     */
    async send(params: OTPSendParams): Promise<OTPSendResult> {
        try {
            return await this.http.request<OTPSendResult>('POST', '/otp/send', toWire(params));
        } catch (err) {
            throw mapSendError(err);
        }
    }

    /**
     * Verify a user-supplied code against an existing `requestId`. On invalid
     * input the SDK throws {@link OTPError}; inspect `err.code` and (for
     * `invalid_code`) `err.attemptsRemaining` to drive UX.
     */
    async verify(params: OTPVerifyParams): Promise<OTPVerifyResult> {
        try {
            return await this.http.request<OTPVerifyResult>('POST', '/otp/verify', {
                request_id: params.requestId,
                code: params.code,
            });
        } catch (err) {
            throw mapVerifyError(err);
        }
    }

    /**
     * Re-issue a fresh code for an existing `requestId` (60 s cooldown). The
     * previous code is invalidated and the attempt counter resets.
     */
    async resend(requestId: string): Promise<OTPSendResult> {
        try {
            return await this.http.request<OTPSendResult>('POST', '/otp/resend', { request_id: requestId });
        } catch (err) {
            throw mapSendError(err);
        }
    }
}

// ── Helpers ───────────────────────────────────────────────────────────────

function toWire(p: OTPSendParams): Record<string, unknown> {
    const out: Record<string, unknown> = {
        channel: p.channel,
    };
    if (p.recipient !== undefined) out.recipient = p.recipient;
    if (p.userId !== undefined) out.user_id = p.userId;
    if (p.externalId !== undefined) out.external_id = p.externalId;
    if (p.length !== undefined) out.length = p.length;
    if (p.alphanumeric !== undefined) out.alphanumeric = p.alphanumeric;
    if (p.ttlSeconds !== undefined) out.ttl_seconds = p.ttlSeconds;
    if (p.maxAttempts !== undefined) out.max_attempts = p.maxAttempts;
    if (p.templateBody !== undefined) out.template_body = p.templateBody;
    if (p.templateData !== undefined) out.template_data = p.templateData;
    return out;
}

function mapSendError(err: unknown): unknown {
    if (!(err instanceof FreeRangeNotifyError)) return err;
    const body = err.body || '';
    switch (err.status) {
        case 404:
            return new OTPError(err.status, body, 'not_found');
        case 410:
            return new OTPError(err.status, body, 'expired');
        case 409:
            if (body.includes('already verified')) {
                return new OTPError(err.status, body, 'already_verified');
            }
            if (body.includes('cooldown')) {
                return new OTPError(err.status, body, 'resend_cooldown');
            }
            return err;
        case 429:
            return new OTPError(err.status, body, 'rate_limited');
        default:
            return err;
    }
}

function mapVerifyError(err: unknown): unknown {
    if (!(err instanceof FreeRangeNotifyError)) return err;
    const parsed = tryParseVerifyBody(err.body);
    const code = parsed?.error;
    if (code) {
        return new OTPError(err.status, err.body, code, parsed?.attempts_remaining);
    }
    switch (err.status) {
        case 404:
            return new OTPError(err.status, err.body, 'not_found');
        case 410:
            return new OTPError(err.status, err.body, 'expired');
        default:
            return err;
    }
}

function tryParseVerifyBody(body: string): OTPVerifyResult | undefined {
    if (!body) return undefined;
    try {
        const out = JSON.parse(body) as OTPVerifyResult;
        return out;
    } catch {
        return undefined;
    }
}
