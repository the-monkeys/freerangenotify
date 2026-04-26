import { useCallback, useMemo } from 'react';
import { providersAPI } from '../services/api';
import type { CustomProvider } from '../types';
import { useApiQuery } from './use-api-query';

/**
 * Single source of truth for an app's registered custom providers.
 *
 * Both the Providers tab (`AppProviders`) and the parent app shell
 * (`AppDetail`, which feeds the Quick Send + Templates tabs) read through
 * this hook so they share the same cache entry. When the Providers tab
 * mutates the list (register / remove / rotate) and calls `refetch`, the
 * shared cache is updated and the subscriber in `useApiQuery` notifies
 * every other consumer — no prop drilling, no manual cross-component
 * refresh, no stale dropdown after deletion.
 *
 * The cache key is `app-providers-${appId}` and is intentionally stable so
 * the matching `mutateApiQueryCache` call from anywhere in the app will
 * propagate to all subscribers.
 */
export function useAppProviders(appId: string | undefined) {
    const fetcher = useCallback(
        () => (appId ? providersAPI.list(appId) : Promise.resolve<CustomProvider[]>([])),
        [appId],
    );

    const query = useApiQuery<CustomProvider[]>(
        fetcher,
        [appId],
        {
            enabled: !!appId,
            cacheKey: appId ? `app-providers-${appId}` : undefined,
        },
    );

    // Derived map keyed by provider name → URL. Used by Quick Send /
    // Templates dropdowns. Recomputed only when `providers` changes, so
    // consumers don't re-render on unrelated state churn.
    const webhooks = useMemo<Record<string, string>>(() => {
        const map: Record<string, string> = {};
        for (const p of query.data ?? []) {
            if (p.channel === 'webhook' && p.active) {
                map[p.name] = p.webhook_url;
            }
        }
        return map;
    }, [query.data]);

    return {
        providers: query.data,
        webhooks,
        loading: query.loading,
        error: query.error,
        refetch: query.refetch,
    };
}
