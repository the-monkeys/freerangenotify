import {test,expect} from '@playwright/test';

test.describe('WhatsApp SMS connections domestic SMS',()=>{
 test('Exotel saves DLT and budget fields without echoing credentials',async({page})=>{
  let saved:any;
  await page.addInitScript(()=>localStorage.setItem('access_token','fixture-jwt'));
  await page.route('**/v1/**',async route=>{
   const path=new URL(route.request().url()).pathname;let data:any={data:[]};
   if(path==='/v1/admin/me')data={user_id:'owner',phone_verified:true};
   else if(path==='/v1/apps/channel')data={data:{app_id:'channel',app_name:'Fixture',api_key:'dashboard:channel',settings:{}}};
   else if(path.includes('/channel-connections/')){
    if(route.request().method()==='PUT')saved=route.request().postDataJSON();
    data={data:{id:'fixture',provider:saved?.provider||'twilio',credential_source:'byoc',revision:saved?2:1,readiness:'pending',writes_enabled:true,exotel_enabled:false,secret_configured:saved?{api_key:true,api_token:true}:{},exotel:saved?.exotel}};
   }
   await route.fulfill({json:data});
  });
  await page.goto('/apps/channel?tab=whatsapp');const panel=page.getByTestId('connection-sms');
  await panel.getByLabel('Provider',{exact:true}).selectOption('exotel');
  await panel.getByLabel('Account SID',{exact:true}).fill('fixture-account');
  await panel.getByLabel('API key',{exact:true}).fill('fixture-key');
  await panel.getByLabel('API token',{exact:true}).fill('fixture-token');
  await panel.getByLabel('DLT sender header',{exact:true}).fill('FRNAPP');
  await panel.getByLabel('DLT entity ID',{exact:true}).fill('1234567890123456789');
  await panel.getByRole('button',{name:'Add DLT template',exact:true}).click();
  await panel.getByLabel('DLT template ID 1',{exact:true}).fill('1234567890123456780');
  await panel.getByLabel('Approved template body 1',{exact:true}).fill('Hello {#var#} from FRN');
  await panel.getByRole('button',{name:'Save connection',exact:true}).click();
  await expect.poll(()=>saved).toBeDefined();expect(saved.secrets).toEqual({api_key:'fixture-key',api_token:'fixture-token'});
  expect(saved.exotel.entity_id).toBe('1234567890123456789');expect(saved.exotel.templates[0].body).toBe('Hello {#var#} from FRN');
  expect(saved.exotel.limits.daily_spend_minor).toBe(0);
  await expect(panel.getByLabel('API token',{exact:true})).toHaveValue('');
  await expect(panel.getByText('Domestic SMS is disabled for this deployment.',{exact:true})).toBeVisible();
  await expect(panel.getByText(/separate from the application's HTTP request limit/)).toBeVisible();
 });
});
