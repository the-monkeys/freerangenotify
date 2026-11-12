import { useState } from 'react';
import { bizBillingAPI } from '../../services/api';
import type { BillingMode, BillingUpgradeReport } from '../../types/bizBilling';
import { Button } from '../ui/button';
import { BillingActionStatus, BillingCard, BillingLoadError, billingSelectClass, formatBillingAmount, useBillingAction } from './BillingForm';
import { useBizBillingResource } from './useBizBillingResource';

type Props = { apiKey: string; appId: string; canManage: boolean; currency: string; mode: BillingMode; onMode: (mode: BillingMode) => void; onSettings: () => void; onCatalog: () => void };
export default function BizBillingSetup({ apiKey, appId, canManage, currency, mode, onMode, onSettings, onCatalog }: Props) {
  const lifecycle = useBizBillingResource(`${appId}:lifecycle`, signal => bizBillingAPI.getLifecycle(apiKey, signal));
  const merchant = useBizBillingResource(`${appId}:merchant:${mode}`, signal => bizBillingAPI.getMerchant(apiKey, mode, signal));
  const [report, setReport] = useState<BillingUpgradeReport>();
  const [confirmed, setConfirmed] = useState(false);
  const action = useBillingAction();
  const manualReady = !lifecycle.error && ["legacy", "active"].includes(lifecycle.data?.status ?? "");
  return <div className="space-y-5">
    <div><h3 className="text-xl font-semibold">Set up customer billing</h3><p className="mt-1 text-sm text-muted-foreground">Create products, invoice your customers and track payments. Online collection is optional.</p></div>
    <div className="grid min-w-0 gap-4 lg:grid-cols-2">
      <BillingCard title="1. Your business">
        <p className="text-sm">Choose your business details, currency, tax and invoice numbering.</p>
        <Button onClick={onSettings}>Configure business details</Button>
      </BillingCard>
      <BillingCard title="2. What you sell">
        <p className="text-sm">A product describes your service. Its plans set prices and billing frequency.</p>
        <Button variant="outline" onClick={onCatalog}>Open catalog</Button>
      </BillingCard>
      <BillingCard title="3. Collect payments">
        <p className={manualReady ? "font-medium text-emerald-700 dark:text-emerald-400" : "text-sm text-muted-foreground"}>{manualReady ? "Manual payments available" : lifecycle.loading ? "Checking billing readiness…" : "Confirm billing readiness before recording payments."}</p>
        <p className="text-sm">Record staff-confirmed cash, direct UPI or bank payments. Connect your own Razorpay account when you want online checkout or customer-authorized automatic payments.</p>
        <label className="block text-sm">Payment account environment<select aria-label="Payment account environment" value={mode} onChange={e => onMode(e.target.value as BillingMode)} className={`${billingSelectClass} mt-2`}><option value="test">Test</option><option value="live">Live</option></select></label>
        {merchant.loading && <p role="status" className="text-sm">Loading payment account…</p>}
        {merchant.error ? <BillingLoadError label="payment account" retry={merchant.retry} /> : merchant.data && <div className="space-y-1 text-sm">
          <p>{mode === 'test' ? 'Test' : 'Live'} account: {merchant.data.configured ? merchant.data.account_id || 'connected' : 'not connected'}</p>
          {merchant.data.configured && <><p>API credentials: {merchant.data.api_authenticated ? 'authenticated' : 'verification needed'}</p><p>Account linkage: {merchant.data.account_verified ? 'verified' : 'signed callback verification needed'}</p><p>Automatic collection: {merchant.data.automatic_ready ? 'available after customer authorization' : 'not ready'}</p></>}
        </div>}
        <Button variant="outline" onClick={onSettings}>Configure payment account</Button>
      </BillingCard>
      <BillingCard title="4. Billing readiness">
        {lifecycle.error ? <BillingLoadError label="billing readiness" retry={lifecycle.retry} /> : lifecycle.loading ? <p role="status">Loading billing readiness…</p> : <>
          <p className="text-sm">{lifecycle.data?.lifecycle_version === 2 ? 'Recoverable customer billing is active.' : 'Existing billing remains available. Review saved records before upgrading this application.'}</p>
          {!lifecycle.data?.new_collection_enabled && <p className="text-sm">New online collection is disabled by the deployment administrator.</p>}
          {lifecycle.data?.lifecycle_version !== 2 && <Button disabled={!canManage || action.busy} onClick={() => void action.run(async () => { setReport(await bizBillingAPI.reviewUpgrade(apiKey, currency)); setConfirmed(false); }, 'Review prepared. Your billing records have not changed.')}>Review existing billing</Button>}
        </>}
        <BillingActionStatus action={action} />
      </BillingCard>
    </div>
    {report && <BillingCard title="Review before enabling upgraded billing">
      <p className="text-sm">Currency: {report.primary_currency}. Existing invoice numbers, amounts and payment-account references are preserved.</p>
      {!!report.issues?.length && <div role="alert" className="space-y-2 text-sm"><p>{report.issues.length} records need an explicit decision before activation.</p><ul className="list-inside list-disc">{report.issues.map(issue => <li key={issue.id} className="break-all">{issue.code.replace(/_/g, ' ')} — {issue.resource_id}</li>)}</ul></div>}
      {!!report.opening_balances?.length && <ul className="space-y-1 text-sm">{report.opening_balances.map((opening, i) => <li key={i} className="break-all">Customer {opening.scope.user_id}: {formatBillingAmount(opening.available_paisa, opening.scope.currency)}</li>)}</ul>}
      {!!report.legacy_gateway_bindings?.length && <details className="text-sm"><summary>Retained payment-account references</summary>{report.legacy_gateway_bindings.map(ref => <p className="break-all" key={ref.kind + ref.id}>{ref.kind}: {ref.id}</p>)}</details>}
      {!report.writers_upgraded && <p className="text-sm">A deployment administrator must confirm that all billing services are ready. This confirmation cannot be supplied by this form.</p>}
      <label className="flex items-start gap-2 text-sm"><input type="checkbox" checked={confirmed} onChange={e => setConfirmed(e.target.checked)} disabled={!canManage || !report.ready} /><span>I reviewed the balances, currencies and retained payment references.</span></label>
      <Button disabled={!canManage || !confirmed || !report.ready || !report.writers_upgraded || !lifecycle.data?.new_collection_enabled || action.busy} onClick={() => void action.run(async () => { const saved = await bizBillingAPI.persistUpgradeReview(apiKey, report); if (!saved.ready || !saved.writers_upgraded) throw new Error('Review changed'); await bizBillingAPI.activateUpgrade(apiKey, saved.id); lifecycle.retry(); setReport(undefined); }, 'Upgraded billing enabled.', 'The review changed or is not ready. Prepare a fresh review before activation.')}>Enable upgraded billing</Button>
    </BillingCard>}
  </div>;
}
