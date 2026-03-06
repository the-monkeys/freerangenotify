import { useState, useEffect, useCallback } from 'react';

interface UseApiQueryResult<T> {
    data: T | null;
    loading: boolean;
    error: string | null;
    refetch: () => Promise<void>;
}

export function useApiQuery<T>(
    fetcher: () => Promise<T>,
    deps: React.DependencyList = [],
    options?: { enabled?: boolean }
): UseApiQueryResult<T> {
    const [data, setData] = useState<T | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    const refetch = useCallback(async () => {
        setLoading(true);
        setError(null);
        try {
            const result = await fetcher();
            setData(result);
        } catch (err: any) {
            const message =
                err?.response?.data?.error ||
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
        if (options?.enabled === false) {
            setLoading(false);
            return;
        }
        refetch();
    }, [refetch, options?.enabled]);

    return { data, loading, error, refetch };
}
