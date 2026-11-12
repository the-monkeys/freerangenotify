import { test, expect } from './fixtures';
import { financialFixture } from './financial-fixture';

test('invoice paid figures use saved money for unpaid, partial, credited and refunded cases', async ({ page }) => {
  await financialFixture(page);
  await page.goto('/apps/billing-a?tab=biz-billing&billing_view=invoices');
  for (const [id, paid] of [['unpaid', 'USD 0.00'], ['partial', 'USD 5.00'], ['paid', 'USD 11.80'], ['credited', 'USD 0.00'], ['refunded', 'USD 11.80']]) {
    await expect(page.getByTestId(`invoice-${id}`).getByTestId('amount-paid')).toHaveText(paid);
  }
  await expect(page.getByTestId('invoice-automatic').getByRole('button', { name: 'Record payment', exact: true })).toBeDisabled();
});

for (const width of [360, 390, 768, 1280]) test(`search customer, preview, issue and record received cash at ${width}px`, async ({ page }) => {
  await page.setViewportSize({ width, height: 900 });
  const fixture = await financialFixture(page);
  await page.goto('/apps/billing-a?tab=biz-billing&billing_view=invoices');
  await page.getByRole('button', { name: 'New invoice', exact: true }).click();
  await page.getByRole('combobox', { name: 'Customer', exact: true }).click();
  await page.getByPlaceholder('Search by name or email').fill('customer1199');
  await page.getByRole('option', { name: /customer1199@example.invalid/ }).click();
  await page.getByLabel('Item 1 description', { exact: true }).fill('Consultation');
  await page.getByLabel('Item 1 unit price', { exact: true }).fill('10.00');
  await page.getByRole('button', { name: 'Preview invoice', exact: true }).click();
  await expect(page.getByTestId('invoice-preview')).toContainText('INR 11.80');
  await page.getByRole('button', { name: 'Save draft', exact: true }).click();
  await expect.poll(() => fixture.invoices[0].user_id).toBe('customer-1199');
  const draft = page.getByTestId(`invoice-${fixture.invoices[0].id}`);
  await draft.getByRole('button', { name: 'Issue invoice', exact: true }).click();
  await draft.getByRole('button', { name: 'Record payment', exact: true }).click();
  await page.getByLabel('Payment method', { exact: true }).selectOption('cash');
  await page.getByLabel('Payment reference', { exact: true }).fill('RECEIPT-001');
  await page.getByLabel('I confirm this payment was received', { exact: true }).check();
  await page.getByRole('button', { name: 'Save payment', exact: true }).click();
  await expect(draft.getByTestId('amount-paid')).toHaveText('INR 11.80');
  expect(fixture.searchRequests.every(request => request.pageSize <= 20)).toBe(true);
  expect(fixture.searchRequests.some(request => request.search === 'customer1199')).toBe(true);
  expect(fixture.payments.at(-1)?.method).toBe('cash');
  expect(fixture.requests.filter(request => /\/payments$/.test(request.path)).every(request => !!request.key)).toBe(true);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
});

test('lost payment response keeps the same request key and never records another payment', async ({ page }) => {
  const fixture = await financialFixture(page, { lostPaymentReply: true });
  await page.goto('/apps/billing-a?tab=biz-billing&billing_view=invoices');
  await page.getByTestId('invoice-unpaid').getByRole('button', { name: 'Record payment', exact: true }).click();
  await page.getByLabel('I confirm this payment was received', { exact: true }).check();
  await page.getByRole('button', { name: 'Save payment', exact: true }).click();
  await expect(page.getByRole('status').filter({ hasText: 'Payment pending' })).toBeVisible();
  await page.getByRole('button', { name: 'Retry saved request', exact: true }).click();
  await expect(page.getByTestId('invoice-unpaid').getByTestId('amount-paid')).toHaveText('USD 11.80');
  const writes = fixture.requests.filter(request => request.path === '/biz/invoices/unpaid/payments');
  expect(writes).toHaveLength(2);
  expect(new Set(writes.map(write => write.key)).size).toBe(1);
  expect(fixture.payments).toHaveLength(2);
});

for(const mode of ['invoice','automatic'])test(`create ${mode} subscription using a searchable customer`,async({page})=>{
  const fixture=await financialFixture(page);
  await page.goto('/apps/billing-a?tab=biz-billing&billing_view=subscriptions');
  await page.getByRole('button',{name:'New subscription',exact:true}).click();
  await page.getByRole('combobox',{name:'Customer',exact:true}).click();
  await page.getByPlaceholder('Search by name or email').fill('customer1199');
  await page.getByRole('option',{name:/customer1199@example.invalid/}).click();
  await page.getByLabel('Subscription plan',{exact:true}).selectOption('inr-fixed');
  await page.getByLabel('Collection mode',{exact:true}).selectOption(mode);
  await page.getByRole('button',{name:'Create subscription',exact:true}).click();
  await expect.poll(()=>fixture.subscriptions.length).toBe(1);
  expect(fixture.subscriptions[0].user_id).toBe('customer-1199');
  expect(fixture.subscriptions[0].collection_mode).toBe(mode);
  await expect(page.getByText(mode==='automatic'?'Awaiting customer authorization. No payment has been confirmed.':'Collection: Invoice',{exact:true})).toBeVisible();
});

test('unsupported automatic plan is explained before creating or authorizing',async({page})=>{
  const fixture=await financialFixture(page);
  await page.goto('/apps/billing-a?tab=biz-billing&billing_view=subscriptions');
  await page.getByRole('button',{name:'New subscription',exact:true}).click();
  await page.getByLabel('Subscription plan',{exact:true}).selectOption('usd-history');
  await page.getByLabel('Collection mode',{exact:true}).selectOption('automatic');
  await expect(page.getByRole('alert')).toContainText('fixed INR recurring plans');
  await expect(page.getByRole('button',{name:'Create subscription',exact:true})).toBeDisabled();
  expect(fixture.subscriptions).toHaveLength(0);
});

test('legacy credit needs review and cannot be applied',async({page})=>{
  await financialFixture(page,{unresolvedCredit:true});
  await page.goto('/apps/billing-a?tab=biz-billing&billing_view=invoices');
  await page.getByTestId('invoice-unpaid').getByRole('button',{name:'View invoice',exact:true}).click();
  await expect(page.getByRole('alert')).toContainText('Existing credits need review');
  await expect(page.getByRole('button',{name:'Apply available credit',exact:true})).toBeDisabled();
});

test('spent overpayment refund is blocked with the form preserved',async({page})=>{
  const fixture=await financialFixture(page,{spentCredit:true});
  await page.goto('/apps/billing-a?tab=biz-billing&billing_view=payments');
  await page.getByRole('button',{name:'Refund payment',exact:true}).click();
  await page.getByLabel('Refund reason',{exact:true}).fill('Customer request');
  await page.getByLabel('I confirm this refund',{exact:true}).check();
  await page.getByRole('button',{name:'Confirm refund',exact:true}).click();
  await expect(page.getByRole('dialog').getByRole('alert')).toContainText('overpayment credit');
  await expect(page.getByLabel('Refund reason',{exact:true})).toHaveValue('Customer request');
  expect(fixture.payments[0].refunded_paisa).toBe(0);
});

test('refund records separately and delivery retry never resends an unknown outcome',async({page})=>{
  const fixture=await financialFixture(page);
  await page.goto('/apps/billing-a?tab=biz-billing&billing_view=payments');
  await page.getByRole('button',{name:'Refund payment',exact:true}).click();
  await page.getByLabel('Refund reason',{exact:true}).fill('Cash returned');
  await page.getByLabel('I confirm this refund',{exact:true}).check();
  await page.getByRole('button',{name:'Confirm refund',exact:true}).click();
  await expect.poll(()=>fixture.payments[0].refunded_paisa).toBe(1180);
  expect(fixture.invoices.find(row=>row.id==='paid')?.amount_paid_paisa).toBe(1180);
  await page.getByRole('button',{name:'Messages / receipts',exact:true}).click();
  await expect(page.getByText('Delivery needs review',{exact:true})).toBeVisible();
  await expect(page.getByRole('button',{name:'Retry message',exact:true}).nth(1)).toBeDisabled();
  await page.getByRole('button',{name:'Retry message',exact:true}).first().click();
  await expect.poll(()=>fixture.deliveries[0].status).toBe('pending');
});

test('pending operation resumes its saved action without resubmitting payment',async({page})=>{
  const fixture=await financialFixture(page);let resumed=0;
  await page.route('**/v1/biz/invoices/unpaid/payments',route=>route.fulfill({status:503,json:{code:'BILLING_RECONCILIATION_PENDING',operation_id:'saved-payment',operation_state:'accepted',retryable:true}}));
  await page.route('**/v1/biz/operations/saved-payment/resume',async route=>{resumed++;fixture.invoices[0].amount_paid_paisa=1180;fixture.invoices[0].amount_due_paisa=0;return route.fulfill({json:{success:true,data:{operation_id:'saved-payment',state:'completed',resources:[{kind:'payment',id:'restored'}]}}});});
  await page.goto('/apps/billing-a?tab=biz-billing&billing_view=invoices');
  await page.getByTestId('invoice-unpaid').getByRole('button',{name:'Record payment',exact:true}).click();
  await page.getByLabel('I confirm this payment was received',{exact:true}).check();
  await page.getByRole('button',{name:'Save payment',exact:true}).click();
  await expect(page.getByRole('dialog').getByRole('status')).toContainText('Payment pending');
  await page.getByRole('button',{name:'Check saved action',exact:true}).click();
  await expect(page.getByTestId('invoice-unpaid').getByTestId('amount-paid')).toHaveText('USD 11.80');
  expect(resumed).toBe(1);
});

for(const width of [360,390,768,1280])test(`financial screens are read-only and contained at ${width}px`,async({page})=>{
  await page.setViewportSize({width,height:900});await financialFixture(page,{role:'viewer'});
  await page.goto('/apps/billing-a?tab=biz-billing&billing_view=invoices');
  await expect(page.getByRole('button',{name:'New invoice',exact:true})).toBeDisabled();
  await expect(page.getByTestId('invoice-unpaid').getByRole('button',{name:'Record payment',exact:true})).toBeDisabled();
  await page.getByRole('button',{name:'Overview',exact:true}).click();
  await expect(page.getByText('Reporting currency: INR',{exact:true})).toBeVisible();
  await page.getByRole('button',{name:'Advanced',exact:true}).click();
  await expect(page.getByRole('button',{name:'Sales',exact:true})).toBeVisible();
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
});

test('invoice errors are visible and recoverable, including empty results',async({page})=>{
  await financialFixture(page);let fail=true;
  await page.route('**/v1/biz/invoices?**',route=>fail?route.fulfill({status:503,json:{error:'Temporary outage'}}):route.fulfill({json:{success:true,data:[],total:0}}));
  await page.goto('/apps/billing-a?tab=biz-billing&billing_view=invoices');
  await expect(page.getByRole('alert')).toContainText('Could not load invoices');fail=false;
  await page.getByRole('button',{name:'Retry invoices',exact:true}).click();
  await expect(page.getByText('No invoices yet.',{exact:true})).toBeVisible();
});

test('slow invoice data cannot cross applications',async({page})=>{
  await financialFixture(page);let started=false;let release=()=>{};const held=new Promise<void>(resolve=>{release=resolve;});
  await page.route('**/v1/biz/invoices?**',async route=>{
    const old=route.request().headers()['x-api-key']?.includes('billing-a');if(old){started=true;await held;}
    try{await route.fulfill({json:{success:true,total:1,data:[{id:old?'only-a':'only-b',invoice_number:old?'Only A invoice':'Only B invoice',currency:'USD',total_paisa:100,amount_paid_paisa:0,amount_due_paisa:100,status:'open'}]}});}catch(error){if(!route.request().failure())throw error;}
  });
  await page.goto('/apps/billing-a?tab=biz-billing&billing_view=invoices');await expect.poll(()=>started).toBe(true);
  await page.evaluate(()=>{history.pushState({},'','/apps/billing-b?tab=biz-billing&billing_view=invoices');dispatchEvent(new PopStateEvent('popstate'));});
  await expect(page.getByText('Only B invoice',{exact:true})).toBeVisible();release();
  await page.waitForTimeout(250);await expect(page.getByText('Only A invoice',{exact:true})).toHaveCount(0);
});

test('dark financial screen and customer picker support keyboard at mobile width',async({page},testInfo)=>{
  await page.setViewportSize({width:360,height:900});await financialFixture(page);
  await page.goto('/apps/billing-a?tab=biz-billing&billing_view=invoices');
  await page.evaluate(()=>document.documentElement.classList.add('dark'));
  await page.getByRole('button',{name:'New invoice',exact:true}).click();
  const customer=page.getByRole('combobox',{name:'Customer',exact:true});await customer.focus();await page.keyboard.press('Enter');
  await page.getByPlaceholder('Search by name or email').fill('customer1199');
  const option=page.getByRole('option',{name:/customer1199@example.invalid/});await option.focus();await page.keyboard.press('Enter');
  await expect(customer).toContainText('customer1199@example.invalid');
  await page.screenshot({path:testInfo.outputPath('invoice-dark-mobile.png'),fullPage:true});
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
});

test('retryable conflict retains the same payment key',async({page})=>{
 await financialFixture(page);let first=true;const keys:string[]=[];
 await page.route('**/v1/biz/invoices/unpaid/payments',route=>{keys.push(route.request().headers()['idempotency-key']);if(first){first=false;return route.fulfill({status:409,json:{code:'BILLING_CONFLICT',retryable:true}});}return route.fallback();});
 await page.goto('/apps/billing-a?tab=biz-billing&billing_view=invoices');await page.getByTestId('invoice-unpaid').getByRole('button',{name:'Record payment',exact:true}).click();await page.getByLabel('I confirm this payment was received',{exact:true}).check();await page.getByRole('button',{name:'Save payment',exact:true}).click();
 await page.getByRole('button',{name:'Retry saved request',exact:true}).click();await expect(page.getByTestId('invoice-unpaid').getByTestId('amount-paid')).toHaveText('USD 11.80');expect(new Set(keys).size).toBe(1);
});
