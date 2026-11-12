import { buildApiUrl } from './apiBase';

export class PortalError extends Error {
  constructor(public status:number,public code:string,public operationID?:string){super(status===401?'This billing link has expired or been revoked. Please request a new link from the business.':status===404?'This billing record could not be found.':'Could not complete this request. Please try again.');}
}
// Deliberately independent of dashboard Axios/auth interceptors and cookies.
export async function portalRequest<T>(token:string,path='',options:{method?:'GET'|'POST';body?:unknown;key?:string;signal?:AbortSignal}={}):Promise<T>{
  const response=await fetch(buildApiUrl(`/portal/${encodeURIComponent(token)}${path}`),{method:options.method??'GET',credentials:'omit',cache:'no-store',referrerPolicy:'no-referrer',signal:options.signal,headers:{Accept:'application/json',...(options.body!==undefined?{'Content-Type':'application/json'}:{}),...(options.key?{'Idempotency-Key':options.key}:{})},...(options.body!==undefined?{body:JSON.stringify(options.body)}:{})});
  const payload=await response.json().catch(()=>({}));
  if(!response.ok)throw new PortalError(response.status,payload.code??'',payload.operation_id);
  return payload.data as T;
}
export interface PortalCheckoutDescriptor {operation_id:string;state:string;key_id?:string;gateway_order_id?:string;gateway_subscription_id?:string;currency:string;amount_due_paisa?:number;amount_paisa?:number;merchant_display_name?:string;total_count?:number;start_at?:string}
export interface PortalOperation {operation_id:string;state:string;result?:{state:string}}
