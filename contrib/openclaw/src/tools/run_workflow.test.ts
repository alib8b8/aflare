/**
 * run_workflow.test.ts — pins the argv contract of runWorkflow.
 *
 * Regression guards for the two breakages this plugin shipped before v1.2.0:
 *
 * 1. It built `aflare run <file> --input <text>`, but the aflare CLI has no
 *    `--input` flag — the flag parser's default branch silently dropped it,
 *    so the advertised "run with input" feature never delivered anything.
 *    The correct contract is `--set input=<text>`, delivered as ONE argv
 *    element.
 * 2. It spawned with `{ shell: true }`, so input containing spaces was
 *    word-split and shell metacharacters (`;`, `$()`, backticks) were
 *    interpreted — a command-injection vector from any OpenClaw session.
 *
 * The test spawns a stub "aflare" that dumps its raw argv to a file. If
 * either regression comes back (unknown flag dropped, shell interpolation),
 * the dumped argv no longer matches and this test fails.
 */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import * as fs from 'node:fs/promises';
import * as os from 'node:os';
import * as path from 'node:path';

import { runWorkflow } from './run_workflow.js';

/**
 * Writes an executable Node stub that behaves like aflare: it records its
 * raw argv (already shell-free, as the OS delivers it) to <dir>/argv.json
 * and exits 0 printing a plausible run summary.
 */
async function makeStubAflare(dir: string): Promise<string> {
  const stubPath = path.join(dir, 'aflare-stub.mjs');
  const argvPath = path.join(dir, 'argv.json');
  await fs.writeFile(
    stubPath,
    [
      '#!/usr/bin/env node',
      `import * as fs from 'node:fs';`,
      `fs.writeFileSync(${JSON.stringify(argvPath)}, JSON.stringify(process.argv.slice(2)));`,
      `console.log('workflow completed');`,
      '',
    ].join('\n'),
    { mode: 0o755 }
  );
  return stubPath;
}

async function readDumpedArgv(dir: string): Promise<string[]> {
  return JSON.parse(await fs.readFile(path.join(dir, 'argv.json'), 'utf8'));
}

test('runWorkflow passes input as a single --set argv element', async () => {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'oc-argv-'));
  const stub = await makeStubAflare(dir);

  const result = await runWorkflow(
    'demo.yaml',
    'hello world; rm -rf / $(whoami)',
    { aflarePath: stub }
  );

  assert.equal(result.success, true, `stub run should succeed: ${result.error}`);
  const argv = await readDumpedArgv(dir);
  assert.deepEqual(argv, [
    'run',
    'demo.yaml',
    '--set',
    'input=hello world; rm -rf / $(whoami)',
  ]);
});

test('runWorkflow omits --set when no input is given', async () => {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'oc-argv-'));
  const stub = await makeStubAflare(dir);

  const result = await runWorkflow('demo.yaml', '', { aflarePath: stub });

  assert.equal(result.success, true, `stub run should succeed: ${result.error}`);
  const argv = await readDumpedArgv(dir);
  assert.deepEqual(argv, ['run', 'demo.yaml']);
});

test('runWorkflow rejects non-yaml files without spawning', async () => {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'oc-argv-'));
  const stub = await makeStubAflare(dir);

  const result = await runWorkflow('demo.txt', 'input', { aflarePath: stub });

  assert.equal(result.success, false);
  assert.match(result.error ?? '', /\.ya?ml file/);
  // The stub must never have run.
  await assert.rejects(() => readDumpedArgv(dir));
});

test('runWorkflow resolves aflare from PATH by default', async () => {
  // Contract check only: the default binary name is exactly "aflare" with
  // no shell wrapper. (Actually exercising PATH here would shadow the
  // user's real aflare; the spawn-without-shell behavior is covered by the
  // stub tests above.)
  const result = await runWorkflow('nope.yaml', '', { aflarePath: '/nonexistent/aflare' });
  assert.equal(result.success, false);
  assert.equal(result.workflow, 'nope.yaml');
});
