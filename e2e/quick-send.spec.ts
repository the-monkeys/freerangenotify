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
