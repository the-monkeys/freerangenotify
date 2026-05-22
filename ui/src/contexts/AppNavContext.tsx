import React, { createContext, useContext, useMemo, useState } from 'react';

interface AppNavContextValue {
    appId: string | null;
    appName: string;
    unreadCount: number;
    setAppId: (id: string | null) => void;
    setAppName: (name: string) => void;
    setUnreadCount: (count: number) => void;
}

const AppNavContext = createContext<AppNavContextValue | null>(null);

export const AppNavProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
    const [appId, setAppId] = useState<string | null>(null);
    const [appName, setAppName] = useState('');
    const [unreadCount, setUnreadCount] = useState(0);

    const value = useMemo(
        () => ({
            appId,
            appName,
            unreadCount,
            setAppId,
            setAppName,
            setUnreadCount,
        }),
        [appId, appName, unreadCount],
    );

    return <AppNavContext.Provider value={value}>{children}</AppNavContext.Provider>;
};

export function useAppNav(): AppNavContextValue {
    const ctx = useContext(AppNavContext);
    if (!ctx) {
        throw new Error('useAppNav must be used within AppNavProvider');
    }
    return ctx;
}

/** Safe read for sidebar when provider may not wrap (should not happen in dashboard) */
export function useAppNavOptional(): AppNavContextValue | null {
    return useContext(AppNavContext);
}
