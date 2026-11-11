import {test} from 'node:test';
import assert from 'node:assert/strict';
const fixture = await import('./biz-billing-fixtures.mjs').catch(e => e.code === 'ERR_MODULE_NOT_FOUND' ? {} : Promise.reject(e));
test('billing fixture children receive no customer credentials', () => {
  assert.equal(typeof fixture.childEnvironment, 'function', 'fixture environment allowlist is missing');
  const env=fixture.childEnvironment({PATH:'fixture-path',ProgramData:'fixture-program-data',ProgramFiles:'fixture-program-files',FREERANGE_PAYMENT_RAZORPAY_KEY_SECRET:'customer-secret',NODE_OPTIONS:'unsafe',VITE_API_TOKEN:'customer-token',GOPROXY:'off'});
  assert.equal(env.PATH,'fixture-path');
  assert.equal(env.GOPROXY,'off');
  assert.equal(env.ProgramData,'fixture-program-data');
  assert.equal(env.ProgramFiles,'fixture-program-files');
  for(const key of ['FREERANGE_PAYMENT_RAZORPAY_KEY_SECRET','NODE_OPTIONS','VITE_API_TOKEN'])assert.equal(env[key],undefined);
});
test('billing fixture identity rejects unrelated containers and default ports', () => {
  assert.equal(typeof fixture.validateManifest,'function','fixture ownership guard is missing');
  assert.throws(()=>fixture.validateManifest({runID:'owned',project:'frn-biz-owned',services:{elasticsearch:{id:'unrelated',url:'http://127.0.0.1:9200'}}},[]),/owned|fixture|identity/i);
});
test('billing fixture command parser does not invoke a shell', () => {
  assert.equal(typeof fixture.parseRun,'function','fixture command parser is missing');
  assert.deepEqual(fixture.parseRun(['run','--stack','--','go','test','./tests/integration']),{stack:true,command:['go','test','./tests/integration']});
  assert.throws(()=>fixture.parseRun(['run','--','']),/command/i);
});
test('billing fixtures cannot start on a remote Docker engine',()=>{
  assert.equal(typeof fixture.assertLocalDocker,'function','local Docker guard is missing');
  assert.throws(()=>fixture.assertLocalDocker('ssh://prod.example.invalid'),/local/i);
  assert.throws(()=>fixture.assertLocalDocker('tcp://192.168.1.15:2376'),/local/i);
  assert.doesNotThrow(()=>fixture.assertLocalDocker('npipe:////./pipe/dockerDesktopLinuxEngine'));
});
