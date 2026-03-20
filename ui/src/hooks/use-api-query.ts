import { useState, useEffect, useCallback } from 'react';

interface UseApiQueryResult<T> {
    data: T | null;
    loading: boolean;
    error: string | null;
    refetch: () => Promise<void>;
}

const apiCache: Record<string, { data: any; timestamp: number }> = {};

interface UseApiQueryOptions {
    enabled?: boolean;
    cacheKey?: string;
    staleTime?: number;
    refetchInterval?: number;
    refetchOnWindowFocus?: boolean;
}

type CacheSubscriber = (data: any) => void;
const cacheSubscribers: Record<string, Set<CacheSubscriber>> = {};

const notifyCacheSubscribers = (key: string, data: any) => {
    const subs = cacheSubscribers[key];
    if (!subs || subs.size === 0) return;
    subs.forEach((subscriber) => subscriber(data));
};

export function mutateApiQueryCache<T>(
    cacheKey: string,
    updater: (current: T | null) => T | null
): void {
    if (!cacheKey) return;
    const current = (apiCache[cacheKey]?.data as T) ?? null;
    const next = updater(current);

    if (next === null) {
        delete apiCache[cacheKey];
        notifyCacheSubscribers(cacheKey, null);
        return;
    }

    apiCache[cacheKey] = {
        data: next,
        timestamp: Date.now(),
    };
    notifyCacheSubscribers(cacheKey, next);
}

export function useApiQuery<T>(
    fetcher: () => Promise<T>,
    deps: React.DependencyList = [],
    options?: UseApiQueryOptions
): UseApiQueryResult<T> {
    const {
        enabled = true,
        cacheKey,
        staleTime = 5 * 60 * 1000,
    } = options || {};

    // Try to get initial data from cache
    const getCachedData = useCallback(() => {
        if (cacheKey && apiCache[cacheKey]) {
            const entry = apiCache[cacheKey];
            if (Date.now() - entry.timestamp < staleTime) {
                return entry.data as T;
            }
        }
        return null;
    }, [cacheKey, staleTime]);

    const [data, setData] = useState<T | null>(getCachedData);
    const [loading, setLoading] = useState(enabled && !data);
    const [error, setError] = useState<string | null>(null);

    const updateCache = useCallback((newData: T) => {
        if (cacheKey) {
            apiCache[cacheKey] = {
                data: newData,
                timestamp: Date.now(),
            };
            notifyCacheSubscribers(cacheKey, newData);
        }
    }, [cacheKey]);

    const refetch = useCallback(async () => {
        setLoading((prev) => prev || data === null);
        setError(null);
        try {
            const result = await fetcher();
            setData(result);
            updateCache(result);
        } catch (err: any) {
            const raw = err?.response?.data?.error;
            const message =
                (typeof raw === 'string' ? raw : raw?.message) ||
                err?.response?.data?.message ||
                err?.message ||
                'An unexpected error occurred';
            setError(message);
        } finally {
            setLoading(false);
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [...deps, data]);

    useEffect(() => {
        if (!enabled) {
            setLoading(false);
            return;
        }

        const cached = getCachedData();
        if (cached) {
            setData(cached);
            setLoading(false);
        }

        let ignore = false;
        const doFetch = async () => {
            // Preserve existing content during background revalidation.
            setLoading((prev) => !cached || prev);
            setError(null);
            try {
                const result = await fetcher();
                if (!ignore) {
                    setData(result);
                    updateCache(result);
                }
            } catch (err: any) {
                if (!ignore) {
                    const raw = err?.response?.data?.error;
                    const message =
                        (typeof raw === 'string' ? raw : raw?.message) ||
                        err?.response?.data?.message ||
                        err?.message ||
                        'An unexpected error occurred';
                    setError(message);
                }
            } finally {
                if (!ignore) setLoading(false);
            }
        };
        doFetch();

        return () => { ignore = true; };
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [...deps, enabled, cacheKey, staleTime]);

    useEffect(() => {
        if (!cacheKey) return;

        const subscriber: CacheSubscriber = (nextData) => {
            setData(nextData as T | null);
        };

        if (!cacheSubscribers[cacheKey]) {
            cacheSubscribers[cacheKey] = new Set();
        }
        cacheSubscribers[cacheKey].add(subscriber);

        return () => {
            const subs = cacheSubscribers[cacheKey];
            if (!subs) return;
            subs.delete(subscriber);
            if (subs.size === 0) {
                delete cacheSubscribers[cacheKey];
            }
        };
    }, [cacheKey]);

    return { data, loading, error, refetch };
}

