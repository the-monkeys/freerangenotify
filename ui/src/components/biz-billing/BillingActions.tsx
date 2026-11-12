import { createContext, useContext, useEffect, useRef, useState, type ReactNode } from 'react';
import { bizBillingAPI } from '../../services/api';
import { Button } from '../ui/button';

type SavedAction = { label:string; path:string; body:unknown; key:string; method:'POST'|'PUT'|'DELETE'; operationID?:string; gateway?:boolean };
type State = { saved?:SavedAction; busy:boolean; pending:boolean; error:string; message:string };
type ActionAPI = State & { retry:()=>void;run:(label:string,path:string,body?:unknown,method?:SavedAction['method'])=>Promise<boolean> };
const Context = createContext<ActionAPI | null>(null);
// Only unfinished, non-secret staff actions live here. Never persist these in browser storage.
const unfinished = new Map<string,SavedAction>();
let actor = '';
const friendly:Record<string,string> = {
  BILLING_LEGACY_REVIEW_REQUIRED:'Existing credits need review before they can be used.',
  BILLING_COLLECTION_LOCKED:'This invoice is owned by automatic collection. Stop that collection before recording a manual payment.',
  BILLING_CURRENCY_MISMATCH:'The selected records use different currencies.',
  BILLING_IDEMPOTENCY_CONFLICT:'This saved request conflicts with an earlier action. Check the record before starting a new action.',
  BILLING_INVALID_AMOUNT:'This amount cannot be applied or refunded. Check the remaining amount and whether any overpayment credit has been used.',
};
export function BillingActions({apiKey,scope,actorID,children}:{apiKey:string;scope:string;actorID:string;children:ReactNode}) {
  if(actor !== actorID){ unfinished.clear(); actor=actorID; }
  const [state,setState]=useState<State>({saved:unfinished.get(scope),busy:false,pending:unfinished.has(scope),error:'',message:''});
  const alive=useRef(true), locked=useRef(false);
  useEffect(()=>{alive.current=true;return()=>{alive.current=false;};},[]);
  useEffect(()=>{
    const guard=(event:BeforeUnloadEvent)=>{if(unfinished.has(scope)){event.preventDefault();event.returnValue='';}};
    window.addEventListener('beforeunload',guard);return()=>window.removeEventListener('beforeunload',guard);
  },[scope]);
  function update(next:State){if(alive.current)setState(next);}
  async function execute(saved:SavedAction,check=false):Promise<boolean>{
    if(locked.current)return false;locked.current=true;
    unfinished.set(scope,saved);update({saved,busy:true,pending:true,error:'',message:''});
    try{
      let result:any;
      if(check && saved.operationID){
        const operationID=encodeURIComponent(saved.operationID);
        if(!saved.gateway){
          try{result=await bizBillingAPI.readBilling<any>(apiKey,`/biz/operations/${operationID}`);}
          catch(error:any){if(error?.response?.status!==404)throw error;saved={...saved,gateway:true};unfinished.set(scope,saved);}
          if(!saved.gateway&&!['completed','rejected'].includes(result?.state))result=await bizBillingAPI.submitBilling<any>(apiKey,`/biz/operations/${operationID}/resume`,{},saved.key);
        }
        if(saved.gateway)result=await bizBillingAPI.readBilling<any>(apiKey,`/biz/gateway-effects/${operationID}`);
      }else result=await bizBillingAPI.submitBilling<any>(apiKey,saved.path,saved.body,saved.key,saved.method);
      if(result?.operation_id && result.state==='rejected'){
        unfinished.delete(scope);update({busy:false,pending:false,error:'The saved action was rejected. Review the record and your form before trying again.',message:''});return false;
      }
      if(result?.operation_id && result.state!=='completed'){
        saved={...saved,operationID:result.operation_id};unfinished.set(scope,saved);
        update({saved,busy:false,pending:true,error:'',message:`${saved.label} pending. Check the saved action before trying anything else.`});return false;
      }
      if(check && saved.gateway && !['completed','confirmed','captured','refunded','succeeded'].includes(result?.state)){
        update({saved,busy:false,pending:true,error:'',message:`${saved.label} pending. Payment provider confirmation is still required.`});return false;
      }
      unfinished.delete(scope);update({busy:false,pending:false,error:'',message:`${saved.label} completed.`});
      if(alive.current)window.dispatchEvent(new Event('billing:changed'));return true;
    }catch(error:any){
      const response=error?.response, data=response?.data??{};
      const pending=!response || response.status>=500 || data.retryable===true || !!data.operation_id || !!saved.operationID;
      if(pending){saved={...saved,operationID:data.operation_id??saved.operationID,gateway:data.code==='BILLING_GATEWAY_PENDING'||saved.gateway};unfinished.set(scope,saved);}
      else unfinished.delete(scope);
      update({saved:pending?saved:undefined,busy:false,pending,error:pending?'':(friendly[data.code]??'Could not complete this action. Your form is unchanged. Refresh the record and check its current status before trying again.'),message:pending?`${saved.label} pending. The response was interrupted or confirmation is still outstanding.`:''});return false;
    }finally{locked.current=false;}
  }
  async function run(label:string,path:string,body:unknown={},method:SavedAction['method']='POST'){
    if(unfinished.has(scope)||locked.current)return false;
    return execute({label,path,body:JSON.parse(JSON.stringify(body)),key:crypto.randomUUID(),method});
  }
  return <Context.Provider value={{...state,run,retry:()=>{if(state.saved)void execute(state.saved,!!state.saved.operationID);}}}>
    <BillingActionNotice/>{children}
  </Context.Provider>;
}
export function useBillingActions(){const context=useContext(Context);if(!context)throw new Error('Billing action context is required');return context;}
export function BillingActionNotice(){const action=useBillingActions();return (action.message||action.error)?<div className="space-y-2 rounded-md border p-3 text-sm">{action.message&&<p role="status">{action.message}</p>}{action.error&&<p role="alert">{action.error}</p>}{action.saved&&<Button type="button" disabled={action.busy} onClick={action.retry}>{action.saved.operationID?'Check saved action':'Retry saved request'}</Button>}</div>:null;}
