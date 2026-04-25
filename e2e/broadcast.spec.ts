/**
 * Broadcast tab — request-shape tests.
 *
 * Verifies that the confirm flow produces a single POST /v1/notifications/broadcast
 * with the expected channel + template_id and no leaked routing keys.
 */
import { test, expect, captureRequestBody } from './fixtures';

test.describe('Broadcast', () => {
    test.beforeEach(async ({ appNotificationsPage }) => {
        const toggle = appNotificationsPage.getByRole('button', { name: /Create Notification|Hide Send Form/ });
        const text = (await toggle.textContent()) || '';
        if (text.includes('Create')) await toggle.click();
        await appNotificationsPage.getByRole('tab', { name: 'Broadcast' }).click();
    });

    test('confirming a broadcast posts to /notifications/broadcast with channel + template_id', async ({
        appNotificationsPage: page,
        state,
    }) => {
        test.skip(!state.emailTemplate, 'No email template found.');

        // SelectTrigger has explicit id="broadcastChannel" — use it directly.
        await page.locator('#broadcastChannel').click();
        await page.getByRole('option', { name: 'Email' }).click();

        await page.locator('#broadcastTemplate').click();
        await page.getByRole('option', { name: state.emailTemplate!.name }).click();

        // Click "Send Broadcast" — this opens the confirmation panel, not the request.
        await page.getByRole('button', { name: 'Send Broadcast' }).click();
        await expect(page.getByText('Confirm Broadcast')).toBeVisible();

        const body = await captureRequestBody(page, '/v1/notifications/broadcast', async () => {
            await page.getByRole('button', { name: 'Yes, Broadcast' }).click();
        });

        expect(body.channel).toBe('email');
        expect(body.template_id).toBe(state.emailTemplate!.id);
        // Broadcast must not carry user_ids / to / webhook_url — those belong on
        // single-send / bulk-send paths.
        expect(body).not.toHaveProperty('user_ids');
        expect(body).not.toHaveProperty('to');
        expect(body).not.toHaveProperty('webhook_url');
    });
});
