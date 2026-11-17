import { test, expect, billingFixture } from './fixtures';
import { financialFixture } from './financial-fixture';

for (const provider of ['meta', 'twilio']) {
  test(`${provider} approved broadcast keeps its payment link and queues one request`, async ({ page }) => {
    await billingFixture(page);
    const sent: any[] = [];
    await page.route('**/v1/templates/**', r => r.fulfill({ json: { templates: [], total: 0 } }));
    await page.route('**/v1/twilio/templates/**', r => r.fulfill({ json: { data: { contents: provider === 'twilio' ? [
      { sid: 'HXfixture', friendly_name: 'Approved billing reminder', variables: { '1': 'Payment link' }, whatsapp: { status: 'approved' } },
      { sid: 'HXpending', friendly_name: 'Unapproved reminder', whatsapp: { status: 'pending' } },
    ] : [] } } }));
    await page.route('**/v1/whatsapp/templates/**', r => r.fulfill({ json: { data: { data: provider === 'meta' ? [
      { name: 'approved_billing_reminder', status: 'APPROVED', language: 'en_US', components: [{ type: 'BODY', text: 'Pay invoice at {{1}}' }] },
      { name: 'unapproved_reminder', status: 'PENDING', language: 'en_US' },
    ] : [] } } }));
    await page.route('**/v1/notifications/broadcast', async r => {
      sent.push({ body: r.request().postDataJSON(), key: r.request().headers()['x-api-key'] });
      await r.fulfill({ status: 202, json: { queued: 3, failed: 0, total_users: 3 } });
    });
    await page.goto('/apps/billing-a?tab=notifications');
    await page.getByRole('button', { name: 'Create Notification', exact: true }).click();
    await page.getByRole('tab', { name: 'Broadcast', exact: true }).click();
    await page.locator('#broadcastChannel').click();
    await page.getByRole('option', { name: 'WhatsApp', exact: true }).click();
    await page.locator('#broadcastTemplate').click();
    await expect(page.getByRole('option', { name: /[Uu]napproved/ })).toHaveCount(0);
    await page.getByRole('option', { name: provider === 'meta' ? 'approved_billing_reminder' : 'Approved billing reminder', exact: true }).click();
    await page.getByPlaceholder('Value for {{1}}', { exact: true }).fill('https://billing.example.invalid/invoice-a');
    await page.getByRole('button', { name: 'Send Broadcast', exact: true }).click();
    expect(sent).toHaveLength(0);
    await page.getByRole('button', { name: 'Yes, Broadcast', exact: true }).click();
    await expect.poll(() => sent.length).toBe(1);
    expect(sent[0].key).toBe('dashboard:billing-a');
    expect(sent[0].body.channel).toBe('whatsapp');
    expect(JSON.stringify(sent[0].body.data)).toContain('https://billing.example.invalid/invoice-a');
    expect(sent[0].body).not.toHaveProperty('user_ids');
    expect(sent[0].body).not.toHaveProperty('gateway_credentials');
  });
}

test('WhatsApp receipt failure and unknown admission remain distinct from payment state', async ({ page }) => {
  const fixture = await financialFixture(page);
  fixture.deliveries.forEach(row => { row.intent.channel = 'whatsapp'; });
  await page.goto('/apps/billing-a?tab=biz-billing&billing_view=invoices');
  await page.getByRole('button', { name: 'Messages / receipts', exact: true }).click();
  await expect(page.getByText('Delivery needs review', { exact: true })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Retry message', exact: true }).nth(1)).toBeDisabled();
  await page.getByRole('button', { name: 'Retry message', exact: true }).first().click();
  await expect.poll(() => fixture.deliveries[0].status).toBe('pending');
  expect(fixture.invoices.find(row => row.id === 'paid')?.amount_paid_paisa).toBe(1180);
});
