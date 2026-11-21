import { execFileSync } from 'node:child_process';
import { mkdtempSync, readFileSync, realpathSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve, sep } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = realpathSync(resolve(dirname(fileURLToPath(import.meta.url)), '..'));
const outputDir = realpathSync(process.env.RUNNER_TEMP || tmpdir());
const mode = process.env.TRAINING_MODE || 'false';

try {
  const url = new URL(process.env.APP_URL);
  if (!['http:', 'https:'].includes(url.protocol) || !url.hostname || url.username || url.password) {
    throw new Error('APP_URL must be an HTTP(S) URL without credentials');
  }
  if (mode !== 'true' && mode !== 'false') throw new Error('TRAINING_MODE must be true or false');
  if (outputDir === root || outputDir.startsWith(root + sep)) {
    throw new Error('RUNNER_TEMP must be outside the working tree');
  }

  const workDir = mkdtempSync(join(outputDir, 'lighthouse-audit-'));
  const reportPath = join(workDir, 'report.json');
  try {
    if (mode === 'true') {
      writeFileSync(reportPath, readFileSync(join(root, 'scripts', 'lighthouse-training.json')));
    } else {
      execFileSync('npx', ['--yes', 'lighthouse@13.5.0', url.href, '--output=json', `--output-path=${reportPath}`, '--chrome-flags=--headless'], { stdio: ['ignore', process.stderr, process.stderr] });
    }
    const report = JSON.parse(readFileSync(reportPath, 'utf8'));
    if (report.runtimeError || !report.audits || !report.categories) {
      throw new Error('Lighthouse did not produce a successful audit');
    }
    console.log(reportPath);
  } catch (error) {
    rmSync(workDir, { recursive: true, force: true });
    throw error;
  }
} catch (error) {
  console.error(`Lighthouse collection failed: ${error.message}`);
  process.exitCode = 1;
}
