import React, { createContext, useContext, useMemo } from 'react';
import { FreeRangeNotify } from '../../js/src';

export interface FreeRangeContextValue {
    client: FreeRangeNotify;
    userId: string;
    subscriberHash?: string;
}

const FreeRangeContext = createContext<FreeRangeContextValue | null>(null);

export interface FreeRangeProviderProps {
    /** Application API key (frn_xxx). */
    apiKey: string;
    /** Internal UUID of the authenticated user. */
    userId: string;
    /** Base URL of the FreeRangeNotify API (e.g. http://localhost:8080/v1). */
    apiBaseURL?: string;
    /** HMAC subscriber hash for authenticated SSE. */
    subscriberHash?: string;
    children: React.ReactNode;
}

/**
 * FreeRangeProvider initializes the JS SDK client and shares it with all
 * child components via React context. Wrap your app (or notification tree)
 * in this provider.
 *
 * ```tsx
 * <FreeRangeProvider apiKey="frn_xxx" userId="user-uuid" apiBaseURL="http://localhost:8080/v1">
 *   <NotificationBell />
 *   <Preferences />
 * </FreeRangeProvider>
 * ```
 */
export function FreeRangeProvider({
    apiKey,
    userId,
    apiBaseURL,
    subscriberHash,
    children,
}: FreeRangeProviderProps) {
    const client = useMemo(
        () => new FreeRangeNotify(apiKey, { baseURL: apiBaseURL }),
        [apiKey, apiBaseURL],
    );

    const value = useMemo<FreeRangeContextValue>(
        () => ({ client, userId, subscriberHash }),
        [client, userId, subscriberHash],
    );

    return (
        <FreeRangeContext.Provider value={value}>
            {children}
        </FreeRangeContext.Provider>
    );
}

/**
 * Access the FreeRangeNotify client and user info from context.
 * Must be used within a <FreeRangeProvider>.
 */
export function useFreeRange(): FreeRangeContextValue {
    const ctx = useContext(FreeRangeContext);
    if (!ctx) {
        throw new Error('useFreeRange must be used within a <FreeRangeProvider>');
    }
    return ctx;
}
