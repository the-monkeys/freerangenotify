// Only edited fields belong in a settings update; untouched connection records
// must not restore a disconnected sender from an old browser snapshot.
export function channelSettingsPatch(before:Record<string,any>,after:Record<string,any>):Record<string,any>{
 const result:Record<string,any>={};
 for(const [key,value] of Object.entries(after)){
  if(key==='secret_configured'||JSON.stringify(value)===JSON.stringify(before[key]))continue;
  if(value&&typeof value==='object'&&!Array.isArray(value)){
   const nested=channelSettingsPatch(before[key]||{},value);
   if(Object.keys(nested).length)result[key]=nested;
  } else result[key]=value;
 }
 return result;
}
