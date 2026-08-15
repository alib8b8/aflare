# @alib8b8/dsh-plugin-aflare

[DeepSeek Harness](https://github.com/deepseek-ai/deepseek-harness) (DSH)
Cordis plugin that exposes [aflare](https://github.com/alib8b8/aflare)
workflow tools as native DSH tools.

Every tool spawns the local `aflare` binary with `execFile` (no shell), so
nothing leaves your machine — the same local-first guarantee aflare itself
makes.

## Tools

| Tool | What it does |
|---|---|
| `aflare_version` | Installed aflare version |
| `aflare_generate` | Create workflow YAML from a description (optional `ai: true` for LLM generation) |
| `aflare_validate` | Validate a workflow YAML |
| `aflare_run` | Execute a workflow YAML and return the output |
| `aflare_template_list` | List built-in templates |
| `aflare_template_run` | Run a template by ID |

## Install

### From source (works today)

```bash
git clone https://github.com/alib8b8/aflare
cd aflare/integrations/dsh-plugin
npm install
npm run build
```

Then register it with a `cordis.yml` patch overlay (absolute path required):

```yaml
- insert:
    - id: aflare-tools
      name: /absolute/path/to/aflare/integrations/dsh-plugin/dist/index.js
```

Start DSH with the overlay:

```bash
pnpm dsh web --patch ./aflare-patch.yml
```

### npm (once published)

Planned: `npm i @alib8b8/dsh-plugin-aflare`, then reference the package name
in `cordis.yml` instead of a local path.

## Configuration

| Env var | Default | Description |
|---|---|---|
| `AFLARE_BIN` | `aflare` | Path to the aflare executable |
| `AFLARE_TIMEOUT_MS` | `300000` | Per-invocation timeout (ms) |

## Security

- All invocations use `execFile` — arguments are passed as argv, never
  through a shell, so tool parameters cannot inject commands.
- Output is truncated at 256 KB to protect model context.
- aflare's own protections (path validation, safe mode, policy engine) apply
  to every run. Set `AFLARE_SAFE_MODE=1` in the DSH process environment for
  untrusted workflows.

## Development

```bash
npm install
npm run typecheck   # strict TS check
npm test            # compiles, then runs integration tests via node --test
```

The tests execute the real `aflare` binary, resolved from `AFLARE_BIN` or
the repo-root build output (`make build` in the repository root). Without a
binary, the registration checks still run and the binary tests skip.

## License

AGPL-3.0-or-later (same as the aflare project).
