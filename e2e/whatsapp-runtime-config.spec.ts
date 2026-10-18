import {test,expect} from '@playwright/test';

test.describe('WhatsApp SMS connections runtime',()=>{
 for(const enabled of [true,false])test(`runtime signup ${enabled?'enabled':'disabled'} after frontend build`,async({page})=>{
  await page.addInitScript(()=>{
   localStorage.setItem('access_token','fixture-jwt');
   (window as any).FB={init:(p:any)=>(window as any).sdkParameters=p,login:()=>{}};
  });
  await page.route('**/v1/**',async route=>{
   const path=new URL(route.request().url()).pathname;let data:any={data:[]};
   if(path==='/v1/admin/me')data={user_id:'owner',phone_verified:true};
   else if(path==='/v1/apps/channel')data={data:{app_id:'channel',app_name:'Channel',api_key:'dashboard:channel',settings:{}}};
   else if(path==='/v1/channel-capabilities')data={data:{meta:{manual:enabled,embedded:enabled,app_id:'runtime-public-app',signup_config_id:'runtime-signup',graph_version:'v24.0'}}};
   else if(path.includes('/channel-connections/'))data={data:{provider:'meta',revision:1,readiness:'ready',writes_enabled:true,meta_enabled:enabled,secret_configured:{}}};
   else if(path==='/v1/admin/whatsapp/channel/status')data={data:{connected:false,provider:'meta'}};
   await route.fulfill({json:data});
  });
  await page.goto('/apps/channel?tab=whatsapp');
  if(enabled){await expect(page.getByRole('button',{name:'Connect with Meta',exact:true})).toBeEnabled();expect(await page.evaluate(()=>(window as any).sdkParameters)).toMatchObject({appId:'runtime-public-app',version:'v24.0'});}
  else{await expect(page.getByRole('button',{name:'Connect with Meta',exact:true})).toHaveCount(0);expect(await page.evaluate(()=>(window as any).sdkParameters)).toBeUndefined();const connection=page.getByTestId('connection-whatsapp');await expect(connection.getByRole('button',{name:'Save connection',exact:true})).toBeDisabled();await expect(connection.getByRole('button',{name:'Validate readiness',exact:true})).toBeDisabled();await expect(connection.getByRole('button',{name:'Disconnect',exact:true})).toBeEnabled();}
 });
});
