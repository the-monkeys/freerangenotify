/**
 * Bulk Send tab ("Advanced") — request-shape tests.
 *
 * Verifies that selecting 2+ recipients triggers a POST /v1/notifications/bulk
 * with the expected `user_ids` array shape.
 */
import { test, expect, captureRequestBody, openTab } from './fixtures';

test.describe('Bulk Send', () => {
    test.beforeEach(async ({ appNotificationsPage }) => {
        await openTab(appNotificationsPage, 'Bulk Send');
    });

    test('selecting 2+ users posts to /notifications/bulk with user_ids array', async ({
        appNotificationsPage: page,
        state,
    }) => {
        test.skip(!state.emailTemplate, 'No email template found.');
        test.skip(state.users.length < 2, 'Need at least 2 users — globalSetup should have created them.');

        // Bulk Send defaults channel=email so we don't need to change it. Open
        // the user picker dialog (button "Select users") and pick all filtered.
        await page.getByRole('button', { name: /Select users|user(s)? selected/ }).click();
        await page.getByRole('button', { name: 'Select all (filtered)' }).click();
        await page.keyboard.press('Escape');

        // Verify multi-select reflected on the trigger button.
        await expect(
            page.getByRole('button', { name: /\d+ users? selected/ }),
        ).toBeVisible();

        // Pick the email template. Bulk tab Template trigger has no id, so
        // target by placeholder text — only one such combobox in this panel.
        await page.getByRole('combobox').filter({ hasText: 'Select a template' }).click();
        await page.getByRole('option', { name: state.emailTemplate!.name }).click();

        const body = await captureRequestBody(page, '/v1/notifications/bulk', async () => {
            await page
                .getByRole('button', { name: /^Send Notification$|^Schedule Notification$/ })
                .click();
        });

        expect(Array.isArray(body.user_ids)).toBe(true);
        expect((body.user_ids as string[]).length).toBeGreaterThanOrEqual(2);
        expect(body.template_id).toBeTruthy();
        expect(body.channel).toBe('email');
    });
});
