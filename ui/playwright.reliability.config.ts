import { defineConfig, devices } from '@playwright/test';

// API fixtures keep these UI compatibility cases independent of credentials and live sends.
export default defineConfig({
    testDir: '../e2e',
    grep: /Delivery reliability UI/,
    fullyParallel: true,
    workers: 2,
    timeout: 30_000,
    expect: { timeout: 5_000 },
    reporter: 'list',
    outputDir: './test-results/reliability',
    use: {
        baseURL: 'http://127.0.0.1:3104',
        ...devices['Desktop Chrome'],
        locale: 'en-US',
        timezoneId: 'UTC',
        screenshot: 'only-on-failure',
    },
    webServer: {
        command: 'npm --prefix ui run dev -- --host 127.0.0.1 --port 3104',
        cwd: '..',
        url: 'http://127.0.0.1:3104',
        reuseExistingServer: !process.env.CI,
    },
});
