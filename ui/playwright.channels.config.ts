import { defineConfig, devices } from '@playwright/test';
export default defineConfig({
 testDir:'../e2e',grep:/WhatsApp SMS connections/,workers:1,timeout:30000,
 reporter:'list',outputDir:'./test-results/channels',
 expect:{timeout:15000},
 use:{baseURL:'http://127.0.0.1:3105',...devices['Desktop Chrome'],timezoneId:'UTC',screenshot:'only-on-failure'},
 webServer:{command:'npm --prefix ui run dev -- --host 127.0.0.1 --port 3105',cwd:'..',url:'http://127.0.0.1:3105',reuseExistingServer:!process.env.CI},
});
