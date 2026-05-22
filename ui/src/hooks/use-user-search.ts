import { useApiQuery } from './use-api-query';
import { useDebounce } from './use-debounce';
import { usersAPI } from '../services/api';
import type { User } from '../types';

interface UseUserSearchOptions {
    page?: number;
    pageSize?: number;
    enabled?: boolean;
    debounceMs?: number;
}

interface UseUserSearchResult {
    users: User[];
    totalCount: number;
    loading: boolean;
    error: string | null;
    refetch: () => Promise<void>;
}

export function useUserSearch(
    apiKey: string,
    searchTerm: string,
    options?: UseUserSearchOptions
): UseUserSearchResult {
    const {
        page = 1,
        pageSize = 50,
        enabled = true,
        debounceMs = 300,
    } = options ?? {};

    const debouncedSearch = useDebounce(searchTerm, debounceMs);

    const {
        data,
        loading,
        error,
        refetch,
    } = useApiQuery(
        () => usersAPI.list(apiKey, page, pageSize, debouncedSearch),
        [apiKey, page, pageSize, debouncedSearch],
        {
            enabled: enabled && !!apiKey,
            cacheKey: `users-search-${apiKey}-${page}-${pageSize}-${debouncedSearch}`,
            staleTime: 30000,
        }
    );

    return {
        users: data?.users ?? [],
        totalCount: data?.total_count ?? 0,
        loading,
        error,
        refetch,
    };
}

export function formatUserLabel(user: User): string {
    const email = user.email || 'No email';
    return user.external_id ? `${email} (${user.external_id})` : email;
}
