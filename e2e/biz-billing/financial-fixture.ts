import type { Page } from '@playwright/test';
import { billingFixture } from './fixtures';

export async function financialFixture(page: Page, options: { lostPaymentReply?: boolean; spentCredit?: boolean; unresolvedCredit?: boolean; role?: string; legacy?: boolean } = {}) {
  const base = await billingFixture(page, options);
  base.plans.push({ id: 'inr-fixed', product_id: 'product-0', name: 'Monthly INR', amount_paisa: 1000, currency: 'INR', billing_cycle: 'monthly', pricing_model: 'flat', active: true });
  const customers = Array.from({ length: 1200 }, (_, i) => ({ user_id: `customer-${i}`, app_id: 'billing-a', full_name: `Customer ${i}`, email: `customer${i}@example.invalid` }));
  const searchRequests: { search: string; pageSize: number }[] = [];
  const requests: { path: string; body: Record<string, any>; key: string }[] = [];
  const source = (id: string, status: string, paid: number, due: number, credit = 0, refunded = 0) => ({
    id, app_id: 'billing-a', user_id: 'customer-1', invoice_number: `INV-${id.toUpperCase()}`, status, currency: 'USD',
    subtotal_paisa: 1000, tax_amount_paisa: 180, total_paisa: 1180, amount_paid_paisa: paid, amount_due_paisa: due, credits_applied: credit,
    cash_refunded_paisa: refunded, collection_owner: '', gateway_mode: 'live', line_items: [{ description: 'Service', quantity: 1, unit_price: 1000 }],
    created_at: '2026-11-10T10:00:00Z', due_date: '2026-11-12T10:00:00Z',
    issued_snapshot: { customer: { name: 'Customer 1', email: 'customer1@example.invalid' }, issuer: { name: 'Fixture Business' } },
  });
  const invoices: Record<string, any>[] = [source('unpaid', 'overdue', 0, 1180), source('partial', 'partially_paid', 500, 680), source('paid', 'paid', 1180, 0), source('credited', 'paid', 0, 0, 1180), source('refunded', 'paid', 1180, 0, 0, 1180), { ...source('automatic', 'open', 0, 1180), collection_owner: 'verified-mandate' }];
  const payments: Record<string, any>[] = [{ id: 'original-payment', user_id: 'customer-1', app_id: 'billing-a', invoice_ids: ['paid'], currency: 'USD', amount_paisa: 1180, method: 'cash', status: 'success', refunded_paisa: 0, refund_reserved_paisa: 0, excess_credit_paisa: options.spentCredit ? 180 : 0, credit_provenance_known: true, created_at: '2026-11-10T10:00:00Z' }];
  const subscriptions: Record<string, any>[] = [];
  const deliveries = [{ id: 'receipt-delivery', app_id: 'billing-a', status: 'failed', failure_code: 'admission_rejected', attempts: 6, may_have_dispatched: false, intent: { template_id: 'biz_payment_received', user_id: 'customer-1', invoice_id: 'paid', channel: 'email' }, created_at: '2026-11-10T10:00:00Z' }, { id: 'unknown-delivery', app_id: 'billing-a', status: 'unknown', failure_code: 'admission_unconfirmed', attempts: 1, may_have_dispatched: true, intent: { template_id: 'biz_invoice_issued', user_id: 'customer-1', invoice_id: 'unpaid', channel: 'email' }, created_at: '2026-11-10T10:00:00Z' }];
  const completed = new Map<string, unknown>();
  let dropped = false;
  await page.route('**/v1/users**', async route => {
    const u = new URL(route.request().url()), search = u.searchParams.get('search') ?? '', pageSize = Number(u.searchParams.get('page_size') ?? 20), number = Number(u.searchParams.get('page') ?? 1);
    const id = u.pathname.replace(/^\/v1\/users\/?/, '');
    if (id) return route.fulfill({ json: { success: true, data: customers.find(customer => customer.user_id === id) } });
    searchRequests.push({ search, pageSize });
    const rows = customers.filter(customer => `${customer.full_name} ${customer.email}`.toLowerCase().includes(search.toLowerCase()));
    return route.fulfill({ json: { success: true, data: { users: rows.slice((number - 1) * pageSize, number * pageSize), total_count: rows.length, page: number, page_size: pageSize } } });
  });
  await page.route('**/v1/biz/**', async route => {
    const request = route.request(), url = new URL(request.url()), path = url.pathname.replace(/^\/v1/, '').replace(/\/$/, '');
    const method = request.method(), body = request.postData() ? request.postDataJSON() : {}, key = request.headers()['idempotency-key'] ?? '';
    const respond = (data: unknown, total?: number) => route.fulfill({ json: { success: true, data, ...(total === undefined ? {} : { total }) } });
    if (method !== 'GET') requests.push({ path, body, key });
    const prefix = `${path}:${key}`;
    if (method !== 'GET' && completed.has(prefix)) return respond(completed.get(prefix));
    if (path === '/biz/invoices/preview') {
      const subtotal = body.line_items.reduce((sum: number, row: any) => sum + row.quantity * row.unit_price - (row.discount_paisa ?? 0), 0);
      return respond({ ...body, subtotal_paisa: subtotal, discount_paisa: 0, tax_amount_paisa: Math.round(subtotal * .18), tax_breakdown: {}, total_paisa: Math.round(subtotal * 1.18), payment_terms: body.payment_terms ?? 'due_on_receipt' });
    }
    if (path === '/biz/invoices') {
      if (method === 'GET') return respond(invoices, invoices.length);
      if (!key) return route.fulfill({ status: 400, json: { error: 'Idempotency key required' } });
      const subtotal = body.line_items.reduce((sum: number, row: any) => sum + row.quantity * row.unit_price, 0);
      const invoice = { ...source(`draft-${invoices.length}`, 'draft', 0, Math.round(subtotal * 1.18)), ...body, invoice_number: '', subtotal_paisa: subtotal, tax_amount_paisa: Math.round(subtotal * .18), total_paisa: Math.round(subtotal * 1.18), issued_snapshot: undefined };
      invoices.unshift(invoice); completed.set(prefix, invoice); return respond(invoice);
    }
    const invoiceMatch = path.match(/^\/biz\/invoices\/([^/]+)(?:\/(.+))?$/);
    if (invoiceMatch) {
      const invoice = invoices.find(row => row.id === invoiceMatch[1]), action = invoiceMatch[2];
      if (!invoice) return route.fulfill({ status: 404, json: { error: 'Invoice not found' } });
      if (!action) return respond(invoice);
      if (action === 'issue') { invoice.status = 'open'; invoice.invoice_number = `INV-NEW-${invoices.length}`; invoice.issued_snapshot = { customer: customers.find(customer => customer.user_id === invoice.user_id), issuer: { name: 'Fixture Business' } }; completed.set(prefix, invoice); return respond(invoice); }
      if (action === 'document') return respond({ ...invoice, invoice_id: invoice.id, issuer: { name: 'Fixture Business' }, customer: invoice.issued_snapshot?.customer ?? customers[1], payments: payments.filter(payment => payment.invoice_ids.includes(invoice.id)), credits: [] });
      if (action === 'payments') {
        if (invoice.collection_owner) return route.fulfill({ status: 409, json: { code: 'BILLING_COLLECTION_LOCKED', error: 'Automatic collection must be stopped first' } });
        if (!key) return route.fulfill({ status: 400, json: { error: 'Idempotency key required' } });
        const amount = body.amount_paisa || invoice.amount_due_paisa;
        const payment = { id: `payment-${payments.length}`, app_id: 'billing-a', user_id: invoice.user_id, invoice_ids: [invoice.id], currency: invoice.currency, method: body.method, reference: body.reference, amount_paisa: amount, refunded_paisa: 0, refund_reserved_paisa: 0, status: 'success', created_at: new Date().toISOString() };
        payments.push(payment); invoice.amount_paid_paisa += amount; invoice.amount_due_paisa = Math.max(0, invoice.amount_due_paisa - amount); invoice.status = invoice.amount_due_paisa ? 'partially_paid' : 'paid'; completed.set(prefix, payment);
        if (options.lostPaymentReply && !dropped) { dropped = true; return route.abort('failed'); }
        return respond(payment);
      }
      if (action === 'apply-credit') { invoice.credits_applied += invoice.amount_due_paisa; invoice.amount_due_paisa = 0; invoice.status = 'paid'; completed.set(prefix, invoice); return respond(invoice); }
    }
    if (path === '/biz/payments') return respond(payments, payments.length);
    const refundMatch = path.match(/^\/biz\/payments\/([^/]+)\/refund$/);
    if (refundMatch) {
      if (options.spentCredit) return route.fulfill({ status: 409, json: { code: 'BILLING_INVALID_AMOUNT', error: 'The overpayment credit has already been spent. This amount cannot be refunded.' } });
      const payment = payments.find(row => row.id === refundMatch[1])!;
      payment.refunded_paisa += body.amount_paisa || payment.amount_paisa - payment.refunded_paisa; payment.status = 'refunded'; completed.set(prefix, payment); return respond(payment);
    }
    if (path === '/biz/subscriptions') {
      if (method === 'GET') return respond(subscriptions, subscriptions.length);
      const plan = base.plans.find(row => row.id === body.plan_id)!;
      const subscription = { ...body, id: `subscription-${subscriptions.length}`, currency: plan.currency, plan_name: plan.name, plan_amount_paisa: plan.amount_paisa, billing_cycle: plan.billing_cycle, status: body.collection_mode === 'automatic' ? 'awaiting_authorization' : 'active', current_period_start: '2026-11-12T00:00:00Z', current_period_end: '2026-12-12T00:00:00Z', created_at: new Date().toISOString() };
      subscriptions.push(subscription); completed.set(prefix, subscription); return respond(subscription);
    }
    if (path.startsWith('/biz/users/') && path.endsWith('/balance')) return respond({ available_paisa: 1180, reserved_paisa: 0, unresolved_legacy: options.unresolvedCredit ?? false, scope: { currency: url.searchParams.get('currency') ?? 'USD' } });
    if (path === '/biz/notification-deliveries') return respond(deliveries, deliveries.length);
    const retryMatch = path.match(/^\/biz\/notification-deliveries\/([^/]+)\/retry$/);
    if (retryMatch) { const row = deliveries.find(item => item.id === retryMatch[1])!; row.status = 'pending'; return respond(row); }
    if (path.startsWith('/biz/analytics/')) return respond({ currency: url.searchParams.get('currency') ?? 'INR', total_paid_paisa: url.searchParams.get('currency') === 'USD' ? 1180 : 2000 });
    return route.fallback();
  });
  return { ...base, customers, searchRequests, invoices, payments, subscriptions, deliveries, requests };
}
