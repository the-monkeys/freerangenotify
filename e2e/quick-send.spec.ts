/**
 * Quick Send tab — request-shape regression tests.
 *
 * Today's bug: a webhook template's `sample_data.webhook_target = "Discord Alerts"`
 * leaked into the request body's `data` map, and the worker resolved it as a
 * routing override — silently sending a Slack-targeted notification to Discord.
 *
 * These tests intercept the outbound POST /v1/quick-send and assert the body
 * shape so a regression of either the original `[object Object]` bug or the
 * routing-key leak fails fast at PR time.
 */
import { test, expect, captureRequestBody, openTab } from './fixtures';
import { test as browserTest, type Page } from '@playwright/test';

test.describe('Quick Send', () => {
    test.beforeEach(async ({ appNotificationsPage }) => {
        await openTab(appNotificationsPage, 'Quick Send');
    });

    test('webhook flow does not leak template sample_data routing keys into request body', async ({
        appNotificationsPage: page,
        state,
    }) => {
        test.skip(
            !state.webhookTemplate,
            'No webhook template found — seed one (e.g. clone webhook_rich_alert from the library).',
        );

        // Quick tab Template Select — Radix doesn't bind <Label htmlFor> so we
        // target by the placeholder text instead.
        await page.getByRole('combobox').filter({ hasText: 'Select a template' }).click();
        await page.getByRole('option', { name: state.webhookTemplate!.name }).click();

        // Pick the configured webhook endpoint (Select of provider name → url).
        await page.getByRole('combobox').filter({ hasText: 'Select webhook endpoint' }).click();
        await page.getByRole('option', { name: state.webhookProvider!.name }).click();

        const body = await captureRequestBody(page, '/v1/quick-send', async () => {
            await page.getByRole('button', { name: /^Send Notification$|^Schedule Notification$/ }).click();
        });

        expect(typeof body.webhook_url).toBe('string');
        expect(body.webhook_url).toBeTruthy();
        const data = (body.data || {}) as Record<string, unknown>;
        expect(data, '`data` must not carry routing keys').not.toHaveProperty('webhook_target');
        expect(data, '`data` must not carry routing keys').not.toHaveProperty('webhook_url');

        for (const [k, v] of Object.entries(data)) {
            expect(
                typeof v === 'string' || typeof v === 'number' || typeof v === 'boolean',
                `data.${k} must be a primitive, got ${typeof v}: ${JSON.stringify(v)}`,
            ).toBe(true);
        }
    });

    test('email flow sends a recipient and template', async ({ appNotificationsPage: page, state }) => {
        test.skip(!state.emailTemplate, 'No email template found.');
        test.skip(state.users.length === 0, 'No users found.');

        // The Quick tab auto-selects users[0] in the recipient dropdown on
        // mount, so we don't need to fill it. Just select the email template.
        await page.getByRole('combobox').filter({ hasText: 'Select a template' }).click();
        await page.getByRole('option', { name: state.emailTemplate!.name }).click();

        const body = await captureRequestBody(page, '/v1/quick-send', async () => {
            await page.getByRole('button', { name: /^Send Notification$|^Schedule Notification$/ }).click();
        });

        expect(typeof body.to).toBe('string');
        expect(body.to).toBeTruthy();
        expect(typeof body.template).toBe('string');
        expect(body.template).toBeTruthy();
    });
});

// These cases catch invented broadcast totals, estimates blocking accepted sends,
// current balances being substituted for historical snapshots, and creation time
// being shown as delivery time. Only the HTTP boundary is replaced.
const reliabilityUsage = {
    plan: 'pro', status: 'active', billing_model: 'credits', messages_sent: 0,
    credits_consumed: 0, credits_remaining: 1500, credits_reserved: 0,
    credits_available: 1500, credits_total: 1500, usage_percent: 0,
    current_period_start: '2026-10-01T00:00:00Z', current_period_end: '2026-11-01T00:00:00Z', days_remaining: 25,
};
const reliabilityRates = {
    currency: 'INR', active_version: 'fixture-v1', effective_at: '2026-10-01T00:00:00Z',
    credit_value_inr: 0.001, channel_credit_cost: { sms: 800, email: 1, whatsapp: 100, inapp: 0 },
    overage_per_message: {}, free_tier_daily_caps: { sms: 3 },
};
const reliabilityUsers = [1, 2].map(i => ({
    user_id: `user-${i}`, app_id: 'reliability', email: `user${i}@example.test`,
    phone: `+1555000000${i}`, full_name: `User ${i}`, external_id: `user-${i}`,
    created_at: '2026-10-01T00:00:00Z', updated_at: '2026-10-01T00:00:00Z',
}));

async function reliabilityPage(page: Page, options: {
    usage?: Record<string, unknown>; usageStatus?: number;
    rates?: Record<string, unknown>; ratesStatus?: number;
    breakdown?: Record<string, unknown>; breakdownStatus?: number;
    notifications?: Record<string, unknown>[];
} = {}) {
    page.on('pageerror', error => console.error(`[reliability browser] ${error.message}`));
    await page.addInitScript(() => localStorage.setItem('access_token', 'ui-fixture'));
    await page.route('**/v1/**', async route => {
        const pathname = new URL(route.request().url()).pathname.replace(/^\/v1/, '').replace(/\/$/, '');
        let data: unknown = { data: [], success: true };
        let status = 200;
        if (pathname === '/admin/me') data = { user_id: 'admin', full_name: 'UI Test', email: 'ui@example.test', phone_verified: true };
        else if (pathname === '/apps/reliability') data = { data: {
            app_id: 'reliability', app_name: 'Delivery reliability', api_key: 'fixture-key',
            settings: { email_config: { smtp_host: 'smtp.example.test', smtp_port: 587, from_email: 'sender@example.test' } },
        } };
        else if (pathname === '/billing/usage') { data = options.usage ?? reliabilityUsage; status = options.usageStatus ?? 200; }
        else if (pathname === '/billing/usage/breakdown') {
            data = options.breakdown ?? { billing_enabled: true, plan: 'pro', breakdown: [] };
            status = options.breakdownStatus ?? 200;
        }
        else if (pathname === '/billing/rates') { data = options.rates ?? reliabilityRates; status = options.ratesStatus ?? 200; }
        else if (pathname === '/users') data = { data: { users: reliabilityUsers, total_count: 2, page: 1, page_size: 100 } };
        else if (pathname.startsWith('/users/')) data = { data: reliabilityUsers.find(u => pathname.endsWith(u.user_id)) };
        else if (pathname === '/templates') data = { templates: ['sms', 'email'].map(channel => ({
            id: `${channel}-template`, name: `${channel.toUpperCase()} fixture`, channel, body: 'Hello', subject: 'Hello',
            variables: [], status: 'active', app_id: 'reliability', created_at: '2026-10-01T00:00:00Z',
        })), total: 2 };
        else if (pathname === '/notifications' && route.request().method() === 'GET') data = { notifications: options.notifications ?? [], total: options.notifications?.length ?? 0 };
        else if (route.request().method() === 'POST') { data = { notification_id: 'accepted', status: 'queued', queued: 2 }; status = 202; }
        await route.fulfill({ status, json: data });
    });
    await page.goto('/apps/reliability?tab=notifications');
    await expect(page.getByRole('button', { name: /Create Notification|Hide Send Form/ })).toBeVisible({ timeout: 15_000 });
}

async function selectReliabilityTemplate(page: Page, name = 'SMS fixture') {
    await page.getByRole('combobox').filter({ hasText: 'Select a template' }).click();
    await page.getByRole('option', { name }).click();
}

const historicalNotification = (metadata?: Record<string, unknown>) => ({
    notification_id: 'historical', user_id: 'user-1', app_id: 'reliability', channel: 'email',
    priority: 'normal', status: 'failed', content: { title: 'Earlier SMTP attempt', body: 'Hello' },
    created_at: '2026-10-07T01:00:00Z', updated_at: '2026-10-07T01:01:00Z',
    failed_at: '2026-10-07T01:01:00Z', error_message: 'Original provider error', metadata,
});

browserTest.describe('Delivery reliability UI', () => {
    browserTest('one SMS uses the active price and remains advisory', async ({ page }) => {
        await reliabilityPage(page);
        await openTab(page, 'Quick Send');
        await selectReliabilityTemplate(page);
        const estimate = page.getByRole('status', { name: 'Cost estimate' });
        await expect(estimate).toContainText('800 credits per recipient');
        await expect(estimate).toContainText('Estimated total: 800 credits');
        await expect(estimate).toContainText('Currently available: 1,500 credits');
        await expect(estimate).toContainText('Advisory');
        const body = await captureRequestBody(page, '/v1/quick-send', () => page.getByRole('button', { name: 'Send Notification', exact: true }).click());
        expect(body.template).toBe('SMS fixture');
        expect(body).not.toHaveProperty('estimated_cost');
    });

    browserTest('two SMS recipients estimate 1600 against 1500 without blocking bulk acceptance', async ({ page }, testInfo) => {
        await reliabilityPage(page);
        await openTab(page, 'Bulk Send');
        await page.getByRole('combobox').filter({ hasText: /^Email$/ }).click();
        await page.getByRole('option', { name: 'SMS', exact: true }).click();
        await page.getByRole('button', { name: 'Select users', exact: true }).click();
        await page.getByRole('button', { name: 'Select all', exact: true }).click();
        await expect(page.getByText('2 selected', { exact: true }).first()).toBeVisible();
        await page.keyboard.press('Escape');
        await selectReliabilityTemplate(page);
        const estimate = page.getByRole('status', { name: 'Cost estimate' });
        await expect(estimate).toContainText('Estimated total: 1,600 credits');
        await expect(estimate).toContainText('exceeds currently available credits');
        await expect(page.getByRole('button', { name: 'Send Notification', exact: true })).toBeEnabled();
        await page.screenshot({ path: testInfo.outputPath('two-recipient-sms.png'), fullPage: true });
        const body = await captureRequestBody(page, '/v1/notifications/bulk', () => page.getByRole('button', { name: 'Send Notification', exact: true }).click());
        expect(body.user_ids).toEqual(['user-1', 'user-2']);
        expect(body.channel).toBe('sms');
        expect(body).not.toHaveProperty('estimated_cost');
    });

    browserTest('unknown broadcast count never uses the loaded user list as a total', async ({ page }) => {
        await reliabilityPage(page);
        await openTab(page, 'Broadcast');
        await page.locator('#broadcastChannel').click();
        await page.getByRole('option', { name: 'SMS', exact: true }).click();
        const estimate = page.getByRole('status', { name: 'Cost estimate' });
        await expect(estimate).toContainText('800 credits per recipient');
        await expect(estimate).toContainText('Recipient count is unknown');
        await expect(estimate).not.toContainText('Estimated total:');
    });

    browserTest('in-app channel uses its active zero-cost rate alias', async ({ page }) => {
        await reliabilityPage(page);
        await openTab(page, 'Bulk Send');
        await page.getByRole('combobox').filter({ hasText: /^Email$/ }).click();
        await page.getByRole('option', { name: 'In-App', exact: true }).click();
        await expect(page.getByRole('status', { name: 'Cost estimate' })).toContainText('0 credits per recipient');
    });

    for (const variant of ['old', 'reserved', 'disabled', 'legacy', 'rates-error', 'usage-error', 'missing-rate', 'breakdown-old', 'breakdown-missing', 'breakdown-error'] as const) {
        browserTest(`compatible billing fallback: ${variant}`, async ({ page }) => {
            const options: Parameters<typeof reliabilityPage>[1] = {};
            if (variant === 'old') {
                const { credits_reserved, credits_available, billing_model, ...old } = reliabilityUsage;
                options.usage = old;
            }
            if (variant === 'reserved') options.usage = { ...reliabilityUsage, credits_reserved: 800, credits_available: undefined };
            if (variant === 'disabled') {
                options.breakdown = { billing_enabled: false };
                options.rates = { ...reliabilityRates, active_version: 'default', channel_credit_cost: { sms: 80, email: 1 } };
            }
            if (variant === 'legacy') options.usage = { ...reliabilityUsage, billing_model: 'legacy', credits_total: 0 };
            if (variant === 'rates-error') options.ratesStatus = 503;
            if (variant === 'usage-error') options.usageStatus = 503;
            if (variant === 'missing-rate') options.rates = { ...reliabilityRates, channel_credit_cost: {} };
            if (variant === 'breakdown-old') options.breakdown = { breakdown: [] };
            if (variant === 'breakdown-missing') options.breakdownStatus = 404;
            if (variant === 'breakdown-error') options.breakdownStatus = 503;
            await reliabilityPage(page, options);
            await openTab(page, 'Quick Send');
            await selectReliabilityTemplate(page);
            const estimate = page.getByRole('status', { name: 'Cost estimate' });
            if (variant === 'old') await expect(estimate).toContainText('Currently available: 1,500 credits');
            else if (variant === 'reserved') await expect(estimate).toContainText('Currently available: 700 credits');
            else if (variant === 'disabled' || variant === 'legacy') {
                await expect(estimate).toContainText(variant === 'disabled' ? 'Billing is disabled' : 'Legacy billing');
                await expect(estimate).not.toContainText('Estimated total:');
                if (variant === 'disabled') {
                    await expect(estimate).not.toContainText('credits per recipient');
                    await expect(estimate).not.toContainText('Currently available:');
                    await expect(estimate).not.toContainText('exceeds currently available credits');
                    const wallet = page.getByText('Current available credits', { exact: true }).locator('..');
                    await expect(wallet).toContainText('Billing disabled');
                    await expect(wallet).not.toContainText('1,500');
                }
            } else if (variant === 'usage-error') {
                await expect(estimate).toContainText('Estimated total: 800 credits');
                await expect(estimate).toContainText('Current availability is unavailable');
            } else if (variant.startsWith('breakdown-')) {
                await expect(estimate).toContainText('Estimated total: 800 credits');
                await expect(estimate).toContainText('Currently available: 1,500 credits');
                await expect(estimate).toContainText('Advisory');
                await expect(page.getByText('Current available credits', { exact: true }).locator('..')).toContainText('1,500 / 1,500');
            } else await expect(estimate).toContainText('Cost estimate unavailable');
            await expect(page.getByRole('button', { name: 'Send Notification', exact: true })).toBeEnabled();
            await expect(page.locator('body')).not.toContainText('NaN');
        });
    }

    browserTest('disabled billing from breakdown suppresses bulk costs and wallet with actual usage and fallback rates', async ({ page }) => {
        await reliabilityPage(page, {
            breakdown: { billing_enabled: false },
            rates: { ...reliabilityRates, active_version: 'default', channel_credit_cost: { sms: 80, email: 1 } },
        });
        const wallet = page.getByText('Current available credits', { exact: true }).locator('..');
        await expect(wallet).toContainText('Billing disabled');
        await expect(wallet).not.toContainText('1,500');
        await openTab(page, 'Bulk Send');
        await page.getByRole('combobox').filter({ hasText: /^Email$/ }).click();
        await page.getByRole('option', { name: 'SMS', exact: true }).click();
        await page.getByRole('button', { name: 'Select users', exact: true }).click();
        await page.getByRole('button', { name: 'Select all', exact: true }).click();
        await expect(page.getByText('2 selected', { exact: true }).first()).toBeVisible();
        await page.keyboard.press('Escape');
        await selectReliabilityTemplate(page);
        const estimate = page.getByRole('status', { name: 'Cost estimate' });
        await expect(estimate).toContainText('Billing is disabled');
        await expect(estimate).not.toContainText('Estimated total:');
        await expect(estimate).not.toContainText('credits per recipient');
        await expect(estimate).not.toContainText('Currently available:');
        await expect(estimate).not.toContainText('exceeds currently available credits');
        const body = await captureRequestBody(page, '/v1/notifications/bulk', () => page.getByRole('button', { name: 'Send Notification', exact: true }).click());
        expect(body.user_ids).toEqual(['user-1', 'user-2']);
        expect(body.channel).toBe('sms');
    });

    browserTest('billing refresh shares explicit disablement between sidebar and open estimate', async ({ page }) => {
        await page.clock.install();
        const options = { breakdown: { billing_enabled: true } };
        await reliabilityPage(page, options);
        await openTab(page, 'Quick Send');
        await selectReliabilityTemplate(page);
        const estimate = page.getByRole('status', { name: 'Cost estimate' });
        await expect(estimate).toContainText('Estimated total: 800 credits');
        options.breakdown = { billing_enabled: false };
        await page.clock.fastForward(61_000);
        await expect(page.getByText('Current available credits', { exact: true }).locator('..')).toContainText('Billing disabled');
        await expect(estimate).toContainText('Billing is disabled');
        await expect(estimate).not.toContainText('Estimated total:');
        await expect(page.getByRole('button', { name: 'Send Notification', exact: true })).toBeEnabled();
    });

    browserTest('unsent failure has no sent time and old errors retain their original text', async ({ page }) => {
        await reliabilityPage(page, { notifications: [historicalNotification()] });
        const row = page.getByRole('row').filter({ hasText: 'Earlier SMTP attempt' });
        const sentColumn = await page.getByRole('columnheader').allTextContents();
        await expect(row.getByRole('cell').nth(sentColumn.indexOf('Sent At'))).toHaveText('-');
        await row.click();
        await expect(page.getByText('Original provider error', { exact: true })).toBeVisible();
        await expect(page.getByText('Created At', { exact: true })).toBeVisible();
        await expect(page.getByText('Attempt-time credit snapshot', { exact: true })).toHaveCount(0);
        await expect(page.getByText('Sent At', { exact: true })).toHaveCount(1); // table header only
    });

    browserTest('sent column uses sent_at rather than created_at', async ({ page }) => {
        await reliabilityPage(page, { notifications: [{ ...historicalNotification(), status: 'sent', sent_at: '2026-10-07T02:00:00Z' }] });
        const row = page.getByRole('row').filter({ hasText: 'Earlier SMTP attempt' });
        const headers = await page.getByRole('columnheader').allTextContents();
        await expect(row.getByRole('cell').nth(headers.indexOf('Sent At'))).toContainText('2:00:00 AM');
    });

    const failures = [
        ['network', 'Network', true],
        ['authentication', 'Authentication', false],
        ['credits_temporarily_reserved', 'temporarily reserved', true],
        ['insufficient_credits', 'Insufficient remaining credits', false],
        ['daily_cap_exceeded', 'Daily channel limit', false],
        ['notification_expired', 'validity window', false],
    ] as const;
    for (const [code, explanation, retryable] of failures) {
        browserTest(`historical ${code} keeps attempt snapshot separate from today's wallet`, async ({ page }, testInfo) => {
            await reliabilityPage(page, { notifications: [historicalNotification({
                failure_code: code, failure_stage: ['network', 'authentication'].includes(code) ? 'connect' : 'billing',
                provider: 'smtp', credential_source: 'byoc', retryable,
                credits_required: 800,
                credits_remaining: code === 'insufficient_credits' ? 700 : 1500,
                credits_reserved: code === 'insufficient_credits' ? 0 : 800,
                credits_available: 700, rate_card_version: 'earlier-v0',
            })] });
            await expect(page.getByText('1,500 / 1,500', { exact: true })).toBeVisible();
            await page.getByRole('row').filter({ hasText: 'Earlier SMTP attempt' }).click();
            const context = page.getByRole('region', { name: 'Failure context' });
            await expect(context).toContainText(code);
            await expect(context).toContainText(explanation);
            await expect(context).toContainText('Application credentials');
            await expect(context).toContainText(retryable ? 'Retryable: Yes' : 'Retryable: No');
            await expect(context).toContainText('Available: 700 credits');
            await expect(context).toContainText(code === 'insufficient_credits' ? 'Remaining: 700 credits' : 'Remaining: 1,500 credits');
            await expect(context).toContainText(code === 'insufficient_credits' ? 'Reserved: 0 credits' : 'Reserved: 800 credits');
            await expect(context).toContainText('earlier-v0');
            await expect(context).not.toContainText('Available: 1,500 credits');
            if (code === 'authentication') {
                await page.setViewportSize({ width: 1440, height: 1080 });
                await page.screenshot({ path: testInfo.outputPath('custom-smtp-failure.png'), fullPage: true, animations: 'disabled' });
            }
        });
    }

    for (const [code, explanation] of [
        ['network_error', 'DNS connectivity'], ['timeout', 'timed out'],
        ['configuration', 'Provider configuration'], ['invalid_request', 'Provider rejected'],
        ['rate_limit', 'Provider rate limit'], ['provider_api', 'Provider API'], ['unknown', 'Unclassified provider failure'],
    ]) {
        browserTest(`provider category ${code} has context without fabricated retry advice`, async ({ page }) => {
            await reliabilityPage(page, { notifications: [historicalNotification({ failure_code: code, failure_stage: 'health' })] });
            await page.getByRole('row').filter({ hasText: 'Earlier SMTP attempt' }).click();
            const context = page.getByRole('region', { name: 'Failure context' });
            await expect(context).toContainText(explanation);
            await expect(context).toContainText('Stage: health');
            await expect(context).not.toContainText('Retryable:');
            await expect(context).not.toContainText('Attempt-time credit snapshot');
        });
    }

    browserTest('partial future diagnostics do not invent balances or retryability', async ({ page }) => {
        await reliabilityPage(page, { notifications: [historicalNotification({ failure_code: 'future_failure', credits_available: 0 })] });
        await page.getByRole('row').filter({ hasText: 'Earlier SMTP attempt' }).click();
        const context = page.getByRole('region', { name: 'Failure context' });
        await expect(context).toContainText('future_failure');
        await expect(context).toContainText('Available: 0 credits');
        await expect(context).not.toContainText('Remaining:');
        await expect(context).not.toContainText('Retryable:');
        await expect(context).not.toContainText('smtp');
    });

    browserTest('unrecognized diagnostic strings and malformed optional amounts stay safe', async ({ page }) => {
        await reliabilityPage(page, { notifications: [historicalNotification({
            failure_code: '__proto__', credential_source: '__proto__', credits_available: '700', retryable: 'true',
        })] });
        await page.getByRole('row').filter({ hasText: 'Earlier SMTP attempt' }).click();
        const context = page.getByRole('region', { name: 'Failure context' });
        await expect(context).toContainText('Code: __proto__');
        await expect(context).toContainText('Credential source: __proto__');
        await expect(context).not.toContainText('Attempt-time credit snapshot');
        await expect(context).not.toContainText('Retryable:');
    });
});
