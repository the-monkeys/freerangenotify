import path from 'node:path';
import {defineConfig} from '@playwright/test';
export default defineConfig({testDir:'.',testMatch:['subscriber-import.spec.ts','exotel-connections.spec.ts','whatsapp-runtime-config.spec.ts','whatsapp-sms-*.spec.ts'],globalSetup:undefined,outputDir:'../test-results/mocked',timeout:30000,workers:2,use:{baseURL:'http://127.0.0.1:3011',storageState:undefined,browserName:'chromium'},webServer:{cwd:path.resolve(__dirname,'..'),command:'node scripts/docs-preview.mjs',url:'http://127.0.0.1:3011',reuseExistingServer:false,timeout:180000}});
