---
name: setup
description: Install aflare (script, release binary, or source build), verify the install with doctor, initialize the ~/.aflare workspace, and run a first workflow end to end
invocation: user
allowed-tools: Read, Edit, Write, Bash
version: 2.0.0
author: alib8b8
license: AGPL-3.0
compatibility: aflare >= 0.12.0
tags: [setup, installation, configuration, aflare]
---

# aflare Setup

Get aflare from zero to a passing first run. Four stages: install → verify →
initialize → first workflow.

## Stage 1 — Install the binary

Pick exactly one path:

| Method | Command / link | Best for |
|--------|----------------|----------|
| Install script (Linux/macOS) | `curl -sL https://raw.githubusercontent.com/alib8b8/aflare/main/install.sh -o install.sh && bash install.sh` | Fastest default |
| Install script (Windows) | `irm https://raw.githubusercontent.com/alib8b8/aflare/main/install.ps1 \| iex` | Fastest default on Windows |
| Release asset | [GitHub Releases](https://github.com/alib8b8/aflare/releases) — pick the archive for your OS/arch | Pinned, checksum-verified installs |
| Build from source | `git clone https://github.com/alib8b8/aflare.git && cd aflare && go install ./cmd/aflare` | Needs Go 1.26+ |

For mainland-China networks, prefix the GitHub URLs with `https://ghproxy.com/`.

## Stage 2 — Verify the install

Two checks, both must pass:

```bash
aflare --version   # prints the installed version
aflare doctor      # environment health check (add --offline to skip network probes)
```

If `aflare` is not found, the install directory is not on `PATH` — check with
`which aflare` (Linux/macOS) or `where aflare` (Windows) and fix the shell
profile.

## Stage 3 — Initialize the workspace

`aflare init` bootstraps the local layout (run without flags for the
interactive flow; `--mcp` / `--agent` take shortcuts for MCP host and agent
config):

```bash
aflare init
```

Resulting layout under `~/.aflare/`:

- `workflows/` — where your workflow YAMLs live
- `config.yaml` — engine configuration (default model, safe mode, API keys)

Secrets belong in the OS keyring, never in plaintext YAML. Store the ones your
workflows reference as `{{secret.*}}`:

```bash
aflare secrets init
aflare secrets set --group api --key service <password>
```

(On headless systems set `AFLARE_SECRETS_PASSWORD` instead of the keyring.)

## Stage 4 — First workflow

Generate a workflow from a plain-language description, validate it, run it:

```bash
aflare create "fetch example.com and save it to example.html"
aflare validate example-workflow.yaml
aflare run example-workflow.yaml
```

Expected outcome: the TUI shows each node executing, ending with a completion
line, and `example.html` exists in the working directory. `aflare list` prints
the full node catalog if you want to hand-edit the YAML next.

## Done means

- [ ] `aflare --version` and `aflare doctor` both succeed
- [ ] `~/.aflare/workflows/` exists
- [ ] One workflow ran to completion with visible output

## Where to go next

- Node catalog: `aflare list`, or [docs/nodes-reference.md](../../docs/nodes-reference.md)
- Custom nodes: [docs/custom-nodes.md](../../docs/custom-nodes.md)
- Ready-made workflows: [examples/](../../examples/)
