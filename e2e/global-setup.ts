/**
 * Playwright globalSetup: authenticates the test user, picks a target app,
 * fetches its API key, ensures basic resources (email template, webhook
 * custom provider, two users) are present, and writes:
 *
 *   tests/e2e/.state/storage.json — Playwright storageState (localStorage tokens)
 *   tests/e2e/.state/state.json   — { appId, apiKey, templates, providers, users }
 *
 * Specs read state.json via fixtures.ts. Run against a live local stack
 * (docker-compose up). Configure credentials in tests/e2e/.env (see .env.example).
 */
import { chromium, request, type APIRequestContext, type FullConfig } from '@playwright/test';
import * as fs from 'node:fs';
import * as path from 'node:path';

const STATE_DIR = path.join(__dirname, '.state');
const STORAGE_PATH = path.join(STATE_DIR, 'storage.json');
const STATE_PATH = path.join(STATE_DIR, 'state.json');

export interface E2EState {
    baseURL: string;
    apiBaseURL: string;
    appId: string;
    apiKey: string;
    accessToken: string;
    refreshToken: string;
    emailTemplate: { id: string; name: string } | null;
    webhookTemplate: { id: string; name: string } | null;
    webhookProvider: { id: string; name: string } | null;
    users: { id: string; email: string }[];
}

function requireEnv(key: string): string {
    const v = process.env[key];
    if (!v) {
        throw new Error(
            `[e2e] Missing required env var ${key}. Copy tests/e2e/.env.example to tests/e2e/.env and fill in credentials.`,
        );
    }
    return v;
}

async function login(api: APIRequestContext, email: string, password: string) {
    const res = await api.post('auth/login', { data: { email, password } });
    if (!res.ok()) {
        throw new Error(`[e2e] /auth/login failed: ${res.status()} ${await res.text()}`);
    }
    const body = await res.json();
    if (!body.access_token || !body.refresh_token) {
        throw new Error(`[e2e] /auth/login response missing tokens: ${JSON.stringify(body)}`);
    }
    return {
        accessToken: body.access_token as string,
        refreshToken: body.refresh_token as string,
        user: body.user,
    };
}

async function pickApp(api: APIRequestContext, accessToken: string, preferredId?: string) {
    const res = await api.get('apps/', { headers: { Authorization: `Bearer ${accessToken}` } });
    if (!res.ok()) throw new Error(`[e2e] /apps/ failed: ${res.status()} ${await res.text()}`);
    const body = await res.json();
    const apps: any[] = body?.data?.applications || body?.applications || body?.data || [];
    if (!Array.isArray(apps) || apps.length === 0) {
        throw new Error('[e2e] No apps found for test user. Create one in the UI first.');
    }
    // The list endpoint masks `api_key`. Specs call getApiKey() to fetch the
    // real key from the detail endpoint. The id field is `app_id` (not `id`).
    const normalize = (a: any) => ({ ...a, id: a.app_id || a.id });
    if (preferredId) {
        const match = apps.find((a) => (a.app_id || a.id) === preferredId);
        if (!match) throw new Error(`[e2e] E2E_APP_ID=${preferredId} not found among user's apps.`);
        return normalize(match);
    }
    return normalize(apps[0]);
}

async function getApiKey(api: APIRequestContext, accessToken: string, appId: string): Promise<string> {
    const res = await api.get(`apps/${appId}`, { headers: { Authorization: `Bearer ${accessToken}` } });
    if (!res.ok()) throw new Error(`[e2e] /apps/${appId} failed: ${res.status()} ${await res.text()}`);
    const body = await res.json();
    const app = body?.data || body;
    const key = app?.api_key || app?.apiKey;
    if (!key) throw new Error(`[e2e] App ${appId} has no api_key field. Regenerate it in the UI.`);
    return key as string;
}

async function listTemplates(api: APIRequestContext, apiKey: string) {
    const res = await api.get('templates/', { headers: { 'X-API-Key': apiKey } });
    if (!res.ok()) {
        // Try fallback shape
        const res2 = await api.get('templates', { headers: { 'X-API-Key': apiKey } });
        if (!res2.ok()) throw new Error(`[e2e] /templates failed: ${res2.status()} ${await res2.text()}`);
        const body2 = await res2.json();
        return (body2?.templates || body2?.data?.templates || []) as any[];
    }
    const body = await res.json();
    return (body?.templates || body?.data?.templates || []) as any[];
}

async function listProviders(api: APIRequestContext, accessToken: string, appId: string) {
    // Custom providers are nested under app.settings.custom_providers on the
    // detail endpoint. The dedicated /apps/${id}/providers route returns the
    // same data; either source is acceptable. We use settings.custom_providers
    // because it's already fetched by getApiKey's caller path conceptually,
    // but we keep this as a separate call so the function is reusable.
    const res = await api.get(`apps/${appId}`, {
        headers: { Authorization: `Bearer ${accessToken}` },
    });
    if (!res.ok()) return [];
    const body = await res.json();
    const providers = body?.data?.settings?.custom_providers || body?.settings?.custom_providers || [];
    return (providers as any[]).map((p) => ({ ...p, id: p.provider_id || p.id }));
}

async function listUsers(api: APIRequestContext, apiKey: string) {
    const res = await api.get('users/', { headers: { 'X-API-Key': apiKey } });
    if (!res.ok()) return [];
    const body = await res.json();
    const users = body?.data?.users || body?.users || body?.data || [];
    return (users as any[]).map((u) => ({ ...u, id: u.user_id || u.id }));
}

async function ensureUsers(api: APIRequestContext, apiKey: string, existing: any[]) {
    const have = existing.length;
    const need = Math.max(0, 2 - have);
    const created: any[] = [];
    for (let i = 0; i < need; i++) {
        const stamp = Date.now();
        const payload = {
            external_id: `e2e-user-${stamp}-${i}`,
            email: `e2e-user-${stamp}-${i}@example.test`,
            full_name: `E2E User ${i}`,
        };
        const res = await api.post('users/', { headers: { 'X-API-Key': apiKey }, data: payload });
        if (res.ok()) {
            const body = await res.json();
            const u = body?.data || body;
            created.push({ ...u, id: u.user_id || u.id });
        } else {
            // eslint-disable-next-line no-console
            console.warn(`[e2e] failed to create test user: ${res.status()} ${await res.text()}`);
        }
    }
    return [...existing, ...created];
}

export default async function globalSetup(_config: FullConfig) {
    const baseURL = process.env.E2E_BASE_URL || 'http://localhost:3000';
    const apiBaseURL = process.env.E2E_API_BASE_URL || 'http://localhost:8080/v1';
    const apiBaseURLWithSlash = apiBaseURL.endsWith('/') ? apiBaseURL : apiBaseURL + '/';
    const email = requireEnv('E2E_USER_EMAIL');
    const password = requireEnv('E2E_USER_PASSWORD');

    fs.mkdirSync(STATE_DIR, { recursive: true });

    const api = await request.newContext({ baseURL: apiBaseURLWithSlash });
    try {
        const { accessToken, refreshToken } = await login(api, email, password);
        const app = await pickApp(api, accessToken, process.env.E2E_APP_ID);
        const apiKey = await getApiKey(api, accessToken, app.id);

        const templates = await listTemplates(api, apiKey);
        const emailTemplate = templates.find((t) => t.channel === 'email') || null;
        const webhookTemplate =
            templates.find((t) => t.channel === 'webhook' && /rich|alert/i.test(t.name)) ||
            templates.find((t) => t.channel === 'webhook') ||
            null;

        const providers = await listProviders(api, accessToken, app.id);
        const webhookProvider =
            providers.find((p) => p.channel === 'webhook' && p.active) ||
            providers.find((p) => p.channel === 'webhook') ||
            null;

        const existingUsers = await listUsers(api, apiKey);
        const users = await ensureUsers(api, apiKey, existingUsers);

        const state: E2EState = {
            baseURL,
            apiBaseURL,
            appId: app.id,
            apiKey,
            accessToken,
            refreshToken,
            emailTemplate: emailTemplate ? { id: emailTemplate.id, name: emailTemplate.name } : null,
            webhookTemplate: webhookTemplate ? { id: webhookTemplate.id, name: webhookTemplate.name } : null,
            webhookProvider: webhookProvider
                ? { id: webhookProvider.id, name: webhookProvider.name }
                : null,
            users: users.slice(0, 5).map((u: any) => ({ id: u.id, email: u.email })),
        };

        fs.writeFileSync(STATE_PATH, JSON.stringify(state, null, 2), 'utf8');

        // Build storageState by visiting the UI and seeding localStorage so
        // ProtectedRoute lets us through without going via the login form.
        const browser = await chromium.launch();
        const ctx = await browser.newContext();
        const page = await ctx.newPage();
        await page.goto(baseURL, { waitUntil: 'domcontentloaded' });
        await page.evaluate(
            ({ at, rt }) => {
                window.localStorage.setItem('access_token', at);
                window.localStorage.setItem('refresh_token', rt);
            },
            { at: accessToken, rt: refreshToken },
        );
        await ctx.storageState({ path: STORAGE_PATH });
        await browser.close();

        // eslint-disable-next-line no-console
        console.log(
            `[e2e] globalSetup OK — app=${app.id} apiKey=***${apiKey.slice(-6)} ` +
            `emailTpl=${emailTemplate?.name || 'NONE'} webhookTpl=${webhookTemplate?.name || 'NONE'} ` +
            `webhookProvider=${webhookProvider?.name || 'NONE'} users=${state.users.length}`,
        );
    } finally {
        await api.dispose();
    }
}
