import { spawn } from 'node:child_process';
import { readFile, mkdir, writeFile } from 'node:fs/promises';
import { resolve, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { childEnvironment, assertLocalDocker, validateManifest, sanitizeOutput } from './biz-billing-fixtures.mjs';
export { sanitizeOutput } from './biz-billing-fixtures.mjs';

const root = resolve(fileURLToPath(new URL('..', import.meta.url)));
export const requiredCases = [
  'TestBizBillingE2ESuite',
  'TestBizBillingV2E2E', 'TestBizBillingRequiredSuiteCannotSkip',
  'TestBizWhatsAppBroadcastAndReminderScope', 'TestBizWorkerConfiguredConcurrency',
  'TestBizBillingGuideContracts', 'TestBizMigrationResumePreservesSources',
  'TestBizMigrationInterruptedStagesPreserveSources', 'TestBizOutboxAdmissionRecovery',
  'TestBizNotificationFailureDoesNotUndoPayment', 'TestBizPreviewAndIssuedDocumentHTTP',
  'TestBizPortalSecretsNeverLogged', 'TestBizPortalPrivacyRateLimitAndUnknownRoute',
  'TestBizPortalExpiredAfterCheckoutCapture', 'TestBizPortalCrossCustomerAndApp',
  'TestBizReviewDefinitiveModeMismatchDoesNotBlockStream',
  'TestBizReviewRecurringRotationProvesNewKeyBeforeRetiredRead',
  'TestBizReviewRecurringReceiptsRemainListable',
  'TestBizReviewInvoiceResendIsDurableScopedAndIdempotent',
  'TestBizReviewOverdueScanReachesInvoicesBeyondFirstPage',
  'TestBizReviewChangedBalanceReconcilesObsoleteCheckout',
  'TestBizReviewUnknownOldPaymentNeverStartsReplacementOrder',
  'TestBizReviewCreditNoteKeepsReferencedSandboxScope',
  'TestBizReviewSandboxSubscriptionsDoNotInflateReports',
  'TestBizObsoleteOrderRequiresPositiveUnpaidProof',
  'TestBizRecurringProofUsesExactSuccessorAndRetainsBinding',
];

export function verifyGoEvents(events, required = requiredCases) {
  const passed = new Set();
  let skipped = 0;
  for (const event of events) {
    if (event.Action === 'fail') throw new Error('Billing test or package failed');
    if (event.Action === 'skip' && event.Test) skipped++;
    if (event.Action === 'pass' && event.Test) passed.add(event.Test);
  }
  if (skipped) throw new Error(`Required billing selection skipped ${skipped} tests`);
  for (const name of required) if (!passed.has(name)) throw new Error(`Missing required billing case: ${name}`);
  return { passed: passed.size, skipped };
}

export function fixtureAPIAuth(source, manifest) {
  const secret = source.FREERANGE_SECURITY_JWT_SECRET;
  if (manifest.disposable !== true || !/^[a-f0-9]{32}$/.test(manifest.runID ?? '') || source.FRN_BIZ_FIXTURE_ID !== manifest.runID || !/^[a-f0-9]{64}$/.test(secret ?? '') || source.JWT_SECRET !== secret || !source.INTEGRATION_ADMIN_TOKEN) {
    throw new Error('Owned disposable fixture signing credentials missing or mismatched');
  }
  return { FRN_BIZ_FIXTURE_ID: manifest.runID, FREERANGE_SECURITY_JWT_SECRET: secret, JWT_SECRET: secret, INTEGRATION_ADMIN_TOKEN: source.INTEGRATION_ADMIN_TOKEN };
}

export async function ownedEnvironment(source = process.env) {
  if (source.FRN_BIZ_REQUIRED !== '1' || source.FREERANGE_FEATURES_BIZ_BILLING_ENABLED !== 'true' || !source.FRN_BIZ_FIXTURE_MANIFEST) {
    throw new Error('Required billing suite needs an owned fixture and billing explicitly enabled');
  }
  const env = { ...childEnvironment(source), FRN_BIZ_REQUIRED: '1', FREERANGE_FEATURES_BIZ_BILLING_ENABLED: 'true', FREERANGE_OTEL_ENABLED: 'false', FRN_DOCS_TEST: '1', VITE_API_BASE_URL: '/v1' };
  const manifest = JSON.parse(await readFile(source.FRN_BIZ_FIXTURE_MANIFEST, 'utf8'));
  if (manifest.disposable !== true) throw new Error('Required billing verification needs a disposable fixture; saved walkthrough data is not eligible');
  const capture = async args => (await execute('docker', args, { env, capture: true })).output.trim();
  assertLocalDocker(env.DOCKER_HOST || await capture(['context', 'inspect', '--format', '{{.Endpoints.docker.Host}}']));
  const inspections = JSON.parse(await capture(['inspect', ...Object.values(manifest.services).map(x => x.id)]));
  validateManifest(manifest, inspections);
  Object.assign(env, fixtureAPIAuth(source, manifest));
  if (!manifest.apiURL || !/^http:\/\/127\.0\.0\.1:[0-9]+$/.test(manifest.apiURL) || !source.INTEGRATION_ADMIN_TOKEN) throw new Error('Required billing HTTP fixture credentials missing');
  const selectedES = manifest.services.elasticsearch.url;
  if (source.FRN_BIZ_TEST_ES_URL !== selectedES || source.INTEGRATION_ES_URL !== selectedES || source.INTEGRATION_BASE_URL !== manifest.apiURL) throw new Error('Required billing fixture endpoint mismatch');
  Object.assign(env, {
    FRN_BIZ_FIXTURE_MANIFEST: source.FRN_BIZ_FIXTURE_MANIFEST,
    FRN_BIZ_TEST_ES_URL: selectedES, INTEGRATION_ES_URL: selectedES,
    FRN_BIZ_TEST_REDIS_ADDR: new URL(manifest.services.redis.url).host,
    INTEGRATION_BASE_URL: manifest.apiURL, INTEGRATION_ADMIN_TOKEN: source.INTEGRATION_ADMIN_TOKEN,
  });
  return { env, manifest };
}

export function execute(command, args, { env, cwd = root, capture = false, json = false } = {}) {
  return new Promise((ok, bad) => {
    const child = spawn(command, args, { env, cwd, shell: false, windowsHide: true, stdio: ['ignore', 'pipe', 'pipe'] });
    let output = '', pending = '';
    const events = [];
    const redact = text => sanitizeOutput(text, Object.entries(env ?? {}).filter(([key]) => /secret|token|encryption/i.test(key)).map(([, value]) => value));
    child.stdout.on('data', chunk => {
      const text = chunk.toString();
      if (json) {
        pending += text;
        const lines = pending.split('\n'); pending = lines.pop();
        for (const line of lines) {
          try { const event = JSON.parse(line); events.push(event); if (event.Output) process.stdout.write(redact(event.Output)); }
          catch { if (line.trim()) process.stdout.write(redact(line+'\n')); }
        }
      } else if (capture) output += text;
      else process.stdout.write(redact(text));
    });
    child.stderr.on('data', text => { if (capture) output += text; else process.stderr.write(redact(text.toString())); });
    child.on('error', bad);
    child.on('exit', code => code === 0 ? ok({ output, events }) : bad(new Error(`${command} exited ${code}`)));
  });
}

export async function verify(source = process.env) {
  const { env } = await ownedEnvironment(source);
  // Run the historical index-clearing suite before new facts are created, and
  // only after the disposable fixture guard. Persistent walkthroughs are refused.
  const legacy = await execute('go', ['test', '-json', '-p', '2', '-count=1', './tests/integration', '-run', '^TestBizBillingE2ESuite$'], { env, json: true });
  verifyGoEvents(legacy.events, ['TestBizBillingE2ESuite']);
  const selection = '^TestBiz';
  const packages = ['./internal/domain/bizbilling', './internal/infrastructure/bizbillingrepo', './internal/infrastructure/payment', './internal/usecases/services', './internal/interfaces/http/handlers', './internal/interfaces/http/middleware', './cmd/worker', './tests/contracts'];
  const domain = await execute('go', ['test', '-json', '-p', '2', '-count=1', '-timeout', '25m', ...packages, '-run', selection], { env, json: true });
  // These E2Es create app-unique records and exercise legacy and v2 concurrently.
  const http = await execute('go', ['test', '-json', '-p', '2', '-count=1', './tests/integration', '-run', '^(TestBizBillingV2E2E|TestBizBillingRequiredSuiteCannotSkip|TestBizWhatsAppBroadcastAndReminderScope)$'], { env, json: true });
  const summary = verifyGoEvents([...legacy.events, ...domain.events, ...http.events]);
  await execute(process.execPath, ['scripts/rehearse-biz-billing-migration.mjs'], { env });
  if (process.platform === 'linux') {
    await execute('go', ['test', '-race', '-p', '2', '-count=1', './internal/domain/bizbilling', './internal/infrastructure/bizbillingrepo', './internal/infrastructure/payment', './internal/usecases/services', './internal/interfaces/http/handlers', './cmd/worker', '-run', '^TestBiz'], { env: { ...env, CGO_ENABLED: '1' } });
  }
  await execute(process.execPath, ['scripts/verify-docs.mjs'], { env });
  for (const config of ['e2e/biz-billing.config.ts', 'e2e/docs.config.ts', 'e2e/mocked.config.ts', 'e2e/reliability.config.ts']) await execute(process.execPath, ['node_modules/@playwright/test/cli.js', 'test', '-c', config], { env });
  await execute(process.execPath, ['node_modules/@playwright/test/cli.js', 'test', '-c', 'e2e/mocked.config.ts', 'whatsapp-runtime-config.spec.ts', '--grep', 'legacy public signup'], { env: { ...env, FRN_CHANNEL_LEGACY_IDS: '1' } });
  await execute(process.execPath, ['scripts/test.mjs'], { env, cwd: join(root, 'sdk/js') });
  await execute(process.execPath, ['node_modules/typescript/bin/tsc', '--noEmit'], { env, cwd: join(root, 'sdk/react') });
  await execute(process.execPath, ['ui/node_modules/typescript/bin/tsc', '--noEmit', '-p', 'ui/tsconfig.json'], { env });
  await execute(process.execPath, ['ui/node_modules/vite/bin/vite.js', 'build', 'ui'], { env });
  const out = resolve(root, 'test-results/biz-billing-verification');
  await mkdir(out, { recursive: true });
  await writeFile(join(out, 'summary.json'), JSON.stringify(summary, null, 2)+'\n');
  console.log('Required business billing verification passed:', summary);
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  verify().catch(error => { console.error(error.message); process.exitCode = 1; });
}
