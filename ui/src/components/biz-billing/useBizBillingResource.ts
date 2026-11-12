import { useCallback, useEffect, useRef, useState } from 'react';

export function useBizBillingResource<T>(key: string, load: (signal: AbortSignal) => Promise<T>) {
  const loader = useRef(load);
  loader.current = load;
  const [attempt, setAttempt] = useState(0);
  const [state, setState] = useState<{ key: string; data?: T; loading: boolean; error?: unknown }>({ key, loading: true });
  useEffect(() => {
    const controller = new AbortController();
    let current = true;
    setState(old => ({ key, data: old.key === key ? old.data : undefined, loading: true }));
    Promise.resolve().then(() => loader.current(controller.signal)).then(data => {
      if (current) setState({ key, data, loading: false });
    }).catch(error => {
      if (current && !controller.signal.aborted) setState(old => ({ key, data: old.key === key ? old.data : undefined, loading: false, error }));
    });
    return () => { current = false; controller.abort(); };
  }, [key, attempt]);
  const retry = useCallback(() => setAttempt(n => n + 1), []);
  useEffect(() => { window.addEventListener('billing:changed', retry); return () => window.removeEventListener('billing:changed', retry); }, [retry]);
  // The key fence also protects the render before effect cleanup runs.
  return { data: state.key === key ? state.data : undefined, loading: state.key !== key || state.loading, error: state.key === key ? state.error : undefined, retry };
}

export function useBillingSearch(value: string) {
  const [search, setSearch] = useState(value);
  useEffect(() => { const timer = setTimeout(() => setSearch(value), 250); return () => clearTimeout(timer); }, [value]);
  return search;
}
