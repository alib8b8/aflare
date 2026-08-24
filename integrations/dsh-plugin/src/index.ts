// aflare tools for DeepSeek Harness (DSH).
//
// Registers curated aflare CLI operations as native DSH tools. Every tool
// spawns the local `aflare` binary via execFile (no shell), so arguments are
// never interpreted by a shell and workflow data never leaves the host.
//
// Configure the binary path with AFLARE_BIN (default: "aflare") when the
// executable is not on PATH.

import type { Context } from '@deepseek-ai/cordis'
import { defineTool } from '@deepseek-ai/dsh-tools'
import { execFile } from 'node:child_process'
import { promisify } from 'node:util'

const execFileAsync = promisify(execFile)

/** Max stdout/stderr returned to the model; longer output is truncated. */
const MAX_OUTPUT = 256 * 1024
/** Default per-invocation timeout (ms); override with AFLARE_TIMEOUT_MS. */
const DEFAULT_TIMEOUT_MS = 5 * 60_000

export const name = 'aflare-tools'
export const inject = ['tools']

// Type alias (not interface) so it is assignable to JsonValue output schemas.
type RunResult = {
  command: string
  exitCode: number
  stdout: string
  stderr: string
}

function truncate(s: string): string {
  if (s.length <= MAX_OUTPUT) return s
  return s.slice(0, MAX_OUTPUT) + '\n...[truncated]'
}

async function runAflare(args: string[]): Promise<RunResult> {
  const bin = process.env.AFLARE_BIN || 'aflare'
  const timeout = Number(process.env.AFLARE_TIMEOUT_MS) || DEFAULT_TIMEOUT_MS
  const command = `${bin} ${args.join(' ')}`
  try {
    const { stdout, stderr } = await execFileAsync(bin, args, {
      timeout,
      maxBuffer: 10 * MAX_OUTPUT,
      windowsHide: true,
    })
    return { command, exitCode: 0, stdout: truncate(stdout), stderr: truncate(stderr) }
  } catch (err: any) {
    // Non-zero exit, timeout, or spawn failure: surface what we have.
    return {
      command,
      exitCode: err?.code ?? -1,
      stdout: truncate(err?.stdout ?? ''),
      stderr: truncate(err?.stderr ?? String(err)),
    }
  }
}

function renderRun(_args: unknown, value: RunResult) {
  const parts = [`$ ${value.command}  (exit ${value.exitCode})`]
  if (value.stdout) parts.push(value.stdout)
  if (value.stderr) parts.push(`[stderr] ${value.stderr}`)
  return [{ type: 'text' as const, text: parts.join('\n') }]
}

export function apply(ctx: Context) {
  ctx.tools.register(
    defineTool({
      name: 'aflare_version',
      description:
        'Show the installed aflare version. Use this first to confirm aflare is available.',
      parameters: {},
      output: { schema: { type: 'string' }, render: (_a, v: string) => [{ type: 'text', text: v }] },
      async execute() {
        const r = await runAflare(['version'])
        return `${r.command}\n${r.stdout || r.stderr}`
      },
    }),
  )

  ctx.tools.register(
    defineTool({
      name: 'aflare_generate',
      description:
        'Generate an aflare workflow YAML from a plain description (e.g. "fetch RSS headlines and save to file"). ' +
        'Set ai=true to use the configured LLM provider instead of keyword matching. Returns the created file path.',
      parameters: {
        description: { type: 'string', required: true, description: 'What the workflow should do' },
        ai: { type: 'boolean', description: 'Use LLM generation (default false)' },
      },
      output: { schema: { type: 'json' }, render: renderRun },
      async execute(args: { description: string; ai?: boolean }) {
        const cliArgs = ['create', args.description]
        if (args.ai) cliArgs.push('--ai')
        return runAflare(cliArgs)
      },
    }),
  )

  ctx.tools.register(
    defineTool({
      name: 'aflare_validate',
      description: 'Validate a workflow YAML file. Returns warnings for unknown nodes or invalid structure.',
      parameters: {
        file: { type: 'string', required: true, description: 'Path to the workflow YAML file' },
      },
      output: { schema: { type: 'json' }, render: renderRun },
      async execute(args: { file: string }) {
        return runAflare(['validate', args.file])
      },
    }),
  )

  ctx.tools.register(
    defineTool({
      name: 'aflare_run',
      description:
        'Execute an aflare workflow YAML file and return the final output. Long-running workflows are supported (default timeout 5 minutes).',
      parameters: {
        file: { type: 'string', required: true, description: 'Path to the workflow YAML file' },
      },
      output: { schema: { type: 'json' }, render: renderRun },
      async execute(args: { file: string }) {
        return runAflare(['run', args.file])
      },
    }),
  )
}
