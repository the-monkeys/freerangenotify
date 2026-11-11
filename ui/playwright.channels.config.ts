import { defineConfig, devices } from '@playwright/test';
export default defineConfig({
 testDir:'../e2e',grep:/WhatsApp SMS connections/,workers:1,timeout:30000,
 reporter:'list',outputDir:'./test-results/channels',
 expect:{timeout:15000},
 use:{baseURL:'http://127.0.0.1:3105',...devices['Desktop Chrome'],timezoneId:'UTC',screenshot:'only-on-failure'},
 webServer:{command:process.env.FRN_CHANNEL_PREVIEW === '1' ? 'npm --prefix ui run preview -- --host 127.0.0.1 --port 3105' : 'npm --prefix ui run dev -- --host 127.0.0.1 --port 3105',cwd:'..',url:'http://127.0.0.1:3105',reuseExistingServer:!process.env.CI&&process.env.FRN_CHANNEL_PREVIEW!=='1'&&process.env.FRN_CHANNEL_LEGACY_IDS!=='1',env:{VITE_META_APP_ID:'',VITE_META_CONFIG_ID:process.env.FRN_CHANNEL_LEGACY_IDS==='1'?'legacy-public-signup':''}},
});
