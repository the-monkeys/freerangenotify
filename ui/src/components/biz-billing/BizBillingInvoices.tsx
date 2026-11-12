import { useState } from 'react';
import { bizBillingAPI } from '../../services/api';
import type { BizInvoice, CreateBizInvoiceRequest, User } from '../../types';
import type { BillingCreditBalance, BillingDocumentData, BillingInvoicePreview, BillingPreferences } from '../../types/bizBilling';
import { Button } from '../ui/button';
import { Input } from '../ui/input';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '../ui/dialog';
import { BillingCard, BillingField, BillingLoadError, billingSelectClass, formatBillingAmount as money, parseBillingAmount } from './BillingForm';
import { useBizBillingResource } from './useBizBillingResource';
import { BillingActionNotice, useBillingActions } from './BillingActions';
import BillingCustomerPicker from './BillingCustomerPicker';
import { statusBadge } from './shared';

export type BillingWorkflowProps={apiKey:string;appId:string;canManage:boolean;preferences?:BillingPreferences};
function NewInvoice({apiKey,appId,preferences,close}:BillingWorkflowProps&{close:()=>void}){
  const action=useBillingActions();const [customer,setCustomer]=useState<User>(),[currency,setCurrency]=useState(preferences?.default_currency??'INR'),[notes,setNotes]=useState('');
  const [rows,setRows]=useState([{description:'',quantity:'1',price:'',discount:'0',tax:''}]);
  const [preview,setPreview]=useState<BillingInvoicePreview>(),[previewing,setPreviewing]=useState(false),[error,setError]=useState('');
  const locked=action.pending||action.busy;
  function payload():CreateBizInvoiceRequest{
    if(!customer)throw new Error('Select a customer.');
    return {user_id:customer.user_id,currency,notes,line_items:rows.map(row=>{
      const quantity=Number(row.quantity);if(!Number.isSafeInteger(quantity)||quantity<1)throw new Error('Use a positive whole-number quantity.');
      if(!row.description.trim())throw new Error('Describe every invoice item.');
      const tax=row.tax===''?undefined:Number(row.tax);if(tax!==undefined&&(!Number.isInteger(tax)||tax<0||tax>28))throw new Error('Tax must be a whole percentage between 0 and 28.');
      return {description:row.description.trim(),quantity,unit_price:parseBillingAmount(row.price),discount_paisa:parseBillingAmount(row.discount),tax_rate_override:tax};
    })};
  }
  async function calculate(){setError('');setPreviewing(true);try{const body=payload();setPreview(await bizBillingAPI.submitBilling<BillingInvoicePreview>(apiKey,'/biz/invoices/preview',body,''));}catch(e){setError(e instanceof Error&&!('response'in e)?e.message:'Could not preview the invoice. Check the customer and amounts.');}finally{setPreviewing(false);}}
  return <form className="space-y-4" onSubmit={e=>{e.preventDefault();setError('');try{const body=payload();void action.run('Invoice draft','/biz/invoices',body).then(ok=>{if(ok)close();});}catch(e){setError((e as Error).message);}}}>
    <fieldset disabled={locked||previewing} className="space-y-4" onChange={()=>setPreview(undefined)}>
      <div><p className="mb-2 text-sm font-medium">Customer</p><BillingCustomerPicker apiKey={apiKey} appId={appId} value={customer} onChange={user=>{setCustomer(user);setPreview(undefined);}}/></div>
      <BillingField label="Invoice currency"><select className={billingSelectClass} value={currency} onChange={e=>setCurrency(e.target.value)}>{(preferences?.supported_currencies??['INR','USD','EUR','GBP']).map(code=><option key={code}>{code}</option>)}</select></BillingField>
      {rows.map((row,index)=><div key={index} className="space-y-3 rounded border p-3">
        {(['description','quantity','price','discount','tax'] as const).map(field=><BillingField key={field} label={`Item ${index+1} ${field==='price'?'unit price':field==='tax'?'tax override (%)':field}`}><Input required={field!=='tax'} value={row[field]} inputMode={field==='description'?'text':'decimal'} onChange={e=>setRows(rows.map((old,i)=>i===index?{...old,[field]:e.target.value}:old))}/></BillingField>)}
        <p className="text-xs text-muted-foreground">Leave tax blank to use business tax settings. Enter 0 for zero tax.</p>
        {rows.length>1&&<Button type="button" variant="outline" onClick={()=>{setRows(rows.filter((_,i)=>i!==index));setPreview(undefined);}}>Remove item {index+1}</Button>}
      </div>)}
      <Button type="button" variant="outline" onClick={()=>{setRows([...rows,{description:'',quantity:'1',price:'',discount:'0',tax:''}]);setPreview(undefined);}}>Add item</Button>
      <BillingField label="Invoice notes"><Input value={notes} onChange={e=>setNotes(e.target.value)}/></BillingField>
      <Button type="button" onClick={()=>void calculate()}>Preview invoice</Button>
    </fieldset>
    {error&&<p role="alert">{error}</p>}
    {preview&&<div data-testid="invoice-preview" className="space-y-2 rounded border p-3"><p>Subtotal: {money(preview.subtotal_paisa,preview.currency)}</p><p>Tax: {money(preview.tax_amount_paisa,preview.currency)}</p><p>Total: {money(preview.total_paisa,preview.currency)}</p><p className="text-sm">Saving a draft does not collect payment.</p><Button disabled={locked} type="submit">Save draft</Button></div>}
  </form>;
}
function PaymentForm({invoice,close}: {invoice:BizInvoice;close:()=>void}){
  const action=useBillingActions();const [amount,setAmount]=useState(String(invoice.amount_due_paisa/100)),[method,setMethod]=useState('cash'),[reference,setReference]=useState(''),[confirmed,setConfirmed]=useState(false),[error,setError]=useState('');
  return <form className="space-y-4" onSubmit={e=>{e.preventDefault();setError('');try{const value=parseBillingAmount(amount);if(!value)throw new Error('Enter a positive received amount.');void action.run('Payment',`/biz/invoices/${encodeURIComponent(invoice.id)}/payments`,{method,amount_paisa:value,reference}).then(ok=>{if(ok)close();});}catch(e){setError((e as Error).message);}}}>
    <p>Record money already received outside the gateway. This does not charge the customer.</p>
    <fieldset disabled={action.pending||action.busy} className="space-y-4">
      <BillingField label={`Received amount (${invoice.currency})`}><Input required inputMode="decimal" value={amount} onChange={e=>setAmount(e.target.value)}/></BillingField>
      <BillingField label="Payment method"><select className={billingSelectClass} value={method} onChange={e=>setMethod(e.target.value)}><option value="cash">Cash</option><option value="upi">Direct UPI</option><option value="bank_transfer">Bank transfer</option><option value="cheque">Cheque</option></select></BillingField>
      <BillingField label="Payment reference"><Input value={reference} maxLength={200} onChange={e=>setReference(e.target.value)}/></BillingField>
      <label className="flex items-start gap-2"><input type="checkbox" required checked={confirmed} onChange={e=>setConfirmed(e.target.checked)}/>I confirm this payment was received</label>
      <Button disabled={!confirmed} type="submit">Save payment</Button>
    </fieldset>{error&&<p role="alert">{error}</p>}
  </form>;
}
function InvoiceDetail({apiKey,appId,invoice,canManage}:BillingWorkflowProps&{invoice:BizInvoice}){
  const doc=useBizBillingResource(`${appId}:document:${invoice.id}`,signal=>bizBillingAPI.readBilling<BillingDocumentData>(apiKey,`/biz/invoices/${encodeURIComponent(invoice.id)}/document`,{},signal));
  const credit=useBizBillingResource(`${appId}:balance:${invoice.user_id}:${invoice.currency}`,signal=>bizBillingAPI.readBilling<BillingCreditBalance>(apiKey,`/biz/users/${encodeURIComponent(invoice.user_id)}/balance`,{currency:invoice.currency},signal));
  const action=useBillingActions();
  return <div className="space-y-4">
    {doc.loading&&<p role="status">Loading invoice document…</p>}{!!doc.error&&<BillingLoadError label="invoice document" retry={doc.retry}/>}
    {doc.data&&<><p>{doc.data.issuer?.name}</p><p>{doc.data.customer?.name??doc.data.customer?.full_name} {doc.data.customer?.email}</p>{doc.data.line_items?.map((row,i)=><p key={i} className="break-words">{row.description} × {row.quantity}: {money(row.unit_price,doc.data!.currency)}</p>)}<p>Issued total: {money(doc.data.total_paisa,doc.data.currency)}</p><p>Recorded payments: {doc.data.payments?.length??0}</p><p>{doc.data.notes}</p></>}
    {!!credit.error&&<BillingLoadError label="customer credit" retry={credit.retry}/>}
    {credit.data&&<><p>Available customer credit: {money(credit.data.available_paisa,invoice.currency)}</p><p>Reserved customer credit: {money(credit.data.reserved_paisa,invoice.currency)}</p>{credit.data.unresolved_legacy&&<p role="alert">Existing credits need review before they can be used.</p>}<Button disabled={!canManage||action.pending||!!invoice.collection_owner||credit.data.unresolved_legacy||!credit.data.available_paisa||invoice.amount_due_paisa<=0} onClick={()=>void action.run('Customer credit',`/biz/invoices/${encodeURIComponent(invoice.id)}/apply-credit`,{})}>Apply available credit</Button></>}
  </div>;
}
export default function BizBillingInvoices(props:BillingWorkflowProps){
  const {apiKey,appId,canManage,preferences}=props;const [offset,setOffset]=useState(0),[status,setStatus]=useState('');
  const result=useBizBillingResource(`${appId}:invoices:${status}:${offset}`,signal=>bizBillingAPI.listBilling<BizInvoice>(apiKey,'/biz/invoices',{status:status||undefined,limit:20,offset},signal));
  const [dialog,setDialog]=useState<{kind:'new'|'payment'|'detail';invoice?:BizInvoice}>();const action=useBillingActions();
  return <div className="space-y-4"><div className="flex flex-wrap gap-3"><Button disabled={!canManage||!preferences||action.pending} onClick={()=>setDialog({kind:'new'})}>New invoice</Button><Button variant="outline" onClick={result.retry}>Refresh invoices</Button></div>
    <BillingField label="Invoice status"><select className={billingSelectClass} value={status} onChange={e=>{setStatus(e.target.value);setOffset(0);}}>{['','draft','open','partially_paid','paid','overdue','void'].map(s=><option key={s} value={s}>{s||'All statuses'}</option>)}</select></BillingField>
    {result.loading&&<p role="status">Loading invoices…</p>}{!!result.error&&<BillingLoadError label="invoices" retry={result.retry}/>}
    {!result.loading&&!result.error&&!result.data?.data.length&&<p>No invoices yet.</p>}
    <div className="grid min-w-0 gap-4 xl:grid-cols-2">{result.data?.data.map(invoice=><div key={invoice.id} data-testid={`invoice-${invoice.id}`}><BillingCard title={invoice.invoice_number||'Draft invoice'}>
      {statusBadge(invoice.status)}<p className="break-all text-sm">{invoice.issued_snapshot?.customer?.name??invoice.issued_snapshot?.customer?.full_name??invoice.user_id} {invoice.issued_snapshot?.customer?.email}</p>
      <dl className="grid grid-cols-2 gap-2 text-sm"><dt>Total</dt><dd>{money(invoice.total_paisa,invoice.currency)}</dd><dt>Amount paid</dt><dd data-testid="amount-paid">{money(invoice.amount_paid_paisa??0,invoice.currency)}</dd><dt>Credit applied</dt><dd>{money(invoice.credits_applied??0,invoice.currency)}</dd><dt>Amount due</dt><dd>{money(invoice.amount_due_paisa,invoice.currency)}</dd><dt>Cash refunded</dt><dd>{money(invoice.cash_refunded_paisa??0,invoice.currency)}</dd></dl>
      <div className="flex flex-wrap gap-2"><Button variant="outline" onClick={()=>setDialog({kind:'detail',invoice})}>View invoice</Button>{invoice.status==='draft'?<Button disabled={!canManage||action.pending} onClick={()=>void action.run('Invoice issue',`/biz/invoices/${encodeURIComponent(invoice.id)}/issue`)}>Issue invoice</Button>:<Button disabled={!canManage||action.pending||invoice.amount_due_paisa<=0||!!invoice.collection_owner||['void','uncollectible'].includes(invoice.status)} onClick={()=>setDialog({kind:'payment',invoice})}>Record payment</Button>}</div>{invoice.collection_owner&&<p className="text-sm">Automatic collection owns this invoice. Manual collection is disabled.</p>}
    </BillingCard></div>)}</div>
    <div className="flex flex-wrap gap-2"><Button variant="outline" disabled={!offset||result.loading} onClick={()=>setOffset(Math.max(0,offset-20))}>Previous invoices</Button><Button variant="outline" disabled={result.loading||(result.data?.data.length??0)<20} onClick={()=>setOffset(offset+20)}>More invoices</Button></div>
    <Dialog open={!!dialog} onOpenChange={open=>{if(!open)setDialog(undefined);}}><DialogContent className="max-h-[90dvh] w-[calc(100%-2rem)] max-w-xl overflow-y-auto"><DialogHeader><DialogTitle>{dialog?.kind==='new'?'New invoice':dialog?.kind==='payment'?'Record received payment':'Invoice details'}</DialogTitle><DialogDescription>Amounts are shown in the invoice currency. Payment is recorded only after confirmation.</DialogDescription></DialogHeader><BillingActionNotice/>{dialog?.kind==='new'&&<NewInvoice {...props} close={()=>setDialog(undefined)}/>} {dialog?.kind==='payment'&&dialog.invoice&&<PaymentForm invoice={dialog.invoice} close={()=>setDialog(undefined)}/>} {dialog?.kind==='detail'&&dialog.invoice&&<InvoiceDetail {...props} invoice={dialog.invoice}/>}</DialogContent></Dialog>
  </div>;
}
