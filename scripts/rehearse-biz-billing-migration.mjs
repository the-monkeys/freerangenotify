import { ownedEnvironment, execute, verifyGoEvents } from './verify-biz-billing.mjs';

// This runner never starts a cluster or reads production configuration. Its
// caller supplies the same owned, disposable fixture used by verification.
const { env, manifest } = await ownedEnvironment();
if (!manifest.disposable) throw new Error('Migration rehearsal requires a disposable fixture; saved walkthrough data is not eligible');
const repo = 'frn-billing-' + manifest.runID;
const es = manifest.services.elasticsearch.url;
const result = await fetch(`${es}/_snapshot/${repo}`, {
  method: 'PUT', headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ type: 'fs', settings: { location: `billing-${manifest.runID}`, compress: true } }),
  signal: AbortSignal.timeout(30000),
});
if (!result.ok) throw new Error('Owned snapshot repository unavailable; check fixture path.repo and snapshot volume permissions');
env.FRN_BIZ_SNAPSHOT_REPO = repo;
try {
  for (let run = 1; run <= 2; run++) {
    const result = await execute('go', ['test', '-json', '-p', '2', '-count=1', './tests/integration', './internal/usecases/services', '-run', '^(TestBizMigrationRehearsalPreservesHistory|TestBizMigrationRehearsalKeepsUnknownFields|TestBizMigrationInterruptedStagesPreserveSources|TestBizAmbiguousCurrencyNeedsReview|TestBizMigrationCurrencySidecarPreservesCreditSource|TestBizMigrationSubscriptionReviewPreservesDatesAndIDs)$'], { env, json: true });
    const summary = verifyGoEvents(result.events, ['TestBizMigrationRehearsalPreservesHistory', 'TestBizMigrationRehearsalKeepsUnknownFields', 'TestBizMigrationInterruptedStagesPreserveSources', 'TestBizAmbiguousCurrencyNeedsReview', 'TestBizMigrationCurrencySidecarPreservesCreditSource', 'TestBizMigrationSubscriptionReviewPreservesDatesAndIDs']);
    console.log(`Fresh synthetic migration/restore rehearsal ${run}:`, summary);
  }
} finally {
  // Unregister only this run's repository; its owned volume is cleaned by the
  // disposable fixture lifecycle, never by a global Docker prune.
  const result = await fetch(`${es}/_snapshot/${repo}`, { method: 'DELETE', signal: AbortSignal.timeout(30000) });
  if (!result.ok) throw new Error('Could not unregister owned snapshot repository');
}
