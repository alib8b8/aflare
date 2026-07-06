---
name: llm-box-workflow
description: >
  Generate and execute terminal workflows using llm-box.
  Use when the user wants to automate multi-step terminal tasks, chain commands,
  fetch URLs, process data, create reusable pipelines, or build CI/CD-like
  automation locally. Trigger on keywords: workflow, pipeline, automate,
  batch process, fetch and save, schedule task.
---

# llm-box Workflow Skill

## When to Use

Use this skill when the user wants to:

- **Automate multi-step terminal tasks** — fetching data, processing it, saving results
- **Create reusable pipelines** — workflows that can be run repeatedly
- **Chain commands** — where output of one step feeds into the next
- **Batch process data** — transform, filter, combine multiple data sources
- **Integrate LLMs into automation** — use Ollama, DeepSeek, or OpenAI-compatible models
- **Replace fragile bash scripts** — with structured, auditable YAML workflows

## How llm-box Works

```
Plain English description → YAML workflow → Execute with TUI progress
```

llm-box generates a YAML workflow file from a natural language description.
The workflow is deterministic and reproducible — same workflow always produces
the same result. Users can edit the YAML by hand if they want to tweak things.

## Quick Reference

### CLI Commands

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

### Available Nodes

**Utility Nodes:**
| Node | Description |
|------|-------------|
| `fetch_url` | Fetch content from a URL (with SSRF protection) |
| `http_request` | Full HTTP client — any method, headers, body |
| `file_read` | Read file contents |
| `file_write` | Write content to a file |
| `execute` | Run shell commands (configurable allowlist) |
| `json_parse` | Extract fields from JSON using dot notation |
| `template_render` | Render Go templates with variables |
| `transform` | Transform text (uppercase, lowercase, trim, replace, regex) |
| `combine` | Merge multiple inputs into one |
| `notify` | Print or send notifications |

**LLM Nodes (15+ providers):**
| Node | Provider |
|------|----------|
| `ollama` | Local models via Ollama |
| `deepseek` | DeepSeek API |
| `openai` | OpenAI-compatible (200+ models via OpenRouter, SiliconFlow, etc.) |
| `qwen` | Alibaba Qwen/通义千问 |
| `glm` | Zhipu GLM/智谱 |
| `kimi` | Moonshot Kimi |
| `minimax` | MiniMax |
| `mistral` | Mistral AI |
| `yi` | 01.AI Yi/零一万物 |
| `baichuan` | Baichuan/百川 |
| `internlm` | InternLM/书生 |
| `xverse` | XVerse/元象 |
| `mimo` | Xiaomi MiMo/小米 |
| `ima` | Tencent IMA/腾讯 |
| `fastgpt` | FastGPT |
| `coze` | WIP — ByteDance Coze (not functional yet) |

**Control Nodes:**
| Node | Description |
|------|-------------|
| `condition` | Conditional execution based on expression |
| `call` | Call another workflow file (nested workflows) |

### YAML Workflow Structure

```yaml
name: my-workflow
description: What this workflow does
vars:
  api_key: "your-api-key"
steps:
  - node: fetch_url
    params:
      url: "https://api.example.com/data"
  - node: json_parse
    params:
      path: "result.items.[0].name"
  - node: file_write
    params:
      path: "output.txt"
  # Conditional step
  - node: notify
    condition: "{{.output}} != ''"
    params:
      message: "Done! Result saved."
```

### Step Features

- **`condition`**: Go template expression, step runs only if true
- **`retry`**: Number of retries on failure (max 10)
- **`delay`**: Delay between retries (e.g., "2s", "1m")
- **`parallel`**: Run multiple steps concurrently
- **`_timeout`**: Per-step timeout (e.g., "30s")

### Parallel Steps Example

```yaml
steps:
  - parallel:
      - node: fetch_url
        params:
          url: "https://api1.example.com"
      - node: fetch_url
        params:
          url: "https://api2.example.com"
    # Combine results
  - node: combine
    params:
      separator: "\n---\n"
```

## Workflow Generation Guidelines

When generating a workflow for the user:

1. **Identify the steps** — break the task into discrete operations
2. **Choose the right nodes** — prefer specific nodes over `execute` when possible
3. **Chain with variables** — use `{{.steps[N].output}}` to reference previous outputs
4. **Add error handling** — use `condition` and `retry` where appropriate
5. **Keep it readable** — add a `description` field, use meaningful step names

### Common Patterns

**Fetch and Save:**
```yaml
steps:
  - node: fetch_url
    params:
      url: "https://example.com/data"
  - node: file_write
    params:
      path: "data.txt"
```

**Fetch, Parse, and Summarize with LLM:**
```yaml
steps:
  - node: fetch_url
    params:
      url: "https://example.com/article"
  - node: ollama
    params:
      model: "llama3"
      prompt: "Summarize: {{.steps[0].output}}"
  - node: file_write
    params:
      path: "summary.md"
```

**Multi-source Aggregation:**
```yaml
steps:
  - parallel:
      - node: fetch_url
        params:
          url: "https://api1.example.com"
      - node: fetch_url
        params:
          url: "https://api2.example.com"
  - node: combine
    params:
      separator: "\n"
  - node: ollama
    params:
      model: "llama3"
      prompt: "Analyze: {{.steps[1].output}}"
```

## Security Notes

- llm-box has built-in SSRF protection (URL validation, DNS rebinding checks)
- Path traversal protection (sandboxed paths, symlink resolution)
- Command injection prevention (shell metachar filtering, optional allowlist)
- Resource limits (file size, response body, step count, timeouts)
- Safe mode disables the `execute` node entirely

## Installation

```bash
# Linux/macOS - download and run install script
curl -sL https://raw.githubusercontent.com/alib8b8/llm-box/main/install.sh -o install.sh
bash install.sh

# Or via Go
go install github.com/alib8b8/llm-box/cmd/llm-box@latest

# Verify installation
llm-box list
```

## Examples

The repo includes 10 ready-to-use workflow examples:
- Daily GitHub summary
- Research assistant (fetch + summarize)
- Documentation generator
- Log monitoring & alerting
- Release notes generator
- Data collector (multi-API aggregation)
- File organizer
- Content workflow
- DevOps automation
- Team weekly report

See: https://github.com/alib8b8/llm-box/tree/main/examples
