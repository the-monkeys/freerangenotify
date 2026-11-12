import { Link } from 'react-router-dom';
import { portalRequest } from '../../services/bizPortal';
import type { BizInvoice } from '../../types';
import type { BillingDocumentData } from '../../types/bizBilling';
import { Button } from '../../components/ui/button';
import { BillingLoadError, formatBillingAmount as money } from '../../components/biz-billing/BillingForm';
import { useBizBillingResource } from '../../components/biz-billing/useBizBillingResource';
import { useBizRazorpayCheckout } from '../../hooks/useBizRazorpayCheckout';
import BillingDocument from './BillingDocument';
import { PortalFailure } from './BillingPortal';
import { printBillingDocument } from './printBillingDocument';

export default function PortalInvoice({token,id,print=false}:{token:string;id:string;print?:boolean}){
  const invoice=useBizBillingResource(`${token}:invoice:${id}`,signal=>portalRequest<BizInvoice>(token,`/invoices/${encodeURIComponent(id)}`,{signal}));
  const document=useBizBillingResource(`${token}:document:${id}`,signal=>portalRequest<BillingDocumentData>(token,`/invoices/${encodeURIComponent(id)}/document`,{signal}));
  const checkout=useBizRazorpayCheckout(token,id,'invoice',()=>{invoice.retry();document.retry();});
  if(checkout.error.startsWith('This billing link'))return <p role="alert">{checkout.error}</p>;
  if(invoice.error)return <PortalFailure error={invoice.error} retry={invoice.retry}/>;
  const row=invoice.data;
  return <div className="space-y-5"><div className="portal-controls flex flex-wrap gap-3"><Link className="underline" to={`/portal/${encodeURIComponent(token)}`}>Back to your billing</Link><Link className="underline" to={`/portal/${encodeURIComponent(token)}/invoices/${encodeURIComponent(id)}/print`}>Printable invoice / receipt</Link><Button variant="outline" disabled={!document.data} onClick={()=>{const article=window.document.querySelector<HTMLElement>('.billing-document');if(article)printBillingDocument(article);}}>Print / Save as PDF</Button></div>
    {invoice.loading&&<p role="status">Loading invoice…</p>}
    {row&&!print&&<section className="portal-controls space-y-4 rounded-xl border bg-card p-4"><h1 className="text-xl font-semibold">Invoice {row.invoice_number}</h1><p>Status: {row.status.replace(/_/g,' ')}</p><p>Amount due: <strong data-testid="portal-amount-due">{money(row.amount_due_paisa,row.currency)}</strong></p><p>Amount paid: {money(row.amount_paid_paisa??0,row.currency)}</p>
      {checkout.state==='confirmed'&&<p role="status">Payment confirmed</p>}{checkout.state==='pending'&&<><p role="status">Awaiting payment confirmation</p><Button onClick={()=>void checkout.check()}>Check payment status</Button></>}{checkout.state==='canceled'&&<p role="status">Checkout closed. No payment has been confirmed.</p>}{checkout.error&&<p role="alert">{checkout.error}</p>}
      <Button disabled={row.amount_due_paisa<=0||!!row.collection_owner||!['open','partially_paid','overdue'].includes(row.status)||['opening','pending','confirmed'].includes(checkout.state)} onClick={()=>void checkout.open(row.gateway_mode??'live')}>Pay invoice</Button>{row.collection_owner&&<p>This invoice is collected through your authorized automatic payment arrangement.</p>}
    </section>}
    {document.loading&&<p role="status">Loading issued document…</p>}{!!document.error&&<><PortalFailure error={document.error} retry={document.retry}/><p className="text-sm">Older invoices without saved issue-time details may need the business to supply the original document.</p></>}{document.data&&<BillingDocument document={document.data}/>}
    {!document.data&&!document.loading&&!document.error&&<BillingLoadError label="issued document" retry={document.retry}/>}
  </div>;
}
