import { defineConfig } from '@playwright/test';
import mocked from './mocked.config';

export default defineConfig({
  ...mocked,
  testMatch: ['quick-send.spec.ts'],
  grep: /Delivery reliability UI/,
  use: { ...mocked.use, timezoneId: 'UTC' },
  outputDir: '../test-results/reliability',
});
