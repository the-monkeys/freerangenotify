import {spawn} from 'node:child_process';
import {readFile} from 'node:fs/promises';
import {resolve,dirname} from 'node:path';
import {fileURLToPath} from 'node:url';
const root=resolve(dirname(fileURLToPath(import.meta.url)),'..');
const allowed=['PATH','Path','SYSTEMROOT','SystemRoot','WINDIR','COMSPEC','ComSpec','PATHEXT','TEMP','TMP','HOME','USERPROFILE','LOCALAPPDATA','APPDATA','GOROOT','GOPATH','GOCACHE','GOMODCACHE','GOTOOLCHAIN','CGO_ENABLED','GOMAXPROCS','FRN_OTP_REDIS_ADDR'];
export function testEnvironment(source=process.env){
 const env=Object.fromEntries(allowed.filter(k=>source[k]).map(k=>[k,source[k]]));
 env.GOPROXY=source.GOPROXY==='off'?'off':'https://proxy.golang.org';env.GOSUMDB=source.GOSUMDB==='off'?'off':'sum.golang.org';
 env.FRN_DOCS_TEST='1';return env;
}
export function validateCases(cases){
 if(!Array.isArray(cases)||!cases.length)throw new Error('Invalid contract: expected nonempty array');const seen=new Set();
 for(const c of cases){if(!c || !/^[a-z0-9-]+$/.test(c.id)||!['GET','POST','PUT','PATCH','DELETE'].includes(c.method)||typeof c.path!=='string'||!c.path.startsWith('/')||!['jwt','api-key','x-api-key','none'].includes(c.auth)||!c.guide||!Number.isInteger(c.status)||c.status<100||c.status>599||!c.response)throw new Error('Invalid contract fixture');if(seen.has(c.id))throw new Error('Duplicate contract ID');seen.add(c.id);}
}
export function run(command,args){return new Promise((resolve,reject)=>{console.log('>',command,...args);const p=spawn(command,args,{cwd:root,env:testEnvironment(),stdio:'inherit',shell:false});p.on('error',reject);p.on('exit',code=>code===0?resolve():reject(new Error(`Documentation check failed (${code}): ${command}`)));});}
export async function verify(){
 validateCases(JSON.parse(await readFile(resolve(root,'examples/docs/api-cases.json'),'utf8')));
 await run('go',['run','./cmd/docsgen','-check']);
 await run('go',['test','-p','2','./tests/contracts','./examples/docs/go','-count=1']);
 await run(process.execPath,['ui/node_modules/typescript/bin/tsc','--noEmit','-p','examples/docs/typescript/tsconfig.json']);
 await run(process.execPath,['scripts/verify-webhook-receiver.mjs']);
}
if(process.argv[1]&&resolve(process.argv[1])===fileURLToPath(import.meta.url)){
 try{if(process.argv.length>2){if(process.argv.length!==4||process.argv[2]!=='--validate-fixture')throw new Error('Usage: verify-docs.mjs [--validate-fixture file]');validateCases(JSON.parse(await readFile(process.argv[3],'utf8')));}else await verify();}catch(error){console.error(error.message);process.exitCode=1;}
}
