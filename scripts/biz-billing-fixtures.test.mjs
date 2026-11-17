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

test('a saved or another disposable FreeRangeNotify cluster prevents a second cluster',()=>{
  assert.equal(typeof fixture.assertNoRunningBillingProject,'function');
  assert.throws(()=>fixture.assertNoRunningBillingProject(['frn-biz-saved']),/already running/i);
  assert.throws(()=>fixture.assertNoRunningBillingProject(['freerangenotify']),/already running/i);
  assert.doesNotThrow(()=>fixture.assertNoRunningBillingProject(['unrelated-project']));
});

test('partial-start cleanup discovers and removes only owned containers, volumes and networks',async()=>{
  assert.equal(typeof fixture.cleanupOwnedFixture,'function');
  const runID='a'.repeat(32),project='frn-biz-'+runID,container='b'.repeat(64),network='c'.repeat(64),volume=project+'_data';
  const labels={'com.freerangenotify.fixture.owner':runID,'com.docker.compose.project':project};
  const mutations=[];
  const docker=async args=>{
    if(args[0]==='ps'){assert.ok(args.includes('--no-trunc'),'cleanup must compare complete container IDs');return container;}
    if(args[0]==='inspect')return JSON.stringify([{Id:container,Config:{Labels:labels}}]);
    if(args[0]==='volume'&&args[1]==='ls')return volume;
    if(args[0]==='volume'&&args[1]==='inspect')return JSON.stringify([{Name:volume,Labels:labels}]);
    if(args[0]==='network'&&args[1]==='ls'){assert.ok(args.includes('--no-trunc'),'cleanup must compare complete network IDs');return network;}
    if(args[0]==='network'&&args[1]==='inspect')return JSON.stringify([{Id:network,Labels:labels}]);
    mutations.push(args);return '';
  };
  await fixture.cleanupOwnedFixture({runID,project},docker);
  assert.ok(mutations.some(args=>args[0]==='rm'&&args.includes(container)));
  assert.ok(mutations.some(args=>args[0]==='volume'&&args[1]==='rm'&&args.includes(volume)));
  assert.ok(mutations.some(args=>args[0]==='network'&&args[1]==='rm'&&args.includes(network)));
});

test('cleanup refuses an ownership mismatch before mutating resources',async()=>{
  assert.equal(typeof fixture.cleanupOwnedFixture,'function');
  let mutations=0;
  const docker=async args=>{
    if(args[0]==='ps')return 'b'.repeat(64);
    if(args[0]==='inspect')return JSON.stringify([{Id:'b'.repeat(64),Config:{Labels:{'com.freerangenotify.fixture.owner':'other'}}}]);
    if(args.includes('rm'))mutations++;
    return '';
  };
  await assert.rejects(()=>fixture.cleanupOwnedFixture({runID:'a'.repeat(32),project:'frn-biz-'+'a'.repeat(32)},docker),/owner|identity/i);
  assert.equal(mutations,0);
});
