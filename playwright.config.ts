import { defineConfig, devices } from '@playwright/test';
import * as path from 'node:path';
import * as fs from 'node:fs';

// Load e2e/.env if present so credentials live outside source control.
const envPath = path.join(__dirname, 'e2e', '.env');
if (fs.existsSync(envPath)) {
    for (const line of fs.readFileSync(envPath, 'utf8').split(/\r?\n/)) {
        const trimmed = line.trim();
        if (!trimmed || trimmed.startsWith('#')) continue;
        const eq = trimmed.indexOf('=');
        if (eq === -1) continue;
        const key = trimmed.slice(0, eq).trim();
        const val = trimmed.slice(eq + 1).trim().replace(/^['"]|['"]$/g, '');
        if (!(key in process.env)) process.env[key] = val;
    }
}

const BASE_URL = process.env.E2E_BASE_URL || 'http://localhost:3000';

export default defineConfig({
    testDir: './e2e',
    timeout: 30_000,
    expect: { timeout: 10_000 },
    fullyParallel: false, // shared backend state — keep specs serial
    forbidOnly: !!process.env.CI,
    retries: process.env.CI ? 1 : 0,
    workers: 1,
    reporter: [['list'], ['html', { open: 'never', outputFolder: 'playwright-report' }]],
    globalSetup: require.resolve('./e2e/global-setup.ts'),
    use: {
        baseURL: BASE_URL,
        storageState: path.join(__dirname, 'e2e', '.state', 'storage.json'),
        trace: 'retain-on-failure',
        screenshot: 'only-on-failure',
        video: 'retain-on-failure',
        actionTimeout: 10_000,
        navigationTimeout: 15_000,
    },
    projects: [
        {
            name: 'chromium',
            use: { ...devices['Desktop Chrome'] },
        },
    ],
});
