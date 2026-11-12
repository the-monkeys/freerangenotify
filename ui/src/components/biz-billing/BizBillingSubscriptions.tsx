import { useState } from 'react';
import { bizBillingAPI } from '../../services/api';
import type { BizPlan, BizSubscription, User } from '../../types';
import { Button } from '../ui/button';
import { Input } from '../ui/input';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '../ui/dialog';
import { BillingCard, BillingField, BillingLoadError, billingSelectClass, formatBillingAmount as money } from './BillingForm';
import { useBillingSearch, useBizBillingResource } from './useBizBillingResource';
import { BillingActionNotice, useBillingActions } from './BillingActions';
import BillingCustomerPicker from './BillingCustomerPicker';
import type { BillingWorkflowProps } from './BizBillingInvoices';
import { statusBadge } from './shared';

export function automaticPlanWarning(plan?:BizPlan){
  if(!plan)return 'Choose a plan first.';
  if(plan.currency!=='INR'||!['flat','per_unit'].includes(plan.pricing_model||'flat')||!['weekly','monthly','quarterly','half_yearly','yearly'].includes(plan.billing_cycle)||!!plan.setup_fee_paisa)return 'Automatic payments support fixed INR recurring plans without setup fees. Choose invoice collection for this plan.';
  return '';
}
function SubscriptionForm({apiKey,appId,subscription,close}:BillingWorkflowProps&{subscription?:BizSubscription;close:()=>void}){
  const action=useBillingActions();const [customer,setCustomer]=useState<User>(),[planID,setPlanID]=useState(subscription?.plan_id??''),[query,setQuery]=useState(''),[offset,setOffset]=useState(0);
  const search=useBillingSearch(query);const plans=useBizBillingResource(`${appId}:subscription-plans:${search}:${offset}`,signal=>bizBillingAPI.listPlansPage(apiKey,{search,offset,limit:20,active:true},signal));
  const [selected,setSelected]=useState<BizPlan>(),[quantity,setQuantity]=useState(subscription?.quantity??1),[trial,setTrial]=useState(0),[mode,setMode]=useState('invoice'),[gatewayMode,setGatewayMode]=useState('test'),[cycles,setCycles]=useState(12),[timing,setTiming]=useState('next_period');
  const plan=selected??plans.data?.data.find(row=>row.id===planID), warning=automaticPlanWarning(plan);
  return <form className="space-y-4" onSubmit={e=>{e.preventDefault();const body=subscription?{plan_id:planID,quantity,change_timing:timing}:{user_id:customer?.user_id,plan_id:planID,quantity,trial_days:trial,collection_mode:mode,...(mode==='automatic'?{gateway_mode:gatewayMode,automatic_cycles:cycles}:{})};void action.run(subscription?'Subscription change':'Subscription',subscription?`/biz/subscriptions/${encodeURIComponent(subscription.id)}`:'/biz/subscriptions',body,subscription?'PUT':'POST').then(ok=>{if(ok)close();});}}>
    <fieldset disabled={action.pending||action.busy} className="space-y-4">
      {!subscription&&<div><p className="mb-2 text-sm">Customer</p><BillingCustomerPicker apiKey={apiKey} appId={appId} value={customer} onChange={setCustomer}/></div>}
      <BillingField label="Search plans"><Input value={query} onChange={e=>{setQuery(e.target.value);setOffset(0);}}/></BillingField>
      {!!plans.error&&<BillingLoadError label="subscription plans" retry={plans.retry}/>}
      <BillingField label="Subscription plan"><select required className={billingSelectClass} value={planID} onChange={e=>{setPlanID(e.target.value);setSelected(plans.data?.data.find(p=>p.id===e.target.value));}}><option value="">Select plan</option>{selected&&!plans.data?.data.some(p=>p.id===selected.id)&&<option value={selected.id}>{selected.name}</option>}{plans.data?.data.map(plan=><option key={plan.id} value={plan.id}>{plan.name} — {money(plan.amount_paisa,plan.currency)} / {plan.billing_cycle}</option>)}</select></BillingField>
      <div className="flex flex-wrap gap-2"><Button type="button" variant="outline" disabled={!offset} onClick={()=>setOffset(Math.max(0,offset-20))}>Previous plans</Button><Button type="button" variant="outline" disabled={(plans.data?.data.length??0)<20} onClick={()=>setOffset(offset+20)}>More plans</Button></div>
      <BillingField label="Quantity"><Input required type="number" min={1} step={1} value={quantity} onChange={e=>setQuantity(Number(e.target.value))}/></BillingField>
      {subscription?<><BillingField label="Change timing"><select className={billingSelectClass} value={timing} onChange={e=>setTiming(e.target.value)}><option value="next_period">Next billing period</option><option value="immediate">Immediately</option></select></BillingField><p className="text-sm">Next-period changes need the reviewed billing upgrade. Immediate changes keep the existing API behavior.</p></>:<>
        <BillingField label="Trial days"><Input required type="number" min={0} max={3650} value={trial} onChange={e=>setTrial(Number(e.target.value))}/></BillingField>
        <BillingField label="Collection mode"><select className={billingSelectClass} value={mode} onChange={e=>setMode(e.target.value)}><option value="invoice">Invoice — customer pays each cycle</option><option value="automatic">Automatic — customer authorizes Razorpay</option></select></BillingField>
        {mode==='automatic'&&<><BillingField label="Payment environment"><select className={billingSelectClass} value={gatewayMode} onChange={e=>setGatewayMode(e.target.value)}><option value="test">Test</option><option value="live">Live</option></select></BillingField><BillingField label="Automatic billing cycles"><Input required type="number" min={1} max={120} value={cycles} onChange={e=>setCycles(Number(e.target.value))}/></BillingField>{warning&&<p role="alert">{warning}</p>}<p>Creating this subscription does not charge the customer. They must authorize it through their billing portal. The server also checks merchant readiness.</p></>}
      </>}
      <Button type="submit" disabled={!planID||(!subscription&&!customer)||(mode==='automatic'&&!!warning)}>{subscription?'Save subscription change':'Create subscription'}</Button>
    </fieldset>
  </form>;
}
export default function BizBillingSubscriptions(props:BillingWorkflowProps){
  const {apiKey,appId,canManage}=props;const [offset,setOffset]=useState(0),[dialog,setDialog]=useState<BizSubscription|'new'>();const action=useBillingActions();
  const result=useBizBillingResource(`${appId}:subscriptions:${offset}`,signal=>bizBillingAPI.listBilling<BizSubscription>(apiKey,'/biz/subscriptions',{limit:20,offset},signal));
  return <div className="space-y-4"><div className="flex flex-wrap gap-2"><Button disabled={!canManage||action.pending} onClick={()=>setDialog('new')}>New subscription</Button><Button variant="outline" onClick={result.retry}>Refresh subscriptions</Button></div>
    {result.loading&&<p role="status">Loading subscriptions…</p>}{!!result.error&&<BillingLoadError label="subscriptions" retry={result.retry}/>}{!result.loading&&!result.error&&!result.data?.data.length&&<p>No subscriptions yet.</p>}
    <div className="grid gap-4 xl:grid-cols-2">{result.data?.data.map(sub=><BillingCard key={sub.id} title={sub.plan_name||'Subscription'}>{statusBadge(sub.status)}<p className="break-all">Customer: {sub.user_id}</p><p>{sub.currency?money(sub.plan_amount_paisa??0,sub.currency):'See plan currency'} / {sub.billing_cycle}</p><p>Collection: {sub.collection_mode==='automatic'?'Automatic':'Invoice'}</p><p>Next period: {sub.current_period_end&&!sub.current_period_end.startsWith('0001')?new Date(sub.current_period_end).toLocaleDateString():'Starts after authorization'}</p>{sub.status==='awaiting_authorization'&&<p>Awaiting customer authorization. No payment has been confirmed.</p>}{sub.pending_change&&<p>Plan or quantity change scheduled for the next billing period.</p>}
      <div className="flex flex-wrap gap-2"><Button variant="outline" disabled={!canManage||action.pending} onClick={()=>setDialog(sub)}>Change plan / quantity</Button>{['active','trialing','paused'].includes(sub.status)&&<Button variant="outline" disabled={!canManage||action.pending} onClick={()=>void action.run(sub.status==='paused'?'Subscription resume':'Subscription pause',`/biz/subscriptions/${encodeURIComponent(sub.id)}/${sub.status==='paused'?'resume':'pause'}`)}>{sub.status==='paused'?'Resume':'Pause'}</Button>}{!['canceled','awaiting_authorization'].includes(sub.status)&&<Button variant="outline" disabled={!canManage||action.pending||sub.cancel_at_period_end} onClick={()=>{if(window.confirm('Cancel this subscription at the end of the current period? Existing invoices and payments will be retained.'))void action.run('Subscription cancellation',`/biz/subscriptions/${encodeURIComponent(sub.id)}/cancel`,{cancel_at_period_end:true});}}>Cancel at period end</Button>}</div>
    </BillingCard>)}</div>
    <div className="flex gap-2"><Button variant="outline" disabled={!offset} onClick={()=>setOffset(Math.max(0,offset-20))}>Previous subscriptions</Button><Button variant="outline" disabled={(result.data?.data.length??0)<20} onClick={()=>setOffset(offset+20)}>More subscriptions</Button></div>
    <Dialog open={!!dialog} onOpenChange={open=>{if(!open)setDialog(undefined);}}><DialogContent className="max-h-[90dvh] w-[calc(100%-2rem)] max-w-xl overflow-y-auto"><DialogHeader><DialogTitle>{dialog==='new'?'New subscription':'Change subscription'}</DialogTitle><DialogDescription>A subscription connects an existing customer to a plan.</DialogDescription></DialogHeader><BillingActionNotice/>{dialog&&<SubscriptionForm {...props} subscription={dialog==='new'?undefined:dialog} close={()=>setDialog(undefined)}/>}</DialogContent></Dialog>
  </div>;
}
