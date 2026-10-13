import {test,expect} from '@playwright/test';

test.describe('Delivery reliability UI channel settings',()=>{
 for (const owner of [false,true]) test(`key reveal remains explicit and ${owner?'owner-only in memory':'denied for viewer'}`,async({page})=>{
  let reveals=0;
  await page.addInitScript(()=>localStorage.setItem('access_token','fixture-jwt'));
  await page.route('**/v1/**',async route=>{
   const path=new URL(route.request().url()).pathname.replace(/^\/v1/,'').replace(/\/$/,'');
   let data:unknown={success:true,data:[]};let status=200;
   if(path==='/admin/me')data={user_id:owner?'owner':'viewer',full_name:'Channel test',email:'test@example.test',phone_verified:true};
   else if(path==='/apps/channel')data={data:{app_id:'channel',app_name:'Channel test',api_key:'dashboard:channel',settings:{}}};
   else if(path==='/apps/channel/reveal-key'){reveals++;status=owner?200:403;data=owner?{success:true,data:{api_key:'fixture-owner-secret'}}:{error:{message:'Owner required'}};}
   await route.fulfill({status,json:data});
  });
  await page.goto('/apps/channel?tab=integration');
  expect(reveals).toBe(0);
  await page.getByRole('button',{name:'Show',exact:true}).click();
  await expect.poll(()=>reveals).toBe(1);
  if(owner){await expect(page.locator('input[value="fixture-owner-secret"]')).toHaveCount(1);await page.getByRole('button',{name:'Hide',exact:true}).click();await expect(page.locator('input[value="fixture-owner-secret"]')).toHaveCount(0);}
  else {await expect(page.getByText('Only the application owner can reveal the API key',{exact:true})).toBeVisible();expect(await page.locator('input[value="fixture-owner-secret"]').count()).toBe(0);}
  expect(await page.evaluate(()=>JSON.stringify(localStorage))).not.toContain('fixture-owner-secret');
 });
 test('unrelated setting saves omit credential blocks and bind the settings revision',async({page})=>{
  let saved:Record<string,unknown>|undefined;
  await page.addInitScript(()=>localStorage.setItem('access_token','fixture-jwt'));
  await page.route('**/v1/**',async route=>{
   const path=new URL(route.request().url()).pathname.replace(/^\/v1/,'').replace(/\/$/,'');
   let data:unknown={success:true,data:[]};
   if(path==='/admin/me')data={user_id:'owner',full_name:'Channel test',email:'owner@example.test',phone_verified:true};
   else if(path==='/apps/channel'&&route.request().method()==='GET')data={data:{app_id:'channel',app_name:'Channel test',api_key:'dashboard:channel',updated_at:'2026-10-14T00:00:00Z',settings:{rate_limit:1000,sms_config:{account_sid:'ACfixture',auth_token:'',from_number:'+123'},whatsapp_config:{provider:'meta',meta_phone_number_id:'123'},secret_configured:{'sms_config.auth_token':true}}}};
   else if(path==='/apps/channel/settings'&&route.request().method()==='PUT'){saved=route.request().postDataJSON();data={success:true};}
   await route.fulfill({status:200,json:data});
  });
  await page.goto('/apps/channel?tab=settings');
  const field=page.getByLabel('Rate Limit (requests/hour)');
  if(!(await field.isVisible()))await page.getByRole('button',{name:'Core Settings',exact:true}).click();
  await field.fill('900');await page.getByRole('button',{name:'Save Configuration',exact:true}).click();
  await expect.poll(()=>saved).toBeDefined();
  expect(saved).toEqual({rate_limit:900,settings_revision:'2026-10-14T00:00:00Z'});
  expect(await page.evaluate(()=>localStorage.getItem('last_api_key'))).toBe('dashboard:channel');
 });
});

