import { test as base, expect, type Page } from '@playwright/test';

export const test = base.extend<{ isolation: void }>({ isolation: [async ({ page }, use) => {
  const unexpected: string[] = [];
  await page.addInitScript(() => localStorage.setItem('access_token', 'billing-fixture-session'));
  await page.route('**/*', async route => {
    const url = new URL(route.request().url());
    if (url.origin === 'http://127.0.0.1:3011' && !url.pathname.startsWith('/v1/')) return route.continue();
    if (url.hostname === 'fonts.googleapis.com') return route.fulfill({ contentType: 'text/css', body: '' });
    unexpected.push(url.origin + url.pathname);
    await route.abort();
  });
  await use();
  expect(unexpected, 'billing fixtures must never contact a real provider').toEqual([]);
}, { auto: true }] });
export { expect };

export async function billingFixture(page: Page, options: { role?: string; gatewayError?: boolean; legacy?: boolean } = {}) {
  const writes: { path: string; method: string; body: Record<string, unknown>; key?: string }[] = [];
  const products = Array.from({ length: 21 }, (_, i) => ({ id: `product-${i}`, app_id: 'billing-a', name: `Product ${i + 1}`, description: 'Service', active: true, tax_rate: 18 }));
  const plans: Record<string, unknown>[] = [{ id: 'usd-history', product_id: 'product-0', name: 'Historical USD plan', amount_paisa: 1000, currency: 'USD', billing_cycle: 'monthly', pricing_model: 'flat', active: true }];
  const config: Record<string, Record<string, unknown>> = {
    preferences: { default_currency: 'INR', supported_currencies: ['INR', 'USD', 'EUR', 'GBP'] },
    branding: { company_name: 'Fixture Business', primary_color: '#4F46E5' },
    tax: { default_tax_rate: 18, tax_inclusive: false },
    numbering: { invoice_prefix: 'INV', estimate_prefix: 'EST', credit_note_prefix: 'CN', contract_prefix: 'CTR', next_invoice_number: 1 },
    payment_terms: { default_terms: 'due_on_receipt' },
    dunning: { enabled: true, steps: [{ day_offset: 1, channel: 'email', template: 'biz_reminder_gentle' }] },
  };
  let gatewayError = options.gatewayError ?? false;
  const addons = new Map<string, Record<string, unknown>[]>();
  const revisions = { test: 0, live: 0 };
  await page.route('**/v1/**', async route => {
    const request = route.request(), url = new URL(request.url()), p = url.pathname.replace(/^\/v1/, '').replace(/\/$/, '');
    const appID = request.headers()['x-api-key']?.includes('billing-b') ? 'billing-b' : 'billing-a';
    const response = (data: unknown, total?: number) => route.fulfill({ json: { success: true, data, ...(total === undefined ? {} : { total }) } });
    if (request.method() !== 'GET') writes.push({ path: p, method: request.method(), body: request.postDataJSON() ?? {}, key: request.headers()['idempotency-key'] });
    if (p === '/admin/me') return route.fulfill({ json: { user_id: options.role === 'viewer' ? 'viewer' : 'owner', full_name: 'Billing review', email: 'billing@example.invalid', phone_verified: true } });
    if (p.startsWith('/apps/') && !p.includes('/team') && p.split('/').length === 3) {
      const id = p.split('/')[2];
      return response({ app_id: id, app_name: id === 'billing-b' ? 'Second Business' : 'Billing Business', api_key: `dashboard:${id}`, admin_user_id: 'owner', role: options.role ?? 'owner', settings: {} });
    }
    if (p === '/apps') return response(['billing-a', 'billing-b'].map(id => ({ app_id: id, app_name: id, api_key: `dashboard:${id}`, admin_user_id: 'owner' })));
    if (p === '/biz/lifecycle') return response({ app_id: appID, lifecycle_version: options.legacy ? 1 : 2, status: options.legacy ? 'legacy' : 'active', primary_currency: 'INR', active_writers: 0, new_collection_enabled: true });
    if (p.startsWith('/biz/gateway-connections/')) {
      if (gatewayError) return route.fulfill({ status: 503, json: { error: 'Payment account temporarily unavailable' } });
      const mode = p.endsWith('/live') ? 'live' : 'test';
      if (request.method() === 'PUT') {
        if (request.postDataJSON().expected_revision !== revisions[mode]) return route.fulfill({ status: 409, json: { error: 'Payment account revision changed' } });
        revisions[mode]++;
      }
      const revision = revisions[mode];
      return response({ app_id: appID, configured: revision > 0, revision, account_id: revision ? 'acc_fixture' : undefined, mode, readiness: revision ? 'authenticated' : 'not_configured', api_authenticated: revision > 0, account_verified: false, automatic_ready: false });
    }
    if (p.startsWith('/biz/config/')) {
      const type = p.split('/').at(-1)!;
      if (request.method() === 'PUT') config[type] = { ...config[type], ...request.postDataJSON() };
      return response({ app_id: appID, config_type: type, data: config[type] ?? {} });
    }
    if (p === '/biz/products') {
      if (request.method() === 'POST') { const product = { ...request.postDataJSON(), id: `created-${products.length}`, app_id: appID, active: true }; products.unshift(product); return response(product); }
      const search = (url.searchParams.get('search') ?? '').toLowerCase();
      const rows = products.filter(x => x.name.toLowerCase().includes(search));
      const offset = Number(url.searchParams.get('offset') ?? 0), limit = Number(url.searchParams.get('limit') ?? 20);
      return response(rows.slice(offset, offset + limit), rows.length);
    }
    if (p.startsWith('/biz/products/')) {
      const product = products.find(x => x.id === p.split('/').at(-1));
      if (request.method() === 'PUT' && product) Object.assign(product, request.postDataJSON());
      return response(product);
    }
    if (p === '/biz/plans') {
      if (request.method() === 'POST') { const plan = { ...request.postDataJSON(), id: `plan-${plans.length}`, active: true }; plans.push(plan); return response(plan); }
      return response(plans, plans.length);
    }
    if (p.startsWith('/biz/plans/')) {
      if (p.includes('/addons')) {
        const planID = p.split('/')[3], rows = addons.get(planID) ?? [];
        if (request.method() === 'POST') { const addon = { ...request.postDataJSON(), id: `addon-${rows.length}`, plan_id: planID }; rows.push(addon); addons.set(planID, rows); return response(addon); }
        if (request.method() === 'DELETE') { addons.set(planID, rows.filter(row => row.id !== p.split('/').at(-1))); return response({}); }
        return response(rows);
      }
      const plan = plans.find(x => x.id === p.split('/').at(-1));
      if (request.method() === 'PUT' && plan) Object.assign(plan, request.postDataJSON());
      return response(plan);
    }
    if (p === '/billing/usage') return route.fulfill({ json: { plan: 'free', credits_remaining: 1500, credits_total: 1500, usage_percent: 0 } });
    return response([], 0);
  });
  return { writes, products, plans, addons, config, recoverGateway: () => { gatewayError = false; }, failGateway: () => { gatewayError = true; }, replaceGatewayRevision: () => { revisions.test++; } };
}
