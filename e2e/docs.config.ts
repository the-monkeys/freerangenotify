import path from 'node:path';
import { defineConfig } from '@playwright/test';
export default defineConfig({testDir:'./docs',globalSetup:undefined,outputDir:'../test-results/docs',timeout:30000,workers:2,use:{baseURL:'http://127.0.0.1:3011',storageState:undefined,browserName:'chromium'},webServer:{cwd:path.resolve(__dirname,'..'),command:'node scripts/docs-preview.mjs',url:'http://127.0.0.1:3011',reuseExistingServer:false,timeout:180000}});
