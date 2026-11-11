import { mkdtempSync, writeFileSync, rmSync, readdirSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { spawnSync } from 'node:child_process';

const output = mkdtempSync(join(tmpdir(), 'frn-sdk-tests-'));
try {
  const compiler = spawnSync(process.execPath, ['node_modules/typescript/bin/tsc', '--module', 'commonjs', '--moduleResolution', 'node', '--outDir', output, '--declaration', 'false'], { stdio: 'inherit' });
  if (compiler.status !== 0) process.exitCode = compiler.status ?? 1;
  else {
    writeFileSync(join(output, 'package.json'), JSON.stringify({ type: 'commonjs' }));
    const tests = readdirSync('tests').filter(name => name.endsWith('.test.cjs')).map(name => resolve('tests', name));
    const result = spawnSync(process.execPath, ['--test', ...tests], { stdio: 'inherit', env: { ...process.env, FRN_SDK_TEST_OUTPUT: output } });
    process.exitCode = result.status ?? 1;
  }
} finally { rmSync(output, { recursive: true, force: true }); }
