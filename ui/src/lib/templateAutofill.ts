import type { User } from '../types';

/**
 * Variable keys that the UI knows how to auto-fill from a user record.
 * Mirrors the backend helper at internal/usecases/template/autofill.go so
 * the Twilio Content API path (rendered by Twilio, not our worker) gets the
 * same personalization as templates we render ourselves.
 */
const NAME_VAR_KEYS = ['name', 'user_name', 'full_name', 'first_name', 'last_name'] as const;

/**
 * Resolves a display name for a user with the same priority order as the
 * backend: full_name -> email local-part -> email-like external_id -> "there".
 */
export function resolveUserDisplayName(user: User | undefined | null): string {
    if (!user) return 'there';

    const fullName = (user.full_name ?? '').trim();
    if (fullName) return fullName;

    const fromEmail = humanizeEmailLocalPart(user.email);
    if (fromEmail) return fromEmail;

    if (user.external_id && user.external_id.includes('@')) {
        const fromExternal = humanizeEmailLocalPart(user.external_id);
        if (fromExternal) return fromExternal;
    }

    return 'there';
}

function humanizeEmailLocalPart(email: string | undefined): string {
    const trimmed = (email ?? '').trim();
    if (!trimmed) return '';
    const at = trimmed.indexOf('@');
    const local = at > 0 ? trimmed.slice(0, at) : trimmed;
    const cleaned = local.replace(/[._]+/g, ' ').trim();
    if (!cleaned) return '';
    return cleaned
        .split(/\s+/)
        .map(w => w.charAt(0).toUpperCase() + w.slice(1))
        .join(' ');
}

function splitName(full: string): { first: string; last: string } {
    const parts = full.split(/\s+/).filter(Boolean);
    if (parts.length === 0) return { first: '', last: '' };
    if (parts.length === 1) return { first: parts[0], last: '' };
    return { first: parts[0], last: parts.slice(1).join(' ') };
}

/**
 * Returns a new variable map with user-derived defaults filled in for any
 * declared key the caller has not yet provided. Mirrors ApplyUserAutoFill:
 *   - only injects keys present in `declaredKeys`
 *   - never overwrites a non-empty existing value
 *   - never injects an empty value
 */
export function applyUserAutoFillVars(
    declaredKeys: readonly string[],
    current: Record<string, string>,
    user: User | undefined | null
): Record<string, string> {
    const declared = new Set(declaredKeys);
    if (declared.size === 0) return { ...current };

    const name = resolveUserDisplayName(user);
    const { first, last } = splitName(name);

    const next = { ...current };
    const inject = (key: string, value: string) => {
        if (!declared.has(key)) return;
        if (!value) return;
        const existing = (next[key] ?? '').trim();
        if (existing !== '') return;
        next[key] = value;
    };

    inject('name', name);
    inject('user_name', name);
    inject('full_name', name);
    inject('first_name', first);
    inject('last_name', last);

    return next;
}

/** Exposed for tests. */
export const __test__ = { humanizeEmailLocalPart, splitName, NAME_VAR_KEYS };
