import React, { createContext, useContext, useState, useEffect, useCallback, ReactNode } from 'react';
import { applicationsAPI } from '../services/api';
import type { Application } from '../types';
import { useAuth } from './AuthContext';

interface AppsContextType {
  apps: Application[];
  loading: boolean;
  refreshApps: () => Promise<void>;
}

const AppsContext = createContext<AppsContextType | undefined>(undefined);

export const AppsProvider: React.FC<{ children: ReactNode }> = ({ children }) => {
  const [apps, setApps] = useState<Application[]>([]);
  const [loading, setLoading] = useState(false);
  const { isAuthenticated } = useAuth();

  const refreshApps = useCallback(async () => {
    if (!isAuthenticated) {
      setApps([]);
      return;
    }
    setLoading(true);
    try {
      const data = await applicationsAPI.list();
      setApps(Array.isArray(data) ? data : []);
    } catch (error) {
      console.error('Failed to fetch applications:', error);
    } finally {
      setLoading(false);
    }
  }, [isAuthenticated]);

  useEffect(() => {
    if (isAuthenticated) {
      refreshApps();
    } else {
      setApps([]);
    }
  }, [isAuthenticated, refreshApps]);

  return (
    <AppsContext.Provider value={{ apps, loading, refreshApps }}>
      {children}
    </AppsContext.Provider>
  );
};

export const useApps = () => {
  const context = useContext(AppsContext);
  if (context === undefined) {
    throw new Error('useApps must be used within an AppsProvider');
  }
  return context;
};
