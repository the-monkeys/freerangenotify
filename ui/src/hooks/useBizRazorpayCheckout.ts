import { useEffect, useRef, useState } from 'react';
import { PortalError, portalRequest, type PortalCheckoutDescriptor, type PortalOperation } from '../services/bizPortal';

type SDKOptions={key:string;order_id?:string;subscription_id?:string;name:string;description:string;handler:(proof:Record<string,string>)=>void;modal:{ondismiss:()=>void}};
type SDKInstance={open:()=>void;on:(event:string,fn:()=>void)=>void};
type SDKWindow={Razorpay?:new(options:SDKOptions)=>SDKInstance};
let sdk:Promise<void>|undefined;
export function loadBizCheckout():Promise<void>{
  if((window as unknown as SDKWindow).Razorpay)return Promise.resolve();
  if(sdk)return sdk;
  sdk=new Promise<void>((resolve,reject)=>{
    const script=document.createElement('script');script.src='https://checkout.razorpay.com/v1/checkout.js';script.referrerPolicy='no-referrer';script.async=true;
    const timer=setTimeout(()=>fail(),15000);
    function fail(){clearTimeout(timer);script.remove();sdk=undefined;reject(new Error('Could not load secure checkout. Check your connection and try again.'));}
    script.onerror=fail;script.onload=()=>{clearTimeout(timer);if(!(window as unknown as SDKWindow).Razorpay){fail();return;}resolve();};document.head.appendChild(script);
  });return sdk;
}

export function useBizRazorpayCheckout(token:string,id:string,kind:'invoice'|'subscription',onConfirmed:()=>void){
  const [state,setState]=useState<'idle'|'opening'|'pending'|'confirmed'|'canceled'|'error'>('idle'),[error,setError]=useState('');
  const key=useRef(crypto.randomUUID()),operation=useRef(''),proof=useRef<Record<string,string>|undefined>(undefined),alive=useRef(true),busy=useRef(false),timer=useRef<ReturnType<typeof setTimeout>|undefined>(undefined),trigger=useRef<HTMLElement|null>(null),polls=useRef(0),controller=useRef(new AbortController());
  const confirmed=useRef(onConfirmed);confirmed.current=onConfirmed;
  const base=kind==='invoice'?`/invoices/${encodeURIComponent(id)}`:`/subscriptions/${encodeURIComponent(id)}`;
  useEffect(()=>{alive.current=true;return()=>{alive.current=false;controller.current.abort();if(timer.current)clearTimeout(timer.current);};},[]);
  useEffect(()=>{if(state==='canceled'||state==='error'){const frame=requestAnimationFrame(()=>trigger.current?.focus());return()=>cancelAnimationFrame(frame);}},[state]);
  async function confirmSavedState():Promise<boolean>{
    // A completed internal journal operation is not necessarily authorization
    // or capture. Re-read the customer-owned financial resource before claiming it.
    const row=await portalRequest<{status:string;collection_state?:string;amount_due_paisa?:number}>(token,base,{signal:controller.current.signal});
    const confirmedState=kind==='invoice'?row.status==='paid'&&row.amount_due_paisa===0:['authorized','active'].includes(row.collection_state??'');
    if(confirmedState&&alive.current){setState('confirmed');confirmed.current();if(timer.current)clearTimeout(timer.current);}
    return confirmedState;
  }
  function fail(cause:unknown){if(!alive.current)return;setError(cause instanceof PortalError?cause.message:cause instanceof Error?cause.message:'Could not complete checkout. Check the invoice before trying again.');setState('error');trigger.current?.focus();}
  async function verify(result:Record<string,string>){
    if(!alive.current)return;
    proof.current=result;if(alive.current){setState('pending');setError('');}
    try{
      const response=await portalRequest<PortalOperation>(token,`${base}/${kind==='invoice'?'verify-payment':'verify-authorization'}`,{method:'POST',body:result,key:key.current,signal:controller.current.signal});
      operation.current=response.operation_id||operation.current;
      if(!await confirmSavedState())schedule();
    }catch(cause){
      if(cause instanceof PortalError&&cause.status===401){fail(cause);return;}
      if(cause instanceof PortalError&&cause.status<500){fail(cause);return;}
      if(cause instanceof PortalError&&cause.operationID)operation.current=cause.operationID;
      schedule();
    }
  }
  function schedule(){if(!alive.current)return;setState('pending');if(timer.current)clearTimeout(timer.current);if(polls.current++<10)timer.current=setTimeout(()=>void check(),3000);}
  async function check(){
    if(busy.current||!alive.current)return;busy.current=true;
    try{
      if(operation.current)await portalRequest<PortalOperation>(token,`/operations/${encodeURIComponent(operation.current)}`,{signal:controller.current.signal});
      if(await confirmSavedState())return;
      if(proof.current)await verify(proof.current);else schedule();
    }catch(cause){if(cause instanceof PortalError&&[401,404].includes(cause.status))fail(cause);else schedule();}finally{busy.current=false;}
  }
  async function open(mode:'test'|'live'='live'){
    if(busy.current||state==='pending')return;busy.current=true;trigger.current=document.activeElement as HTMLElement;setError('');setState('opening');
    try{
      await loadBizCheckout();
      const descriptor=await portalRequest<PortalCheckoutDescriptor>(token,`${base}/${kind==='invoice'?'pay':'authorize'}`,{method:'POST',key:key.current,body:kind==='invoice'?{}:{mode},signal:controller.current.signal});
      operation.current=descriptor.operation_id;
      if(!descriptor.key_id||!(kind==='invoice'?descriptor.gateway_order_id:descriptor.gateway_subscription_id)){schedule();return;}
      const Constructor=(window as unknown as SDKWindow).Razorpay!;
      const instance=new Constructor({key:descriptor.key_id,...(kind==='invoice'?{order_id:descriptor.gateway_order_id}:{subscription_id:descriptor.gateway_subscription_id}),name:descriptor.merchant_display_name||'Business billing',description:kind==='invoice'?'Invoice payment':'Recurring payment authorization',handler:result=>{void verify(result);},modal:{ondismiss:()=>{if(alive.current&&!proof.current)setState('canceled');}}});
      instance.on('payment.failed',()=>fail(new Error('Checkout failed. No payment has been confirmed. Check the invoice status before retrying.')));instance.open();
    }catch(cause){if(cause instanceof PortalError&&cause.operationID){operation.current=cause.operationID;schedule();}else fail(cause);}finally{busy.current=false;}
  }
  return {state,error,open,check};
}
