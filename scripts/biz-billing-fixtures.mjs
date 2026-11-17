import {spawn} from 'node:child_process';
import {mkdtemp,writeFile,readFile,rm,open} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {resolve,join} from 'node:path';
import {fileURLToPath} from 'node:url';
import {randomUUID,randomBytes,createHmac} from 'node:crypto';
import {createServer} from 'node:net';

const root=resolve(fileURLToPath(new URL('..',import.meta.url)));
const compose=join(root,'tests/fixtures/biz-billing.compose.yml');
const ownerLabel='com.freerangenotify.fixture.owner';
export function sanitizeOutput(text,secrets=[]){
  let clean=text.replace(/(\/portal\/)[^/\s?"']+/g,'$1[REDACTED]')
    .replace(/\bpt2_[A-Za-z0-9_-]+/g,'[REDACTED]')
    .replace(/("(?:token|access_token|key_secret|webhook_secret|authorization)"\s*:\s*")[^"]*"/gi,'$1[REDACTED]"');
  for(const value of secrets.filter(Boolean))clean=clean.replaceAll(value,'[REDACTED]');
  return clean;
}
export function childEnvironment(source=process.env){
  const names=['PATH','Path','SYSTEMROOT','SystemRoot','WINDIR','COMSPEC','ComSpec','PATHEXT','TEMP','TMP','USERPROFILE','HOME','HOMEDRIVE','HOMEPATH','ProgramData','PROGRAMDATA','ProgramFiles','PROGRAMFILES','LOCALAPPDATA','APPDATA','GOROOT','GOPATH','GOCACHE','GOMODCACHE','GOTOOLCHAIN','GOMAXPROCS','CGO_ENABLED','DOCKER_HOST','DOCKER_CONTEXT'];
  const env=Object.fromEntries(names.filter(k=>source[k]).map(k=>[k,source[k]]));
  env.GOPROXY=source.GOPROXY==='off'?'off':'https://proxy.golang.org';
  env.GOSUMDB=source.GOSUMDB==='off'?'off':'sum.golang.org';
  return env;
}
export function parseRun(args){
  if(args[0]!=='run')throw new Error('Expected run [--stack] -- command');
  const marker=args.indexOf('--');const options=args.slice(1,marker);
  if(marker<1||options.some(x=>x!=='--stack')||!args[marker+1]?.trim())throw new Error('A shell-free command is required');
  return {stack:options.includes('--stack'),command:args.slice(marker+1)};
}
export function assertLocalDocker(host){
  if(!host?.startsWith('npipe://')&&!host?.startsWith('unix://')&&!/^tcp:\/\/(127\.0\.0\.1|localhost):[0-9]+$/.test(host??''))throw new Error('Billing fixtures require a local Docker engine');
}
export function assertNoRunningBillingProject(projects){
  if(projects.some(name=>/^(frn-biz-|freerangenotify)/i.test(name)))throw new Error('A FreeRangeNotify cluster is already running; stop it without deleting its volumes before disposable verification');
}
export async function cleanupOwnedFixture({runID,project},docker){
  if(!/^[a-f0-9]{32}$/.test(runID)||project!==`frn-biz-${runID}`)throw new Error('Unowned cleanup identity');
  const discover=async args=>(await docker(args)).split(/\s+/).filter(Boolean);
  const verify=(rows,ids,kind)=>{
    if(rows.length!==ids.length)throw new Error('Incomplete cleanup ownership inventory');
    for(const row of rows){
      const labels=kind==='container'?row.Config?.Labels:row.Labels;
      const id=kind==='volume'?row.Name:row.Id;
      if(!ids.includes(id)||labels?.[ownerLabel]!==runID||labels?.['com.docker.compose.project']!==project)throw new Error('Cleanup resource ownership mismatch');
    }
  };
  // Discover by nonce, not a possibly missing or partially constructed manifest.
  const containers=await discover(['ps','-aq','--no-trunc','--filter',`label=${ownerLabel}=${runID}`]);
  if(containers.length)verify(JSON.parse(await docker(['inspect',...containers])),containers,'container');
  const volumes=await discover(['volume','ls','-q','--filter',`label=${ownerLabel}=${runID}`]);
  if(volumes.length)verify(JSON.parse(await docker(['volume','inspect',...volumes])),volumes,'volume');
  const networks=await discover(['network','ls','-q','--no-trunc','--filter',`label=${ownerLabel}=${runID}`]);
  if(networks.length)verify(JSON.parse(await docker(['network','inspect',...networks])),networks,'network');
  // Volume/network removal is deliberately not forced: Docker refuses any
  // remaining unowned attachment. Container IDs were verified above.
  if(containers.length)await docker(['rm','-f','-v',...containers]);
  if(volumes.length)await docker(['volume','rm',...volumes]);
  if(networks.length)await docker(['network','rm',...networks]);
}
export function validateManifest(manifest,inspections){
  if(!/^[a-f0-9]{32}$/.test(manifest.runID)||manifest.project!==`frn-biz-${manifest.runID}`)throw new Error('Unowned fixture identity');
  for(const [name,target] of Object.entries(manifest.services??{})){
    const info=inspections.find(i=>i.Id===target.id);
    const u=new URL(target.url);
    if(!info||info.Config?.Labels?.[ownerLabel]!==manifest.runID||info.Config?.Labels?.['com.docker.compose.project']!==manifest.project||info.Config?.Labels?.['com.docker.compose.service']!==name||u.hostname!=='127.0.0.1'||Number(u.port)<10000)throw new Error('Unowned fixture container or endpoint');
    const port=name==='elasticsearch'?'9200/tcp':'6379/tcp';
    if(!info.NetworkSettings?.Ports?.[port]?.some(p=>p.HostIp==='127.0.0.1'&&p.HostPort===u.port))throw new Error('Fixture endpoint identity mismatch');
  }
  if(!manifest.services?.elasticsearch||!manifest.services?.redis)throw new Error('Missing owned fixture services');
  return manifest;
}
function run(command,args,{env,cwd=root,capture=false,stdio}={}){
  return new Promise((resolve,reject)=>{
    const child=spawn(command,args,{env,cwd,shell:false,windowsHide:true,stdio:stdio??(capture?['ignore','pipe','pipe']:'inherit')});
    let out='',err='';if(capture){child.stdout.on('data',x=>out+=x);child.stderr.on('data',x=>err+=x);}
    child.on('error',reject);child.on('exit',code=>code===0?resolve(out.trim()):reject(new Error(`${command} exited ${code}${capture?`: ${err.slice(-1500)}`:''}`)));
  });
}
async function freePort(){const s=createServer();await new Promise((ok,bad)=>{s.once('error',bad);s.listen(0,'127.0.0.1',ok);});const port=s.address().port;await new Promise(ok=>s.close(ok));return port;}
async function healthy(url){
  for(let i=0;i<90;i++){try{const r=await fetch(url,{signal:AbortSignal.timeout(1500)});if(r.ok)return;}catch{}await new Promise(ok=>setTimeout(ok,1000));}
  throw new Error('Owned fixture API health timeout');
}
async function startStack(dir,env,manifest,processes){
  const ext=process.platform==='win32'?'.exe':'';const secret=randomBytes(32).toString('hex');const port=await freePort();
  const redis=new URL(manifest.services.redis.url);
  Object.assign(env,{INTEGRATION_BASE_URL:`http://127.0.0.1:${port}`,INTEGRATION_ES_URL:manifest.services.elasticsearch.url,
    FREERANGE_DATABASE_URLS:manifest.services.elasticsearch.url,FREERANGE_REDIS_HOST:'127.0.0.1',FREERANGE_REDIS_PORT:redis.port,
    FREERANGE_SECURITY_JWT_SECRET:secret,JWT_SECRET:secret,FREERANGE_SECURITY_OPS_SECRET:randomBytes(32).toString('hex'),FREERANGE_SECURITY_OPS_ENABLED:'true',FREERANGE_OTP_REDIS_ADDR:`127.0.0.1:${redis.port}`,
    FREERANGE_FEATURES_BIZ_BILLING_ENABLED:'true',FREERANGE_FEATURES_BILLING_ENABLED:'false',FREERANGE_BIZ_BILLING_NEW_COLLECTION_ENABLED:'true',FREERANGE_PAYMENT_PROVIDER:'mock',FREERANGE_OTEL_ENABLED:'false',FREERANGE_SERVER_PORT:String(port),FREERANGE_SERVER_HOST:'127.0.0.1',FREERANGE_BIZ_BILLING_PORTAL_BASE_URL:'https://portal.fixture.invalid/portal',FREERANGE_CONNECTION_ENCRYPTION_KEYS:`fixture:${randomBytes(32).toString('base64')}`});
  await writeFile(join(dir,'config.yaml'),JSON.stringify({features:{biz_billing_enabled:true},database:{urls:[manifest.services.elasticsearch.url]},redis:{host:'127.0.0.1',port:Number(redis.port)},server:{host:'127.0.0.1',port},payment:{provider:'mock'},biz_billing:{portal_base_url:'https://portal.fixture.invalid/portal'}}));
  for(const binary of ['migrate','server','worker'])await run('go',['build','-p','2','-o',join(dir,binary+ext),`./cmd/${binary}`],{env});
  await run(join(dir,'migrate'+ext),['up','--timeout','3m'],{env,cwd:dir,capture:true});
  const adminID='00000000-0000-0000-0000-000000000001';
  const seed=await fetch(`${env.INTEGRATION_ES_URL}/auth_users/_doc/${adminID}?refresh=true`,{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({user_id:adminID,email:'ci-admin@test.local',password_hash:'',full_name:'Fixture admin',is_active:true,created_at:new Date().toISOString(),updated_at:new Date().toISOString()})});
  if(!seed.ok)throw new Error('Could not seed owned fixture admin');
  const b64=x=>Buffer.from(JSON.stringify(x)).toString('base64url');const now=Math.floor(Date.now()/1000);
  const unsigned=`${b64({alg:'HS256',typ:'JWT'})}.${b64({user_id:adminID,email:'ci-admin@test.local',iat:now,nbf:now,exp:now+3600})}`;
  env.INTEGRATION_ADMIN_TOKEN=`${unsigned}.${createHmac('sha256',secret).update(unsigned).digest('base64url')}`;
  for(const binary of ['server','worker']){
    const log=await open(join(dir,`${binary}.log`),'w');
    const child=spawn(join(dir,binary+ext),[],{env,cwd:dir,shell:false,windowsHide:true,stdio:['ignore',log.fd,log.fd]});child.on('error',()=>{});processes.push(child);await log.close();
  }
  await healthy(`${env.INTEGRATION_BASE_URL}/v1/health`);manifest.apiURL=env.INTEGRATION_BASE_URL;
}
async function main(args){
  const {stack,command}=parseRun(args);const runID=randomUUID().replaceAll('-','');const project=`frn-biz-${runID}`;
  const dir=await mkdtemp(join(tmpdir(),'frn-biz-fixture-'));const env={...childEnvironment(),FRN_BIZ_FIXTURE_ID:runID,FRN_BIZ_REQUIRED:'1'};const processes=[];
  const composeArgs=['compose','-p',project,'-f',compose];let manifest;
  try{
    const dockerHost=env.DOCKER_HOST||await run('docker',['context','inspect','--format','{{.Endpoints.docker.Host}}'],{env,capture:true});assertLocalDocker(dockerHost);
    const projects=await run('docker',['ps','--format','{{.Label "com.docker.compose.project"}}'],{env,capture:true});assertNoRunningBillingProject(projects.split(/\s+/).filter(Boolean));
    await run('docker',[...composeArgs,'up','-d','--wait','--wait-timeout','600','elasticsearch','redis'],{env});
    const inspections=[];manifest={runID,project,disposable:true,services:{}};
    for(const [name,port] of [['elasticsearch','9200/tcp'],['redis','6379/tcp']]){
      const id=await run('docker',[...composeArgs,'ps','-q',name],{env,capture:true});
      const [info]=JSON.parse(await run('docker',['inspect',id],{env,capture:true}));inspections.push(info);
      manifest.services[name]={id:info.Id,url:`http://127.0.0.1:${info.NetworkSettings.Ports[port][0].HostPort}`};
    }
    validateManifest(manifest,inspections);env.FRN_BIZ_FIXTURE_MANIFEST=join(dir,'manifest.json');await writeFile(env.FRN_BIZ_FIXTURE_MANIFEST,JSON.stringify(manifest));
    env.FRN_BIZ_TEST_ES_URL=manifest.services.elasticsearch.url;env.INTEGRATION_ES_URL=manifest.services.elasticsearch.url;env.FRN_BIZ_TEST_REDIS_ADDR=new URL(manifest.services.redis.url).host;
    if(stack)await startStack(dir,env,manifest,processes);
    await writeFile(env.FRN_BIZ_FIXTURE_MANIFEST,JSON.stringify(manifest));
    await run(command[0],command.slice(1),{env});
  }catch(error){
    const ids=await run('docker',['ps','-aq','--filter',`label=${ownerLabel}=${runID}`],{env,capture:true}).catch(()=>'');
    for(const id of ids.split(/\s+/).filter(Boolean))await run('docker',['logs','--tail','35',id],{env}).catch(()=>{});
    if(stack){for(const name of ['server','worker']){const log=await readFile(join(dir,`${name}.log`),'utf8').catch(()=>'');if(log)console.error(sanitizeOutput(log.slice(-4000),Object.entries(env).filter(([key])=>/secret|token|encryption/i.test(key)).map(([,value])=>value)));}}
    throw error;
  }finally{
    for(const child of processes){child.kill();await new Promise(ok=>{if(child.exitCode!==null)return ok();child.once('exit',ok);setTimeout(ok,5000);});}
    const docker=args=>run('docker',args,{env,capture:true});
    assertLocalDocker(env.DOCKER_HOST||await docker(['context','inspect','--format','{{.Endpoints.docker.Host}}']));
    await cleanupOwnedFixture({runID,project},docker);
    if(resolve(dir).startsWith(resolve(tmpdir(),'frn-biz-fixture-')))await rm(dir,{recursive:true,force:true});
  }
}
if(process.argv[1]&&resolve(process.argv[1])===fileURLToPath(import.meta.url))main(process.argv.slice(2)).catch(e=>{console.error(e.message);process.exitCode=1;});
