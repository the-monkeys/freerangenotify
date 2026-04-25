/**
 * Test fixtures: load state.json produced by global-setup and expose helpers
 * for navigating to the app's notifications page and intercepting outbound
 * send requests so specs can assert payload shape.
 */
import { test as base, expect, type Page, type Request } from '@playwright/test';
import * as fs from 'node:fs';
import * as path from 'node:path';
import type { E2EState } from './global-setup';

const STATE_PATH = path.join(__dirname, '.state', 'state.json');

function loadState(): E2EState {
    if (!fs.existsSync(STATE_PATH)) {
        throw new Error(`[e2e] state.json missing — globalSetup did not run. Path: ${STATE_PATH}`);
    }
    return JSON.parse(fs.readFileSync(STATE_PATH, 'utf8')) as E2EState;
}

interface Fixtures {
    state: E2EState;
    appNotificationsPage: Page;
}

export const test = base.extend<Fixtures>({
    state: async ({ }, use) => {
        await use(loadState());
    },

    /** Navigates to the app detail page Notifications tab. */
    appNotificationsPage: async ({ page, state }, use) => {
        await page.goto(`/apps/${state.appId}?tab=notifications`);
        // Wait for the Notifications panel's "Create Notification" toggle to
        // appear so the rest of the spec can rely on a stable starting state.
        await page
            .getByRole('button', { name: /Create Notification|Hide Send Form/ })
            .waitFor({ state: 'visible', timeout: 15_000 });
        await use(page);
    },
});

/**
 * Captures the next outbound JSON request to the given URL substring made
 * inside `action`. Returns the parsed body so specs can assert shape.
 *
 * Example:
 *   const body = await captureRequestBody(page, '/v1/quick-send', () => clickSend());
 *   expect(body.webhook_url).toBe('https://hooks.slack.com/...');
 */
export async function captureRequestBody(
    page: Page,
    urlSubstring: string,
    action: () => Promise<unknown>,
): Promise<Record<string, unknown>> {
    const reqPromise = page.waitForRequest(
        (req: Request) => req.url().includes(urlSubstring) && req.method() === 'POST',
        { timeout: 15_000 },
    );
    await action();
    const req = await reqPromise;
    const raw = req.postData();
    if (!raw) throw new Error(`[e2e] Request to ${urlSubstring} had no body`);
    return JSON.parse(raw) as Record<string, unknown>;
}

export { expect };

/**
 * Opens the "Send Form" container (toggle button) and switches to the named
 * tab. The form is hidden by default behind a "Create Notification" button.
 */
export async function openTab(page: Page, tab: 'Quick Send' | 'Bulk Send' | 'Broadcast') {
    const toggle = page.getByRole('button', { name: /Create Notification|Hide Send Form/ });
    const text = (await toggle.textContent()) || '';
    if (text.includes('Create')) await toggle.click();
    await page.getByRole('tab', { name: tab }).click();
}
