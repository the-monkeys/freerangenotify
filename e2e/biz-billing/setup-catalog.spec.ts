import { test, expect, billingFixture } from './fixtures';

for (const width of [360, 390, 768, 1280]) test(`guided setup and typed settings at ${width}px`, async ({ page }) => {
  await page.setViewportSize({ width, height: 900 });
  const fixture = await billingFixture(page);
  await page.goto('/apps/billing-a?tab=biz-billing');
  await expect(page.getByRole('heading', { name: 'Set up customer billing' })).toBeVisible();
  await expect(page.getByText('Manual payments available', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Business settings', exact: true }).click();
  await expect(page.getByLabel('Default currency')).toHaveValue('INR');
  await page.getByLabel('Default currency').selectOption('USD');
  await page.getByRole('button', { name: 'Save default currency', exact: true }).click();
  await expect.poll(() => fixture.config.preferences.default_currency).toBe('USD');
  await page.getByLabel('Default tax rate (%)').fill('0');
  await page.getByRole('button', { name: 'Save tax settings', exact: true }).click();
  await expect.poll(() => fixture.config.tax.default_tax_rate).toBe(0);
  await page.getByLabel('Invoice prefix').fill('CLINIC');
  await page.getByRole('button', { name: 'Save numbering', exact: true }).click();
  await expect.poll(() => fixture.config.numbering.invoice_prefix).toBe('CLINIC');
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
});

test('payment account errors stay visible and retry preserves the form', async ({ page }) => {
  const fixture = await billingFixture(page, { gatewayError: true });
  await page.goto('/apps/billing-a?tab=biz-billing');
  await expect(page.getByRole('alert').filter({ hasText: 'Could not load payment account' })).toBeVisible();
  fixture.recoverGateway();
  await page.getByRole('button', { name: 'Retry payment account', exact: true }).click();
  await expect(page.getByRole('alert').filter({ hasText: 'Could not load payment account' })).toHaveCount(0);
});

test('failed lifecycle readiness does not claim manual billing is available', async ({ page }) => {
  await billingFixture(page);
  await page.route('**/v1/biz/lifecycle', route => route.fulfill({ status: 503, json: { error: 'Billing readiness unavailable' } }));
  await page.goto('/apps/billing-a?tab=biz-billing');
  await expect(page.getByRole('alert').filter({ hasText: 'Could not load billing readiness' })).toBeVisible();
  await expect(page.getByText('Manual payments available', { exact: true })).toHaveCount(0);
});

test('gateway credentials are write-only and viewer has no editing controls', async ({ page }) => {
  const fixture = await billingFixture(page);
  await page.goto('/apps/billing-a?tab=biz-billing');
  await page.getByRole('button', { name: 'Business settings', exact: true }).click();
  await page.getByLabel('Razorpay account ID').fill('acc_fixture');
  await page.getByLabel('Razorpay key ID').fill('fixture-key');
  await page.getByLabel('Razorpay key secret').fill('ONLY_IN_FORM_FIXTURE');
  await page.getByRole('button', { name: 'Save payment account', exact: true }).click();
  await expect(page.getByLabel('Razorpay key secret')).toHaveValue('');
  expect(await page.evaluate(() => JSON.stringify(localStorage))).not.toContain('ONLY_IN_FORM_FIXTURE');
  expect(fixture.writes.find(x => x.path.endsWith('/test'))?.body.expected_revision).toBe(0);
  await page.unroute('**/v1/**');
  await billingFixture(page, { role: 'viewer' });
  await page.reload();
  await expect(page.getByText('Read-only billing access', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Business settings', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Save payment account', exact: true })).toBeDisabled();
});

for (const width of [360, 390, 768, 1280]) test(`catalog pagination and new INR plans preserve historical USD money at ${width}px`, async ({ page }) => {
  await page.setViewportSize({ width, height: 900 });
  const fixture = await billingFixture(page);
  await page.goto('/apps/billing-a?tab=biz-billing');
  await page.getByRole('button', { name: 'Catalog', exact: true }).click();
  await expect(page.getByText('Product 1', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Next products', exact: true }).click();
  await expect(page.getByText('Product 21', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Plans', exact: true }).click();
  await expect(page.getByText('USD 10.00', { exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'New plan', exact: true }).click();
  await expect(page.getByLabel('Plan currency')).toHaveValue('INR');
  await page.getByLabel('Plan name').fill('Monthly INR');
  await page.getByLabel('Plan product', { exact: true }).selectOption('product-0');
  await page.getByLabel('Plan price').fill('499.95');
  await page.getByRole('button', { name: 'Save plan', exact: true }).click();
  await expect.poll(() => fixture.plans.length).toBe(2);
  expect(fixture.plans[1]).toMatchObject({ currency: 'INR', amount_paisa: 49995 });
  expect(fixture.plans[0]).toMatchObject({ currency: 'USD', amount_paisa: 1000 });
});

test('add-on prices inherit the parent plan currency', async ({ page }) => {
  const fixture = await billingFixture(page);
  await page.goto('/apps/billing-a?tab=biz-billing&billing_view=catalog');
  await page.getByRole('button', { name: 'Plans', exact: true }).click();
  await page.getByRole('button', { name: 'Manage add-ons for Historical USD plan', exact: true }).click();
  await page.getByLabel('Add-on name', { exact: true }).fill('Support');
  await page.getByLabel('Add-on price (USD)', { exact: true }).fill('2.50');
  await page.getByRole('button', { name: 'Add add-on', exact: true }).click();
  await expect.poll(() => fixture.addons.get('usd-history')?.length).toBe(1);
  expect(fixture.addons.get('usd-history')?.[0].amount_paisa).toBe(250);
  await expect(page.getByText('USD 2.50', { exact: true })).toBeVisible();
});

test('long catalog names remain contained on mobile', async ({ page }) => {
  await page.setViewportSize({ width: 360, height: 900 });
  const fixture = await billingFixture(page);
  fixture.products[0].name = 'LongOffering'.repeat(20);
  await page.goto('/apps/billing-a?tab=biz-billing&billing_view=catalog');
  await expect(page.getByText(fixture.products[0].name, { exact: true }).first()).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
});

test('a failed credential save preserves input for an explicit retry', async ({ page }) => {
  const fixture = await billingFixture(page);
  await page.goto('/apps/billing-a?tab=biz-billing&billing_view=settings');
  await page.getByLabel('Razorpay account ID', { exact: true }).fill('acc_fixture');
  await page.getByLabel('Razorpay key ID', { exact: true }).fill('fixture-key');
  await page.getByLabel('Razorpay key secret', { exact: true }).fill('RETRY_MEMORY_ONLY');
  fixture.failGateway();
  await page.getByRole('button', { name: 'Save payment account', exact: true }).click();
  await expect(page.getByRole('alert').filter({ hasText: 'Could not save the payment account' })).toBeVisible();
  await expect(page.getByLabel('Razorpay key secret', { exact: true })).toHaveValue('RETRY_MEMORY_ONLY');
  expect(fixture.writes.filter(write => write.path.endsWith('/test'))).toHaveLength(1);
  fixture.recoverGateway();
  await page.getByRole('button', { name: 'Save payment account', exact: true }).click();
  await expect(page.getByLabel('Razorpay key secret', { exact: true })).toHaveValue('');
});

test('a stale payment account needs refresh and an explicit credential retry', async ({ page }) => {
  const fixture = await billingFixture(page);
  await page.goto('/apps/billing-a?tab=biz-billing&billing_view=settings');
  await page.getByLabel('Razorpay account ID', { exact: true }).fill('acc_fixture');
  await page.getByLabel('Razorpay key ID', { exact: true }).fill('fixture-key');
  await page.getByLabel('Razorpay key secret', { exact: true }).fill('ROTATION_MEMORY_ONLY');
  fixture.replaceGatewayRevision();
  await page.getByRole('button', { name: 'Save payment account', exact: true }).click();
  await expect(page.getByRole('alert').filter({ hasText: 'Could not save the payment account' })).toBeVisible();
  expect(fixture.writes.filter(write => write.path.endsWith('/test'))).toHaveLength(1);
  await page.getByRole('button', { name: 'Refresh payment account status', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Save payment account', exact: true })).toBeEnabled();
  await expect(page.getByLabel('Razorpay key secret', { exact: true })).toHaveValue('ROTATION_MEMORY_ONLY');
  await page.getByRole('button', { name: 'Save payment account', exact: true }).click();
  await expect(page.getByLabel('Razorpay key secret', { exact: true })).toHaveValue('');
  expect(fixture.writes.filter(write => write.path.endsWith('/test')).map(write => write.body.expected_revision)).toEqual([0, 1]);
});

test('catalog data from a slow previous application cannot cross into the next one', async ({ page }) => {
  await billingFixture(page);
  let started = false;
  let release: () => void = () => {};
  const held = new Promise<void>(resolve => { release = resolve; });
  await page.route('**/v1/biz/products*', async route => {
    const old = route.request().headers()['x-api-key']?.includes('billing-a');
    if (old) { started = true; await held; }
    try { await route.fulfill({ json: { success: true, total: 1, data: [{ id: old ? 'a-only' : 'b-only', app_id: old ? 'billing-a' : 'billing-b', name: old ? 'Only A offering' : 'Only B offering', active: true }] } }); }
    catch (error) { if (!route.request().failure()) throw error; }
  });
  await page.goto('/apps/billing-a?tab=biz-billing&billing_view=catalog');
  await expect.poll(() => started).toBe(true);
  await page.evaluate(() => { history.pushState({}, '', '/apps/billing-b?tab=biz-billing&billing_view=catalog'); dispatchEvent(new PopStateEvent('popstate')); });
  await expect(page.getByText('Only B offering', { exact: true })).toBeVisible();
  release();
  await page.waitForTimeout(250);
  await expect(page.getByText('Only A offering', { exact: true })).toHaveCount(0);
});

for (const width of [360, 390, 768, 1280]) test(`product and plan updates and archive preserve existing currencies at ${width}px`, async ({ page }) => {
  await page.setViewportSize({ width, height: 900 });
  const fixture = await billingFixture(page);
  await page.goto('/apps/billing-a?tab=biz-billing&billing_view=catalog');
  await page.getByRole('button', { name: 'New product', exact: true }).click();
  await page.getByLabel('Product name', { exact: true }).fill('Clinic service');
  await page.getByRole('button', { name: 'Save product', exact: true }).click();
  await page.getByRole('button', { name: 'Edit product Clinic service', exact: true }).click();
  await page.getByLabel('Product name', { exact: true }).fill('Consultation');
  await page.getByRole('button', { name: 'Save product', exact: true }).click();
  await page.getByRole('button', { name: 'Archive product Consultation', exact: true }).click();
  await expect.poll(() => fixture.products.find(product => product.name === 'Consultation')?.active).toBe(false);
  await page.getByRole('button', { name: 'Plans', exact: true }).click();
  await page.getByRole('button', { name: 'Edit plan Historical USD plan', exact: true }).click();
  await page.getByLabel('Plan name', { exact: true }).fill('Updated USD plan');
  await page.getByRole('button', { name: 'Save plan', exact: true }).click();
  await expect.poll(() => fixture.plans[0].name).toBe('Updated USD plan');
  expect(fixture.plans[0]).toMatchObject({ currency: 'USD', amount_paisa: 1000 });
  await page.getByRole('button', { name: 'Archive plan Updated USD plan', exact: true }).click();
  await expect.poll(() => fixture.plans[0].active).toBe(false);
});

for (const width of [360, 390, 768, 1280]) test(`dark catalog keyboard navigation at ${width}px`, async ({ page }, testInfo) => {
  await page.setViewportSize({ width, height: 900 });
  await page.emulateMedia({ colorScheme: 'dark' });
  await billingFixture(page);
  await page.goto('/apps/billing-a?tab=biz-billing&billing_view=catalog');
  await page.evaluate(() => document.documentElement.classList.add('dark'));
  await page.getByRole('button', { name: 'Plans', exact: true }).click();
  await page.getByRole('button', { name: 'New plan', exact: true }).click();
  await page.getByLabel('Plan currency', { exact: true }).focus();
  await page.keyboard.press('u');
  await expect(page.getByLabel('Plan currency', { exact: true })).toHaveValue('USD');
  await page.keyboard.press('Shift+Tab');
  await expect(page.getByLabel('Plan price', { exact: true })).toBeFocused();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  if (width === 360) await page.screenshot({ path: testInfo.outputPath('plan-dark-mobile.png') });
});
