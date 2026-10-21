import type {DomesticSMSConfig} from '../../services/api';
import {Input} from '../ui/input';
import {Label} from '../ui/label';
import {Button} from '../ui/button';

export function emptyDomesticSMS():DomesticSMSConfig{return {region:'mumbai',entity_id:'',sms_type:'transactional',verification_message_id:'',approval_reference:'',approved_at:'',templates:[],limits:{messages_per_minute:0,units_per_minute:0,daily_units:0,daily_spend_minor:0,cost_minor_per_unit:0,currency:'INR',max_segments:3,rate_reference:'',rate_verified_at:''}}}

export default function ExotelFields({value,onChange,disabled}:{value:DomesticSMSConfig;onChange:(value:DomesticSMSConfig)=>void;disabled:boolean}){
 const set=(field:keyof DomesticSMSConfig,v:unknown)=>onChange({...value,[field]:v});
 const limit=(field:keyof DomesticSMSConfig['limits'],v:unknown)=>onChange({...value,limits:{...value.limits,[field]:v}});
 return <fieldset disabled={disabled} className="space-y-3 border rounded p-3">
  <legend className="px-1 font-medium">Indian domestic SMS</legend>
  <Label htmlFor="exotel-region">Account region</Label><select id="exotel-region" value={value.region} onChange={e=>set('region',e.target.value)} className="block border rounded p-2 bg-background"><option value="mumbai">Mumbai</option><option value="singapore">Singapore</option></select>
  <Label htmlFor="exotel-entity">DLT entity ID</Label><Input id="exotel-entity" value={value.entity_id} onChange={e=>set('entity_id',e.target.value)}/>
  <Label htmlFor="exotel-type">SMS type</Label><select id="exotel-type" value={value.sms_type} onChange={e=>set('sms_type',e.target.value)} className="block border rounded p-2 bg-background"><option value="transactional">Transactional / OTP</option><option value="transactional_opt_in">Service explicit / opted in</option><option value="promotional">Promotional</option></select>
  {value.templates.map((t,i)=><div key={i} className="space-y-2 border rounded p-2">
   <Label htmlFor={`exotel-template-${i}`}>DLT template ID {i+1}</Label><Input id={`exotel-template-${i}`} value={t.id} onChange={e=>set('templates',value.templates.map((v,j)=>j===i?{...v,id:e.target.value}:v))}/>
   <Label htmlFor={`exotel-body-${i}`}>Approved template body {i+1}</Label><textarea id={`exotel-body-${i}`} className="w-full border rounded p-2 bg-background" value={t.body} onChange={e=>set('templates',value.templates.map((v,j)=>j===i?{...v,body:e.target.value}:v))}/>
   <Label htmlFor={`exotel-length-${i}`}>Maximum characters per variable {i+1}</Label><Input id={`exotel-length-${i}`} type="number" min={1} max={1000} value={t.variable_max_length} onChange={e=>set('templates',value.templates.map((v,j)=>j===i?{...v,variable_max_length:Number(e.target.value)}:v))}/>
   <Button type="button" variant="outline" onClick={()=>set('templates',value.templates.filter((_,j)=>j!==i))}>Remove template {i+1}</Button>
  </div>)}
  <Button type="button" variant="outline" onClick={()=>set('templates',[...value.templates,{id:'',body:'',variable_max_length:30}])}>Add DLT template</Button>
  <p className="text-sm text-muted-foreground">Copy the approved body exactly, including {'{#var#}'} placeholders. Multiple bindings require dlt_template_id in message data. This route sends only to Indian mobile numbers.</p>
  <Label htmlFor="exotel-approval">DLT approval reference</Label><Input id="exotel-approval" value={value.approval_reference} onChange={e=>set('approval_reference',e.target.value)} placeholder="https://…"/>
  <label className="flex gap-2"><input type="checkbox" checked={!!value.approved_at} onChange={e=>set('approved_at',e.target.checked?new Date().toISOString():'')}/>I confirmed these entity, sender and template approvals in Exotel</label>
  <Label htmlFor="exotel-verification">Previously delivered SMS SID for account validation</Label><Input id="exotel-verification" value={value.verification_message_id} onChange={e=>set('verification_message_id',e.target.value)}/>
  <p className="text-sm text-muted-foreground">Validation reads an existing delivered message from your account and sender. It does not send a test message or certify DLT approval automatically.</p>
  {([
   ['messages_per_minute','Messages per minute'],['units_per_minute','SMS segments per minute'],['daily_units','Daily SMS segment budget'],['daily_spend_minor','Daily vendor spend budget (minor currency units)'],['cost_minor_per_unit','Verified vendor cost per segment (minor currency units)'],['max_segments','Maximum segments per message'],
  ] as const).map(([key,label])=><div key={key}><Label htmlFor={`exotel-${key}`}>{label}</Label><Input id={`exotel-${key}`} type="number" min={0} value={value.limits[key]} onChange={e=>limit(key,Number(e.target.value))}/></div>)}
  <Label htmlFor="exotel-currency">Vendor currency</Label><Input id="exotel-currency" value={value.limits.currency} onChange={e=>limit('currency',e.target.value.toUpperCase())}/>
  <Label htmlFor="exotel-rate-reference">Provider rate reference</Label><Input id="exotel-rate-reference" value={value.limits.rate_reference} onChange={e=>limit('rate_reference',e.target.value)} placeholder="https://…"/>
  <label className="flex gap-2"><input type="checkbox" checked={!!value.limits.rate_verified_at} onChange={e=>limit('rate_verified_at',e.target.checked?new Date().toISOString():'')}/>I confirmed this segment cost with Exotel</label>
  <p className="text-sm text-muted-foreground">Sender capacity and estimated vendor spend are separate from the application's HTTP request limit. Zero budgets disable dispatch. Delivery attempts consume conservative budget estimates; actual provider invoices may differ. FRN credit pricing stays unchanged until a new tariff is activated.</p>
 </fieldset>;
}
