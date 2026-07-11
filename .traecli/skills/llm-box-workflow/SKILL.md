---
name: llm-box-workflow
description: >
  Generate and execute deterministic terminal workflows using llm-box.
  Use when the user wants to automate multi-step terminal tasks, chain commands,
  fetch URLs and save results, process data through pipelines, integrate LLMs
  (Ollama/DeepSeek/Qwen/GLM etc.) into automation, build CI/CD-like automation
  locally, or replace fragile bash scripts with structured auditable YAML.
  Trigger keywords: workflow, pipeline, automate, batch process, fetch and save,
  schedule task, chain commands, summarize with LLM, data aggregation.
---

# llm-box Workflow Skill

## When to Use

Use this skill when the user wants to:

- **Automate multi-step terminal tasks** — fetch data, process it, save results
- **Create reusable pipelines** — workflows that can be run repeatedly
- **Chain commands** — where output of one step feeds into the next
- **Batch process data** — transform, filter, combine multiple data sources
- **Integrate LLMs into automation** — use Ollama, DeepSeek, or OpenAI-compatible models
- **Replace fragile bash scripts** — with structured, auditable YAML workflows
- **Build CI/CD-like automation** — locally, without external services

## How llm-box Works

```
Plain English description → YAML workflow → Execute with TUI progress
```

llm-box generates a YAML workflow file from a natural language description.
The workflow is **deterministic and reproducible** — same workflow always produces
the same result. Users can edit the YAML by hand to tweak things.

## Prerequisites

llm-box must be installed. If not installed, suggest one of:

```bash
# Linux/macOS - install script
curl -sL https://raw.githubusercontent.com/alib8b8/llm-box/main/install.sh -o install.sh
bash install.sh

# Or via Go
go install github.com/alib8b8/llm-box/cmd/llm-box@latest

# Verify
llm-box list
```

## Quick Reference

### Core CLI Commands

```bash
# Generate a workflow from plain English
llm-box create "<description>"

# Run a workflow file
llm-box run <workflow.yaml>

# List all available nodes
llm-box list

# Validate a workflow file without running
llm-box validate <workflow.yaml>

# Run in safe mode (disables execute node)
llm-box --safe-mode run <workflow.yaml>

# Dry run (show steps without executing)
llm-box --dry-run run <workflow.yaml>
```

### Minimal Workflow Example

```yaml
name: fetch-and-save
description: Fetch a URL and save to file
steps:
  - node: fetch_url
    params:
      url: "https://example.com/data"
  - node: file_write
    params:
      path: "output.txt"
```

## Workflow Generation Guidelines

When generating a workflow for the user:

1. **Identify the steps** — break the task into discrete operations
2. **Choose the right nodes** — prefer specific nodes over `execute` when possible
3. **Chain with variables** — use `{{.steps[N].output}}` to reference previous outputs
4. **Add error handling** — use `condition` and `retry` where appropriate
5. **Keep it readable** — add a `description` field, use meaningful step names
6. **Prefer safe mode** — suggest `--safe-mode` when `execute` node is not essential

## Step Features

- **`condition`**: Go template expression, step runs only if true
- **`retry`**: Number of retries on failure (max 10)
- **`delay`**: Delay between retries (e.g., "2s", "1m")
- **`parallel`**: Run multiple steps concurrently
- **`_timeout`**: Per-step timeout (e.g., "30s")

## Security Notes

- Built-in SSRF protection (URL validation, DNS rebinding checks)
- Path traversal protection (sandboxed paths, symlink resolution)
- Command injection prevention (shell metachar filtering, optional allowlist)
- Resource limits (file size, response body, step count, timeouts)
- Safe mode disables the `execute` node entirely
- Sensitive data (tokens, API keys) redacted in logs

## For More Details

This skill uses progressive disclosure. For complete reference:

- **Full node catalog & parameters**: see [nodes-reference.md](nodes-reference.md)
- **Ready-to-use workflow examples**: see [examples.md](examples.md)

## Version

- v0.4.0 — initial TRAE skill release
