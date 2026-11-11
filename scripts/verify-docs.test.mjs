import {test} from 'node:test';
import assert from 'node:assert/strict';
import {mkdtemp,writeFile,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';import {join} from 'node:path';import {spawnSync} from 'node:child_process';
import {testEnvironment,validateCases} from './verify-docs.mjs';
test('documentation children receive an explicit environment allowlist',()=>{const e=testEnvironment({PATH:'tools',SECRET_TOKEN:'customer',FREERANGE_PROVIDERS_TWILIO_TOKEN:'customer',VITE_REAL_KEY:'customer',FRN_OTP_REDIS_ADDR:'127.0.0.1:16390'});assert.equal(e.PATH,'tools');assert.equal(e.FRN_OTP_REDIS_ADDR,'127.0.0.1:16390');assert.equal(e.SECRET_TOKEN,undefined);assert.equal(e.FREERANGE_PROVIDERS_TWILIO_TOKEN,undefined);assert.equal(e.VITE_REAL_KEY,undefined);});
test('invalid contract fixture makes the executable verifier fail',async()=>{const dir=await mkdtemp(join(tmpdir(),'frn-invalid-docs-'));try{const file=join(dir,'cases.json');await writeFile(file,JSON.stringify([{id:'broken',method:'EXEC',path:'oops',status:202}]));const p=spawnSync(process.execPath,['scripts/verify-docs.mjs','--validate-fixture',file],{encoding:'utf8',env:testEnvironment()});assert.notEqual(p.status,0);assert.match(p.stderr,/invalid contract/i);}finally{await rm(dir,{recursive:true,force:true});}});
test('duplicate contract IDs are rejected',()=>{const fixture={id:'a',method:'GET',path:'/users',auth:'api-key',guide:'users',status:200,response:{}};assert.throws(()=>validateCases([fixture,fixture]),/duplicate/i);});
