import {useEffect,useState} from 'react';
import {channelConnectionsAPI,environmentsAPI,type ChannelConnectionView} from '../../services/api';
import type {Environment} from '../../types';
import {Button} from '../ui/button';
import {Input} from '../ui/input';
import {Label} from '../ui/label';
import {Card,CardHeader,CardTitle,CardContent} from '../ui/card';
import {toast} from 'sonner';
import ExotelFields,{emptyDomesticSMS} from './ExotelFields';

function Connection({appId,channel,environmentId,onChanged,onLoaded}:{appId:string;channel:'sms'|'whatsapp';environmentId:string;onChanged:()=>Promise<void>;onLoaded?:(view:ChannelConnectionView)=>void}){
 const [view,setView]=useState<ChannelConnectionView|null>(null);const [error,setError]=useState('');const [busy,setBusy]=useState(false);
 const [domestic,setDomestic]=useState(emptyDomesticSMS);
 const [form,setForm]=useState({provider:channel==='sms'?'twilio':'meta',credential_source:'byoc',account_id:'',sender_id:'',token:'',signing:''});
 useEffect(()=>{let live=true;setView(null);setError('');channelConnectionsAPI.get(appId,channel,environmentId).then(v=>{if(live){setView(v);setDomestic(v.exotel||emptyDomesticSMS());onLoaded?.(v);setForm({provider:v.provider|| (channel==='sms'?'twilio':'meta'),credential_source:v.credential_source||'byoc',account_id:v.account_id||'',sender_id:v.sender_id||'',token:'',signing:''})}}).catch(()=>{if(live)setError('Could not load this connection. Check your access and retry.')});return()=>{live=false}},[appId,channel,environmentId,onLoaded]);
 const save=async(disconnect=false)=>{if(!view)return;setBusy(true);setError('');try{
  const patch:Record<string,unknown>={revision:view.revision??0};if(disconnect)patch.disconnect=true;else{
   for(const field of ['provider','credential_source','account_id','sender_id'] as const){if(view.inherited||form[field]!==view[field]||!view.revision)patch[field]=form[field]}
   if(form.provider==='exotel')patch.exotel=domestic;
   patch.secrets={...(form.token?{[form.provider==='meta'?'access_token':form.provider==='exotel'?'api_token':'auth_token']:form.token}:{}),...(form.signing?{[form.provider==='exotel'?'api_key':'app_secret']:form.signing}:{})};
  }
  const v=await channelConnectionsAPI.update(appId,channel,environmentId,patch);setView(v);setForm(f=>({...f,token:'',signing:''}));await onChanged();toast.success(disconnect?'Connection disconnected':'Connection saved. Validate readiness before sending.');
 }catch(e:any){setError(e.response?.data?.error?.message||e.response?.data?.message||'Connection update failed. Reload before retrying.')}finally{setBusy(false)}};
 const validate=async()=>{if(!view)return;setBusy(true);try{setView(await channelConnectionsAPI.validate(appId,channel,environmentId,view.revision));toast.success('Readiness checked. No message was sent.')}catch{setError('Validation failed. Reload the connection and retry.')}finally{setBusy(false)}};
 const metaDisabled=form.provider==='meta'&&view?.meta_enabled===false;
 const prefix=`${channel}-connection`;
 let phones:{id:string;display_phone_number?:string}[]=[];try{phones=JSON.parse(view?.facts?.available_phone_numbers||'[]')}catch{phones=[]}
 return <Card data-testid={`connection-${channel}`}><CardHeader><CardTitle>{channel==='sms'?'SMS connection':'WhatsApp connection'}</CardTitle></CardHeader><CardContent className="space-y-3">
  {error&&<p role="alert">{error}</p>}{!view&&!error&&<p>Loading connection…</p>}
  {view&&<><p>Readiness: <strong>{view.readiness}</strong>{view.inherited?' (inherited from application default)':''}</p>
   <Label htmlFor={`${prefix}-provider`}>Provider</Label><select id={`${prefix}-provider`} value={form.provider} disabled={busy||!view.writes_enabled} onChange={e=>setForm({...form,provider:e.target.value,credential_source:e.target.value==='exotel'?'byoc':form.credential_source,token:'',signing:''})} className="block border rounded p-2 bg-background"><option value="twilio">Twilio</option>{channel==='sms'&&<option value="exotel">Exotel (Indian domestic SMS)</option>}{channel==='whatsapp'&&<option value="meta">Meta Cloud API</option>}</select>
   <Label htmlFor={`${prefix}-source`}>Credentials</Label><select id={`${prefix}-source`} value={form.credential_source} disabled={busy||!view.writes_enabled} onChange={e=>setForm({...form,credential_source:e.target.value})} className="block border rounded p-2 bg-background"><option value="byoc">My business account</option><option value="system" disabled={form.provider==='exotel'}>Platform account</option></select>
   <Label htmlFor={`${prefix}-account`}>{form.provider==='meta'?'WhatsApp Business Account ID':'Account SID'}</Label><Input id={`${prefix}-account`} value={form.account_id} onChange={e=>setForm({...form,account_id:e.target.value})} disabled={busy||!view.writes_enabled}/>
   <Label htmlFor={`${prefix}-sender`}>{form.provider==='meta'?'Phone number ID':form.provider==='exotel'?'DLT sender header':'Sender phone number'}</Label><Input id={`${prefix}-sender`} value={form.sender_id} onChange={e=>setForm({...form,sender_id:e.target.value})} disabled={busy||!view.writes_enabled}/>
   {form.provider==='meta'&&phones.length>0&&<><Label htmlFor={`${prefix}-phone-choice`}>Owned Meta phone number</Label><select id={`${prefix}-phone-choice`} value={form.sender_id} disabled={busy||!view.writes_enabled} onChange={e=>setForm({...form,sender_id:e.target.value})} className="block border rounded p-2 bg-background"><option value="">Select a phone number</option>{phones.map(phone=><option key={phone.id} value={phone.id}>{phone.display_phone_number||phone.id} ({phone.id})</option>)}</select></>}
   {form.credential_source==='byoc'&&<><Label htmlFor={`${prefix}-token`}>{form.provider==='meta'?'Access token':form.provider==='exotel'?'API token':'Auth token'}</Label><Input id={`${prefix}-token`} type="password" autoComplete="new-password" value={form.token} onChange={e=>setForm({...form,token:e.target.value})} placeholder="Leave blank to retain configured token" disabled={busy||!view.writes_enabled}/>{(form.provider==='meta'||form.provider==='exotel')&&<><Label htmlFor={`${prefix}-signing`}>{form.provider==='exotel'?'API key':'Customer Meta app secret'}</Label><Input id={`${prefix}-signing`} type="password" autoComplete="new-password" value={form.signing} onChange={e=>setForm({...form,signing:e.target.value})} disabled={busy||!view.writes_enabled}/></>}</>}
   <div className="flex flex-wrap gap-2"><Button disabled={busy||!view.writes_enabled||metaDisabled} onClick={()=>save()}>Save connection</Button><Button variant="outline" disabled={busy||metaDisabled||!view.revision||view.readiness==='disconnected'} onClick={validate}>Validate readiness</Button><Button variant="outline" disabled={busy||!view.writes_enabled} onClick={()=>save(true)}>Disconnect</Button></div>
   {metaDisabled&&<p>Meta onboarding is disabled for this deployment. Existing connection details remain available.</p>}
   {form.provider==='exotel'&&<><ExotelFields value={domestic} onChange={setDomestic} disabled={busy||!view.writes_enabled}/>{view.exotel_enabled===false&&<p>Domestic SMS is disabled for this deployment.</p>}</>}
   {!view.writes_enabled&&<p>Connection updates will become available after the platform upgrade. Existing settings continue to work.</p>}
   <p className="text-sm text-muted-foreground">Saving credentials does not confirm sender registration or billing readiness. Validation sends no paid message.</p>
   {Object.entries(view.facts||{}).filter(([key])=>key!=='available_phone_numbers').map(([key,value])=><p key={key} className="text-sm">{key.replace(/_/g,' ')}: {value}</p>)}
   <a href={form.provider==='meta'?'https://business.facebook.com/latest/whatsapp_manager/phone_numbers/':form.provider==='exotel'?'https://my.exotel.com/':'https://console.twilio.com/'} target="_blank" rel="noreferrer" className="underline">Manage sender in provider console</a>
  </>}
 </CardContent></Card>;
}
export default function ChannelConnections({appId,environmentId,onEnvironmentChange,onChanged,onWhatsAppLoaded}:{appId:string;environmentId:string;onEnvironmentChange:(id:string)=>void;onChanged:()=>Promise<void>;onWhatsAppLoaded?:(view:ChannelConnectionView)=>void}){
 const [environments,setEnvironments]=useState<Environment[]>([]);const [environmentError,setEnvironmentError]=useState('');
 useEffect(()=>{let live=true;environmentsAPI.list(appId).then(rows=>{if(live)setEnvironments(rows||[])}).catch(()=>{if(live)setEnvironmentError('Could not load environments; application default remains selected.')});return()=>{live=false}},[appId]);
 return <div className="space-y-4"><Label htmlFor="messaging-environment">Messaging environment</Label><select id="messaging-environment" className="block border rounded p-2 bg-background" value={environmentId} onChange={e=>onEnvironmentChange(e.target.value)}><option value="default">Application default</option>{environments.map(env=><option key={env.id} value={env.id}>{env.name}</option>)}</select>{environmentError&&<p role="alert">{environmentError}</p>}{(['whatsapp','sms'] as const).map(channel=><Connection key={`${appId}:${environmentId}:${channel}`} appId={appId} channel={channel} environmentId={environmentId} onChanged={onChanged} onLoaded={channel==='whatsapp'?onWhatsAppLoaded:undefined}/>)}</div>
}
