import { spawn } from 'node:child_process';
const allowed=['PATH','Path','SYSTEMROOT','SystemRoot','WINDIR','COMSPEC','ComSpec','PATHEXT','TEMP','TMP','HOME','USERPROFILE','LOCALAPPDATA','APPDATA'];
const env=Object.fromEntries(allowed.filter(k=>process.env[k]).map(k=>[k,process.env[k]]));
env.FRN_DOCS_TEST='1';env.VITE_API_BASE_URL=process.env.FRN_DOCS_API_BASE || '/v1';
if(process.env.FRN_CHANNEL_LEGACY_IDS==='1'){env.VITE_META_APP_ID='legacy-public-app';env.VITE_META_CONFIG_ID='legacy-public-signup';}
function run(args){return new Promise((resolve,reject)=>{const p=spawn(process.execPath,args,{env,stdio:'inherit'});p.on('error',reject);p.on('exit',code=>code===0?resolve():reject(new Error(`command failed ${code}`)));});}
await run(['ui/node_modules/vite/bin/vite.js','build','--config','ui/vite.config.ts','--outDir','dist-docs','--emptyOutDir','ui']);
const child=spawn(process.execPath,['ui/node_modules/vite/bin/vite.js','preview','--config','ui/vite.config.ts','--outDir','dist-docs','--host','127.0.0.1','--port','3011','--strictPort','ui'],{env,stdio:'inherit'});
for(const signal of ['SIGINT','SIGTERM'])process.on(signal,()=>child.kill(signal));child.on('exit',code=>process.exit(code??1));
