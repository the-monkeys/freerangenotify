import { lazy, Suspense, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';
import { bizBillingAPI } from '../services/api';
import type { TeamRole } from '../types';
import type { BillingMode } from '../types/bizBilling';
import { Button } from './ui/button';
import BizBillingSetup from './biz-billing/BizBillingSetup';
import BizBillingSettings from './biz-billing/BizBillingSettings';
import BizBillingCatalog from './biz-billing/BizBillingCatalog';
import { BillingLoadError } from './biz-billing/BillingForm';
import { useBizBillingResource } from './biz-billing/useBizBillingResource';

const LegacyWorkflows = lazy(() => import('./biz-billing/BizBillingLegacyWorkflows'));
const Sales = lazy(() => import('./biz-billing/BizBillingSalesPanel'));
const Operations = lazy(() => import('./biz-billing/BizBillingOpsPanel'));
const Insights = lazy(() => import('./biz-billing/BizBillingInsightsPanel'));
const views = ['setup', 'catalog', 'subscriptions', 'invoices', 'payments', 'settings', 'advanced'] as const;
type View = typeof views[number];
type Props = { apiKey: string; appId: string; role?: TeamRole; ownerId?: string };

function BillingShell({ apiKey, appId, role, ownerId }: Props) {
  const { user } = useAuth();
  const [params, setParams] = useSearchParams();
  const value = params.get('billing_view');
  const view: View = views.includes(value as View) ? value as View : 'setup';
  const canManage = role ? ['owner', 'admin', 'editor'].includes(role) : !!ownerId && ownerId === user?.user_id;
  const [mode, setMode] = useState<BillingMode>('test');
  const [advanced, setAdvanced] = useState('sales');
  const preferences = useBizBillingResource(`${appId}:preferences`, signal => bizBillingAPI.getBillingConfig(apiKey, 'preferences', signal));
  function navigate(next: View) {
    if (next === view) return;
    if (!window.dispatchEvent(new Event('billing:before-navigate', { cancelable: true })) && !window.confirm('Discard unsaved billing changes?')) return;
    setParams(previous => { const updated = new URLSearchParams(previous); updated.set('billing_view', next); return updated; }, { replace: true });
  }
  function changeMode(next: BillingMode) {
    if (next === mode) return;
    if (!window.dispatchEvent(new Event('billing:payment-mode-change', { cancelable: true })) && !window.confirm('Discard unsaved payment-account credentials?')) return;
    setMode(next);
  }
  const labels: Record<View, string> = { setup: 'Setup', catalog: 'Catalog', subscriptions: 'Subscriptions', invoices: 'Invoices', payments: 'Payments', settings: 'Business settings', advanced: 'Advanced' };
  return <section className="min-w-0 space-y-5" aria-label="Business billing">
    <header><h2 className="text-2xl font-semibold">Business billing</h2><p className="mt-1 text-sm text-muted-foreground">Bill your own customers for your products and services.</p></header>
    {!canManage && <p role="status" className="rounded-md border p-3 text-sm">Read-only billing access</p>}
    <nav aria-label="Business billing sections" className="flex flex-wrap gap-2">{views.map(item => <Button key={item} variant={view === item ? 'default' : 'outline'} aria-current={view === item ? 'page' : undefined} onClick={() => navigate(item)}>{labels[item]}</Button>)}</nav>
    {preferences.error ? <BillingLoadError label="currency preferences" retry={preferences.retry} /> : null}
    {view === 'setup' && <BizBillingSetup apiKey={apiKey} appId={appId} canManage={canManage} currency={preferences.data?.default_currency ?? 'INR'} mode={mode} onMode={changeMode} onSettings={() => navigate('settings')} onCatalog={() => navigate('catalog')} />}
    {view === 'settings' && <BizBillingSettings apiKey={apiKey} appId={appId} canManage={canManage} mode={mode} onMode={changeMode} onCurrencySaved={preferences.retry} />}
    {view === 'catalog' && <BizBillingCatalog apiKey={apiKey} appId={appId} canManage={canManage} preferences={preferences.error ? undefined : preferences.data} />}
    <Suspense fallback={<p role="status">Loading billing section…</p>}>
      {(view === 'subscriptions' || view === 'invoices' || view === 'payments') && <fieldset disabled={!canManage} className="min-w-0"><LegacyWorkflows key={`${appId}:${view}`} apiKey={apiKey} appId={appId} view={view} /></fieldset>}
      {view === 'advanced' && <div className="min-w-0 space-y-4"><div className="flex flex-wrap gap-2">{['sales', 'operations', 'insights'].map(item => <Button key={item} variant={advanced === item ? 'default' : 'outline'} onClick={() => setAdvanced(item)}>{item[0].toUpperCase() + item.slice(1)}</Button>)}</div><fieldset disabled={!canManage} className="min-w-0">{advanced === 'sales' && <Sales apiKey={apiKey} />}{advanced === 'operations' && <Operations apiKey={apiKey} />}{advanced === 'insights' && <Insights apiKey={apiKey} />}</fieldset></div>}
    </Suspense>
  </section>;
}
export default function AppBizBilling(props: Props) { return <BillingShell key={props.appId} {...props} />; }
