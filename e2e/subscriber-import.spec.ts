import { test, expect, type Page } from '@playwright/test';

async function importPage(page: Page, submit: (body: {users: Record<string,string>[]}) => unknown) {
 page.on('pageerror',error=>console.error(error.message));
 await page.addInitScript(() => localStorage.setItem('access_token', 'ui-fixture'));
 await page.route('**/v1/**', async route => {
  const path = new URL(route.request().url()).pathname.replace(/^\/v1/, '').replace(/\/$/, '');
  let data: unknown = {data: [],success: true};
  if (path === '/admin/me') data = {user_id:'admin',full_name:'Import test',email:'ui@example.test',phone_verified:true};
  else if (path === '/apps/import') data = {data:{app_id:'import',app_name:'Import test',api_key:'fixture-key',settings:{}}};
  else if (path === '/users') data = {data:{users:[],total_count:0,page:1,page_size:20}};
  else if (path === '/billing/usage') data = {plan:'pro',status:'active',billing_model:'credits',credits_total:1500,credits_remaining:1500,credits_available:1500,credits_reserved:0,usage_percent:0,messages_sent:0,days_remaining:20,current_period_start:'2026-10-01T00:00:00Z',current_period_end:'2026-11-01T00:00:00Z'};
  else if (path === '/billing/rates') data = {currency:'INR',active_version:'fixture',channel_credit_cost:{email:5},free_tier_daily_caps:{},overage_per_message:{}};
  else if (path === '/users/bulk') data = submit(route.request().postDataJSON());
  await route.fulfill({status:path==='/users/bulk'?201:200,json:data});
 });
 await page.goto('/apps/import?tab=users');
 await expect(page.getByRole('button',{name:'Import CSV',exact:true})).toBeVisible({timeout:30000});
}

test.describe('Delivery reliability UI subscriber imports',()=>{
 test('a transport failure stops subsequent chunks without retrying unknown writes',async({page})=>{
  let calls=0;
  await importPage(page,body=>({created:body.users.length,updated:0,skipped:0,total:body.users.length}));
  await page.route('**/users/bulk',async route=>{
   calls++;
   const body=route.request().postDataJSON();
   await route.fulfill({status:calls===2?503:201,json:calls===2?{message:'storage unavailable'}:{created:body.users.length,updated:0,skipped:0,total:body.users.length}});
  });
  const csv='email\n'+Array.from({length:1001},(_,i)=>`person${i}@example.com`).join('\n');
  await page.locator('input[type=file]').setInputFiles({name:'subscribers.csv',mimeType:'text/csv',buffer:Buffer.from(csv)});
  const result=page.getByRole('status',{name:'CSV import progress'});
  await expect(result).toContainText('Import stopped after 500 / 1001');
  await expect(result).toContainText('current batch may have been written');expect(calls).toBe(2);
 });
 test('one 6001-row file is automatically chunked and quoted fields survive',async({page})=>{
  const batches: Record<string,string>[][]=[];
  await importPage(page,body=>{batches.push(body.users);return {created:body.users.length,updated:0,skipped:0,total:body.users.length};});
  const csv='email,full_name,phone,external_id\n'+Array.from({length:6001},(_,i)=>`person${i}@example.com,"Person, ${i}",,ext_${i}`).join('\n');
  await page.locator('input[type=file]').setInputFiles({name:'subscribers.csv',mimeType:'text/csv',buffer:Buffer.from(csv)});
  await expect(page.getByRole('status',{name:'CSV import progress'})).toContainText('6001 created',{timeout:20000});
  expect(batches.length).toBe(13);expect(batches.every(b=>b.length<=500)).toBe(true);
  expect(batches.flat()).toHaveLength(6001);expect(batches[0][0].full_name).toBe('Person, 0');
 });
 test('invalid rows block writes and item failures preserve original row numbers',async({page})=>{
  let calls=0;
  await importPage(page,body=>{calls++;return {created:body.users.length-1,updated:0,skipped:0,total:body.users.length,errors:[{index:1,email:body.users[1].email,message:'write rejected'}]};});
  const upload=async(csv:string)=>page.locator('input[type=file]').setInputFiles({name:'subscribers.csv',mimeType:'text/csv',buffer:Buffer.from(csv)});
  await upload('email,full_name\na@example.com,A\n,B');
  await expect(page.getByRole('status',{name:'CSV import progress'})).toContainText('Row 3');expect(calls).toBe(0);
  await upload('email,full_name\na@example.com,"A, B"\nb@example.com,B');
  await expect(page.getByRole('status',{name:'CSV import progress'})).toContainText('Row 3: write rejected');expect(calls).toBe(1);
 });
});

