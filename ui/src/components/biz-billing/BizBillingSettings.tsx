import { useEffect, useRef, useState, type ReactNode } from 'react';
import { bizBillingAPI, buildApiUrl } from '../../services/api';
import type { BillingConfigMap, BillingMode, BillingPreferences } from '../../types/bizBilling';
import { Button } from '../ui/button';
import { Input } from '../ui/input';
import { BillingActionStatus, BillingCard, BillingField, BillingLoadError, billingSelectClass, useBillingAction } from './BillingForm';
import { useBizBillingResource } from './useBizBillingResource';

type Props = { apiKey: string; appId: string; canManage: boolean; mode: BillingMode; onMode: (mode: BillingMode) => void; onCurrencySaved: () => void };

function ConfigSection<K extends keyof BillingConfigMap>({ apiKey, appId, canManage, kind, title, saveLabel, onSaved, children }: {
  apiKey: string; appId: string; canManage: boolean; kind: K; title: string; saveLabel: string;
  onSaved?: () => void; children: (value: BillingConfigMap[K], change: (next: BillingConfigMap[K]) => void) => ReactNode;
}) {
  const resource = useBizBillingResource(`${appId}:config:${kind}`, signal => bizBillingAPI.getBillingConfig(apiKey, kind, signal));
  const [form, setForm] = useState<BillingConfigMap[K]>();
  const dirty = useRef(false);
  const action = useBillingAction();
  useEffect(() => { if (resource.data && !dirty.current) setForm(resource.data); }, [resource.data]);
  useEffect(() => {
    const leave = (event: BeforeUnloadEvent) => { if (dirty.current) { event.preventDefault(); event.returnValue = ''; } };
    const navigate = (event: Event) => { if (dirty.current) event.preventDefault(); };
    window.addEventListener('beforeunload', leave); window.addEventListener('billing:before-navigate', navigate);
    return () => { window.removeEventListener('beforeunload', leave); window.removeEventListener('billing:before-navigate', navigate); };
  }, []);
  return <BillingCard title={title}>
    {resource.error ? <BillingLoadError label={title.toLowerCase()} retry={resource.retry} /> : null}
    {!form ? <p role="status">Loading {title.toLowerCase()}…</p> : <form onSubmit={event => {
      event.preventDefault();
      const payload = kind === 'preferences' ? { default_currency: (form as BillingPreferences).default_currency } : form;
      void action.run(async () => {
        const result = await bizBillingAPI.saveBillingConfig(apiKey, kind, payload as Partial<BillingConfigMap[K]>);
        setForm({ ...form, ...result }); dirty.current = false; resource.retry(); onSaved?.();
      }, `${title} saved.`);
    }} className="space-y-4">
      <fieldset disabled={!canManage || action.busy || resource.loading} className="grid min-w-0 gap-4 sm:grid-cols-2">
        {children(form, next => { dirty.current = true; setForm(next); })}
        <div className="sm:col-span-2"><Button type="submit">{action.busy ? 'Saving…' : saveLabel}</Button></div>
      </fieldset>
      <BillingActionStatus action={action} />
    </form>}
  </BillingCard>;
}

function GatewaySettings({ apiKey, appId, canManage, mode }: Omit<Props, 'onMode' | 'onCurrencySaved'>) {
  const merchant = useBizBillingResource(`${appId}:merchant:${mode}`, signal => bizBillingAPI.getMerchant(apiKey, mode, signal));
  const [account, setAccount] = useState(''), [keyID, setKeyID] = useState(''), [secret, setSecret] = useState(''), [webhook, setWebhook] = useState('');
  const dirty = useRef(false);
  const action = useBillingAction();
  useEffect(() => { if (merchant.data && !dirty.current) setAccount(merchant.data.account_id ?? ''); }, [merchant.data]);
  useEffect(() => {
    const navigate = (event: Event) => { if (dirty.current) event.preventDefault(); };
    window.addEventListener('billing:before-navigate', navigate);
    window.addEventListener('billing:payment-mode-change', navigate);
    return () => { window.removeEventListener('billing:before-navigate', navigate); window.removeEventListener('billing:payment-mode-change', navigate); };
  }, []);
  const changed = (setter: (value: string) => void, value: string) => { dirty.current = true; setter(value); };
  return <div className="space-y-4">
    {merchant.error ? <BillingLoadError label="payment account" retry={merchant.retry} /> : null}
    <p className="text-sm">Use your business's own account. Leave saved credential fields empty to keep them. Credentials are write-only.</p>
    {merchant.loading && <p role="status">Loading payment account…</p>}
    <Button type="button" variant="outline" disabled={merchant.loading || action.busy} onClick={merchant.retry}>Refresh payment account status</Button>
    {merchant.data?.configured && <p className="break-words text-sm">Saved {mode} account: {merchant.data.account_id || 'connected'}</p>}
    <form className="space-y-4" onSubmit={event => { event.preventDefault(); if (!merchant.data) return; void action.run(async () => {
      await bizBillingAPI.saveMerchant(apiKey, mode, { expected_revision: merchant.data!.revision, account_id: account.trim(), ...(keyID ? { key_id: keyID } : {}), ...(secret ? { key_secret: secret } : {}), ...(webhook ? { webhook_secret: webhook } : {}) });
      setKeyID(''); setSecret(''); setWebhook(''); dirty.current = false; merchant.retry();
    }, 'Payment account saved.', 'Could not save the payment account. Check the account details and reload its current status before trying again.'); }}>
      <fieldset disabled={!canManage || action.busy || merchant.loading || !merchant.data || !!merchant.error} className="grid min-w-0 gap-4 sm:grid-cols-2">
        <BillingField label="Razorpay account ID"><Input autoComplete="off" required value={account} onChange={e => changed(setAccount, e.target.value)} /></BillingField>
        <BillingField label="Razorpay key ID"><Input autoComplete="off" required={!merchant.data?.configured} value={keyID} onChange={e => changed(setKeyID, e.target.value)} /></BillingField>
        <BillingField label="Razorpay key secret"><Input type="password" autoComplete="new-password" required={!merchant.data?.configured} value={secret} onChange={e => changed(setSecret, e.target.value)} /></BillingField>
        <BillingField label="Razorpay webhook secret"><Input type="password" autoComplete="new-password" value={webhook} onChange={e => changed(setWebhook, e.target.value)} /></BillingField>
        <div className="sm:col-span-2"><Button type="submit">{action.busy ? 'Saving…' : 'Save payment account'}</Button></div>
      </fieldset>
      <BillingActionStatus action={action} />
    </form>
    {merchant.data?.connection_id && <div className="space-y-2"><BillingField label="Razorpay webhook URL"><Input readOnly value={new URL(buildApiUrl(`/webhooks/biz/razorpay/${encodeURIComponent(merchant.data.connection_id)}`), window.location.origin).href} /></BillingField><p className="text-sm text-muted-foreground">Configure signed Razorpay callbacks for this connection. API authentication, verified account linkage and automatic-payment readiness are checked separately.</p></div>}
    {merchant.data?.configured && <Button variant="outline" disabled={!canManage || action.busy || merchant.loading} onClick={() => {
      if (window.confirm('Disconnect new online collection? Existing payments and their recovery records will be retained.')) void action.run(async () => { await bizBillingAPI.disconnectMerchant(apiKey, mode, merchant.data!.revision); setKeyID(''); setSecret(''); setWebhook(''); dirty.current = false; merchant.retry(); }, 'New collection disconnected. Existing payment recovery remains available.');
    }}>Disconnect payment account</Button>}
  </div>;
}

export default function BizBillingSettings(props: Props) {
  const { mode, onMode } = props;
  return <div className="space-y-5">
    <div><h3 className="text-xl font-semibold">Business settings</h3><p className="mt-1 text-sm text-muted-foreground">Defaults apply to new forms and future invoices. Saved financial records keep their original currency and amounts.</p></div>
    <ConfigSection {...props} kind="preferences" title="Currency preferences" saveLabel="Save default currency" onSaved={props.onCurrencySaved}>{(value, change) => <>
      <BillingField label="Default currency"><select className={billingSelectClass} value={value.default_currency} onChange={e => change({ ...value, default_currency: e.target.value })}>{[...new Set([value.default_currency, ...(value.supported_currencies ?? ['INR'])])].map(currency => <option key={currency} value={currency}>{currency}</option>)}</select></BillingField>
      <p className="self-center text-sm text-muted-foreground">This selects the currency for new forms. Currency conversion is not performed.</p>
    </>}</ConfigSection>
    <ConfigSection {...props} kind="branding" title="Business details" saveLabel="Save business details">{(value, change) => <>
      <BillingField label="Business name"><Input required maxLength={255} value={value.company_name ?? ''} onChange={e => change({ ...value, company_name: e.target.value })} /></BillingField>
      <BillingField label="Business email"><Input type="email" value={value.email ?? ''} onChange={e => change({ ...value, email: e.target.value })} /></BillingField>
      <BillingField label="Business phone"><Input type="tel" value={value.phone ?? ''} onChange={e => change({ ...value, phone: e.target.value })} /></BillingField>
      <BillingField label="Invoice footer"><Input value={value.footer_text ?? ''} onChange={e => change({ ...value, footer_text: e.target.value })} /></BillingField>
      <BillingField label="Address"><Input value={value.address?.line1 ?? ''} onChange={e => change({ ...value, address: { ...value.address, line1: e.target.value } })} /></BillingField>
      <BillingField label="City"><Input value={value.address?.city ?? ''} onChange={e => change({ ...value, address: { ...value.address, city: e.target.value } })} /></BillingField>
      <BillingField label="State"><Input value={value.address?.state ?? ''} onChange={e => change({ ...value, address: { ...value.address, state: e.target.value } })} /></BillingField>
      <BillingField label="Postal code"><Input value={value.address?.pincode ?? ''} onChange={e => change({ ...value, address: { ...value.address, pincode: e.target.value } })} /></BillingField>
      <BillingField label="Country code"><Input maxLength={2} placeholder="IN" value={value.address?.country ?? ''} onChange={e => change({ ...value, address: { ...value.address, country: e.target.value.toUpperCase() } })} /></BillingField>
    </>}</ConfigSection>
    <ConfigSection {...props} kind="tax" title="Tax settings" saveLabel="Save tax settings">{(value, change) => <>
      <BillingField label="Default tax rate (%)"><Input type="number" min={0} max={28} step={1} required value={value.default_tax_rate} onChange={e => change({ ...value, default_tax_rate: Number(e.target.value) })} /></BillingField>
      <BillingField label="Business GSTIN"><Input maxLength={15} value={value.business_gstin ?? ''} onChange={e => change({ ...value, business_gstin: e.target.value.toUpperCase() })} /></BillingField>
      <BillingField label="Business state code"><Input value={value.business_state ?? ''} onChange={e => change({ ...value, business_state: e.target.value })} /></BillingField>
      <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={value.tax_inclusive} onChange={e => change({ ...value, tax_inclusive: e.target.checked })} />Prices include tax by default</label>
    </>}</ConfigSection>
    <ConfigSection {...props} kind="numbering" title="Invoice numbering" saveLabel="Save numbering">{(value, change) => <>
      <BillingField label="Invoice prefix"><Input required maxLength={30} value={value.invoice_prefix ?? 'INV'} onChange={e => change({ ...value, invoice_prefix: e.target.value })} /></BillingField>
      <BillingField label="Estimate prefix"><Input required maxLength={30} value={value.estimate_prefix ?? 'EST'} onChange={e => change({ ...value, estimate_prefix: e.target.value })} /></BillingField>
      <BillingField label="Credit note prefix"><Input required maxLength={30} value={value.credit_note_prefix ?? 'CN'} onChange={e => change({ ...value, credit_note_prefix: e.target.value })} /></BillingField>
      <BillingField label="Contract prefix"><Input required maxLength={30} value={value.contract_prefix ?? 'CTR'} onChange={e => change({ ...value, contract_prefix: e.target.value })} /></BillingField>
      <p className="text-sm text-muted-foreground sm:col-span-2">Issued numbers are preserved. Saving a prefix does not restart the document sequence.</p>
    </>}</ConfigSection>
    <ConfigSection {...props} kind="payment_terms" title="Payment terms" saveLabel="Save payment terms">{(value, change) => <>
      <BillingField label="Default payment terms"><select className={billingSelectClass} value={value.default_terms} onChange={e => change({ ...value, default_terms: e.target.value })}>{['due_on_receipt', 'net_15', 'net_30', 'net_60', 'custom'].map(term => <option key={term} value={term}>{term.replace(/_/g, ' ')}</option>)}</select></BillingField>
      {value.default_terms === 'custom' && <BillingField label="Payment due in days"><Input type="number" required min={0} max={365} value={value.custom_days ?? 0} onChange={e => change({ ...value, custom_days: Number(e.target.value) })} /></BillingField>}
    </>}</ConfigSection>
    <BillingCard title="Online payment account (optional)">
      <BillingField label="Payment account environment"><select className={billingSelectClass} value={mode} onChange={e => onMode(e.target.value as BillingMode)}><option value="test">Test</option><option value="live">Live</option></select></BillingField>
      <GatewaySettings key={`${props.appId}:${mode}`} {...props} />
    </BillingCard>
  </div>;
}
