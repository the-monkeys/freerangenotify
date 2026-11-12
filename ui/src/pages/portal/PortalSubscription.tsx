import { useState } from 'react';
import { Link } from 'react-router-dom';
import { portalRequest } from '../../services/bizPortal';
import type { BizSubscription } from '../../types';
import { Button } from '../../components/ui/button';
import { useBizBillingResource } from '../../components/biz-billing/useBizBillingResource';
import { formatBillingAmount as money } from '../../components/biz-billing/BillingForm';
import { useBizRazorpayCheckout } from '../../hooks/useBizRazorpayCheckout';
import { PortalFailure } from './BillingPortal';
export default function PortalSubscription({token,id}:{token:string;id:string}){
  const subscription=useBizBillingResource(`${token}:subscription:${id}`,signal=>portalRequest<BizSubscription>(token,`/subscriptions/${encodeURIComponent(id)}`,{signal}));
  const [consent,setConsent]=useState(false);const checkout=useBizRazorpayCheckout(token,id,'subscription',subscription.retry);
  if(subscription.error)return <PortalFailure error={subscription.error} retry={subscription.retry}/>;
  if(checkout.error.startsWith('This billing link'))return <p role="alert">{checkout.error}</p>;
  const row=subscription.data;
  return <section className="space-y-4 rounded-xl border bg-card p-4"><Link className="underline" to={`/portal/${encodeURIComponent(token)}`}>Back to your billing</Link>{subscription.loading&&<p role="status">Loading subscription…</p>}{row&&<><h1 className="text-xl font-semibold">{row.plan_name||'Your subscription'}</h1><p>Status: {row.status.replace(/_/g,' ')}</p><p>{row.currency?money(row.plan_amount_paisa??0,row.currency):'See your invoice for pricing'} / {row.billing_cycle||'billing period'}</p><p>Collection: {row.collection_mode==='automatic'?'Automatic':'Invoice'}</p><p>Automatic authorization applies to future billing periods. It does not pay outstanding invoices.</p>{row.collection_mode==='automatic'&&row.status==='awaiting_authorization'&&<><label className="flex gap-2"><input type="checkbox" checked={consent} onChange={event=>setConsent(event.target.checked)}/>I authorize the recurring payment arrangement</label><Button disabled={!consent||['opening','pending','confirmed'].includes(checkout.state)} onClick={()=>void checkout.open(row.gateway_mode??'live')}>Authorize automatic payments</Button></>}{checkout.state==='confirmed'&&<p role="status">Authorization confirmed</p>}{checkout.state==='pending'&&<><p role="status">Awaiting authorization confirmation</p><Button onClick={()=>void checkout.check()}>Check authorization status</Button></>}{checkout.state==='canceled'&&<p role="status">Checkout closed. No payment has been confirmed.</p>}{checkout.error&&<p role="alert">{checkout.error}</p>}</>}</section>;
}
