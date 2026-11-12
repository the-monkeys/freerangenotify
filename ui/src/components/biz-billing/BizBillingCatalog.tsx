import { useState } from 'react';
import { bizBillingAPI } from '../../services/api';
import type { BizPlan, BizProduct } from '../../types';
import type { BillingPreferences } from '../../types/bizBilling';
import { Button } from '../ui/button';
import { Input } from '../ui/input';
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription } from '../ui/dialog';
import { BillingActionStatus, BillingCard, BillingField, BillingLoadError, billingSelectClass, formatBillingAmount, parseBillingAmount, useBillingAction } from './BillingForm';
import { useBillingSearch, useBizBillingResource } from './useBizBillingResource';
import BizBillingAddons from './BizBillingAddons';

type Props = { apiKey: string; appId: string; canManage: boolean; preferences?: BillingPreferences };
type ProductFormProps = Props & { product?: BizProduct; close: () => void; saved: () => void };

function ProductForm({ apiKey, canManage, product, close, saved }: ProductFormProps) {
  const [name, setName] = useState(product?.name ?? ''), [description, setDescription] = useState(product?.description ?? '');
  const [hsn, setHSN] = useState(product?.hsn_code ?? ''), [unit, setUnit] = useState(product?.unit ?? '');
  const [tax, setTax] = useState(product?.tax_rate ?? 0);
  const action = useBillingAction();
  return <form className="space-y-4" onSubmit={event => { event.preventDefault(); void action.run(async () => {
    const payload = { name: name.trim(), description, hsn_code: hsn, unit, tax_rate: tax };
    if (product?.id) await bizBillingAPI.updateProduct(apiKey, product.id, payload); else await bizBillingAPI.createProduct(apiKey, payload);
    saved(); close();
  }, 'Product saved.'); }}>
    <fieldset disabled={!canManage || action.busy} className="space-y-4">
      <BillingField label="Product name"><Input required maxLength={255} value={name} onChange={e => setName(e.target.value)} /></BillingField>
      <BillingField label="Product description"><Input maxLength={1000} value={description} onChange={e => setDescription(e.target.value)} /></BillingField>
      <div className="grid gap-4 sm:grid-cols-2">
        <BillingField label="HSN / SAC code"><Input maxLength={8} value={hsn} onChange={e => setHSN(e.target.value)} /></BillingField>
        <BillingField label="Unit"><Input maxLength={50} value={unit} onChange={e => setUnit(e.target.value)} placeholder="month, hour, item" /></BillingField>
        <BillingField label="Product tax rate (%)"><Input type="number" min={0} max={28} required value={tax} onChange={e => setTax(Number(e.target.value))} /></BillingField>
      </div>
      <Button type="submit">{action.busy ? 'Saving…' : 'Save product'}</Button>
    </fieldset>
    <BillingActionStatus action={action} />
  </form>;
}

function PlanForm({ apiKey, appId, canManage, preferences, plan, saved, close }: Props & { plan?: BizPlan; close: () => void; saved: () => void }) {
  const [name, setName] = useState(plan?.name ?? ''), [productID, setProduct] = useState(plan?.product_id ?? '');
  const [productSearch, setProductSearch] = useState(''), [productOffset, setProductOffset] = useState(0);
  const search = useBillingSearch(productSearch);
  const products = useBizBillingResource(`${appId}:plan-products:${search}:${productOffset}`, signal => bizBillingAPI.listProductsPage(apiKey, { search, offset: productOffset, limit: 20, active: true }, signal));
  const [price, setPrice] = useState(String((plan?.amount_paisa ?? 0) / 100)), [currency, setCurrency] = useState(plan?.currency ?? preferences?.default_currency ?? 'INR');
  const [cycle, setCycle] = useState(plan?.billing_cycle ?? 'monthly'), [cycleDays, setCycleDays] = useState(plan?.cycle_days ?? 30);
  const [model, setModel] = useState<NonNullable<BizPlan['pricing_model']>>(plan?.pricing_model ?? 'flat');
  const [trial, setTrial] = useState(plan?.trial_days ?? 0), [setup, setSetup] = useState(String((plan?.setup_fee_paisa ?? 0) / 100));
  const [inclusive, setInclusive] = useState(plan?.tax_inclusive ?? false);
  const [tiers, setTiers] = useState((plan?.tiers ?? []).map(tier => ({ upTo: String(tier.up_to), unit: String(tier.unit_paisa / 100), flat: String((tier.flat_paisa ?? 0) / 100) })));
  const [validation, setValidation] = useState('');
  const action = useBillingAction();
  const options = products.data?.data ?? [];
  return <form className="space-y-4" onSubmit={event => {
    event.preventDefault(); setValidation('');
    let payload: Partial<BizPlan>;
    try {
      const normalizedTiers = tiers.map(tier => ({ up_to: Number(tier.upTo), unit_paisa: parseBillingAmount(tier.unit), flat_paisa: parseBillingAmount(tier.flat) }));
      if ((model === 'tiered' || model === 'volume') && (!normalizedTiers.length || normalizedTiers.some(tier => !Number.isSafeInteger(tier.up_to) || tier.up_to < 0))) throw new Error('Add valid whole-number pricing tiers.');
      payload = { name: name.trim(), product_id: productID, amount_paisa: parseBillingAmount(price), currency, billing_cycle: cycle, cycle_days: cycle === 'custom' ? cycleDays : 0, pricing_model: model, trial_days: trial, setup_fee_paisa: parseBillingAmount(setup), tax_inclusive: inclusive, tiers: model === 'tiered' || model === 'volume' ? normalizedTiers : [] };
    } catch (error) { setValidation(error instanceof Error ? error.message : 'Check the plan amounts.'); return; }
    void action.run(async () => { if (plan?.id) await bizBillingAPI.updatePlan(apiKey, plan.id, payload); else await bizBillingAPI.createPlan(apiKey, payload); saved(); close(); }, 'Plan saved.');
  }}>
    <fieldset disabled={!canManage || action.busy} className="space-y-4">
      <BillingField label="Plan name"><Input required maxLength={255} value={name} onChange={e => setName(e.target.value)} /></BillingField>
      {!plan && <BillingField label="Search product catalog"><Input value={productSearch} onChange={e => { setProductSearch(e.target.value); setProductOffset(0); }} placeholder="Search products" /></BillingField>}
      {products.error ? <BillingLoadError label="plan products" retry={products.retry} /> : null}
      <BillingField label="Plan product"><select required disabled={!!plan || products.loading} className={billingSelectClass} value={productID} onChange={e => setProduct(e.target.value)}><option value="">Select a product</option>{plan && !options.some(product => product.id === plan.product_id) && <option value={plan.product_id}>Existing product</option>}{options.map(product => <option key={product.id} value={product.id}>{product.name}</option>)}</select></BillingField>
      {!plan && <div className="flex flex-wrap gap-2"><Button type="button" variant="outline" disabled={productOffset === 0} onClick={() => setProductOffset(n => Math.max(0, n - 20))}>Previous product options</Button><Button type="button" variant="outline" disabled={productOffset + options.length >= (products.data?.total ?? 0)} onClick={() => setProductOffset(n => n + 20)}>More product options</Button></div>}
      <div className="grid gap-4 sm:grid-cols-2">
        <BillingField label="Plan price"><Input required inputMode="decimal" value={price} onChange={e => setPrice(e.target.value)} /></BillingField>
        <BillingField label="Plan currency"><select className={billingSelectClass} value={currency} onChange={e => setCurrency(e.target.value)}>{[...new Set([currency, ...(preferences?.supported_currencies ?? ['INR'])])].map(value => <option key={value} value={value}>{value}</option>)}</select></BillingField>
        <BillingField label="Billing frequency"><select className={billingSelectClass} value={cycle} onChange={e => setCycle(e.target.value)}>{['weekly', 'monthly', 'quarterly', 'half_yearly', 'yearly', 'custom'].map(value => <option key={value} value={value}>{value.replace(/_/g, ' ')}</option>)}</select></BillingField>
        <BillingField label="Pricing model"><select className={billingSelectClass} value={model} onChange={e => setModel(e.target.value as typeof model)}><option value="flat">Flat amount</option><option value="per_unit">Per unit</option><option value="tiered">Tiered</option><option value="volume">Volume</option></select></BillingField>
        {cycle === 'custom' && <BillingField label="Days per billing cycle"><Input type="number" required min={1} max={3660} value={cycleDays} onChange={e => setCycleDays(Number(e.target.value))} /></BillingField>}
        <BillingField label="Trial days"><Input type="number" required min={0} max={365} value={trial} onChange={e => setTrial(Number(e.target.value))} /></BillingField>
        <BillingField label="Setup fee"><Input inputMode="decimal" required value={setup} onChange={e => setSetup(e.target.value)} /></BillingField>
      </div>
      <label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={inclusive} onChange={e => setInclusive(e.target.checked)} />Price includes tax</label>
      {(model === 'tiered' || model === 'volume') && <div className="space-y-3"><p className="text-sm">Enter tiers in increasing quantity order. Use 0 for the final unlimited tier.</p>{tiers.map((tier, index) => <div key={index} className="grid gap-2 rounded-md border p-3 sm:grid-cols-3">
        <BillingField label={`Tier ${index + 1} up to quantity`}><Input type="number" min={0} required value={tier.upTo} onChange={e => setTiers(rows => rows.map((row, i) => i === index ? { ...row, upTo: e.target.value } : row))} /></BillingField>
        <BillingField label={`Tier ${index + 1} unit price`}><Input inputMode="decimal" required value={tier.unit} onChange={e => setTiers(rows => rows.map((row, i) => i === index ? { ...row, unit: e.target.value } : row))} /></BillingField>
        <BillingField label={`Tier ${index + 1} flat fee`}><Input inputMode="decimal" required value={tier.flat} onChange={e => setTiers(rows => rows.map((row, i) => i === index ? { ...row, flat: e.target.value } : row))} /></BillingField>
        <Button type="button" variant="outline" onClick={() => setTiers(rows => rows.filter((_, i) => i !== index))}>Remove tier {index + 1}</Button>
      </div>)}<Button type="button" variant="outline" onClick={() => setTiers(rows => [...rows, { upTo: '0', unit: '0', flat: '0' }])}>Add pricing tier</Button></div>}
      {validation && <p role="alert" className="text-sm text-destructive">{validation}</p>}
      <Button type="submit" disabled={!preferences || (!plan && !productID)}>{action.busy ? 'Saving…' : 'Save plan'}</Button>
    </fieldset>
    <BillingActionStatus action={action} />
  </form>;
}

export default function BizBillingCatalog(props: Props) {
  const { apiKey, appId, canManage, preferences } = props;
  const [view, setView] = useState<'products' | 'plans'>('products');
  const [searchInput, setSearch] = useState(''), [offset, setOffset] = useState(0);
  const search = useBillingSearch(searchInput);
  const products = useBizBillingResource(`${appId}:catalog-products:${view}:${search}:${offset}`, signal => view === 'products' ? bizBillingAPI.listProductsPage(apiKey, { search, limit: 20, offset }, signal) : Promise.resolve({ success: true, data: [] as BizProduct[], total: 0 }));
  const plans = useBizBillingResource(`${appId}:catalog-plans:${view}:${search}:${offset}`, signal => view === 'plans' ? bizBillingAPI.listPlansPage(apiKey, { search, limit: 20, offset }, signal) : Promise.resolve({ success: true, data: [] as BizPlan[], total: 0 }));
  const [productForm, setProductForm] = useState<{ product?: BizProduct }>();
  const [planForm, setPlanForm] = useState<{ plan?: BizPlan }>();
  const [addonPlan, setAddonPlan] = useState<BizPlan>();
  const action = useBillingAction();
  const resource = view === 'products' ? products : plans;
  const rows = resource.data?.data ?? [], total = resource.data?.total ?? rows.length;
  const refresh = () => { products.retry(); plans.retry(); };
  return <div className="space-y-4">
    <div className="flex flex-wrap items-start justify-between gap-3"><div><h3 className="text-xl font-semibold">Product catalog</h3><p className="text-sm text-muted-foreground">Manage offerings and prices. Existing issued invoices keep their saved details.</p></div><Button disabled={!canManage || (view === 'plans' && !preferences)} onClick={() => view === 'products' ? setProductForm({}) : setPlanForm({})}>{view === 'products' ? 'New product' : 'New plan'}</Button></div>
    <div className="flex flex-wrap gap-2"><Button variant={view === 'products' ? 'default' : 'outline'} aria-pressed={view === 'products'} onClick={() => { setView('products'); setOffset(0); setSearch(''); }}>Products</Button><Button variant={view === 'plans' ? 'default' : 'outline'} aria-pressed={view === 'plans'} onClick={() => { setView('plans'); setOffset(0); setSearch(''); }}>Plans</Button></div>
    <BillingField label={`Search ${view}`}><Input value={searchInput} onChange={e => { setSearch(e.target.value); setOffset(0); }} /></BillingField>
    {resource.error ? <BillingLoadError label={view} retry={resource.retry} /> : null}
    {resource.loading && <p role="status">Loading {view}…</p>}
    {!resource.loading && !resource.error && !rows.length && <p className="text-sm">No {view} found. Create your first {view === 'products' ? 'product' : 'plan'} to get started.</p>}
    <div className="grid min-w-0 gap-3 md:grid-cols-2">{view === 'products' ? (products.data?.data ?? []).map(product => <BillingCard key={product.id} title={product.name}>
      <p className="break-words text-sm text-muted-foreground">{product.description || 'No description'}</p><p className="text-sm">{product.active ? 'Active' : 'Archived'}</p>
      <div className="flex flex-wrap gap-2"><Button variant="outline" aria-label={`Edit product ${product.name}`} disabled={!canManage} onClick={() => setProductForm({ product })}>Edit</Button><Button variant="outline" aria-label={`${product.active ? 'Archive' : 'Restore'} product ${product.name}`} disabled={!canManage || action.busy} onClick={() => void action.run(async () => { await bizBillingAPI.updateProduct(apiKey, product.id!, { active: !product.active }); refresh(); }, 'Product updated.')}>{product.active ? 'Archive' : 'Restore'}</Button></div>
    </BillingCard>) : (plans.data?.data ?? []).map(plan => <BillingCard key={plan.id} title={plan.name}>
      <p className="font-medium">{formatBillingAmount(plan.amount_paisa, plan.currency)}</p><p className="text-sm">{plan.billing_cycle.replace(/_/g, ' ')} · {plan.active ? 'Active' : 'Archived'}</p>
      <div className="flex flex-wrap gap-2"><Button variant="outline" aria-label={`Edit plan ${plan.name}`} disabled={!canManage} onClick={() => setPlanForm({ plan })}>Edit</Button><Button variant="outline" aria-label={`${plan.active ? 'Archive' : 'Restore'} plan ${plan.name}`} disabled={!canManage || action.busy} onClick={() => void action.run(async () => { await bizBillingAPI.updatePlan(apiKey, plan.id!, { active: !plan.active }); refresh(); }, 'Plan updated.')}>{plan.active ? 'Archive' : 'Restore'}</Button><Button variant="outline" aria-label={`Manage add-ons for ${plan.name}`} disabled={!plan.currency} onClick={() => setAddonPlan(plan)}>Add-ons</Button></div>
    </BillingCard>)}</div>
    <div className="flex flex-wrap items-center gap-3"><Button variant="outline" disabled={offset === 0 || resource.loading} onClick={() => setOffset(n => Math.max(0, n - 20))}>Previous {view}</Button><span className="text-sm">{total ? `${offset + 1}–${offset + rows.length} of ${total}` : '0 results'}</span><Button variant="outline" disabled={resource.loading || offset + rows.length >= total} onClick={() => setOffset(n => n + 20)}>Next {view}</Button></div>
    <BillingActionStatus action={action} />
    <Dialog open={!!productForm} onOpenChange={open => { if (!open) setProductForm(undefined); }}><DialogContent className="max-h-[85dvh] overflow-y-auto"><DialogHeader><DialogTitle>{productForm?.product ? 'Edit product' : 'Create product'}</DialogTitle><DialogDescription>Describe the offering. Prices are configured in its plans.</DialogDescription></DialogHeader>{productForm && <ProductForm key={productForm.product?.id ?? 'new'} {...props} product={productForm.product} close={() => setProductForm(undefined)} saved={refresh} />}</DialogContent></Dialog>
    <Dialog open={!!planForm} onOpenChange={open => { if (!open) setPlanForm(undefined); }}><DialogContent className="max-h-[85dvh] overflow-y-auto"><DialogHeader><DialogTitle>{planForm?.plan ? 'Edit plan' : 'Create plan'}</DialogTitle><DialogDescription>Choose the product, currency and price. Saving a plan does not charge a customer.</DialogDescription></DialogHeader>{planForm && <PlanForm key={planForm.plan?.id ?? 'new'} {...props} plan={planForm.plan} close={() => setPlanForm(undefined)} saved={refresh} />}</DialogContent></Dialog>
    <Dialog open={!!addonPlan} onOpenChange={open => { if (!open) setAddonPlan(undefined); }}><DialogContent className="max-h-[85dvh] overflow-y-auto"><DialogHeader><DialogTitle>Plan add-ons</DialogTitle><DialogDescription>Optional extras for this plan. Currency is inherited from its parent plan.</DialogDescription></DialogHeader>{addonPlan && <BizBillingAddons key={addonPlan.id} {...props} plan={addonPlan} />}</DialogContent></Dialog>
  </div>;
}
