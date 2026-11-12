import { Children, cloneElement, useEffect, useId, useRef, useState, type ReactElement, type ReactNode } from 'react';
import { Button } from '../ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '../ui/card';

export const billingSelectClass = 'h-10 w-full min-w-0 rounded-md border border-input bg-background px-3 text-sm disabled:opacity-50';
export function BillingField({ label, children }: { label: string; children: ReactNode }) {
  const generatedID = useId();
  const control = Children.only(children) as ReactElement<{ id?: string }>;
  const id = control.props.id ?? generatedID;
  return <div className="flex min-w-0 flex-col gap-2 text-sm font-medium"><label htmlFor={id}>{label}</label>{cloneElement(control, { id })}</div>;
}
export function BillingCard({ title, children }: { title: string; children: ReactNode }) {
  return <Card className="min-w-0"><CardHeader><CardTitle className="break-words text-base [overflow-wrap:anywhere]">{title}</CardTitle></CardHeader><CardContent className="space-y-4">{children}</CardContent></Card>;
}
export function BillingLoadError({ label, retry }: { label: string; retry: () => void }) {
  return <div role="alert" className="space-y-2 rounded-md border border-destructive/30 p-3 text-sm"><p>Could not load {label}.</p><Button type="button" variant="outline" onClick={retry}>Retry {label}</Button></div>;
}
export function useBillingAction() {
  const alive = useRef(true);
  useEffect(() => { alive.current = true; return () => { alive.current = false; }; }, []);
  const [busy, setBusy] = useState(false), [error, setError] = useState(''), [message, setMessage] = useState('');
  async function run(action: () => Promise<void>, success: string, failure = 'Could not save these changes. Check the current settings and try again.') {
    if (busy) return;
    setBusy(true); setError(''); setMessage('');
    try { await action(); if (alive.current) setMessage(success); }
    catch { if (alive.current) setError(failure); }
    finally { if (alive.current) setBusy(false); }
  }
  return { busy, error, message, run };
}
export function BillingActionStatus({ action }: { action: ReturnType<typeof useBillingAction> }) {
  return <>{action.error && <p role="alert" className="text-sm text-destructive">{action.error}</p>}{action.message && <p role="status" className="text-sm text-emerald-700 dark:text-emerald-400">{action.message}</p>}</>;
}
export function parseBillingAmount(input: string): number {
  if (!/^\d+(\.\d{1,2})?$/.test(input.trim())) throw new Error('Enter an amount with at most two decimal places.');
  const [major, decimals = ''] = input.trim().split('.');
  const amount = Number(major) * 100 + Number(decimals.padEnd(2, '0'));
  if (!Number.isSafeInteger(amount) || amount < 0) throw new Error('Amount is outside the supported range.');
  return amount;
}
export function formatBillingAmount(amount: number, currency: string): string {
  if (!Number.isSafeInteger(amount)) return `${currency} —`;
  const negative = amount < 0, value = Math.abs(amount);
  return `${currency} ${negative ? '-' : ''}${Math.trunc(value / 100)}.${String(value % 100).padStart(2, '0')}`;
}
