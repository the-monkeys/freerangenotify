import { useAuth } from '../contexts/AuthContext';
import { billingAPI } from '../services/api';
import type { BillingUsage } from '../types';
import { useApiQuery } from './use-api-query';

interface BillingState {
    usage: BillingUsage | null;
    billingEnabled?: boolean;
}

async function fetchBillingState(): Promise<BillingState> {
    const [usage, breakdown] = await Promise.allSettled([
        billingAPI.getUsage(),
        billingAPI.getUsageBreakdown(),
    ]);
    const flag = breakdown.status === 'fulfilled' ? breakdown.value?.billing_enabled : undefined;
    return {
        usage: usage.status === 'fulfilled' ? usage.value : null,
        // Missing/failed old endpoints do not imply either enabled or disabled billing.
        billingEnabled: typeof flag === 'boolean' ? flag : undefined,
    };
}

/** Share current wallet and supported enablement signal across sidebar and send forms. */
export function useBillingState(enabled = true) {
    const { user } = useAuth();
    const query = useApiQuery(fetchBillingState, [user?.user_id], {
        enabled: enabled && !!user?.user_id,
        cacheKey: `billing-state-${user?.user_id}`,
        staleTime: 60_000,
    });
    return {
        usage: query.data?.usage ?? null,
        billingEnabled: query.data?.billingEnabled,
        loading: query.loading,
        refetch: query.refetch,
    };
}
