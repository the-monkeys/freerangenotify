import {test,expect} from '@playwright/test';

test.describe('WhatsApp SMS connections replies',()=>{
 test('queued reply is not reported as sent and later failure is visible',async({page})=>{
  let reply:any;let failed=false;
  await page.addInitScript(()=>localStorage.setItem('access_token','fixture-jwt'));
  await page.route('**/v1/**',async route=>{
   const path=new URL(route.request().url()).pathname;let data:any={data:[]};
   if(path==='/v1/admin/me')data={user_id:'owner',phone_verified:true};
   else if(path==='/v1/apps/channel')data={data:{app_id:'channel',app_name:'Channel',api_key:'dashboard:channel',settings:{}}};
   else if(path.includes('/channel-connections/'))data={data:{provider:'meta',readiness:'ready',revision:1,writes_enabled:true,secret_configured:{}}};
   else if(path.endsWith('/reply')){reply=route.request().postDataJSON();data={success:true,accepted:true,message:'Reply queued',notification_id:'reply-fixture',status:'queued'};}
   else if(path.includes('/notifications/reply-fixture'))data={notification_id:'reply-fixture',status:failed?'failed':'queued',error_message:failed?'insufficient credits':''};
   else if(path.endsWith('/messages'))data={data:[],total:0};
   else if(path.includes('/whatsapp/conversations'))data={data:[{contact_wa_id:'919876543210',contact_name:'Fixture contact',last_message_text:'hello',last_message_at:'2026-10-14T01:00:00Z',unread_count:1,csw_open:true}],total:1};
   await route.fulfill({json:data});
  });
  await page.goto('/apps/channel?tab=whatsapp');await page.getByText('Fixture contact',{exact:true}).click();
  const input=page.getByPlaceholder('Type a reply...');await input.fill('hello');await input.press('Enter');
  await expect(page.getByText('Reply queued',{exact:true})).toBeVisible();expect(reply.idempotency_key).toBeTruthy();
  await expect(page.getByText('Reply sent',{exact:true})).toHaveCount(0);
  failed=true;await expect(page.getByText('Reply failed: insufficient credits',{exact:true})).toBeVisible({timeout:15000});
 });
});
