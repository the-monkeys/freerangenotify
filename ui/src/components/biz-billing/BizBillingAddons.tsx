import { useState } from 'react';
import { bizBillingAPI } from '../../services/api';
import type { BizPlan } from '../../types';
import { Button } from '../ui/button';
import { Input } from '../ui/input';
import { BillingActionStatus, BillingField, BillingLoadError, billingSelectClass, formatBillingAmount, parseBillingAmount, useBillingAction } from './BillingForm';
import { useBizBillingResource } from './useBizBillingResource';

export default function BizBillingAddons({ apiKey, appId, plan, canManage }: { apiKey: string; appId: string; plan: BizPlan; canManage: boolean }) {
  const addons = useBizBillingResource(`${appId}:addons:${plan.id}`, () => bizBillingAPI.listPlanAddons(apiKey, plan.id!));
  const [name, setName] = useState(''), [price, setPrice] = useState(''), [cycle, setCycle] = useState('');
  const [validation, setValidation] = useState('');
  const action = useBillingAction();
  return <div className="space-y-4">
    <p className="text-sm">Add-ons use this plan's currency, {plan.currency}. Existing invoice amounts keep their saved values.</p>
    {addons.error ? <BillingLoadError label="plan add-ons" retry={addons.retry} /> : null}
    {addons.loading && <p role="status">Loading plan add-ons…</p>}
    <ul className="space-y-3">{(addons.data ?? []).map(addon => <li key={addon.id} className="flex min-w-0 flex-wrap items-center justify-between gap-3 rounded-md border p-3"><div className="min-w-0"><p className="break-words font-medium">{addon.name}</p><p className="text-sm">{formatBillingAmount(addon.amount_paisa, plan.currency)}</p></div><Button type="button" variant="outline" aria-label={`Remove add-on ${addon.name}`} disabled={!canManage || action.busy} onClick={() => void action.run(async () => { await bizBillingAPI.removePlanAddon(apiKey, plan.id!, addon.id!); addons.retry(); }, 'Add-on removed.', 'This add-on could not be removed. Referenced billing records must be preserved.')}>Remove</Button></li>)}</ul>
    <form className="space-y-4" onSubmit={event => {
      event.preventDefault(); setValidation('');
      let amount: number;
      try { amount = parseBillingAmount(price); if (amount <= 0) throw new Error('Add-on price must be greater than zero.'); }
      catch (error) { setValidation(error instanceof Error ? error.message : 'Check the add-on price.'); return; }
      void action.run(async () => { await bizBillingAPI.addPlanAddon(apiKey, plan.id!, { name: name.trim(), amount_paisa: amount, ...(cycle ? { billing_cycle: cycle } : {}) }); setName(''); setPrice(''); addons.retry(); }, 'Add-on saved.');
    }}>
      <fieldset disabled={!canManage || action.busy} className="space-y-4">
        <BillingField label="Add-on name"><Input required maxLength={255} value={name} onChange={e => setName(e.target.value)} /></BillingField>
        <BillingField label={`Add-on price (${plan.currency})`}><Input inputMode="decimal" required value={price} onChange={e => setPrice(e.target.value)} /></BillingField>
        <BillingField label="Add-on billing frequency"><select className={billingSelectClass} value={cycle} onChange={e => setCycle(e.target.value)}><option value="">Same as plan</option>{['weekly', 'monthly', 'quarterly', 'half_yearly', 'yearly'].map(value => <option key={value} value={value}>{value.replace(/_/g, ' ')}</option>)}</select></BillingField>
        {validation && <p role="alert" className="text-sm text-destructive">{validation}</p>}
        <Button type="submit">{action.busy ? 'Saving…' : 'Add add-on'}</Button>
      </fieldset>
    </form>
    <BillingActionStatus action={action} />
  </div>;
}
