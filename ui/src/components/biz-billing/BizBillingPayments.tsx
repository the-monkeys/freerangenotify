import { useState } from 'react';
import { bizBillingAPI } from '../../services/api';
import type { BizPayment } from '../../types';
import { Button } from '../ui/button';
import { Input } from '../ui/input';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '../ui/dialog';
import { BillingCard, BillingField, BillingLoadError, formatBillingAmount as money, parseBillingAmount } from './BillingForm';
import { useBizBillingResource } from './useBizBillingResource';
import { BillingActionNotice, useBillingActions } from './BillingActions';
import type { BillingWorkflowProps } from './BizBillingInvoices';
import { statusBadge } from './shared';
function Refund({payment,close}:{payment:BizPayment;close:()=>void}){
  const action=useBillingActions();const remaining=payment.amount_paisa-(payment.refunded_paisa??0)-(payment.refund_reserved_paisa??0);
  const [amount,setAmount]=useState(String(remaining/100)),[reason,setReason]=useState(''),[confirmed,setConfirmed]=useState(false),[error,setError]=useState('');
  return <form className="space-y-4" onSubmit={e=>{e.preventDefault();try{const value=parseBillingAmount(amount);if(value<=0||value>remaining)throw new Error('Enter an amount within the refundable balance.');setError('');void action.run('Refund',`/biz/payments/${encodeURIComponent(payment.id)}/refund`,{amount_paisa:value,reason}).then(ok=>{if(ok)close();});}catch(e){setError((e as Error).message);}}}><p>Gateway refunds wait for provider confirmation. For cash or direct UPI, record this only after returning the money. A refund does not erase the original payment.</p><p>Overpayment credit that has already been spent cannot be refunded.</p><fieldset disabled={action.pending||action.busy} className="space-y-4"><BillingField label={`Refund amount (${payment.currency})`}><Input required inputMode="decimal" value={amount} onChange={e=>setAmount(e.target.value)}/></BillingField><BillingField label="Refund reason"><Input required maxLength={500} value={reason} onChange={e=>setReason(e.target.value)}/></BillingField><label className="flex gap-2"><input required type="checkbox" checked={confirmed} onChange={e=>setConfirmed(e.target.checked)}/>I confirm this refund</label><Button type="submit" disabled={!confirmed}>Confirm refund</Button></fieldset>{error&&<p role="alert">{error}</p>}</form>;
}
export default function BizBillingPayments({apiKey,appId,canManage}:BillingWorkflowProps){
  const [offset,setOffset]=useState(0),[payment,setPayment]=useState<BizPayment>();const action=useBillingActions();
  const result=useBizBillingResource(`${appId}:payments:${offset}`,signal=>bizBillingAPI.listBilling<BizPayment>(apiKey,'/biz/payments',{limit:20,offset},signal));
  return <div className="space-y-4"><Button variant="outline" onClick={result.retry}>Refresh payments</Button>{result.loading&&<p role="status">Loading payments…</p>}{!!result.error&&<BillingLoadError label="payments" retry={result.retry}/>}{!result.loading&&!result.error&&!result.data?.data.length&&<p>No payments received yet. Issuing an invoice does not create a payment.</p>}
    <div className="grid gap-4 xl:grid-cols-2">{result.data?.data.map(row=><BillingCard key={row.id} title={`${row.method} payment`}>{statusBadge(row.status)}<p>Received: {money(row.amount_paisa,row.currency)}</p><p>Refunded: {money(row.refunded_paisa??0,row.currency)}</p><p>Refund pending: {money(row.refund_reserved_paisa??0,row.currency)}</p><p className="break-all">Reference: {row.reference||row.gateway_payment_id||'—'}</p><Button disabled={!canManage||action.pending||!['success','partially_refunded'].includes(row.status)||row.amount_paisa<=(row.refunded_paisa??0)+(row.refund_reserved_paisa??0)} onClick={()=>setPayment(row)}>Refund payment</Button></BillingCard>)}</div>
    <div className="flex gap-2"><Button variant="outline" disabled={!offset} onClick={()=>setOffset(Math.max(0,offset-20))}>Previous payments</Button><Button variant="outline" disabled={(result.data?.data.length??0)<20} onClick={()=>setOffset(offset+20)}>More payments</Button></div>
    <Dialog open={!!payment} onOpenChange={open=>{if(!open)setPayment(undefined);}}><DialogContent className="max-h-[90dvh] w-[calc(100%-2rem)] max-w-xl overflow-y-auto"><DialogHeader><DialogTitle>Refund payment</DialogTitle><DialogDescription>Confirm the amount and original payment before proceeding.</DialogDescription></DialogHeader><BillingActionNotice/>{payment&&<Refund payment={payment} close={()=>setPayment(undefined)}/>}</DialogContent></Dialog>
  </div>;
}
