import { test } from 'node:test';
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdtempSync, writeFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
const verifier = await import('./verify-biz-billing.mjs').catch(e => e.code === 'ERR_MODULE_NOT_FOUND' ? {} : Promise.reject(e));

test('required billing verification fails closed before launching without explicit fixture flags', () => {
  const result = spawnSync(process.execPath, ['scripts/verify-biz-billing.mjs'], { env: { ...process.env, FRN_BIZ_REQUIRED: '', FREERANGE_FEATURES_BIZ_BILLING_ENABLED: '', FRN_BIZ_FIXTURE_MANIFEST: '' }, encoding: 'utf8' });
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /required|fixture|enabled/i);
});

test('saved walkthrough manifests are rejected before Docker or API access', () => {
  const directory = mkdtempSync(join(tmpdir(), 'frn-biz-admission-'));
  try {
    const file = join(directory, 'manifest.json');
    writeFileSync(file, JSON.stringify({ disposable: false }));
    const result = spawnSync(process.execPath, ['scripts/verify-biz-billing.mjs'], { env: { ...process.env, FRN_BIZ_REQUIRED: '1', FREERANGE_FEATURES_BIZ_BILLING_ENABLED: 'true', FRN_BIZ_FIXTURE_MANIFEST: file }, encoding: 'utf8' });
    assert.notEqual(result.status, 0);
    assert.match(result.stderr, /disposable/i);
  } finally {
    assert.ok(resolve(directory).startsWith(resolve(tmpdir(), 'frn-biz-admission-')));
    rmSync(directory, { recursive: true });
  }
});

test('a required test cannot disappear, skip, or pass underneath a failed package', () => {
  assert.equal(typeof verifier.verifyGoEvents, 'function');
  const pass = [{ Action: 'pass', Package: 'suite', Test: 'TestRequired' }, { Action: 'pass', Package: 'suite' }];
  assert.deepEqual(verifier.verifyGoEvents(pass, ['TestRequired']), { passed: 1, skipped: 0 });
  assert.throws(() => verifier.verifyGoEvents([], ['TestRequired']), /missing/i);
  assert.throws(() => verifier.verifyGoEvents([{ Action: 'skip', Test: 'TestRequired' }], ['TestRequired']), /skip/i);
  assert.throws(() => verifier.verifyGoEvents([...pass, { Action: 'fail', Package: 'suite' }], ['TestRequired']), /fail/i);
});

test('failure artifacts remove private links and supplied fixture credentials', () => {
  assert.equal(typeof verifier.sanitizeOutput, 'function');
  const raw = 'http://127.0.0.1:12345/v1/portal/pt2_private/invoices?x=1 token-fixture raw=pt2_new_private {"token":"legacy-private","key_secret":"account-private"}';
  const safe = verifier.sanitizeOutput(raw, ['token-fixture']);
  assert.ok(!safe.includes('pt2_private'));
  assert.ok(!safe.includes('token-fixture'));
  assert.ok(!safe.includes('pt2_new_private'));
  assert.ok(!safe.includes('legacy-private'));
  assert.ok(!safe.includes('account-private'));
});

test('owned fixture API tests retain generated signing keys but no customer environment', () => {
  assert.equal(typeof verifier.fixtureAPIAuth, 'function');
  const runID = 'a'.repeat(32), secret = 'b'.repeat(64);
  const source = { FRN_BIZ_FIXTURE_ID: runID, FREERANGE_SECURITY_JWT_SECRET: secret, JWT_SECRET: secret, INTEGRATION_ADMIN_TOKEN: 'fixture-token', CUSTOMER_SECRET: 'never-copy' };
  const manifest = { disposable: true, runID };
  const copied = verifier.fixtureAPIAuth(source, manifest);
  assert.deepEqual(copied, { FRN_BIZ_FIXTURE_ID: runID, FREERANGE_SECURITY_JWT_SECRET: secret, JWT_SECRET: secret, INTEGRATION_ADMIN_TOKEN: 'fixture-token' });
  assert.deepEqual(verifier.fixtureAPIAuth(copied, manifest), copied, 'nested rehearsal keeps exactly the owned fixture credentials');
  assert.throws(() => verifier.fixtureAPIAuth(source, { ...manifest, disposable: false }), /fixture/i);
  assert.throws(() => verifier.fixtureAPIAuth({ ...source, FREERANGE_SECURITY_JWT_SECRET: '' }, manifest), /signing|fixture/i);
  assert.throws(() => verifier.fixtureAPIAuth({ ...source, FRN_BIZ_FIXTURE_ID: 'other' }, manifest), /fixture/i);
});
