import { useState, useEffect, useCallback } from 'react';

interface UseApiQueryResult<T> {
    data: T | null;
    loading: boolean;
    error: string | null;
    refetch: () => Promise<void>;
}

const apiCache: Record<string, { data: any; timestamp: number }> = {};

export function useApiQuery<T>(
    fetcher: () => Promise<T>,
    deps: React.DependencyList = [],
    options?: { enabled?: boolean; cacheKey?: string; staleTime?: number }
): UseApiQueryResult<T> {
    const { enabled = true, cacheKey, staleTime = 5 * 60 * 1000 } = options || {};

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
        }
    }, [cacheKey]);

    const refetch = useCallback(async () => {
        setLoading(true);
        setError(null);
        try {
            const result = await fetcher();
            setData(result);
            updateCache(result);
        } catch (err: any) {
            setData(null);
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
    }, deps);

    useEffect(() => {
        if (!enabled) {
            setLoading(false);
            return;
        }

        const cached = getCachedData();
        if (cached) {
            setData(cached);
            setLoading(false);
            return;
        }

        let ignore = false;
        const doFetch = async () => {
            setLoading(true);
            setError(null);
            try {
                const result = await fetcher();
                if (!ignore) {
                    setData(result);
                    updateCache(result);
                }
            } catch (err: any) {
                if (!ignore) {
                    setData(null);
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

    return { data, loading, error, refetch };
}

