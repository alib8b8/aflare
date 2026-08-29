# aflare-action

Run [aflare](https://github.com/alib8b8/aflare) AI workflows in GitHub Actions — deterministic YAML pipelines with dozens of built-in nodes, LLM steps, SSRF protection, and tamper-evident audit trails. Installs the prebuilt release binary (checksum-verified) in seconds; no Docker build, no compilation.

## Usage

```yaml
- uses: alib8b8/aflare/action@main
  with:
    workflow: .aflare/pr-review.yaml
    set: |
      pr_number=${{ github.event.pull_request.number }}
```

> **Why `@main`?** The `action/` directory landed after the `v0.11.0` tag, so `@v0.11.0` fails with "Can't find action.yml". Pin to the first release tag that ships the action (v0.12.0+) once available — pinning the `version` input (below) keeps the installed CLI reproducible in the meantime.

### Inputs

| Input | Default | Description |
|-------|---------|-------------|
| `version` | `latest` | aflare release to install (e.g. `v0.11.0`). Pin for reproducible CI. |
| `workflow` | — | Path to the workflow YAML to run (relative to `working-directory`). Omit to only install the CLI for later steps. |
| `set` | — | Workflow parameters, one `KEY=VALUE` per line (forwarded as `--set`; referenced in templates as `{{var.KEY}}`). |
| `safe-mode` | `false` | Run with `execute` and external nodes disabled. |
| `validate-only` | `false` | Validate the workflow without executing it. |
| `working-directory` | `.` | Working directory for the run. |
| `token` | `github.token` | Token used to resolve the latest release (avoids API rate limits). |

### Outputs

| Output | Description |
|--------|-------------|
| `version` | Installed aflare version. |
| `output` | Final output block of the workflow run (empty for validate-only / install-only). |

The step also appends a run summary (command, exit code, final output) to `$GITHUB_STEP_SUMMARY`.

## Examples

**Gate a PR on a review workflow** (fails the job when the workflow fails):

```yaml
on: pull_request
jobs:
  review:
    runs-on: ubuntu-latest
    permissions:
      pull-requests: write
    steps:
      - uses: actions/checkout@v4
      - uses: alib8b8/aflare/action@main
        env:
          LLM_API_KEY: ${{ secrets.LLM_API_KEY }}
        with:
          workflow: .aflare/pr-review.yaml
          set: |
            pr_number=${{ github.event.pull_request.number }}
            repo=${{ github.repository }}
```

**Install the CLI only**, then run several workflows in later steps:

```yaml
- uses: alib8b8/aflare/action@main
- run: aflare validate .aflare/deploy.yaml
- run: aflare --safe-mode run .aflare/audit.yaml
```

**Scheduled content pipeline** with a pinned version:

```yaml
on:
  schedule:
    - cron: '0 6 * * *'
jobs:
  digest:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: alib8b8/aflare/action@main
        with:
          version: v0.11.0
          workflow: .aflare/daily-digest.yaml
          set: |
            date=${{ github.run_id }}
      - uses: actions/upload-artifact@v4
        with:
          name: digest
          path: out/digest.md
```

## Notes

- **Secrets**: pass LLM keys / webhook URLs via `env:` or `secrets:` — aflare redacts 10+ secret patterns from logs and stores encrypted secrets with AES-GCM. Never put them in `set:`.
- **Failure gating**: the action propagates aflare's exit code, so a failed workflow step fails the job. Use `continue-on-error: true` on the step for report-only mode.
- **Runner support**: Linux and macOS runners on amd64/arm64. The CLI talks to public APIs by default; for intranet-only endpoints configure the connector policy in your workflow.
- **Data stays on the runner**: aflare is local-first with no telemetry of your workflow payloads.

## Development

This action lives in the aflare repo (`action/`). The E2E workflow in
`.github/workflows/action-test.yml` dogfoods it on every change. Scripts are
POSIX bash; `install.sh` and `run.sh` can be tested locally by faking the
runner environment (`GITHUB_PATH`, `GITHUB_OUTPUT`, `GITHUB_STEP_SUMMARY`,
`RUNNER_TEMP`).
