# llm-box Extension

llm-box is a terminal-first workflow automation engine. It generates executable YAML workflows from plain English descriptions.

## When to Use

Use llm-box when the user wants to:

- **Automate multi-step terminal tasks** — fetching data, processing it, saving results
- **Create reusable pipelines** — workflows that can be run repeatedly
- **Chain commands** — where output of one step feeds into the next
- **Batch process data** — transform, filter, combine multiple data sources
- **Integrate LLMs into automation** — use Ollama, DeepSeek, or OpenAI-compatible models
- **Replace fragile bash scripts** — with structured, auditable YAML workflows

## How It Works

```
Plain English description → YAML workflow → Execute with progress
```

llm-box generates a YAML workflow file from a natural language description.
The workflow is deterministic and reproducible — same workflow always produces
the same result. Users can edit the YAML by hand if they want to tweak things.

## MCP Tools

This extension provides the following MCP tools:

- **create_workflow** — Generate a YAML workflow from a natural language description
- **run_workflow** — Run a workflow by name or path
- **run_workflow_yaml** — Run a workflow from inline YAML content
- **list_nodes** — List all available workflow nodes
- **validate_workflow** — Validate a workflow YAML file without executing

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
| `condition` | Branch based on conditions |
| `call` | Call another workflow as a subroutine |

**LLM Nodes:**
| Node | Provider |
|------|----------|
| `ollama` | Local models via Ollama |
| `deepseek` | DeepSeek API |
| `openai` | OpenAI-compatible APIs |
| `qwen` | Alibaba Qwen |
| `glm` | Zhipu GLM |
| `kimi` | Moonshot Kimi |
| `mistral` | Mistral AI |
| `yi` | 01.AI Yi |
| `coze` | ByteDance Coze |
| `baichuan` | Baichuan AI |
| `minimax` | MiniMax |
| `fastgpt` | FastGPT |
| `internlm` | InternLM |
| `xverse` | XVERSE |
| `ima` | IMA |

## Installation

If llm-box is not installed, install it first:

```bash
curl -sL https://raw.githubusercontent.com/alib8b8/llm-box/main/install.sh -o install.sh && bash install.sh
```

Or build from source:

```bash
go install github.com/alib8b8/llm-box/cmd/llm-box@latest
```

## Best Practices

1. **Start with simple workflows** — build complexity incrementally
2. **Use `--dry-run` first** — preview steps before executing
3. **Use `--safe-mode`** — when running untrusted workflows
4. **Save workflows to version control** — they're just YAML files
5. **Use `call` node for reusable parts** — break large workflows into smaller ones
