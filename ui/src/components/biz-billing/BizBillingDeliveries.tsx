import { bizBillingAPI } from '../../services/api';
import type { BillingDelivery } from '../../types/bizBilling';
import { Button } from '../ui/button';
import { BillingCard, BillingLoadError } from './BillingForm';
import { useBizBillingResource } from './useBizBillingResource';
import { useBillingActions } from './BillingActions';
import { useState } from 'react';
export default function BizBillingDeliveries({apiKey,appId,canManage}:{apiKey:string;appId:string;canManage:boolean}){
  const [offset,setOffset]=useState(0);const action=useBillingActions();
  const result=useBizBillingResource(`${appId}:deliveries:${offset}`,signal=>bizBillingAPI.listBilling<BillingDelivery>(apiKey,'/biz/notification-deliveries',{limit:20,offset},signal));
  const labels:Record<string,string>={pending:'Waiting to send',accepted:'Accepted for sending',delivered:'Delivered',unknown:'Delivery needs review',failed:'Could not send',sandbox:'Suppressed in test mode'};
  return <div className="space-y-4"><h3 className="font-semibold">Billing messages and receipts</h3><p className="text-sm">A recorded payment and a delivered receipt are separate results. An uncertain delivery is never resent automatically.</p><Button variant="outline" onClick={result.retry}>Refresh deliveries</Button>{result.loading&&<p role="status">Loading deliveries…</p>}{!!result.error&&<BillingLoadError label="deliveries" retry={result.retry}/>}{!result.loading&&!result.error&&!result.data?.data.length&&<p>No billing messages yet.</p>}{result.data?.data.map(row=><BillingCard key={row.id} title={labels[row.status]??row.status}><p>{row.intent.channel} — {row.intent.template_id}</p><p className="break-all">Customer: {row.intent.user_id}</p><p>Attempts: {row.attempts}</p><Button disabled={!canManage||action.pending||row.status!=='failed'||row.may_have_dispatched} onClick={()=>void action.run('Message retry',`/biz/notification-deliveries/${encodeURIComponent(row.id)}/retry`)}>Retry message</Button></BillingCard>)}<div className="flex gap-2"><Button variant="outline" disabled={!offset} onClick={()=>setOffset(Math.max(0,offset-20))}>Previous deliveries</Button><Button variant="outline" disabled={(result.data?.data.length??0)<20} onClick={()=>setOffset(offset+20)}>More deliveries</Button></div></div>;
}
