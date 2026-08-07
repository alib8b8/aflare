---
name: aflare
description: Generate and execute terminal workflows using aflare. Use when the user wants to automate multi-step terminal tasks, chain commands, fetch URLs, process data, create reusable pipelines, or build CI/CD-like automation locally.
invocation: both
allowed-tools: Read, Edit, Write, Bash, Glob, Grep, WebFetch
version: 0.6.0
author: aflare
license: AGPL-3.0
compatibility: claude-code >= 0.7.0
tags: [workflow, automation, cli, pipeline, terminal]
---

## Overview

aflare is an AI-powered terminal workflow engine that generates and executes multi-step automation pipelines from natural language descriptions. It supports 30+ agent nodes including LLM providers, code analysis, file operations, web fetching, and system integrations. Workflows are defined as deterministic YAML files that can be version-controlled and reused.

Key features:
- Natural language to YAML workflow generation
- 30+ built-in nodes (LLM, fetch, execute, transform, etc.)
- MCP protocol support for external tool integration
- Security-hardened with SSRF protection, path traversal prevention, and resource limits
- On-device LLM inference support (1B-8B models)

## Prerequisites

- aflare CLI installed (`go install github.com/alib8b8/aflare/cmd/aflare@latest`)
- Go 1.21+ or a pre-built binary
- Optional: Ollama for local LLM inference

## Instructions

1. **Generate a workflow**: Describe the task in natural language
2. **Review the YAML**: The generated workflow is deterministic and editable
3. **Execute**: Run the workflow with progress tracking
4. **Chain outputs**: Use `{{.steps[N].output}}` to pass data between steps

### CLI Commands

```bash
# Generate a workflow from plain English
aflare create "fetch weather data and save to file"

# Run a workflow file
aflare run workflow.yaml

# List all available nodes
aflare list

# Validate a workflow without executing
aflare validate workflow.yaml

# Dry run (show steps without executing)
aflare --dry-run run workflow.yaml

# Safe mode (disables execute node)
aflare --safe-mode run workflow.yaml
```

### Available Nodes

**Utility Nodes:**
| Node | Description |
|------|-------------|
| `fetch_url` | Fetch content from a URL (SSRF protected) |
| `http_request` | Full HTTP client — any method, headers, body |
| `file_read` | Read file contents |
| `file_write` | Write content to a file |
| `execute` | Run shell commands (configurable allowlist) |
| `json_parse` | Extract fields from JSON using dot notation |
| `template_render` | Render Go templates with variables |
| `transform` | Transform text (uppercase, lowercase, trim, replace, regex) |
| `combine` | Merge multiple inputs into one |
| `notify` | Print or send notifications |

**LLM Nodes:**
| Node | Provider |
|------|----------|
| `ollama` | Local models via Ollama |
| `deepseek` | DeepSeek API |
| `openai` | OpenAI-compatible |
| `qwen` | Alibaba Qwen |
| `glm` | Zhipu GLM |
| `kimi` | Moonshot Kimi |
| `mistral` | Mistral AI |
| `yi` | 01.AI Yi |

**Control Nodes:**
| Node | Description |
|------|-------------|
| `condition` | Conditional execution based on expression |
| `call` | Call another workflow file (nested) |

## Output

The skill generates a YAML workflow file and optionally executes it. Output includes:
- Generated workflow YAML
- Execution progress and logs
- Final output from the last step
- Error details if any step fails

## Examples

**Example 1: Fetch and Save**
```yaml
name: fetch-and-save
description: Fetch data from API and save to file
steps:
  - node: fetch_url
    params:
      url: "https://api.example.com/data"
  - node: file_write
    params:
      path: "data.txt"
      content: "{{.steps[0].output}}"
```

**Example 2: Fetch, Parse, and Summarize**
```yaml
name: summarize-article
description: Fetch article and summarize with LLM
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
      content: "{{.steps[1].output}}"
```

**Example 3: Multi-source Aggregation**
```yaml
name: aggregate-sources
description: Fetch from multiple APIs and analyze
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

## Resources

- **GitHub**: https://github.com/alib8b8/aflare
- **GitCode**: https://gitcode.com/aflare/aflare
- **Documentation**: https://gitcode.com/aflare/aflare/blob/main/README.md
- **Issues**: https://github.com/alib8b8/aflare/issues
- **License**: AGPL-3.0