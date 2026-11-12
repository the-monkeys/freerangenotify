import { useId, useState } from 'react';
import { usersAPI } from '../../services/api';
import type { User } from '../../types';
import { Button } from '../ui/button';
import { Input } from '../ui/input';
import { BillingLoadError } from './BillingForm';
import { useBillingSearch, useBizBillingResource } from './useBizBillingResource';

export default function BillingCustomerPicker({apiKey,appId,value,onChange,disabled=false}:{apiKey:string;appId:string;value?:User;onChange:(user:User)=>void;disabled?:boolean}){
  const id=useId();const [open,setOpen]=useState(false),[query,setQuery]=useState(''),[page,setPage]=useState(1);
  const search=useBillingSearch(query);
  const result=useBizBillingResource(`${appId}:customers:${open}:${search}:${page}`,signal=>open?usersAPI.list(apiKey,page,20,search,signal):Promise.resolve(undefined));
  return <div className="min-w-0 space-y-2"><Button type="button" role="combobox" aria-label="Customer" aria-controls={id} aria-expanded={open} className="h-auto min-h-10 w-full justify-start whitespace-normal break-all text-left" variant="outline" disabled={disabled} onClick={()=>setOpen(!open)}>{value?`${value.full_name||value.email} — ${value.email}`:'Select customer'}</Button>
    {open&&<div className="space-y-2 rounded-md border p-3">
      <Input autoFocus placeholder="Search by name or email" aria-label="Search customers" value={query} onChange={e=>{setQuery(e.target.value);setPage(1);}} />
      {result.loading&&<p role="status">Loading customers…</p>}
      {!!result.error&&<BillingLoadError label="customers" retry={result.retry}/>}
      <div id={id} role="listbox" aria-label="Customers" className="max-h-52 overflow-y-auto">
        {result.data?.users.map(user=><button key={user.user_id} role="option" aria-selected={value?.user_id===user.user_id} type="button" className="block w-full break-all rounded p-2 text-left text-sm hover:bg-muted focus-visible:outline-2" onClick={()=>{onChange(user);setOpen(false);}}>{user.full_name||user.email} — {user.email}</button>)}
      </div>
      {!result.loading&&!result.error&&result.data?.users.length===0&&<p>No customers found. Add a subscriber first.</p>}
      <div className="flex items-center gap-2"><Button type="button" variant="outline" disabled={page===1||result.loading} onClick={()=>setPage(page-1)}>Previous customers</Button><span className="text-sm">{page}</span><Button type="button" variant="outline" disabled={!result.data||page*20>=result.data.total_count||result.loading} onClick={()=>setPage(page+1)}>More customers</Button></div>
    </div>}
  </div>;
}
