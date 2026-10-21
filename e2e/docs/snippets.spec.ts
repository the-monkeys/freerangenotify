import {test,expect} from './fixtures';
test('published snippets use backend placeholders and named SSE events',async({page})=>{await page.goto('/docs/sse');await expect(page.locator('article')).toContainText("addEventListener('notification'");await expect(page.locator('article')).toContainText('sse_token');await page.goto('/docs/authentication');await expect(page.locator('article')).toContainText('API_KEY');});

test('owner can copy a placeholder SSE integration snippet',async({page,context})=>{
 await context.grantPermissions(['clipboard-read','clipboard-write']);
 await page.addInitScript(()=>localStorage.setItem('access_token','fixture-owner'));
 const code="const stream = new EventSource('/v1/sse?sse_token=YOUR_SSE_TOKEN');\nstream.addEventListener('notification', event => console.log(event.data));";
 await page.route('**/v1/**',async route=>{const path=new URL(route.request().url()).pathname;let data:unknown={data:[],success:true};
 if(path==='/v1/admin/me')data={user_id:'owner',full_name:'Owner',phone_verified:true};
 else if(path==='/v1/apps/docs')data={data:{app_id:'docs',app_name:'Docs fixture',admin_user_id:'owner',api_key:'dashboard:docs',settings:{}}};
 else if(path==='/v1/apps/docs/code-samples')data={data:{javascript:{sse:{sender:{title:'Send',code:'Authorization: Bearer YOUR_API_KEY'},receiver:{title:'Listen',code}}}}};
 else if(path==='/v1/billing/usage')data={plan:'free',credits_total:1500,credits_remaining:1500,credits_available:1500,credits_reserved:0,days_remaining:10};
 await route.fulfill({json:data});});
 await page.goto('/apps/docs?tab=overview');await page.getByRole('tab',{name:'JavaScript',exact:true}).click();await page.getByRole('tab',{name:'Receiver (SSE)',exact:true}).click();await page.getByRole('button',{name:'Copy',exact:true}).click();expect((await page.evaluate(()=>navigator.clipboard.readText())).replace(/\r\n/g,'\n')).toBe(code);await expect(page.getByText('Show Token',{exact:true})).toHaveCount(0);
});
