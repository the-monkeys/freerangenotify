import {test,expect,spec} from './fixtures';
for(const failure of ['http','html','invalid','schema'])test(`reference fallback ${failure}`,async({page})=>{
 await page.route('**/v1/swagger/doc.json',route=>failure==='http'?route.fulfill({status:503}):route.fulfill({contentType:failure==='html'?'text/html':'application/json',body:failure==='html'?'<html>wrong</html>':failure==='invalid'?'{':JSON.stringify({paths:{}})}));
 await page.route('**/swagger.json',route=>route.fulfill({json:spec}));await page.goto('/docs/api');await expect(page.getByRole('heading',{name:'Fixture API'})).toBeVisible();await page.getByRole('button',{name:/POST.*v1\/notifications/i}).click();await expect(page.getByText('Required fields: channel',{exact:false})).toHaveCount(2);await expect(page.getByText('Request example',{exact:true})).toBeVisible();await expect(page.getByText('Response 202',{exact:true})).toBeVisible();
});
test('configured API base and recursive schemas',async({page})=>{let fetched='';await page.route('**/v1/swagger/doc.json',route=>{fetched=route.request().url();return route.fulfill({json:spec});});await page.goto('/docs/api');await expect(page.getByRole('heading',{name:'Fixture API'})).toBeVisible();expect(fetched).toContain(process.env.FRN_DOCS_API_BASE||'/v1');await page.getByRole('button',{name:/POST.*v1\/notifications/i}).click();expect((await page.locator('main').last().innerText()).length).toBeLessThan(30000);});
test('failed remote and fallback report error',async({page})=>{await page.route('**/swagger*',route=>route.fulfill({status:500}));await page.route('**/v1/swagger/doc.json',route=>route.fulfill({json:{swagger:'3.0',paths:{}}}));await page.goto('/docs/api');await expect(page.getByRole('heading',{name:'Failed to load API Reference'})).toBeVisible();});

for(const kind of ['response','schema','parameters'])test(`malformed nested ${kind} selects fallback`,async({page})=>{
 const bad:any=structuredClone(spec);bad.info.title='Malformed remote';if(kind==='response')bad.paths['/v1/notifications'].post.responses['202']=null;else if(kind==='schema')bad.definitions.Send.properties.channel=null;else bad.paths['/v1/notifications'].post.parameters='bad';
 await page.route('**/v1/swagger/doc.json',route=>route.fulfill({json:bad}));await page.route('**/swagger.json',route=>route.fulfill({json:spec}));await page.goto('/docs/api');await expect(page.getByRole('heading',{name:'Fixture API',exact:true})).toBeVisible();await page.getByRole('button',{name:/POST.*v1\/notifications/i}).click();await expect(page.getByText('Request example',{exact:true})).toBeVisible();
});

test('the generated bundled fallback renders connection request fields',async({page})=>{
 await page.route('**/v1/swagger/doc.json',route=>route.fulfill({status:503}));await page.goto('/docs/api');await expect(page.getByRole('heading',{name:'FreeRangeNotify API',exact:true})).toBeVisible();await page.getByPlaceholder('Search endpoints by path, method, or tag...').fill('channel-connections');await page.getByRole('button',{name:/PUT.*channel-connections/i}).click();await expect(page.getByText('Required fields: revision',{exact:true})).toBeVisible();
});
