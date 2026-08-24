// Minimal integration tests for the aflare DSH plugin.
//
// `npm test` compiles src/ to dist/ first, then runs the compiled tests with
// the built-in Node test runner. Tests that execute the real `aflare` binary
// resolve it from AFLARE_BIN or the repo-root build output (`make build` in
// the repository root) and skip gracefully when neither exists, so an
// npm-only consumer can still run `npm test` for the registration checks.

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { existsSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join, resolve } from 'node:path'
import type { Context } from '@deepseek-ai/cordis'
import type { ToolDefinition, ToolRunContext } from '@deepseek-ai/dsh-tools'
import { apply } from './index.js'

function findAflareBin(): string | undefined {
  const candidates = [
    process.env.AFLARE_BIN,
    resolve(process.cwd(), '../../aflare'), // repo-root `make build` output
  ].filter((p): p is string => !!p)
  return candidates.find((p) => existsSync(p))
}

function registerAll(): ToolDefinition[] {
  const defs: ToolDefinition[] = []
  const ctx = {
    tools: {
      register(d: ToolDefinition) {
        defs.push(d)
        return () => {}
      },
    },
  }
  apply(ctx as unknown as Context)
  return defs
}

function fakeExec(): ToolRunContext {
  return {
    callId: 'test',
    rootCallId: 'test',
    name: 'test',
    arguments: {},
    signal: AbortSignal.timeout(120_000),
    deferContext() {},
    concludeTurn() {},
  } as unknown as ToolRunContext
}

test('apply registers the four aflare tools', () => {
  const defs = registerAll()
  assert.deepEqual(
    defs.map((d) => d.name),
    ['aflare_version', 'aflare_generate', 'aflare_validate', 'aflare_run'],
  )
  for (const d of defs) {
    assert.ok(d.description.length > 20, `${d.name} needs a model-facing description`)
    assert.equal(typeof d.execute, 'function')
    assert.equal(typeof d.output.render, 'function')
  }
})

test('aflare_version executes the real binary', async (t) => {
  const bin = findAflareBin()
  if (!bin) return t.skip('aflare binary not found (build with `make build` or set AFLARE_BIN)')
  process.env.AFLARE_BIN = bin

  const tool = registerAll().find((d) => d.name === 'aflare_version')!
  // aflare_version declares a string output schema, so execute returns text.
  const value = (await tool.execute({}, fakeExec())) as string
  assert.match(value, /aflare version/)
  assert.ok(value.length > 0)

  const blocks = tool.output.render({}, value)
  assert.equal(blocks.length, 1)
  assert.match((blocks[0] as { text: string }).text, /aflare version/)
})

test('aflare_validate accepts a minimal workflow', async (t) => {
  const bin = findAflareBin()
  if (!bin) return t.skip('aflare binary not found (build with `make build` or set AFLARE_BIN)')
  process.env.AFLARE_BIN = bin

  const dir = mkdtempSync(join(tmpdir(), 'aflare-dsh-'))
  const file = join(dir, 'smoke.yaml')
  writeFileSync(
    file,
    'name: dsh-plugin-smoke\nsteps:\n  - node: combine\n    params:\n      format: text\n  - node: file_write\n    params:\n      path: out.txt\n',
  )
  try {
    const tool = registerAll().find((d) => d.name === 'aflare_validate')!
    const value = (await tool.execute({ file }, fakeExec())) as {
      exitCode: number
      stdout: string
      stderr: string
    }
    assert.equal(value.exitCode, 0, `stdout: ${value.stdout}\nstderr: ${value.stderr}`)
    assert.match(value.stdout, /✅/)
  } finally {
    rmSync(dir, { recursive: true, force: true })
  }
})
