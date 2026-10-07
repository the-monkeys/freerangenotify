import type { BillingUsage } from '../types';

export function isCreditAmount(value: unknown): value is number {
    return typeof value === 'number' && Number.isFinite(value) && value >= 0;
}

/** Current wallet only. Historical attempt balances must come from notification metadata. */
export function availableCredits(usage: BillingUsage | null, billingEnabled?: boolean): number | null {
    if (!usage || billingEnabled === false || usage.billing_model === 'legacy' || usage.status === 'none') return null;
    if (isCreditAmount(usage.credits_available)) return usage.credits_available;
    if (!isCreditAmount(usage.credits_remaining)) return null;
    const reserved = usage.credits_reserved ?? 0;
    if (!isCreditAmount(reserved)) return null;
    return Math.max(0, usage.credits_remaining - reserved);
}
